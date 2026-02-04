package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// Commander orchestrates the swarm of agents.
type Commander struct {
	mu sync.RWMutex

	agents    map[string]*Agent
	store     *store.Store
	registry  *provider.Registry
	bus       *EventBus
	config    *config.Config
	mcp       *mcp.Proxy
	maxAgents int
}

// NewCommander creates a new commander.
func NewCommander(s *store.Store, reg *provider.Registry, bus *EventBus, cfg *config.Config) *Commander {
	return &Commander{
		agents:    make(map[string]*Agent),
		store:     s,
		registry:  reg,
		bus:       bus,
		config:    cfg,
		maxAgents: cfg.Swarm.MaxAgents,
	}
}

// SetMCP sets the MCP proxy for all agents.
func (c *Commander) SetMCP(proxy *mcp.Proxy) {
	c.mu.Lock()
	c.mcp = proxy
	// Set MCP on all existing agents
	for _, agent := range c.agents {
		agent.SetMCP(proxy)
	}
	c.mu.Unlock()
}

// LoadAgents loads existing agents from the store.
// Agents are loaded in stopped state; they must be explicitly started.
func (c *Commander) LoadAgents() error {
	stored, err := c.store.ListAgents()
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sa := range stored {
		p, err := c.registry.Get(sa.Provider)
		if err != nil {
			// Provider not available, skip agent
			continue
		}

		agent := NewAgent(sa.ID, sa.Name, sa.CreatedAt, p, c.store, c.bus)
		if c.mcp != nil {
			agent.SetMCP(c.mcp)
		}
		c.agents[sa.ID] = agent
	}

	return nil
}

// CreateAgent creates a new agent with the given name and provider.
func (c *Commander) CreateAgent(name, providerName string) (*Agent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.agents) >= c.maxAgents {
		return nil, fmt.Errorf("max agents (%d) reached", c.maxAgents)
	}

	// Get provider
	p, err := c.registry.Get(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}

	// Get provider config for model
	provCfg, ok := c.config.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider config not found: %s", providerName)
	}

	// Create in store
	stored, err := c.store.CreateAgent(name, providerName, provCfg.Model)
	if err != nil {
		return nil, fmt.Errorf("create agent in store: %w", err)
	}

	// Create runtime agent
	agent := NewAgent(stored.ID, stored.Name, stored.CreatedAt, p, c.store, c.bus)
	if c.mcp != nil {
		agent.SetMCP(c.mcp)
	}
	c.agents[stored.ID] = agent

	// Emit event
	c.bus.Publish(Event{
		Type:      EventAgentCreated,
		AgentID:   stored.ID,
		AgentName: stored.Name,
		Timestamp: time.Now(),
	})

	return agent, nil
}

// DeleteAgent removes an agent from the swarm.
func (c *Commander) DeleteAgent(id string, purgeMemories bool) error {
	c.mu.Lock()
	agent, ok := c.agents[id]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("agent not found: %s", id)
	}

	delete(c.agents, id)
	c.mu.Unlock()

	// Stop if running (outside of commander lock to avoid deadlock)
	if agent.State() == AgentStateRunning {
		agent.Stop()
	}

	// Delete from store (memories cascade)
	if err := c.store.DeleteAgent(id); err != nil {
		return fmt.Errorf("delete agent from store: %w", err)
	}

	// Emit event
	c.bus.Publish(Event{
		Type:      EventAgentDeleted,
		AgentID:   id,
		AgentName: agent.Name(),
		Timestamp: time.Now(),
	})

	return nil
}

// GetAgent returns an agent by ID.
func (c *Commander) GetAgent(id string) (*Agent, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agent, ok := c.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", id)
	}
	return agent, nil
}

// ListAgents returns all agents.
func (c *Commander) ListAgents() []*Agent {
	c.mu.RLock()
	defer c.mu.RUnlock()

	agents := make([]*Agent, 0, len(c.agents))
	for _, a := range c.agents {
		agents = append(agents, a)
	}
	return agents
}

// StartAgent starts an agent by ID.
func (c *Commander) StartAgent(id string) error {
	agent, err := c.GetAgent(id)
	if err != nil {
		return err
	}
	return agent.Start()
}

// StopAgent stops an agent by ID.
func (c *Commander) StopAgent(id string) error {
	agent, err := c.GetAgent(id)
	if err != nil {
		return err
	}
	return agent.Stop()
}

// ConfigureAgent updates an agent's provider and model.
func (c *Commander) ConfigureAgent(id, providerName string) error {
	c.mu.Lock()
	agent, ok := c.agents[id]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("agent not found: %s", id)
	}
	c.mu.Unlock()

	// Get new provider
	p, err := c.registry.Get(providerName)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	// Get provider config
	provCfg, ok := c.config.Providers[providerName]
	if !ok {
		return fmt.Errorf("provider config not found: %s", providerName)
	}

	// Update store
	if err := c.store.UpdateAgentConfig(id, providerName, provCfg.Model); err != nil {
		return fmt.Errorf("update store: %w", err)
	}

	// Update runtime
	agent.SetProvider(p)

	// Emit event
	c.bus.Publish(Event{
		Type:      EventAgentConfigChanged,
		AgentID:   id,
		AgentName: agent.Name(),
		Data: ConfigChangeData{
			Provider: providerName,
			Model:    provCfg.Model,
		},
		Timestamp: time.Now(),
	})

	return nil
}

// SendMessage sends a message to a specific agent (synchronous).
func (c *Commander) SendMessage(id, content string) error {
	agent, err := c.GetAgent(id)
	if err != nil {
		return err
	}
	return agent.SendMessage(content, store.MemorySourceDirect)
}

// SendMessageAsync sends a message to a specific agent without waiting for processing.
// Returns immediately after validating the agent exists and is running.
func (c *Commander) SendMessageAsync(id, content string) error {
	agent, err := c.GetAgent(id)
	if err != nil {
		return err
	}
	if agent.State() != AgentStateRunning {
		return fmt.Errorf("agent not running")
	}
	go func() {
		if err := agent.SendMessage(content, store.MemorySourceDirect); err != nil {
			// Error is published to bus by agent.SendMessage
		}
	}()
	return nil
}

// Broadcast sends a message to all running agents (synchronous).
func (c *Commander) Broadcast(content string) error {
	c.mu.RLock()
	agents := make([]*Agent, 0)
	for _, a := range c.agents {
		if a.State() == AgentStateRunning {
			agents = append(agents, a)
		}
	}
	c.mu.RUnlock()

	// Check if any agents are running
	if len(agents) == 0 {
		return fmt.Errorf("no running agents to receive broadcast")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Data:      MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	var errs []error
	for _, a := range agents {
		if err := a.SendMessage(content, store.MemorySourceBroadcast); err != nil {
			errs = append(errs, fmt.Errorf("agent %s: %w", a.ID(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("broadcast failed for %d agent(s): %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// BroadcastAsync sends a message to all running agents without waiting for processing.
// Returns immediately after validating at least one agent is running.
func (c *Commander) BroadcastAsync(content string) error {
	c.mu.RLock()
	agents := make([]*Agent, 0)
	for _, a := range c.agents {
		if a.State() == AgentStateRunning {
			agents = append(agents, a)
		}
	}
	c.mu.RUnlock()

	// Check if any agents are running
	if len(agents) == 0 {
		return fmt.Errorf("no running agents to receive broadcast")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Data:      MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	// Send to each agent asynchronously
	for _, a := range agents {
		agent := a // capture for goroutine
		go func() {
			if err := agent.SendMessage(content, store.MemorySourceBroadcast); err != nil {
				// Error is published to bus by agent.SendMessage
			}
		}()
	}

	return nil
}

// StopAll stops all running agents.
func (c *Commander) StopAll() {
	c.mu.RLock()
	agents := make([]*Agent, 0)
	for _, a := range c.agents {
		agents = append(agents, a)
	}
	c.mu.RUnlock()

	for _, a := range agents {
		if a.State() == AgentStateRunning {
			a.Stop()
		}
	}
}

// AgentCount returns the current number of agents.
func (c *Commander) AgentCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.agents)
}

// MaxAgents returns the maximum allowed agents.
func (c *Commander) MaxAgents() int {
	return c.maxAgents
}

// Store returns the store for direct access (e.g., for testing).
func (c *Commander) Store() *store.Store {
	return c.store
}
