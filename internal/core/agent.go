package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// Agent represents a single AI agent in the swarm.
type Agent struct {
	mu sync.RWMutex

	id       string
	name     string
	provider provider.Provider
	store    *store.Store
	bus      *EventBus

	state  AgentState
	cancel context.CancelFunc

	// For runtime tracking
	lastError error
}

// NewAgent creates a new agent from stored data.
func NewAgent(id, name string, p provider.Provider, s *store.Store, bus *EventBus) *Agent {
	return &Agent{
		id:       id,
		name:     name,
		provider: p,
		store:    s,
		bus:      bus,
		state:    AgentStateIdle,
	}
}

// ID returns the agent's unique identifier.
func (a *Agent) ID() string {
	return a.id
}

// Name returns the agent's display name.
func (a *Agent) Name() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
}

// State returns the agent's current state.
func (a *Agent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// ProviderName returns the name of the agent's provider.
func (a *Agent) ProviderName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.provider == nil {
		return ""
	}
	return a.provider.Name()
}

// LastError returns the last error encountered by the agent.
func (a *Agent) LastError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastError
}

// SetProvider updates the agent's provider.
func (a *Agent) SetProvider(p provider.Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = p
}

// Start begins the agent's processing loop.
func (a *Agent) Start() error {
	a.mu.Lock()
	if a.state == AgentStateRunning {
		a.mu.Unlock()
		return fmt.Errorf("agent already running")
	}

	oldState := a.state
	a.state = AgentStateRunning
	a.lastError = nil

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.mu.Unlock()

	// Update store
	if err := a.store.UpdateAgentState(a.id, store.AgentStateRunning); err != nil {
		a.mu.Lock()
		a.state = AgentStateErrored
		a.lastError = err
		a.mu.Unlock()
		return err
	}

	// Emit state change event
	a.emitStateChange(oldState, AgentStateRunning)

	// Start the processing goroutine
	go a.run(ctx)

	return nil
}

// Stop halts the agent's processing loop.
func (a *Agent) Stop() error {
	a.mu.Lock()
	if a.state != AgentStateRunning {
		a.mu.Unlock()
		return fmt.Errorf("agent not running")
	}

	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}

	oldState := a.state
	a.state = AgentStateStopped
	a.mu.Unlock()

	// Update store
	if err := a.store.UpdateAgentState(a.id, store.AgentStateStopped); err != nil {
		return err
	}

	// Emit state change event
	a.emitStateChange(oldState, AgentStateStopped)

	return nil
}

// SendMessage sends a message to the agent for processing.
func (a *Agent) SendMessage(content string) error {
	a.mu.RLock()
	state := a.state
	p := a.provider
	a.mu.RUnlock()

	if state != AgentStateRunning {
		return fmt.Errorf("agent not running")
	}

	// Store the user message
	if _, err := a.store.AddMemory(a.id, store.MemoryRoleUser, content); err != nil {
		return fmt.Errorf("store message: %w", err)
	}

	// Emit message event
	a.bus.Publish(Event{
		Type:      EventAgentMessage,
		AgentID:   a.id,
		AgentName: a.name,
		Data:      MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	// Get conversation history
	memories, err := a.store.GetMemories(a.id)
	if err != nil {
		return fmt.Errorf("get memories: %w", err)
	}

	// Convert to provider messages
	messages := make([]provider.Message, len(memories))
	for i, m := range memories {
		messages[i] = provider.Message{
			Role:    string(m.Role),
			Content: m.Content,
		}
	}

	// Get response from provider
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, err := p.Chat(ctx, messages)
	if err != nil {
		a.mu.Lock()
		a.lastError = err
		a.mu.Unlock()

		a.bus.Publish(Event{
			Type:      EventAgentError,
			AgentID:   a.id,
			AgentName: a.name,
			Data:      ErrorData{Error: err.Error()},
			Timestamp: time.Now(),
		})
		return fmt.Errorf("provider chat: %w", err)
	}

	// Store the assistant response
	if _, err := a.store.AddMemory(a.id, store.MemoryRoleAssistant, response); err != nil {
		return fmt.Errorf("store response: %w", err)
	}

	// Emit response event
	a.bus.Publish(Event{
		Type:      EventAgentResponse,
		AgentID:   a.id,
		AgentName: a.name,
		Data:      MessageData{Role: "assistant", Content: response},
		Timestamp: time.Now(),
	})

	return nil
}

// run is the agent's main processing loop.
func (a *Agent) run(ctx context.Context) {
	<-ctx.Done()
	// Context cancelled, agent stopped
}

func (a *Agent) emitStateChange(oldState, newState AgentState) {
	a.bus.Publish(Event{
		Type:      EventAgentStateChanged,
		AgentID:   a.id,
		AgentName: a.name,
		Data: StateChangeData{
			OldState: oldState,
			NewState: newState,
		},
		Timestamp: time.Now(),
	})
}

func (a *Agent) setErrorState(err error) {
	a.mu.Lock()
	oldState := a.state
	a.state = AgentStateErrored
	a.lastError = err
	a.mu.Unlock()

	a.store.UpdateAgentState(a.id, store.AgentStateErrored)
	a.emitStateChange(oldState, AgentStateErrored)

	a.bus.Publish(Event{
		Type:      EventAgentError,
		AgentID:   a.id,
		AgentName: a.name,
		Data:      ErrorData{Error: err.Error()},
		Timestamp: time.Now(),
	})
}
