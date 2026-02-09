package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xonecas/zoea-nova/internal/constants"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// Mysis represents a single AI mysis in the swarm.
type Mysis struct {
	mu sync.RWMutex

	id        string
	name      string
	createdAt time.Time
	provider  provider.Provider
	store     *store.Store
	bus       *EventBus
	mcp       *mcp.Proxy
	commander *Commander // Reference to parent commander for WaitGroup

	state  MysisState
	ctx    context.Context
	cancel context.CancelFunc

	// turnMu ensures only one turn runs at a time.
	turnMu sync.Mutex

	// For runtime tracking
	lastError              error
	currentAccountUsername string
	activityState          ActivityState
	activityUntil          time.Time
	lastServerTick         int64
	lastServerTickAt       time.Time
	tickDuration           time.Duration
	encouragementCount     int // Counter for consecutive synthetic encouragements (limit: 3 before idle)
}

type contextStats struct {
	MemoryCount    int
	MessageCount   int
	ContentBytes   int
	ReasoningBytes int
	RoleCounts     map[string]int
	SourceCounts   map[string]int
	ToolCallCount  int
}

// NewMysis creates a new mysis from stored data.
func NewMysis(id, name string, createdAt time.Time, p provider.Provider, s *store.Store, bus *EventBus, cmd ...*Commander) *Mysis {
	var commander *Commander
	if len(cmd) > 0 {
		commander = cmd[0]
	}
	return &Mysis{
		id:            id,
		name:          name,
		createdAt:     createdAt,
		provider:      p,
		store:         s,
		bus:           bus,
		commander:     commander,
		state:         MysisStateIdle,
		activityState: ActivityStateIdle,
	}
}

func (m *Mysis) computeMemoryStats(memories []*store.Memory) contextStats {
	stats := contextStats{
		MemoryCount:  len(memories),
		RoleCounts:   make(map[string]int),
		SourceCounts: make(map[string]int),
	}

	for _, m := range memories {
		stats.ContentBytes += len(m.Content)
		stats.ReasoningBytes += len(m.Reasoning)
		stats.RoleCounts[string(m.Role)]++
		stats.SourceCounts[string(m.Source)]++
	}

	return stats
}

func (m *Mysis) computeMessageStats(messages []provider.Message) contextStats {
	stats := contextStats{
		MessageCount: len(messages),
	}

	for _, msg := range messages {
		stats.ContentBytes += len(msg.Content)
		stats.ToolCallCount += len(msg.ToolCalls)
	}

	return stats
}

// SetMCP sets the MCP proxy for tool calling.
func (m *Mysis) SetMCP(proxy *mcp.Proxy) {
	a := m
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mcp = proxy
}

// ID returns the mysis unique identifier.
func (m *Mysis) ID() string {
	a := m
	return a.id
}

// Name returns the mysis display name.
func (m *Mysis) Name() string {
	a := m
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
}

// CreatedAt returns when the mysis was created.
func (m *Mysis) CreatedAt() time.Time {
	a := m
	return a.createdAt
}

// State returns the mysis current state.
func (m *Mysis) State() MysisState {
	a := m
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// validateCanAcceptMessage checks if a mysis in the given state can accept messages.
// Returns nil if messages are allowed, error with user-facing message if not.
//
// Valid states for accepting messages:
//   - idle: Mysis is not running but can accept messages (will process when started)
//   - running: Mysis is actively running and processing messages
//
// Invalid states for accepting messages:
//   - stopped: User explicitly stopped the mysis, requires relaunch
//   - errored: Mysis encountered an error, requires relaunch
func validateCanAcceptMessage(state MysisState) error {
	switch state {
	case MysisStateIdle, MysisStateRunning:
		return nil
	case MysisStateStopped:
		return fmt.Errorf("mysis stopped - press 'r' to relaunch")
	case MysisStateErrored:
		return fmt.Errorf("mysis errored - press 'r' to relaunch")
	default:
		return fmt.Errorf("unknown mysis state: %s", state)
	}
}

// ProviderName returns the name of the mysis provider.
func (m *Mysis) ProviderName() string {
	a := m
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.provider == nil {
		return ""
	}
	return a.provider.Name()
}

// LastError returns the last error encountered by the mysis.
func (m *Mysis) LastError() error {
	a := m
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastError
}

// CurrentAccountUsername returns the username of the account this mysis is using.
func (m *Mysis) CurrentAccountUsername() string {
	a := m
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentAccountUsername
}

// ActivityState returns the mysis current activity.
func (m *Mysis) ActivityState() ActivityState {
	a := m
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activityState
}

// SetProvider updates the mysis provider.
func (m *Mysis) SetProvider(p provider.Provider) {
	a := m
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = p
}

// SetErrorState sets the mysis to errored state with the given error.
// Used for testing error recovery scenarios.
func (m *Mysis) SetErrorState(err error) {
	m.setErrorState(err)
}

// Start begins the mysis processing loop.
func (m *Mysis) Start() error {
	a := m
	a.mu.Lock()
	if a.state == MysisStateRunning {
		a.mu.Unlock()
		return fmt.Errorf("mysis already running")
	}

	oldState := a.state

	// If restarting from errored state, cleanup any existing context/goroutine
	if oldState == MysisStateErrored && a.cancel != nil {
		a.cancel() // Cancel old context
		a.mu.Unlock()
		// Wait for old goroutine to exit
		done := make(chan struct{})
		go func() {
			a.turnMu.Lock()
			close(done)
			a.turnMu.Unlock()
		}()
		select {
		case <-done:
			// Old goroutine exited
		case <-time.After(2 * time.Second):
			log.Warn().Str("mysis", a.name).Msg("Timeout waiting for errored goroutine to exit")
		}
		a.mu.Lock()
	}

	a.mu.Unlock()

	// Create context first (before any state changes)
	ctx, cancel := context.WithCancel(context.Background())

	// Update store BEFORE changing in-memory state
	// This ensures we don't start the goroutine if persistence fails
	if err := a.store.UpdateMysisState(a.id, store.MysisStateRunning); err != nil {
		cancel() // Clean up context since we're not starting
		return fmt.Errorf("failed to update state in store: %w", err)
	}

	// Now that store update succeeded, update in-memory state
	a.mu.Lock()
	a.state = MysisStateRunning
	a.lastError = nil
	a.activityState = ActivityStateIdle
	a.activityUntil = time.Time{}
	a.encouragementCount = 0 // Reset encouragement counter when starting/restarting
	a.ctx = ctx
	a.cancel = cancel
	a.mu.Unlock()

	// Add system prompt if it doesn't exist yet
	system, err := a.store.GetSystemMemory(a.id)
	if err != nil || system == nil {
		systemPrompt := a.buildSystemPrompt()
		a.store.AddMemory(a.id, store.MemoryRoleSystem, store.MemorySourceSystem, systemPrompt, "", "")
	}

	// Emit state change event
	a.emitStateChange(oldState, MysisStateRunning)

	// Track goroutine in WaitGroup (only after state change succeeded)
	if a.commander != nil {
		a.commander.wg.Add(1)
	}

	// Start the processing goroutine (only after all setup succeeded)
	// Initial encouragement message (if needed) is added by getContextMemories() in run() loop
	go a.run(ctx)

	return nil
}

// Stop halts the mysis processing loop.
func (m *Mysis) Stop() error {
	a := m
	a.mu.Lock()
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		return nil
	}

	// Cancel context to signal goroutine to stop
	if a.cancel != nil {
		a.cancel()
	}

	// Set state to Stopped IMMEDIATELY (before waiting)
	// This closes the race window where setError() could override with Errored
	oldState := a.state
	a.state = MysisStateStopped
	a.mu.Unlock()

	// Update store (if fails, in-memory state already correct)
	if err := a.store.UpdateMysisState(a.id, store.MysisStateStopped); err != nil {
		log.Warn().Err(err).Str("mysis", a.name).Msg("Failed to persist Stopped state")
		// Continue anyway - in-memory state is authoritative
	}

	// Emit state change event (TUI will update immediately)
	a.emitStateChange(oldState, MysisStateStopped)

	// Wait for current turn to finish with timeout
	// IMPORTANT: Don't clear ctx/cancel until AFTER turn completes
	// Otherwise SendMessageFrom gets nil ctx and creates uncanceled Background context
	done := make(chan struct{})
	go func() {
		a.turnMu.Lock()
		close(done)
		a.turnMu.Unlock()
	}()

	select {
	case <-done:
		// Turn finished successfully
	case <-time.After(5 * time.Second):
		log.Warn().Str("mysis", a.name).Msg("Stop timeout - forcing shutdown")
		// Continue with cleanup even if turn didn't complete
	}

	// Clear context references AFTER turn completes
	a.mu.Lock()
	a.ctx = nil
	a.cancel = nil
	a.mu.Unlock()

	// Release resources
	a.releaseCurrentAccount()

	// Close provider HTTP client
	if a.provider != nil {
		if err := a.provider.Close(); err != nil {
			log.Warn().Err(err).Str("mysis", a.name).Msg("Failed to close provider")
		}
	}

	return nil
}

// SendMessageFrom sends a message to the mysis for processing with sender tracking.
// The source parameter indicates whether this is a direct or broadcast message.
func (m *Mysis) SendMessageFrom(content string, source store.MemorySource, senderID string) error {
	a := m
	a.turnMu.Lock()
	defer a.turnMu.Unlock()

	a.mu.RLock()
	state := a.state
	p := a.provider
	mcpProxy := a.mcp
	a.mu.RUnlock()

	if err := validateCanAcceptMessage(state); err != nil {
		return err
	}

	// Determine role based on source
	role := store.MemoryRoleUser
	if source == store.MemorySourceSystem {
		role = store.MemoryRoleSystem
	}

	// Store the message (skip empty internal trigger messages from run loop)
	if content != "" {
		if err := a.store.AddMemory(a.id, role, source, content, "", senderID); err != nil {
			return fmt.Errorf("store message: %w", err)
		}

		// Reset encouragement counter - we have a real user message now
		a.mu.Lock()
		a.encouragementCount = 0
		a.mu.Unlock()

		// Emit message event
		a.bus.Publish(Event{
			Type:      EventMysisMessage,
			MysisID:   a.id,
			MysisName: a.name,
			Message:   &MessageData{Role: string(role), Content: content},
			Timestamp: time.Now(),
		})
	}

	// If mysis is idle, auto-start it (direct messages should wake idle myses)
	// Note: Empty content means this is an internal trigger from run() loop
	if state == MysisStateIdle && content != "" {
		if err := a.Start(); err != nil {
			// Log warning but don't fail the message send
			// The message is already stored and will be processed when manually started
			log.Warn().Err(err).Str("mysis", a.name).Msg("Failed to auto-start idle mysis on direct message")
		}
		// Return early - run() loop will process the message
		return nil
	}

	// If this is an internal trigger but mysis is not running, skip processing
	if content == "" && state != MysisStateRunning {
		return nil
	}

	// Create context for the entire conversation turn
	a.mu.RLock()
	parentCtx := a.ctx
	a.mu.RUnlock()

	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithTimeout(parentCtx, constants.LLMRequestTimeout)
	defer cancel()

	// Get available tools from MCP proxy
	var tools []provider.Tool
	if mcpProxy != nil {
		mcpTools, err := mcpProxy.ListTools(ctx)
		if err != nil {
			// Log error but continue - mysis can still chat without tools
			a.publishCriticalEvent(Event{
				Type:      EventMysisError,
				MysisID:   a.id,
				MysisName: a.name,
				Error:     &ErrorData{Error: fmt.Sprintf("Failed to load tools: %v", err)},
				Timestamp: time.Now(),
			})
		} else {
			tools = make([]provider.Tool, len(mcpTools))
			toolNames := make([]string, len(mcpTools))
			for i, t := range mcpTools {
				tools[i] = provider.Tool{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				}
				toolNames[i] = t.Name
			}
			// Log available tools (only on first message or if debugging)
			if len(tools) > 0 {
				a.bus.Publish(Event{
					Type:      EventMysisMessage,
					MysisID:   a.id,
					MysisName: a.name,
					Message:   &MessageData{Role: "system", Content: fmt.Sprintf("Tools available: %s", strings.Join(toolNames, ", "))},
					Timestamp: time.Now(),
				})
			}
		}
	} else {
		a.publishCriticalEvent(Event{
			Type:      EventMysisError,
			MysisID:   a.id,
			MysisName: a.name,
			Error:     &ErrorData{Error: "MCP proxy not configured - no tools available"},
			Timestamp: time.Now(),
		})
	}

	// Track if synthetic encouragement was added (for counter increment after turn completes)
	var addedSyntheticEncouragement bool

	// Loop: keep calling LLM until we get a final text response
	for iteration := 0; iteration < constants.MaxToolIterations; iteration++ {
		// Get recent conversation history (keeps context small for faster inference)
		memories, addedSynthetic, err := a.getContextMemories()
		if err != nil {
			a.setError(err)
			return fmt.Errorf("get memories: %w", err)
		}

		// Track if synthetic encouragement was added on first iteration
		// (subsequent iterations reuse same context, so only first matters)
		if iteration == 0 {
			addedSyntheticEncouragement = addedSynthetic
		}

		// Check if encouragement limit reached (counter incremented after turn completes)
		a.mu.RLock()
		count := a.encouragementCount
		a.mu.RUnlock()

		if count >= 3 {
			// No real user messages for 3 consecutive autonomous turns - go idle
			a.setIdle("No user messages after 3 encouragements")
			return nil
		}

		memoryStats := a.computeMemoryStats(memories)
		log.Debug().
			Str("mysis_id", a.id).
			Str("mysis_name", a.name).
			Str("stage", "context_memories").
			Int("memory_count", memoryStats.MemoryCount).
			Int("message_count", 0).
			Int("content_bytes", memoryStats.ContentBytes).
			Int("reasoning_bytes", memoryStats.ReasoningBytes).
			Interface("role_counts", memoryStats.RoleCounts).
			Interface("source_counts", memoryStats.SourceCounts).
			Int("tool_call_count", 0).
			Msg("Context stats")

		// Convert to provider messages
		messages := a.memoriesToMessages(memories)
		messageStats := a.computeMessageStats(messages)
		log.Debug().
			Str("mysis_id", a.id).
			Str("mysis_name", a.name).
			Str("stage", "messages_converted").
			Int("memory_count", memoryStats.MemoryCount).
			Int("message_count", messageStats.MessageCount).
			Int("content_bytes", messageStats.ContentBytes).
			Int("reasoning_bytes", memoryStats.ReasoningBytes).
			Interface("role_counts", memoryStats.RoleCounts).
			Interface("source_counts", memoryStats.SourceCounts).
			Int("tool_call_count", messageStats.ToolCallCount).
			Msg("Context stats")

		// Signal LLM activity start
		a.bus.Publish(Event{
			Type:      EventNetworkLLM,
			MysisID:   a.id,
			MysisName: a.name,
			Timestamp: time.Now(),
		})
		log.Debug().
			Str("mysis_id", a.id).
			Str("mysis_name", a.name).
			Str("stage", "before_llm_call").
			Int("memory_count", memoryStats.MemoryCount).
			Int("message_count", messageStats.MessageCount).
			Int("content_bytes", messageStats.ContentBytes).
			Int("reasoning_bytes", memoryStats.ReasoningBytes).
			Interface("role_counts", memoryStats.RoleCounts).
			Interface("source_counts", memoryStats.SourceCounts).
			Int("tool_call_count", messageStats.ToolCallCount).
			Msg("Context stats")

		// Set activity state to indicate LLM call in progress
		a.setActivity(ActivityStateLLMCall, time.Time{})

		// Get response from provider
		var response *provider.ChatResponse
		if len(tools) > 0 {
			response, err = p.ChatWithTools(ctx, messages, tools)
		} else {
			// No tools available, use simple chat
			text, chatErr := p.Chat(ctx, messages)
			if chatErr != nil {
				err = chatErr
			} else {
				response = &provider.ChatResponse{Content: text}
			}
		}

		// Clear LLM activity state after call completes (success or failure)
		a.setActivity(ActivityStateIdle, time.Time{})

		if err != nil {
			log.Error().
				Str("mysis", a.name).
				Str("provider", p.Name()).
				Err(err).
				Msg("Provider returned error")
			a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})
			a.setError(err)
			return fmt.Errorf("provider chat: %w", err)
		}

		if response.Reasoning != "" {
			log.Debug().Str("mysis", a.name).Int("reasoning_len", len(response.Reasoning)).Msg("LLM reasoning captured")
		}

		// If we have tool calls, execute them
		if len(response.ToolCalls) > 0 {
			// Store the assistant's tool call request
			toolCallJSON := a.formatToolCallsForStorage(response.ToolCalls)
			if err := a.store.AddMemory(a.id, store.MemoryRoleAssistant, store.MemorySourceLLM, toolCallJSON, response.Reasoning, ""); err != nil {
				a.setError(err)
				return fmt.Errorf("store tool call: %w", err)
			}

			// Emit event showing which tools are being called
			toolNames := make([]string, len(response.ToolCalls))
			for i, tc := range response.ToolCalls {
				toolNames[i] = tc.Name
			}
			a.bus.Publish(Event{
				Type:      EventMysisMessage,
				MysisID:   a.id,
				MysisName: a.name,
				Message:   &MessageData{Role: "assistant", Content: fmt.Sprintf("Calling tools: %s", strings.Join(toolNames, ", "))},
				Timestamp: time.Now(),
			})

			// Execute each tool call
			for _, tc := range response.ToolCalls {
				// Signal MCP activity
				a.bus.Publish(Event{
					Type:      EventNetworkMCP,
					MysisID:   a.id,
					MysisName: a.name,
					Timestamp: time.Now(),
				})

				// Set activity state to indicate MCP call in progress
				a.setActivity(ActivityStateMCPCall, time.Time{})

				result, execErr := a.executeToolCall(ctx, mcpProxy, tc)

				// Clear MCP activity state after call completes
				a.setActivity(ActivityStateIdle, time.Time{})

				// Signal MCP activity complete
				a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})

				a.updateActivityFromToolResult(result, execErr)

				// Store the tool result
				resultContent := a.formatToolResult(tc.ID, tc.Name, result, execErr)
				if err := a.store.AddMemory(a.id, store.MemoryRoleTool, store.MemorySourceTool, resultContent, "", ""); err != nil {
					a.setError(err)
					return fmt.Errorf("store tool result: %w", err)
				}

				// Emit tool result event
				a.bus.Publish(Event{
					Type:      EventMysisMessage,
					MysisID:   a.id,
					MysisName: a.name,
					Message:   &MessageData{Role: "tool", Content: fmt.Sprintf("[%s] %s", tc.Name, a.formatToolResultDisplay(result, execErr))},
					Timestamp: time.Now(),
				})

				if execErr != nil && isToolTimeout(execErr) {
					a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})
					a.setError(execErr)
					return fmt.Errorf("tool call timed out: %w", execErr)
				}

				if execErr != nil && isToolRetryExhausted(execErr) {
					a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})
					a.setError(execErr)
					return fmt.Errorf("tool call failed after retries: %w", execErr)
				}
			}

			// Continue loop to get next LLM response
			continue
		}

		// No tool calls - we have a final response
		finalResponse := response.Content
		if finalResponse == "" && response.Reasoning == "" {
			finalResponse = constants.FallbackLLMResponse
		}

		// Store the assistant response
		if err := a.store.AddMemory(a.id, store.MemoryRoleAssistant, store.MemorySourceLLM, finalResponse, response.Reasoning, ""); err != nil {
			a.setError(err)
			return fmt.Errorf("store response: %w", err)
		}

		// Signal network idle
		a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})

		// Emit response event
		a.bus.Publish(Event{
			Type:      EventMysisResponse,
			MysisID:   a.id,
			MysisName: a.name,
			Message:   &MessageData{Role: "assistant", Content: finalResponse},
			Timestamp: time.Now(),
		})

		// Increment encouragement counter if this was an autonomous turn (no user message)
		// If synthetic encouragement was added, this means no real user message existed
		if addedSyntheticEncouragement {
			a.mu.Lock()
			a.encouragementCount++
			count := a.encouragementCount
			a.mu.Unlock()

			log.Debug().
				Str("mysis", a.name).
				Int("count", count).
				Msg("Autonomous turn completed - incremented encouragement counter")

			// If we've hit the limit, transition to idle immediately
			if count >= 3 {
				a.setIdle("No user messages after 3 encouragements")
				return nil
			}
		}

		return nil
	}

	// Max tool iterations reached - end this turn gracefully and continue to next turn
	// This is NOT an error - the mysis made progress and should continue autonomous operation
	log.Warn().
		Str("mysis", a.name).
		Int("max_iterations", constants.MaxToolIterations).
		Msg("Max tool iterations reached - ending turn, will continue next turn")

	// Signal network idle
	a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})

	// Increment encouragement counter if this was an autonomous turn
	// Same logic as successful turn completion (lines 697-713)
	if addedSyntheticEncouragement {
		a.mu.Lock()
		a.encouragementCount++
		count := a.encouragementCount
		a.mu.Unlock()

		log.Debug().
			Str("mysis", a.name).
			Int("count", count).
			Msg("Autonomous turn completed (max iterations) - incremented encouragement counter")

		// If we've hit the limit, transition to idle immediately
		if count >= 3 {
			a.setIdle("No user messages after 3 encouragements")
			return nil
		}
	}

	// Return success - turn completed, continue to next turn
	return nil
}

// SendMessage sends a message to the mysis for processing.
func (m *Mysis) SendMessage(content string, source store.MemorySource) error {
	a := m
	return a.SendMessageFrom(content, source, "")
}

// QueueBroadcast stores a broadcast message and triggers async processing.
// Unlike SendMessage, this does not block waiting for the mysis to process.
// Returns immediately after storing the message in the database.
func (m *Mysis) QueueBroadcast(content string, senderID string) error {
	a := m

	a.mu.RLock()
	state := a.state
	a.mu.RUnlock()

	if err := validateCanAcceptMessage(state); err != nil {
		return err
	}

	// Store the message immediately (fast DB write, no LLM call)
	if err := a.store.AddMemory(a.id, store.MemoryRoleUser, store.MemorySourceBroadcast, content, "", senderID); err != nil {
		return fmt.Errorf("store broadcast: %w", err)
	}

	// Reset encouragement counter - we have a real user message now
	a.mu.Lock()
	a.encouragementCount = 0
	a.mu.Unlock()

	// Emit message event
	a.bus.Publish(Event{
		Type:      EventMysisMessage,
		MysisID:   a.id,
		MysisName: a.name,
		Message:   &MessageData{Role: "user", Content: content},
		Timestamp: time.Now(),
	})

	// If mysis is idle, auto-start it (broadcasts should wake idle myses)
	if state == MysisStateIdle {
		if err := a.Start(); err != nil {
			// Log warning but don't fail the broadcast
			// The message is already stored and will be processed when manually started
			log.Warn().Err(err).Str("mysis", a.name).Msg("Failed to auto-start idle mysis on broadcast")
		}
	}

	return nil
}

// memoriesToMessages converts stored memories to provider messages.
func (m *Mysis) memoriesToMessages(memories []*store.Memory) []provider.Message {
	a := m
	messages := make([]provider.Message, 0, len(memories))

	for _, m := range memories {
		msg := provider.Message{
			Role:    string(m.Role),
			Content: m.Content,
		}

		// Handle tool role - needs ToolCallID
		if m.Role == store.MemoryRoleTool {
			// Extract tool call ID from stored format: "tool_call_id:content"
			idx := strings.Index(m.Content, constants.ToolCallStorageFieldDelimiter)
			if idx <= 0 {
				log.Warn().
					Str("content", m.Content).
					Msg("Skipping malformed tool result - missing delimiter")
				continue // Skip this message
			}
			toolCallID := m.Content[:idx]
			if toolCallID == "" {
				log.Warn().
					Str("content", m.Content).
					Msg("Skipping tool result with empty tool_call_id")
				continue // Skip this message
			}
			msg.ToolCallID = toolCallID
			msg.Content = m.Content[idx+1:]
		}

		// Handle assistant messages with tool calls
		if m.Role == store.MemoryRoleAssistant && strings.HasPrefix(m.Content, constants.ToolCallStoragePrefix) {
			// Parse tool calls from stored format
			msg.ToolCalls = a.parseStoredToolCalls(m.Content)
			if len(msg.ToolCalls) > 0 {
				msg.Content = "" // Only clear content if tool calls were parsed successfully
			} else {
				log.Warn().
					Str("content", m.Content).
					Msg("Failed to parse tool calls from assistant message - keeping original content")
				// Keep original content as fallback
			}
		}

		messages = append(messages, msg)
	}

	return messages
}

// executeToolCall executes a single tool call via MCP proxy.
func (m *Mysis) executeToolCall(ctx context.Context, mcpProxy *mcp.Proxy, tc provider.ToolCall) (*mcp.ToolResult, error) {
	a := m
	if mcpProxy == nil {
		return &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "MCP not configured"}},
			IsError: true,
		}, nil
	}

	caller := mcp.CallerContext{
		MysisID:   a.id,
		MysisName: a.name,
	}

	result, err := mcpProxy.CallTool(ctx, caller, tc.Name, tc.Arguments)
	if err == nil && result != nil && !result.IsError {
		switch tc.Name {
		case "login", "register":
			var args struct {
				Username string `json:"username"`
			}
			if err := json.Unmarshal(tc.Arguments, &args); err == nil {
				a.setCurrentAccount(args.Username)
			}
		case "logout":
			a.releaseCurrentAccount()
		}
	}

	return result, err
}

func (m *Mysis) setCurrentAccount(username string) {
	a := m
	if username == "" {
		return
	}

	a.mu.Lock()
	previous := a.currentAccountUsername
	a.currentAccountUsername = username
	a.mu.Unlock()

	if previous != "" && previous != username {
		_ = a.store.ReleaseAccount(previous)
	}
}

func (m *Mysis) releaseCurrentAccount() {
	a := m
	a.mu.Lock()
	username := a.currentAccountUsername
	a.currentAccountUsername = ""
	a.mu.Unlock()

	if username != "" {
		_ = a.store.ReleaseAccount(username)
	}
}

// formatToolCallsForStorage formats tool calls for storage in memory.
func (m *Mysis) formatToolCallsForStorage(calls []provider.ToolCall) string {
	var parts []string
	for _, tc := range calls {
		parts = append(parts, fmt.Sprintf("%s%s%s%s%s", tc.ID, constants.ToolCallStorageFieldDelimiter, tc.Name, constants.ToolCallStorageFieldDelimiter, string(tc.Arguments)))
	}
	return constants.ToolCallStoragePrefix + strings.Join(parts, constants.ToolCallStorageRecordDelimiter)
}

// parseStoredToolCalls parses tool calls from stored format.
func (m *Mysis) parseStoredToolCalls(stored string) []provider.ToolCall {
	stored = strings.TrimPrefix(stored, constants.ToolCallStoragePrefix)
	if stored == "" {
		return nil
	}

	var calls []provider.ToolCall
	parts := strings.Split(stored, constants.ToolCallStorageRecordDelimiter)
	for _, part := range parts {
		fields := strings.SplitN(part, constants.ToolCallStorageFieldDelimiter, constants.ToolCallStorageFieldCount)
		if len(fields) >= constants.ToolCallStorageFieldCount {
			args := json.RawMessage(fields[2])
			if !json.Valid(args) {
				log.Warn().
					Str("tool_call_id", fields[0]).
					Str("tool_name", fields[1]).
					Msg("Invalid JSON in tool call arguments - using empty object")
				args = json.RawMessage("{}")
			}
			calls = append(calls, provider.ToolCall{
				ID:        fields[0],
				Name:      fields[1],
				Arguments: args,
			})
		}
	}
	return calls
}

// formatToolResult formats a tool result for storage (includes ID for LLM context).
func (m *Mysis) formatToolResult(toolCallID, toolName string, result *mcp.ToolResult, err error) string {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Sprintf("%s%sError calling %s: tool call timed out: %v", toolCallID, constants.ToolCallStorageFieldDelimiter, toolName, err)
		}
		if errors.Is(err, mcp.ErrToolRetryExhausted) {
			return fmt.Sprintf("%s%sError calling %s: MCP call failed after retries: %v", toolCallID, constants.ToolCallStorageFieldDelimiter, toolName, err)
		}
		return fmt.Sprintf("%s%sError calling %s: %v. Check the tool's required parameters and try again.", toolCallID, constants.ToolCallStorageFieldDelimiter, toolName, err)
	}

	var texts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}

	content := strings.Join(texts, "\n")
	if result.IsError {
		if strings.Contains(content, "empty_entry") {
			content = fmt.Sprintf("Error calling %s: %s. The entry field must contain non-empty text. Example: captains_log_add({\"entry\": \"Your message here\"})", toolName, content)
		} else {
			content = fmt.Sprintf("Error calling %s: %s", toolName, content)
		}
	}

	return fmt.Sprintf("%s%s%s", toolCallID, constants.ToolCallStorageFieldDelimiter, content)
}

func isToolTimeout(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

func isToolRetryExhausted(err error) bool {
	return errors.Is(err, mcp.ErrToolRetryExhausted)
}

// formatToolResultDisplay formats a tool result for UI display (human-readable).
func (m *Mysis) formatToolResultDisplay(result *mcp.ToolResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	var texts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}

	content := strings.Join(texts, "\n")
	if result.IsError {
		content = "Error: " + content
	}

	// Truncate long results for display
	if len(content) > constants.ToolResultDisplayMaxChars {
		content = content[:constants.ToolResultDisplayTruncateTo] + constants.ToolResultDisplayEllipsis
	}

	return content
}

// setError sets the mysis last error, transitions to errored state, and emits events.
// This method ensures proper state machine compliance by transitioning to MysisStateErrored.
func (m *Mysis) setError(err error) {
	a := m
	a.mu.Lock()

	// Check state AFTER acquiring lock to close race window
	// This prevents Stop() from being overridden by concurrent setError() calls
	oldState := a.state

	// If mysis was intentionally stopped, don't override with error state
	// This prevents race where Stop() cancels context, causing in-flight turn
	// to error with "context canceled" and override the Stopped state
	if oldState == MysisStateStopped {
		a.mu.Unlock()
		log.Debug().Str("mysis", a.name).Err(err).Msg("Ignoring error - mysis was intentionally stopped")
		return
	}

	log.Error().
		Str("mysis", a.name).
		Str("old_state", string(oldState)).
		Err(err).
		Msg("Mysis transitioning to errored state")

	a.lastError = err
	a.state = MysisStateErrored
	a.mu.Unlock()

	// Update store
	if updateErr := a.store.UpdateMysisState(a.id, store.MysisStateErrored); updateErr != nil {
		log.Warn().Err(updateErr).Str("mysis", a.id).Msg("Failed to update state in store")
	}

	// Emit state change event
	a.emitStateChange(oldState, MysisStateErrored)

	// Release account (if any) to allow restart
	a.releaseCurrentAccount()

	// Emit error event
	a.publishCriticalEvent(Event{
		Type:      EventMysisError,
		MysisID:   a.id,
		MysisName: a.name,
		Error:     &ErrorData{Error: err.Error()},
		Timestamp: time.Now(),
	})
}

func (m *Mysis) setIdle(reason string) {
	a := m
	a.mu.Lock()
	oldState := a.state

	if oldState == MysisStateStopped {
		a.mu.Unlock()
		log.Debug().Str("mysis", a.name).Str("reason", reason).Msg("Ignoring idle transition - mysis was intentionally stopped")
		return
	}

	if oldState == MysisStateIdle {
		a.mu.Unlock()
		return
	}

	log.Warn().
		Str("mysis", a.name).
		Str("old_state", string(oldState)).
		Str("reason", reason).
		Msg("Mysis transitioning to idle state")

	// Cancel context to stop run loop goroutine (like Stop() does)
	if a.cancel != nil {
		a.cancel()
	}

	a.state = MysisStateIdle
	a.lastError = nil
	a.mu.Unlock()

	if updateErr := a.store.UpdateMysisState(a.id, store.MysisStateIdle); updateErr != nil {
		log.Warn().Err(updateErr).Str("mysis", a.id).Msg("Failed to update state in store")
	}

	a.emitStateChange(oldState, MysisStateIdle)
	a.releaseCurrentAccount()
}

// extractLatestToolLoop finds the most recent tool-call message (assistant role
// with tool_calls) and returns it plus all subsequent tool results.
//
// Parameters:
//   - memories: Slice of memories in chronological order (oldest first, newest last)
//     as returned by GetRecentMemories
//
// Returns:
//   - []*store.Memory: Slice containing [tool-call-message, result1, result2, ...]
//     in chronological order, or empty slice if no tool loop found
//
// Behavior:
//   - Scans backwards from the end to find the most recent assistant message
//     with content prefixed by constants.ToolCallStoragePrefix
//   - Collects all consecutive tool role messages that follow the tool call
//   - Stops at the first non-tool message (end of loop)
//   - Returns empty slice if no tool call is found
//
// Rationale:
//
//	By extracting only the latest tool loop, we ensure OpenAI-compatible providers
//	receive properly paired tool calls and results without orphaned tool messages.
//	This prevents API errors from malformed message sequences.
//
// Example:
//
//	Given memories: [user_msg, assistant_with_tools, tool_result1, tool_result2, user_msg2]
//	Returns: [assistant_with_tools, tool_result1, tool_result2]
func (m *Mysis) extractLatestToolLoop(memories []*store.Memory) []*store.Memory {
	if len(memories) == 0 {
		return nil
	}

	// Scan backwards from the end (newest first) to find the most recent tool call
	toolCallIdx := -1
	for i := len(memories) - 1; i >= 0; i-- {
		mem := memories[i]
		if mem.Role == store.MemoryRoleAssistant &&
			strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			toolCallIdx = i
			break
		}
	}

	// No tool calls found
	if toolCallIdx == -1 {
		return nil
	}

	// Collect the tool call message and all subsequent tool results
	result := make([]*store.Memory, 0, 4) // Pre-allocate for typical case (1 call + 2-3 results)
	result = append(result, memories[toolCallIdx])

	// Scan forward (later in time) for consecutive tool results
	for i := toolCallIdx + 1; i < len(memories); i++ {
		mem := memories[i]

		// Only collect tool role messages
		if mem.Role == store.MemoryRoleTool {
			result = append(result, mem)
		} else {
			// Stop at first non-tool message (end of loop)
			break
		}
	}

	return result
}

// findLastUserPromptIndex finds the index of the most recent user-initiated prompt.
// User prompts include:
// - Direct messages (source: direct)
// - Broadcasts (source: broadcast)
// - System nudges (source: system, role: user)
//
// Returns -1 if no user prompt is found.
//
// This defines the "current turn" boundary - everything from this index onward
// is part of the current conversation turn and should be included in full context.
func (m *Mysis) findLastUserPromptIndex(memories []*store.Memory) int {
	// Scan backwards to find most recent user prompt
	for i := len(memories) - 1; i >= 0; i-- {
		mem := memories[i]
		if mem.Role == store.MemoryRoleUser {
			// User messages from direct, broadcast, or system (nudges) all count as prompts
			if mem.Source == store.MemorySourceDirect ||
				mem.Source == store.MemorySourceBroadcast ||
				mem.Source == store.MemorySourceSystem {
				return i
			}
		}
	}
	return -1
}

// getContextMemories returns memories for LLM context with turn-aware composition.
// Composes context as: [system prompt] + [historical context] + [current turn].
//
// Returns:
//   - []*store.Memory: Composed context slice ready for provider conversion
//   - error: Database error from GetRecentMemories or GetSystemMemory
//
// Context Composition Strategy:
//  1. System prompt (if available) - provides mysis identity and mission
//  2. Historical context - memories before the current turn (compressed via extractLatestToolLoop)
//  3. Current turn - ALL memories from the last user prompt onward (uncompressed)
//
// Turn Boundary:
//   - "Current turn" starts at the most recent user-initiated prompt (direct message, broadcast, or synthetic encouragement)
//   - Everything from that prompt forward is included in full to preserve multi-step tool reasoning
//   - Historical turns (before current turn boundary) are compressed to save context
//
// Rationale:
//   - Multi-step tool calling (login → get_status → get_notifications) requires complete tool history
//   - Historical turns can be compressed without breaking current reasoning
//   - Preserves session_id and other tool results from within the current turn
//   - Prevents orphaned tool calls by keeping complete tool loops together
//
// Example context for multi-step turn:
//
//	[
//	  {role: system, content: "You are Mysis Alpha..."},
//	  {role: user, source: direct, content: "Check ship status"},
//	  {role: assistant, content: "[TOOL_CALLS]call_1:login:{}"},
//	  {role: tool, content: "call_1:session_id: abc123"},
//	  {role: assistant, content: "[TOOL_CALLS]call_2:get_status:{\"session_id\":\"abc123\"}"},
//	  {role: tool, content: "call_2:Ship health: 100%"}
//	]
func (m *Mysis) getContextMemories() ([]*store.Memory, bool, error) {
	// Track if synthetic encouragement was added this call
	addedSynthetic := false

	// Get all recent memories
	allMemories, err := m.store.GetRecentMemories(m.id, constants.MaxContextMessages)
	if err != nil {
		return nil, false, err
	}

	// Find current turn boundary
	turnBoundaryIdx := m.findLastUserPromptIndex(allMemories)

	result := make([]*store.Memory, 0, 20) // Pre-allocate for typical size

	// Step 1: Add system prompt (if available)
	system, err := m.store.GetSystemMemory(m.id)
	if err == nil && system != nil {
		result = append(result, system)
	}

	// Step 2: Add historical context (before current turn)
	if turnBoundaryIdx > 0 {
		historicalMemories := allMemories[:turnBoundaryIdx]
		// Apply compression to historical turns - extract only latest tool loop
		historicalToolLoop := m.extractLatestToolLoop(historicalMemories)
		if len(historicalToolLoop) > 0 {
			result = append(result, historicalToolLoop...)
		}
	}

	// Step 3: Add current turn (from last user prompt onward)
	if turnBoundaryIdx >= 0 {
		// Include ALL messages from current turn without compression
		currentTurnMemories := allMemories[turnBoundaryIdx:]
		result = append(result, currentTurnMemories...)
	} else {
		// No user prompt found in sliding window
		// Check if a broadcast exists in DB (even if outside the 20-message window)
		// This ensures myses stay running when they have mission directives
		broadcast, err := m.store.GetMostRecentBroadcast(m.id)
		if err != nil {
			// Database error - return error to caller rather than treating as "no broadcast"
			return nil, false, fmt.Errorf("check for broadcast: %w", err)
		}

		if broadcast != nil {
			// Broadcast exists! Include it to keep mysis running
			// Reset encouragement counter since we have a real user message
			m.mu.Lock()
			m.encouragementCount = 0
			m.mu.Unlock()

			// Include recent tool loop for continuity
			historicalToolLoop := m.extractLatestToolLoop(allMemories)
			if len(historicalToolLoop) > 0 {
				result = append(result, historicalToolLoop...)
			}

			// Check if broadcast is already in allMemories (edge case protection)
			alreadyInContext := false
			for _, mem := range allMemories {
				if mem.ID == broadcast.ID {
					alreadyInContext = true
					break
				}
			}

			// Add the broadcast as the user prompt (if not already present)
			if !alreadyInContext {
				result = append(result, broadcast)
			}
		} else {
			// No broadcast exists - add synthetic encouragement message
			// Include recent tool loop to maintain conversation continuity
			historicalToolLoop := m.extractLatestToolLoop(allMemories)
			if len(historicalToolLoop) > 0 {
				result = append(result, historicalToolLoop...)
			}

			// Then add synthetic encouragement message
			nudgeContent := "Continue your mission. Check notifications and coordinate with the swarm."
			nudgeMemory := &store.Memory{
				Role:      store.MemoryRoleUser,
				Source:    store.MemorySourceSystem,
				Content:   nudgeContent,
				SenderID:  "",
				CreatedAt: time.Now(),
			}
			result = append(result, nudgeMemory)

			// Mark that synthetic encouragement was added
			// Counter will be incremented once per turn by caller after turn completes
			addedSynthetic = true
		}
	}

	// Apply compression to reduce context size and ensure API compliance
	result = m.compactSnapshots(result)
	result = m.removeOrphanedToolCalls(result)

	return result, addedSynthetic, nil
}

// compactSnapshots removes redundant snapshot tool results, keeping only the most recent
// result for each snapshot tool. This prevents state-heavy tools from crowding out
// conversation history while ensuring the latest state is available.
func (m *Mysis) compactSnapshots(memories []*store.Memory) []*store.Memory {
	a := m
	if len(memories) == 0 {
		return memories
	}

	toolCallNames := a.toolCallNameIndex(memories)

	// Track the most recent snapshot tool result for each tool
	latestSnapshot := make(map[string]int) // tool name -> index in memories

	// First pass: identify tool results and track latest for each snapshot tool
	for i, m := range memories {
		if m.Role != store.MemoryRoleTool {
			continue
		}

		// Extract tool name from stored format: "tool_call_id:content"
		toolName := a.extractToolNameFromResult(m.Content, toolCallNames)
		if toolName == "" {
			continue
		}

		// If this is a snapshot tool, track its position
		if a.isSnapshotTool(toolName) {
			latestSnapshot[toolName] = i
		}
	}

	// Second pass: build result, skipping older snapshot tool results
	result := make([]*store.Memory, 0, len(memories))
	for i, m := range memories {
		// Keep non-tool memories
		if m.Role != store.MemoryRoleTool {
			result = append(result, m)
			continue
		}

		// Extract tool name
		toolName := a.extractToolNameFromResult(m.Content, toolCallNames)
		if toolName == "" {
			result = append(result, m)
			continue
		}

		// If this is a snapshot tool, only keep if it's the latest
		if a.isSnapshotTool(toolName) {
			if latestSnapshot[toolName] == i {
				result = append(result, m)
			}
			// Skip older snapshots
			continue
		}

		// Keep non-snapshot tool results
		result = append(result, m)
	}

	return result
}

// collectValidToolResultIDs extracts all tool call IDs from tool result messages.
// Used to validate that assistant tool calls have corresponding tool results.
func (m *Mysis) collectValidToolResultIDs(memories []*store.Memory) map[string]bool {
	validToolResults := make(map[string]bool)

	for _, mem := range memories {
		if mem.Role == store.MemoryRoleTool {
			idx := strings.Index(mem.Content, constants.ToolCallStorageFieldDelimiter)
			if idx > 0 {
				toolCallID := mem.Content[:idx]
				validToolResults[toolCallID] = true
			}
		}
	}

	return validToolResults
}

// removeOrphanedToolCalls removes assistant messages with tool calls that don't have
// corresponding tool result messages. This can happen due to:
// 1. Context window cutting off tool results but keeping tool calls
// 2. Context compaction removing tool result messages
//
// OpenAI Chat Completions API may crash if assistant tool calls don't have matching results.
func (m *Mysis) removeOrphanedToolCalls(memories []*store.Memory) []*store.Memory {
	validToolResults := m.collectValidToolResultIDs(memories)

	result := make([]*store.Memory, 0, len(memories))
	for _, mem := range memories {
		// Check if this is an assistant message with tool calls
		if mem.Role == store.MemoryRoleAssistant &&
			strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			calls := m.parseStoredToolCalls(mem.Content)
			allCallsHaveResults := true
			for _, call := range calls {
				if !validToolResults[call.ID] {
					log.Debug().
						Str("tool_call_id", call.ID).
						Str("tool_name", call.Name).
						Msg("Removing orphaned assistant tool call - no matching tool result")
					allCallsHaveResults = false
					break
				}
			}
			if !allCallsHaveResults {
				continue // Skip assistant message with orphaned tool calls
			}
		}
		result = append(result, mem)
	}

	if len(result) < len(memories) {
		log.Debug().
			Int("removed", len(memories)-len(result)).
			Int("original", len(memories)).
			Int("final", len(result)).
			Msg("Removed orphaned assistant tool calls for OpenAI compliance")
	}

	return result
}

func (m *Mysis) extractToolNameFromResult(content string, toolCallNames map[string]string) string {
	idx := strings.Index(content, constants.ToolCallStorageFieldDelimiter)
	if idx <= 0 {
		return ""
	}

	callID := content[:idx]
	return toolCallNames[callID]
}

func (m *Mysis) toolCallNameIndex(memories []*store.Memory) map[string]string {
	index := make(map[string]string)
	for _, mem := range memories {
		if mem.Role != store.MemoryRoleAssistant {
			continue
		}
		if !strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			continue
		}
		calls := m.parseStoredToolCalls(mem.Content)
		for _, call := range calls {
			if call.ID == "" || call.Name == "" {
				continue
			}
			index[call.ID] = call.Name
		}
	}

	return index
}

func (m *Mysis) isSnapshotTool(toolName string) bool {
	if toolName == "" {
		return false
	}
	if strings.HasPrefix(toolName, "get_") {
		return true
	}
	switch toolName {
	case "zoea_swarm_status", "zoea_list_myses":
		return true
	default:
		return false
	}
}

// run is the mysis main processing loop.
// Takes autonomous turns until reaching idle state (after 3 encouragements with no user messages).
// According to MESSAGE_FORMAT_GUARANTEES.md lines 100-121:
// - Turn 1: getContextMemories() adds synthetic, counter=1, LLM responds
// - Turn 2: Still no user message, synthetic added, counter=2, LLM responds
// - Turn 3: Still no user message, synthetic added, counter=3, transitions to idle
// External messages (broadcast/direct) call SendMessageFrom directly, which handles subsequent turns.
func (m *Mysis) run(ctx context.Context) {
	a := m
	// Signal goroutine completion when exiting
	if a.commander != nil {
		defer a.commander.wg.Done()
	}

	// Autonomous turn loop - continues until idle, stopped, errored, or context canceled
	for {
		// Process one turn using existing SendMessageFrom infrastructure
		// Use empty content - getContextMemories() will add synthetic encouragement if needed
		// This reuses all the complex LLM loop logic, tool calling, error handling, etc.
		if err := a.SendMessageFrom("", store.MemorySourceSystem, ""); err != nil {
			log.Debug().Err(err).Str("mysis", a.name).Msg("Error processing autonomous turn")
			// Error already handled by SendMessageFrom -> setError()
			return
		}

		// Check state after turn completes
		a.mu.RLock()
		state := a.state
		a.mu.RUnlock()

		// Exit if mysis transitioned to terminal state
		switch state {
		case MysisStateIdle:
			log.Debug().Str("mysis", a.name).Msg("Autonomous turn loop exiting - mysis idle")
			return
		case MysisStateStopped:
			log.Debug().Str("mysis", a.name).Msg("Autonomous turn loop exiting - mysis stopped")
			return
		case MysisStateErrored:
			log.Debug().Str("mysis", a.name).Msg("Autonomous turn loop exiting - mysis errored")
			return
		}

		// Wait before next turn (2 seconds delay)
		select {
		case <-time.After(2 * time.Second):
			// Continue to next turn
		case <-ctx.Done():
			// Context canceled (Stop() called)
			log.Debug().Str("mysis", a.name).Msg("Autonomous turn loop exiting - context canceled")
			return
		}
	}
}

// buildSystemPrompt creates the system prompt with the latest swarm broadcast injected.
func (m *Mysis) buildSystemPrompt() string {
	a := m
	base := constants.SystemPrompt

	// Get most recent commander broadcast (sender_id is empty for commander broadcasts)
	broadcasts, err := a.store.GetRecentBroadcasts(10) // Get more to filter for commander broadcasts
	if err != nil {
		return strings.Replace(base, "{{LATEST_BROADCAST}}", constants.BroadcastFallback, 1)
	}

	// Find the most recent commander broadcast (empty sender_id)
	var commanderBroadcast *store.BroadcastMessage
	for _, b := range broadcasts {
		if b.SenderID == "" {
			commanderBroadcast = b
			break
		}
	}

	if commanderBroadcast == nil {
		// No commander broadcasts yet - show fallback
		return strings.Replace(base, "{{LATEST_BROADCAST}}", constants.BroadcastFallback, 1)
	}

	// Format broadcast section (only content, no sender name)
	broadcastSection := fmt.Sprintf(constants.BroadcastSectionTemplate, commanderBroadcast.Content)

	return strings.Replace(base, "{{LATEST_BROADCAST}}", broadcastSection, 1)
}

func (m *Mysis) updateActivityFromToolResult(result *mcp.ToolResult, err error) {
	a := m
	if err != nil || result == nil || result.IsError {
		return
	}

	now := time.Now()
	payload, ok := parseToolResultPayload(result)
	var currentTick int64
	var currentTickOK bool
	if ok {
		currentTick, currentTickOK = findCurrentTick(payload)
		if currentTickOK {
			a.updateServerTick(now, currentTick)
		}
	}

	arrivalTick, found := findIntField(payload, "arrival_tick", "travel_arrival_tick")
	if found {
		if currentTickOK && arrivalTick <= currentTick {
			a.setActivity(ActivityStateIdle, time.Time{})
			return
		}
		until := a.estimateTravelUntil(now, arrivalTick, currentTick, currentTickOK)
		a.setActivity(ActivityStateTraveling, until)
		return
	}

	if progress, ok := findFloatField(payload, "travel_progress"); ok {
		if progress >= 1 {
			a.setActivity(ActivityStateIdle, time.Time{})
			return
		}
		if progress > 0 {
			a.setActivity(ActivityStateTraveling, now.Add(constants.WaitStateNudgeInterval))
			return
		}
	}

	if cooldownTicks, found := findIntField(payload, "cooldown_ticks", "cooldown_remaining"); found && cooldownTicks > 0 {
		until := a.estimateCooldownUntil(now, cooldownTicks)
		a.setActivity(ActivityStateCooldown, until)
		return
	}

}

func (m *Mysis) setActivity(state ActivityState, until time.Time) {
	a := m
	a.mu.Lock()
	a.activityState = state
	a.activityUntil = until
	a.mu.Unlock()
}

func (m *Mysis) clearActivityIf(state ActivityState) {
	a := m
	a.mu.Lock()
	if a.activityState == state {
		a.activityState = ActivityStateIdle
		a.activityUntil = time.Time{}
	}
	a.mu.Unlock()
}

func (m *Mysis) updateServerTick(now time.Time, tick int64) {
	a := m
	if tick <= 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.lastServerTick > 0 && tick > a.lastServerTick && !a.lastServerTickAt.IsZero() {
		elapsed := now.Sub(a.lastServerTickAt)
		delta := tick - a.lastServerTick
		if elapsed > 0 && delta > 0 {
			a.tickDuration = elapsed / time.Duration(delta)
		}
	}

	a.lastServerTick = tick
	a.lastServerTickAt = now
}

func (m *Mysis) estimateTravelUntil(now time.Time, arrivalTick, currentTick int64, currentTickOK bool) time.Time {
	a := m
	if currentTickOK {
		a.mu.RLock()
		tickDuration := a.tickDuration
		a.mu.RUnlock()
		if tickDuration > 0 && arrivalTick > currentTick {
			return now.Add(time.Duration(arrivalTick-currentTick) * tickDuration)
		}
	}

	return now.Add(constants.WaitStateNudgeInterval)
}

func (m *Mysis) estimateCooldownUntil(now time.Time, cooldownTicks int64) time.Time {
	a := m
	a.mu.RLock()
	tickDuration := a.tickDuration
	a.mu.RUnlock()

	if tickDuration > 0 && cooldownTicks > 0 {
		return now.Add(time.Duration(cooldownTicks) * tickDuration)
	}

	return now.Add(constants.WaitStateNudgeInterval)
}

func parseToolResultPayload(result *mcp.ToolResult) (interface{}, bool) {
	if result == nil {
		return nil, false
	}

	var texts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			texts = append(texts, block.Text)
		}
	}

	content := strings.TrimSpace(strings.Join(texts, "\n"))
	if content == "" {
		return nil, false
	}
	if !strings.HasPrefix(content, "{") && !strings.HasPrefix(content, "[") {
		return nil, false
	}

	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	var payload interface{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, false
	}

	return payload, true
}

func findIntField(payload interface{}, keys ...string) (int64, bool) {
	queue := []interface{}{payload}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		switch value := current.(type) {
		case map[string]interface{}:
			for _, key := range keys {
				if raw, ok := value[key]; ok {
					if number, ok := normalizeInt(raw); ok {
						return number, true
					}
				}
			}

			childKeys := make([]string, 0, len(value))
			for key := range value {
				childKeys = append(childKeys, key)
			}
			sort.Strings(childKeys)
			for _, key := range childKeys {
				queue = append(queue, value[key])
			}
		case []interface{}:
			queue = append(queue, value...)
		}
	}

	return 0, false
}

func findFloatField(payload interface{}, keys ...string) (float64, bool) {
	queue := []interface{}{payload}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		switch value := current.(type) {
		case map[string]interface{}:
			for _, key := range keys {
				if raw, ok := value[key]; ok {
					if number, ok := normalizeFloat(raw); ok {
						return number, true
					}
				}
			}

			childKeys := make([]string, 0, len(value))
			for key := range value {
				childKeys = append(childKeys, key)
			}
			sort.Strings(childKeys)
			for _, key := range childKeys {
				queue = append(queue, value[key])
			}
		case []interface{}:
			queue = append(queue, value...)
		}
	}

	return 0, false
}

func findCurrentTick(payload interface{}) (int64, bool) {
	if number, ok := findIntFieldAtKeys(payload, "current_tick"); ok {
		return number, true
	}
	if number, ok := findIntFieldInWrappers(payload, "current_tick"); ok {
		return number, true
	}
	if number, ok := findIntField(payload, "current_tick"); ok {
		return number, true
	}
	if number, ok := findIntFieldAtKeys(payload, "tick"); ok {
		return number, true
	}
	if number, ok := findIntFieldInWrappers(payload, "tick"); ok {
		return number, true
	}
	if number, ok := findIntField(payload, "tick"); ok {
		return number, true
	}
	return 0, false
}

func findIntFieldAtKeys(payload interface{}, keys ...string) (int64, bool) {
	value, ok := payload.(map[string]interface{})
	if !ok {
		return 0, false
	}

	for _, key := range keys {
		if raw, ok := value[key]; ok {
			if number, ok := normalizeInt(raw); ok {
				return number, true
			}
		}
	}

	return 0, false
}

func findIntFieldInWrappers(payload interface{}, key string) (int64, bool) {
	value, ok := payload.(map[string]interface{})
	if !ok {
		return 0, false
	}

	wrappers := []string{"data", "result", "payload", "response"}
	for _, wrapper := range wrappers {
		child, ok := value[wrapper]
		if !ok {
			continue
		}
		if number, ok := findIntField(child, key); ok {
			return number, true
		}
	}

	return 0, false
}

func normalizeInt(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		number, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return number, true
	case string:
		number, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

func normalizeFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		number, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return number, true
	case string:
		number, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

func (m *Mysis) emitStateChange(oldState, newState MysisState) {
	a := m
	a.publishCriticalEvent(Event{
		Type:      EventMysisStateChanged,
		MysisID:   a.id,
		MysisName: a.name,
		State: &StateChangeData{
			OldState: oldState,
			NewState: newState,
		},
		Timestamp: time.Now(),
	})
}

func (m *Mysis) setErrorState(err error) {
	a := m
	a.mu.Lock()
	oldState := a.state

	// If mysis was intentionally stopped, don't override with error state
	if oldState == MysisStateStopped {
		a.mu.Unlock()
		log.Debug().Str("mysis", a.name).Err(err).Msg("Ignoring error - mysis was intentionally stopped")
		return
	}

	a.state = MysisStateErrored
	a.lastError = err
	a.mu.Unlock()

	if err := a.store.UpdateMysisState(a.id, store.MysisStateErrored); err != nil {
		log.Warn().
			Err(err).
			Str("mysis_id", a.id).
			Str("mysis_name", a.name).
			Msg("failed to update mysis state to errored")
	}
	a.emitStateChange(oldState, MysisStateErrored)

	// Release account (if any) to allow restart
	a.releaseCurrentAccount()

	a.publishCriticalEvent(Event{
		Type:      EventMysisError,
		MysisID:   a.id,
		MysisName: a.name,
		Error:     &ErrorData{Error: err.Error()},
		Timestamp: time.Now(),
	})
}

func (m *Mysis) publishCriticalEvent(event Event) {
	a := m
	if a.bus.PublishBlocking(event, constants.EventBusPublishTimeout) {
		return
	}

	log.Warn().
		Str("event_type", string(event.Type)).
		Str("mysis_id", a.id).
		Str("mysis_name", a.name).
		Msg("event bus publish timeout")
}

// collectValidToolCallIDs extracts all tool call IDs from assistant messages.
// Used to validate that tool result messages reference existing tool calls.
func (m *Mysis) collectValidToolCallIDs(memories []*store.Memory) map[string]bool {
	validToolCalls := make(map[string]bool)

	for _, mem := range memories {
		if mem.Role == store.MemoryRoleAssistant &&
			strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			calls := m.parseStoredToolCalls(mem.Content)
			for _, call := range calls {
				validToolCalls[call.ID] = true
			}
		}
	}

	return validToolCalls
}

// removeOrphanedToolMessages removes tool result messages that don't have
// a corresponding assistant tool call message. This can happen due to:
// 1. Context window cutting off tool calls but keeping results
// 2. Context compaction removing tool call messages
//
// OpenAI Chat Completions API requires tool results to reference valid tool calls.
func (m *Mysis) removeOrphanedToolMessages(memories []*store.Memory) []*store.Memory {
	validToolCalls := m.collectValidToolCallIDs(memories)

	result := make([]*store.Memory, 0, len(memories))
	for _, mem := range memories {
		// Check if this is a tool result message
		if mem.Role == store.MemoryRoleTool {
			idx := strings.Index(mem.Content, constants.ToolCallStorageFieldDelimiter)
			if idx > 0 {
				toolCallID := mem.Content[:idx]
				if !validToolCalls[toolCallID] {
					log.Debug().
						Str("tool_call_id", toolCallID).
						Msg("Removing orphaned tool result - no matching tool call")
					continue // Skip orphaned result
				}
			} else {
				// Malformed tool result - skip it
				log.Warn().
					Str("content", mem.Content).
					Msg("Skipping malformed tool result")
				continue
			}
		}
		result = append(result, mem)
	}

	if len(result) < len(memories) {
		log.Debug().
			Int("removed", len(memories)-len(result)).
			Int("original", len(memories)).
			Int("final", len(result)).
			Msg("Removed orphaned tool messages for OpenAI compliance")
	}

	return result
}
