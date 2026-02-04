package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// Commander orchestrates the swarm of myses.
type Commander struct {
	mu sync.RWMutex

	myses    map[string]*Mysis
	store    *store.Store
	registry *provider.Registry
	bus      *EventBus
	config   *config.Config
	mcp      *mcp.Proxy
	maxMyses int
}

// NewCommander creates a new commander.
func NewCommander(s *store.Store, reg *provider.Registry, bus *EventBus, cfg *config.Config) *Commander {
	return &Commander{
		myses:    make(map[string]*Mysis),
		store:    s,
		registry: reg,
		bus:      bus,
		config:   cfg,
		maxMyses: cfg.Swarm.MaxMyses,
	}
}

// SetMCP sets the MCP proxy for all myses.
func (c *Commander) SetMCP(proxy *mcp.Proxy) {
	c.mu.Lock()
	c.mcp = proxy
	// Set MCP on all existing myses
	for _, mysis := range c.myses {
		mysis.SetMCP(proxy)
	}
	c.mu.Unlock()
}

// LoadMyses loads existing myses from the store.
// Myses are loaded in stopped state; they must be explicitly started.
func (c *Commander) LoadMyses() error {
	stored, err := c.store.ListMyses()
	if err != nil {
		return fmt.Errorf("list myses: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, sm := range stored {
		p, err := c.registry.Create(sm.Provider, sm.Model, sm.Temperature)
		if err != nil {
			// Provider not available, skip mysis
			continue
		}

		mysis := NewMysis(sm.ID, sm.Name, sm.CreatedAt, p, c.store, c.bus)
		if c.mcp != nil {
			mysis.SetMCP(c.mcp)
		}
		c.myses[sm.ID] = mysis
	}

	return nil
}

// CreateMysis creates a new mysis with the given name and provider.
func (c *Commander) CreateMysis(name, providerName string) (*Mysis, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.myses) >= c.maxMyses {
		return nil, fmt.Errorf("max myses (%d) reached", c.maxMyses)
	}

	// Get provider config for model
	provCfg, ok := c.config.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("provider config not found: %s", providerName)
	}

	// Create provider instance for this mysis
	p, err := c.registry.Create(providerName, provCfg.Model, provCfg.Temperature)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	// Create in store
	stored, err := c.store.CreateMysis(name, providerName, provCfg.Model, provCfg.Temperature)
	if err != nil {
		return nil, fmt.Errorf("create mysis in store: %w", err)
	}

	// Create runtime mysis
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, p, c.store, c.bus)
	if c.mcp != nil {
		mysis.SetMCP(c.mcp)
	}
	c.myses[stored.ID] = mysis

	// Emit event
	c.bus.Publish(Event{
		Type:      EventMysisCreated,
		MysisID:   stored.ID,
		MysisName: stored.Name,
		Timestamp: time.Now(),
	})

	return mysis, nil
}

// DeleteMysis removes a mysis from the swarm.
func (c *Commander) DeleteMysis(id string, purgeMemories bool) error {
	c.mu.Lock()
	mysis, ok := c.myses[id]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("mysis not found: %s", id)
	}

	delete(c.myses, id)
	c.mu.Unlock()

	// Stop if running (outside of commander lock to avoid deadlock)
	if mysis.State() == MysisStateRunning {
		mysis.Stop()
	}

	// Release any claimed accounts before deleting
	if err := c.store.ReleaseAccountsByMysis(id); err != nil {
		// Log but don't fail the delete
		log.Warn().Err(err).Str("mysis_id", id).Msg("failed to release accounts on delete")
	}

	// Delete from store (memories cascade)
	if err := c.store.DeleteMysis(id); err != nil {
		return fmt.Errorf("delete mysis from store: %w", err)
	}

	// Emit event
	c.bus.Publish(Event{
		Type:      EventMysisDeleted,
		MysisID:   id,
		MysisName: mysis.Name(),
		Timestamp: time.Now(),
	})

	return nil
}

// GetMysis returns a mysis by ID.
func (c *Commander) GetMysis(id string) (*Mysis, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	mysis, ok := c.myses[id]
	if !ok {
		return nil, fmt.Errorf("mysis not found: %s", id)
	}
	return mysis, nil
}

// ListMyses returns all myses.
func (c *Commander) ListMyses() []*Mysis {
	c.mu.RLock()
	defer c.mu.RUnlock()

	myses := make([]*Mysis, 0, len(c.myses))
	for _, m := range c.myses {
		myses = append(myses, m)
	}
	return myses
}

// StartMysis starts a mysis by ID.
func (c *Commander) StartMysis(id string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}
	return mysis.Start()
}

// StopMysis stops a mysis by ID.
func (c *Commander) StopMysis(id string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}

	// Release any claimed accounts
	if err := c.store.ReleaseAccountsByMysis(id); err != nil {
		// Log but don't fail the stop
		log.Warn().Err(err).Str("mysis_id", id).Msg("failed to release accounts on stop")
	}

	return mysis.Stop()
}

// ConfigureMysis updates a mysis provider and model.
func (c *Commander) ConfigureMysis(id, providerName, model string) error {
	c.mu.Lock()
	mysis, ok := c.myses[id]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("mysis not found: %s", id)
	}
	c.mu.Unlock()

	// Get provider config
	provCfg, ok := c.config.Providers[providerName]
	if !ok {
		return fmt.Errorf("provider config not found: %s", providerName)
	}
	temperature := provCfg.Temperature

	// Get new provider
	p, err := c.registry.Create(providerName, model, temperature)
	if err != nil {
		return fmt.Errorf("create provider: %w", err)
	}

	// Update store
	if err := c.store.UpdateMysisConfig(id, providerName, model, temperature); err != nil {
		return fmt.Errorf("update store: %w", err)
	}

	// Update runtime
	mysis.SetProvider(p)

	// Emit event
	c.bus.Publish(Event{
		Type:      EventMysisConfigChanged,
		MysisID:   id,
		MysisName: mysis.Name(),
		Data: ConfigChangeData{
			Provider: providerName,
			Model:    model,
		},
		Timestamp: time.Now(),
	})

	return nil
}

// SendMessage sends a message to a specific mysis (synchronous).
func (c *Commander) SendMessage(id, content string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}
	return mysis.SendMessage(content, store.MemorySourceDirect)
}

// SendMessageAsync sends a message to a specific mysis without waiting for processing.
// Returns immediately after validating the mysis exists and is running.
func (c *Commander) SendMessageAsync(id, content string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}
	if mysis.State() != MysisStateRunning {
		return fmt.Errorf("mysis not running")
	}
	go func() {
		if err := mysis.SendMessage(content, store.MemorySourceDirect); err != nil {
			// Error is published to bus by mysis.SendMessage
		}
	}()
	return nil
}

// Broadcast sends a message to all running myses (synchronous).
func (c *Commander) Broadcast(content string) error {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		if m.State() == MysisStateRunning {
			myses = append(myses, m)
		}
	}
	c.mu.RUnlock()

	// Check if any myses are running
	if len(myses) == 0 {
		return fmt.Errorf("no running myses to receive broadcast")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Data:      MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	var errs []error
	for _, m := range myses {
		if err := m.SendMessage(content, store.MemorySourceBroadcast); err != nil {
			errs = append(errs, fmt.Errorf("mysis %s: %w", m.ID(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("broadcast failed for %d mysis: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// BroadcastAsync sends a message to all running myses without waiting for processing.
// Returns immediately after validating at least one mysis is running.
func (c *Commander) BroadcastAsync(content string) error {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		if m.State() == MysisStateRunning {
			myses = append(myses, m)
		}
	}
	c.mu.RUnlock()

	// Check if any myses are running
	if len(myses) == 0 {
		return fmt.Errorf("no running myses to receive broadcast")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Data:      MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	// Send to each mysis asynchronously
	for _, m := range myses {
		mysis := m
		go func() {
			if err := mysis.SendMessage(content, store.MemorySourceBroadcast); err != nil {
				// Error is published to bus by mysis.SendMessage
			}
		}()
	}

	return nil
}

// StopAll stops all running myses.
func (c *Commander) StopAll() {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		myses = append(myses, m)
	}
	c.mu.RUnlock()

	for _, m := range myses {
		if m.State() == MysisStateRunning {
			m.Stop()
		}
	}
}

// MysisCount returns the current number of myses.
func (c *Commander) MysisCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.myses)
}

// MaxMyses returns the maximum allowed myses.
func (c *Commander) MaxMyses() int {
	return c.maxMyses
}

// Store returns the store for direct access (e.g., for testing).
func (c *Commander) Store() *store.Store {
	return c.store
}
