package core

import (
	"context"
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
	wg sync.WaitGroup // Tracks running mysis goroutines

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

		mysis := NewMysis(sm.ID, sm.Name, sm.CreatedAt, p, c.store, c.bus, c)
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
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, p, c.store, c.bus, c)
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
	mysis.releaseCurrentAccount()

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

	// Don't increment WaitGroup here - mysis.Start() will do it
	// This avoids double-counting when restarting errored myses
	return mysis.Start()
}

// StopMysis stops a mysis by ID.
func (c *Commander) StopMysis(id string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
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
		Config: &ConfigChangeData{
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
// Returns immediately after validating the mysis exists.
// State validation is done inside mysis.SendMessage.
func (c *Commander) SendMessageAsync(id, content string) error {
	mysis, err := c.GetMysis(id)
	if err != nil {
		return err
	}
	// State validation is done inside mysis.SendMessage
	go func() {
		if err := mysis.SendMessage(content, store.MemorySourceDirect); err != nil {
			// Error is published to bus by mysis.SendMessage
		}
	}()
	return nil
}

// Broadcast sends a message to all running myses.
// Stores the message immediately and triggers async processing.
// Returns quickly without waiting for LLM processing.
func (c *Commander) Broadcast(content string) error {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		// Include myses in idle or running state (can accept messages)
		if validateCanAcceptMessage(m.State()) == nil {
			myses = append(myses, m)
		}
	}
	c.mu.RUnlock()

	// Check if any myses can receive broadcast
	if len(myses) == 0 {
		return fmt.Errorf("no myses available to receive broadcast (all stopped or errored)")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Message:   &MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	// Queue broadcast to each mysis (non-blocking)
	var errs []error
	for _, m := range myses {
		if err := m.QueueBroadcast(content, ""); err != nil {
			errs = append(errs, fmt.Errorf("mysis %s: %w", m.ID(), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("broadcast failed for %d mysis: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// BroadcastFrom sends a message to all running myses except the sender.
func (c *Commander) BroadcastFrom(senderID, content string) error {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		// Include myses in idle or running state (can accept messages), excluding sender
		if validateCanAcceptMessage(m.State()) == nil && m.ID() != senderID {
			myses = append(myses, m)
		}
	}
	c.mu.RUnlock()

	if len(myses) == 0 {
		return fmt.Errorf("no recipients for broadcast (sender excluded or all stopped/errored)")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Message:   &MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	var errs []error
	for _, m := range myses {
		if err := m.SendMessageFrom(content, store.MemorySourceBroadcast, senderID); err != nil {
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
	return c.BroadcastFrom("", content)
}

// StopAll stops all running myses with a 10-second timeout.
func (c *Commander) StopAll() {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		myses = append(myses, m)
	}
	c.mu.RUnlock()

	// Stop with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		// Stop all running myses
		for _, m := range myses {
			if m.State() == MysisStateRunning {
				if err := m.Stop(); err != nil {
					log.Warn().Err(err).Str("mysis", m.Name()).Msg("Failed to stop mysis")
				}
			}
		}
		// Wait for all mysis goroutines to complete
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Int("count", len(myses)).Msg("All myses stopped")
	case <-ctx.Done():
		log.Warn().Msg("StopAll timeout - some myses may still be running")
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

// GetStateCounts returns the count of myses in each state.
func (c *Commander) GetStateCounts() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	counts := map[string]int{
		"running": 0,
		"idle":    0,
		"stopped": 0,
		"errored": 0,
	}

	for _, m := range c.myses {
		state := string(m.State())
		counts[state]++
	}

	return counts
}

// Store returns the store for direct access (e.g., for testing).
func (c *Commander) Store() *store.Store {
	return c.store
}

// AggregateTick returns the maximum lastServerTick across all myses.
// Returns 0 if no myses exist or if no mysis has received tick data.
func (c *Commander) AggregateTick() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var maxTick int64
	for _, m := range c.myses {
		m.mu.RLock()
		tick := m.lastServerTick
		m.mu.RUnlock()

		if tick > maxTick {
			maxTick = tick
		}
	}

	return maxTick
}
