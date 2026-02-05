package core

import (
	"context"
	"encoding/json"
	"fmt"
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

	state  MysisState
	ctx    context.Context
	cancel context.CancelFunc

	// turnMu ensures only one turn runs at a time.
	turnMu sync.Mutex

	// For runtime tracking
	lastError              error
	currentAccountUsername string
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
func NewMysis(id, name string, createdAt time.Time, p provider.Provider, s *store.Store, bus *EventBus) *Mysis {
	return &Mysis{
		id:        id,
		name:      name,
		createdAt: createdAt,
		provider:  p,
		store:     s,
		bus:       bus,
		state:     MysisStateIdle,
	}
}

func (a *Mysis) computeMemoryStats(memories []*store.Memory) contextStats {
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

func (a *Mysis) computeMessageStats(messages []provider.Message) contextStats {
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
func (a *Mysis) SetMCP(proxy *mcp.Proxy) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mcp = proxy
}

// ID returns the mysis unique identifier.
func (a *Mysis) ID() string {
	return a.id
}

// Name returns the mysis display name.
func (a *Mysis) Name() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.name
}

// CreatedAt returns when the mysis was created.
func (a *Mysis) CreatedAt() time.Time {
	return a.createdAt
}

// State returns the mysis current state.
func (a *Mysis) State() MysisState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// ProviderName returns the name of the mysis provider.
func (a *Mysis) ProviderName() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.provider == nil {
		return ""
	}
	return a.provider.Name()
}

// LastError returns the last error encountered by the mysis.
func (a *Mysis) LastError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastError
}

// CurrentAccountUsername returns the username of the account this mysis is using.
func (a *Mysis) CurrentAccountUsername() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentAccountUsername
}

// SetProvider updates the mysis provider.
func (a *Mysis) SetProvider(p provider.Provider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.provider = p
}

// Start begins the mysis processing loop.
func (a *Mysis) Start() error {
	a.mu.Lock()
	if a.state == MysisStateRunning {
		a.mu.Unlock()
		return fmt.Errorf("mysis already running")
	}

	oldState := a.state
	a.state = MysisStateRunning
	a.lastError = nil

	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	a.cancel = cancel
	a.mu.Unlock()

	// Update store
	if err := a.store.UpdateMysisState(a.id, store.MysisStateRunning); err != nil {
		a.mu.Lock()
		a.state = MysisStateErrored
		a.lastError = err
		a.mu.Unlock()
		return err
	}

	// Add system prompt if this is the first time starting (no memories yet)
	count, err := a.store.CountMemories(a.id)
	if err == nil && count == 0 {
		a.store.AddMemory(a.id, store.MemoryRoleSystem, store.MemorySourceSystem, constants.SystemPrompt, "", "")
	}

	// Emit state change event
	a.emitStateChange(oldState, MysisStateRunning)

	// Start the processing goroutine
	go a.run(ctx)

	// Trigger initial turn to encourage autonomy
	go a.SendMessage(constants.ContinuePrompt, store.MemorySourceSystem)

	return nil
}

// Stop halts the mysis processing loop.
func (a *Mysis) Stop() error {
	a.mu.Lock()
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		return fmt.Errorf("mysis not running")
	}

	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()

	// Wait for current turn to finish
	a.turnMu.Lock()
	defer a.turnMu.Unlock()

	a.mu.Lock()
	a.cancel = nil
	a.ctx = nil

	oldState := a.state
	a.state = MysisStateStopped
	a.mu.Unlock()

	// Update store
	if err := a.store.UpdateMysisState(a.id, store.MysisStateStopped); err != nil {
		return err
	}

	// Emit state change event
	a.emitStateChange(oldState, MysisStateStopped)
	a.releaseCurrentAccount()

	return nil
}

// SendMessageFrom sends a message to the mysis for processing with sender tracking.
// The source parameter indicates whether this is a direct or broadcast message.
func (a *Mysis) SendMessageFrom(content string, source store.MemorySource, senderID string) error {
	a.turnMu.Lock()
	defer a.turnMu.Unlock()

	a.mu.RLock()
	state := a.state
	p := a.provider
	mcpProxy := a.mcp
	a.mu.RUnlock()

	if state != MysisStateRunning {
		return fmt.Errorf("mysis not running")
	}

	// Determine role based on source
	role := store.MemoryRoleUser
	if source == store.MemorySourceSystem {
		role = store.MemoryRoleSystem
	}

	// Store the message
	if err := a.store.AddMemory(a.id, role, source, content, "", senderID); err != nil {
		return fmt.Errorf("store message: %w", err)
	}

	// Emit message event
	a.bus.Publish(Event{
		Type:      EventMysisMessage,
		MysisID:   a.id,
		MysisName: a.name,
		Data:      MessageData{Role: string(role), Content: content},
		Timestamp: time.Now(),
	})

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
			a.bus.Publish(Event{
				Type:      EventMysisError,
				MysisID:   a.id,
				MysisName: a.name,
				Data:      ErrorData{Error: fmt.Sprintf("Failed to load tools: %v", err)},
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
					Data:      MessageData{Role: "system", Content: fmt.Sprintf("Tools available: %s", strings.Join(toolNames, ", "))},
					Timestamp: time.Now(),
				})
			}
		}
	} else {
		a.bus.Publish(Event{
			Type:      EventMysisError,
			MysisID:   a.id,
			MysisName: a.name,
			Data:      ErrorData{Error: "MCP proxy not configured - no tools available"},
			Timestamp: time.Now(),
		})
	}

	// Loop: keep calling LLM until we get a final text response
	for iteration := 0; iteration < constants.MaxToolIterations; iteration++ {
		// Get recent conversation history (keeps context small for faster inference)
		memories, err := a.getContextMemories()
		if err != nil {
			a.setError(err)
			return fmt.Errorf("get memories: %w", err)
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

		if err != nil {
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
				Data:      MessageData{Role: "assistant", Content: fmt.Sprintf("Calling tools: %s", strings.Join(toolNames, ", "))},
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

				result, execErr := a.executeToolCall(ctx, mcpProxy, tc)

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
					Data:      MessageData{Role: "tool", Content: fmt.Sprintf("[%s] %s", tc.Name, a.formatToolResultDisplay(result, execErr))},
					Timestamp: time.Now(),
				})
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
			Data:      MessageData{Role: "assistant", Content: finalResponse},
			Timestamp: time.Now(),
		})

		return nil
	}

	// Signal network idle on max iterations
	a.bus.Publish(Event{Type: EventNetworkIdle, MysisID: a.id, Timestamp: time.Now()})

	return fmt.Errorf("max tool iterations (%d) exceeded", constants.MaxToolIterations)
}

// SendMessage sends a message to the mysis for processing.
func (a *Mysis) SendMessage(content string, source store.MemorySource) error {
	return a.SendMessageFrom(content, source, "")
}

// memoriesToMessages converts stored memories to provider messages.
func (a *Mysis) memoriesToMessages(memories []*store.Memory) []provider.Message {
	messages := make([]provider.Message, 0, len(memories))

	for _, m := range memories {
		msg := provider.Message{
			Role:    string(m.Role),
			Content: m.Content,
		}

		// Handle tool role - needs ToolCallID
		if m.Role == store.MemoryRoleTool {
			// Extract tool call ID from stored format: "tool_call_id:content"
			if idx := strings.Index(m.Content, constants.ToolCallStorageFieldDelimiter); idx > 0 {
				msg.ToolCallID = m.Content[:idx]
				msg.Content = m.Content[idx+1:]
			}
		}

		// Handle assistant messages with tool calls
		if m.Role == store.MemoryRoleAssistant && strings.HasPrefix(m.Content, constants.ToolCallStoragePrefix) {
			// Parse tool calls from stored format
			msg.Content = ""
			msg.ToolCalls = a.parseStoredToolCalls(m.Content)
		}

		messages = append(messages, msg)
	}

	return messages
}

// executeToolCall executes a single tool call via MCP proxy.
func (a *Mysis) executeToolCall(ctx context.Context, mcpProxy *mcp.Proxy, tc provider.ToolCall) (*mcp.ToolResult, error) {
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

func (a *Mysis) setCurrentAccount(username string) {
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

func (a *Mysis) releaseCurrentAccount() {
	a.mu.Lock()
	username := a.currentAccountUsername
	a.currentAccountUsername = ""
	a.mu.Unlock()

	if username != "" {
		_ = a.store.ReleaseAccount(username)
	}
}

// formatToolCallsForStorage formats tool calls for storage in memory.
func (a *Mysis) formatToolCallsForStorage(calls []provider.ToolCall) string {
	var parts []string
	for _, tc := range calls {
		parts = append(parts, fmt.Sprintf("%s%s%s%s%s", tc.ID, constants.ToolCallStorageFieldDelimiter, tc.Name, constants.ToolCallStorageFieldDelimiter, string(tc.Arguments)))
	}
	return constants.ToolCallStoragePrefix + strings.Join(parts, constants.ToolCallStorageRecordDelimiter)
}

// parseStoredToolCalls parses tool calls from stored format.
func (a *Mysis) parseStoredToolCalls(stored string) []provider.ToolCall {
	stored = strings.TrimPrefix(stored, constants.ToolCallStoragePrefix)
	if stored == "" {
		return nil
	}

	var calls []provider.ToolCall
	parts := strings.Split(stored, constants.ToolCallStorageRecordDelimiter)
	for _, part := range parts {
		fields := strings.SplitN(part, constants.ToolCallStorageFieldDelimiter, constants.ToolCallStorageFieldCount)
		if len(fields) >= constants.ToolCallStorageFieldCount {
			calls = append(calls, provider.ToolCall{
				ID:        fields[0],
				Name:      fields[1],
				Arguments: json.RawMessage(fields[2]),
			})
		}
	}
	return calls
}

// formatToolResult formats a tool result for storage (includes ID for LLM context).
func (a *Mysis) formatToolResult(toolCallID, toolName string, result *mcp.ToolResult, err error) string {
	if err != nil {
		return fmt.Sprintf("%s:Error calling %s: %v. Check the tool's required parameters and try again.", toolCallID, toolName, err)
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

// formatToolResultDisplay formats a tool result for UI display (human-readable).
func (a *Mysis) formatToolResultDisplay(result *mcp.ToolResult, err error) string {
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

// setError sets the mysis last error and emits an error event.
func (a *Mysis) setError(err error) {
	a.mu.Lock()
	a.lastError = err
	a.mu.Unlock()

	a.bus.Publish(Event{
		Type:      EventMysisError,
		MysisID:   a.id,
		MysisName: a.name,
		Data:      ErrorData{Error: err.Error()},
		Timestamp: time.Now(),
	})
}

// getContextMemories returns memories for LLM context: system prompt + recent messages.
// This keeps context small for faster inference while preserving essential information.
// Compacts repeated snapshot tool results to prefer recent state.
func (a *Mysis) getContextMemories() ([]*store.Memory, error) {
	// Get recent memories (limited for performance)
	recent, err := a.store.GetRecentMemories(a.id, constants.MaxContextMessages)
	if err != nil {
		return nil, err
	}

	// Apply compaction to remove redundant snapshot tool results
	compacted := a.compactSnapshots(recent)

	// Always try to fetch the system prompt and prepend it if not already first
	system, err := a.store.GetSystemMemory(a.id)
	if err != nil {
		// No system prompt found - this is okay, just use compacted memories
		return compacted, nil
	}

	// Check if system prompt is already the first message
	if len(compacted) > 0 && compacted[0].ID == system.ID {
		return compacted, nil
	}

	// Prepend system prompt to compacted memories
	result := make([]*store.Memory, 0, len(compacted)+1)
	result = append(result, system)
	result = append(result, compacted...)
	return result, nil
}

// compactSnapshots removes redundant snapshot tool results, keeping only the most recent
// result for each snapshot tool. This prevents state-heavy tools from crowding out
// conversation history while ensuring the latest state is available.
func (a *Mysis) compactSnapshots(memories []*store.Memory) []*store.Memory {
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
		if constants.SnapshotTools[toolName] {
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
		if constants.SnapshotTools[toolName] {
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

func (a *Mysis) extractToolNameFromResult(content string, toolCallNames map[string]string) string {
	idx := strings.Index(content, constants.ToolCallStorageFieldDelimiter)
	if idx <= 0 {
		return ""
	}

	callID := content[:idx]
	return toolCallNames[callID]
}

func (a *Mysis) toolCallNameIndex(memories []*store.Memory) map[string]string {
	index := make(map[string]string)
	for _, m := range memories {
		if m.Role != store.MemoryRoleAssistant {
			continue
		}
		if !strings.HasPrefix(m.Content, constants.ToolCallStoragePrefix) {
			continue
		}
		calls := a.parseStoredToolCalls(m.Content)
		for _, call := range calls {
			if call.ID == "" || call.Name == "" {
				continue
			}
			index[call.ID] = call.Name
		}
	}

	return index
}

// run is the mysis main processing loop.
func (a *Mysis) run(ctx context.Context) {
	// Ticker to nudge the mysis if it's idle
	ticker := time.NewTicker(constants.IdleNudgeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Only nudge if the mysis is running and not already in a turn
			a.mu.RLock()
			isRunning := a.state == MysisStateRunning
			a.mu.RUnlock()

			if isRunning {
				// Only nudge if not already in a turn
				if a.turnMu.TryLock() {
					a.turnMu.Unlock()
					go a.SendMessage(constants.ContinuePrompt, store.MemorySourceSystem)
				}
			}
		}
	}
}

func (a *Mysis) emitStateChange(oldState, newState MysisState) {
	a.bus.Publish(Event{
		Type:      EventMysisStateChanged,
		MysisID:   a.id,
		MysisName: a.name,
		Data: StateChangeData{
			OldState: oldState,
			NewState: newState,
		},
		Timestamp: time.Now(),
	})
}

func (a *Mysis) setErrorState(err error) {
	a.mu.Lock()
	oldState := a.state
	a.state = MysisStateErrored
	a.lastError = err
	a.mu.Unlock()

	a.store.UpdateMysisState(a.id, store.MysisStateErrored)
	a.emitStateChange(oldState, MysisStateErrored)

	a.bus.Publish(Event{
		Type:      EventMysisError,
		MysisID:   a.id,
		MysisName: a.name,
		Data:      ErrorData{Error: err.Error()},
		Timestamp: time.Now(),
	})
}
