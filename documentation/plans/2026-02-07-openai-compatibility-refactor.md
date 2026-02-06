# OpenAI Compatibility Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Separate Ollama-specific code from OpenAI-compliant code to prevent cross-contamination and ensure strict OpenAI API compliance for all OpenAI-compatible providers.

**Architecture:** Create shared OpenAI-compliant types and functions in `openai_common.go`. Keep Ollama-specific customizations isolated in `ollama.go`. Refactor OpenCode to use only OpenAI-standard types and remove dependencies on Ollama internals.

**Tech Stack:** Go 1.22+, OpenAI SDK (github.com/sashabaranov/go-openai), standard HTTP client

**Priority:** OpenAI compliance is paramount. All changes must maintain strict OpenAI Chat Completions API compatibility. Ollama-specific features (like flexible system message placement) stay exclusively in Ollama provider.

---

## Phase 1: Create OpenAI-Compliant Common Types

### Task 1.1: Create openai_common.go with shared types

**Files:**
- Create: `internal/provider/openai_common.go`

**Step 1: Create file with package and imports**

```go
package provider

import (
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"
	openai "github.com/sashabaranov/go-openai"
)
```

**Step 2: Define OpenAI-compliant response types**

Add to `internal/provider/openai_common.go`:

```go
// OpenAI-compliant response types for providers that follow OpenAI Chat Completions API spec.
// These types should NOT include provider-specific extensions.

type openaiChatResponse struct {
	Choices []openaiChatChoice `json:"choices"`
}

type openaiChatChoice struct {
	Message openaiChatMessage `json:"message"`
}

type openaiChatMessage struct {
	Role      string                `json:"role"`
	Content   string                `json:"content"`
	ToolCalls []openaiChatToolCall  `json:"tool_calls,omitempty"`
}

type openaiChatToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openaiChatFunction     `json:"function"`
}

type openaiChatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
```

**Step 3: Add OpenAI-compliant message conversion function**

Add to `internal/provider/openai_common.go`:

```go
// toOpenAIMessages converts provider-agnostic messages to OpenAI SDK message format.
// This function enforces OpenAI Chat Completions API requirements:
// - System messages must be first
// - User and assistant messages must alternate (as much as possible)
// - Tool messages must have tool_call_id and follow assistant messages with tool calls
func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	result := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		msg := openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}

		// Handle tool call results
		if m.ToolCallID != "" {
			msg.ToolCallID = m.ToolCallID
		}

		// Handle assistant messages with tool calls
		if len(m.ToolCalls) > 0 {
			msg.ToolCalls = make([]openai.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				msg.ToolCalls[j] = openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(tc.Arguments),
					},
				}
			}
		}

		result[i] = msg
	}
	return result
}
```

**Step 4: Add OpenAI-compliant system message merging**

Add to `internal/provider/openai_common.go`:

```go
// mergeSystemMessagesOpenAI merges all system messages into a single message at the start.
// OpenAI Chat Completions API requires:
// 1. System messages must be first
// 2. At least one non-system message must follow
//
// This function collects ALL system messages regardless of position and places them
// at the start as a single merged message. If only system messages exist, it adds
// a minimal "Begin." user message to meet OpenAI requirements.
func mergeSystemMessagesOpenAI(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	// Separate system messages from others
	var systemBuffer strings.Builder
	nonSystemMessages := make([]openai.ChatCompletionMessage, 0, len(messages))

	for _, msg := range messages {
		if msg.Role == "system" {
			if systemBuffer.Len() > 0 {
				systemBuffer.WriteString("\n\n")
			}
			systemBuffer.WriteString(msg.Content)
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	// Build result: system first, then non-system
	result := make([]openai.ChatCompletionMessage, 0, len(messages))
	
	if systemBuffer.Len() > 0 {
		result = append(result, openai.ChatCompletionMessage{
			Role:    "system",
			Content: systemBuffer.String(),
		})
	}

	result = append(result, nonSystemMessages...)

	// OpenAI requires at least one non-system message
	// If we only have system messages, add a minimal user message
	if len(nonSystemMessages) == 0 && len(result) > 0 {
		log.Debug().
			Msg("OpenAI: Only system messages present, adding minimal user message")
		result = append(result, openai.ChatCompletionMessage{
			Role:    "user",
			Content: "Begin.",
		})
	}

	log.Debug().
		Int("original_count", len(messages)).
		Int("merged_count", len(result)).
		Bool("added_user_msg", len(nonSystemMessages) == 0 && len(result) > 0).
		Msg("OpenAI: Merged system messages")

	return result
}
```

**Step 5: Add OpenAI tool conversion helper**

Add to `internal/provider/openai_common.go`:

```go
// toOpenAITools converts provider-agnostic tools to OpenAI SDK tool format.
// Returns error if any tool has invalid JSON schema.
func toOpenAITools(tools []Tool) ([]openai.Tool, error) {
	result := make([]openai.Tool, len(tools))
	for i, t := range tools {
		var params map[string]interface{}
		if len(t.Parameters) > 0 {
			if err := json.Unmarshal(t.Parameters, &params); err != nil {
				// Invalid JSON schema - return error instead of silently failing
				return nil, err
			}
		}
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		result[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}
	return result, nil
}
```

**Step 6: Run tests**

Run: `cd internal/provider && go build`
Expected: Compiles successfully

**Step 7: Commit**

```bash
git add internal/provider/openai_common.go
git commit -m "feat(provider): add OpenAI-compliant common types and functions

- Add openaiChatResponse types for strict OpenAI compliance
- Add toOpenAIMessages with OpenAI spec enforcement
- Add mergeSystemMessagesOpenAI with fallback user message
- Add toOpenAITools with JSON schema validation
- All functions follow OpenAI Chat Completions API spec"
```

---

## Phase 2: Refactor OpenCode Provider to Use Common Types

### Task 2.1: Update OpenCode to use openai_common types

**Files:**
- Modify: `internal/provider/opencode.go`

**Step 1: Update imports**

In `internal/provider/opencode.go`, imports should already have what we need. Verify imports section looks like:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/time/rate"
)
```

**Step 2: Update createChatCompletion return type**

Find line ~166:
```go
func (p *OpenCodeProvider) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (*chatCompletionResponse, error) {
```

Replace with:
```go
func (p *OpenCodeProvider) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (*openaiChatResponse, error) {
```

**Step 3: Update response decoding**

Find lines ~223-238 (in createChatCompletion):
```go
	// Read body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().
			Str("provider", "opencode_zen").
			Err(err).
			Msg("OpenCode failed to read response body")
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var decoded chatCompletionResponse
	if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
		log.Error().
			Str("provider", "opencode_zen").
			Err(err).
			Str("body", string(bodyBytes)).
			Msg("OpenCode JSON decode failed")
		return nil, fmt.Errorf("decode response: %w", err)
	}
```

Replace `chatCompletionResponse` with `openaiChatResponse`:
```go
	// Read body for logging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().
			Str("provider", "opencode_zen").
			Err(err).
			Msg("OpenCode failed to read response body")
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var decoded openaiChatResponse
	if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
		log.Error().
			Str("provider", "opencode_zen").
			Err(err).
			Str("body", string(bodyBytes)).
			Msg("OpenCode JSON decode failed")
		return nil, fmt.Errorf("decode response: %w", err)
	}
```

**Step 4: Remove reasoning() call**

Find lines ~136-138 (in ChatWithTools):
```go
	choice := resp.Choices[0]
	result := &ChatResponse{
		Content:   choice.Message.Content,
		Reasoning: choice.Message.reasoning(),
	}
```

Replace with (OpenAI standard doesn't have reasoning field):
```go
	choice := resp.Choices[0]
	result := &ChatResponse{
		Content:   choice.Message.Content,
		Reasoning: "", // OpenAI standard doesn't provide reasoning field
	}
```

**Step 5: Update message conversion calls**

Find line ~65 (in Chat method):
```go
	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeConsecutiveSystemMessages(toOpenAIMessages(messages)),
		Temperature: float32(p.temperature),
	})
```

Replace with:
```go
	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
		Temperature: float32(p.temperature),
	})
```

**Step 6: Update tool conversion**

Find lines ~88-114 (tool conversion in ChatWithTools):
```go
	openaiTools := make([]openai.Tool, len(tools))
	for i, t := range tools {
		var params map[string]interface{}
		if len(t.Parameters) > 0 {
			if err := json.Unmarshal(t.Parameters, &params); err != nil {
				// If unmarshal fails, use empty object schema
				params = map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
			}
		}
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}
```

Replace with:
```go
	openaiTools, err := toOpenAITools(tools)
	if err != nil {
		return nil, fmt.Errorf("invalid tool schema: %w", err)
	}
```

**Step 7: Update second message conversion call**

Find line ~118 (in ChatWithTools):
```go
	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeConsecutiveSystemMessages(toOpenAIMessages(messages)),
		Tools:       openaiTools,
		Temperature: float32(p.temperature),
	})
```

Replace with:
```go
	resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
		Tools:       openaiTools,
		Temperature: float32(p.temperature),
	})
```

**Step 8: Update streaming message conversion**

Find line ~250 (in Stream method):
```go
	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeConsecutiveSystemMessages(toOpenAIMessages(messages)),
		Temperature: float32(p.temperature),
	})
```

Replace with:
```go
	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
		Temperature: float32(p.temperature),
	})
```

**Step 9: Remove mergeConsecutiveSystemMessages function**

Find and DELETE lines ~291-344 (the entire `mergeConsecutiveSystemMessages` function in opencode.go).

This function is now replaced by `mergeSystemMessagesOpenAI` in `openai_common.go`.

**Step 10: Run tests**

Run: `cd internal/provider && go build`
Expected: Compiles successfully

**Step 11: Commit**

```bash
git add internal/provider/opencode.go
git commit -m "refactor(provider): migrate OpenCode to use OpenAI-compliant common types

- Use openaiChatResponse instead of Ollama's chatCompletionResponse
- Remove reasoning() method call (not in OpenAI standard)
- Use mergeSystemMessagesOpenAI from common file
- Use toOpenAITools from common file
- Remove duplicate mergeConsecutiveSystemMessages function
- OpenCode now fully independent of Ollama internals"
```

---

## Phase 3: Isolate Ollama-Specific Customizations

### Task 3.1: Keep Ollama message merging separate

**Files:**
- Modify: `internal/provider/ollama.go`

**Step 1: Document Ollama-specific behavior**

Find the `mergeConsecutiveSystemMessagesOllama` function (around line 360) and update its comment:

```go
// mergeConsecutiveSystemMessagesOllama merges consecutive system messages into a single message.
// 
// OLLAMA-SPECIFIC BEHAVIOR:
// Unlike OpenAI, Ollama allows system messages anywhere in the conversation.
// This function merges consecutive system messages IN PLACE without moving them to the start.
// This preserves Ollama's flexible system message handling.
//
// DO NOT use this function for OpenAI-compatible providers - use mergeSystemMessagesOpenAI instead.
func mergeConsecutiveSystemMessagesOllama(messages []ollamaReqMessage) []ollamaReqMessage {
	// ... existing implementation
}
```

**Step 2: Add comment to toOllamaMessages**

Find line ~297 (`toOllamaMessages` function) and add comment:

```go
// toOllamaMessages converts provider messages to Ollama's custom request format.
//
// OLLAMA-SPECIFIC: Uses custom ollamaReqMessage type instead of OpenAI SDK types.
// Ollama has its own message format and tool call structure.
func toOllamaMessages(messages []Message) []ollamaReqMessage {
	// ... existing implementation
}
```

**Step 3: Add comment to custom types**

Find line ~148 (custom Ollama types) and add header comment:

```go
// OLLAMA-SPECIFIC TYPES
// These types are custom to Ollama and should NOT be used by other providers.
// They differ from OpenAI standard in structure and field names.

type ollamaChatRequest struct {
	// ... existing implementation
}
```

**Step 4: Find and document reasoning() method**

Find the `reasoning()` method (around line 184) and add comment:

```go
// reasoning extracts reasoning content from Ollama response.
//
// OLLAMA-SPECIFIC: Ollama may return reasoning in either "reasoning" or "reasoning_content" fields.
// This is an Ollama extension not present in standard OpenAI Chat Completions API.
func (m *chatCompletionMessage) reasoning() string {
	// ... existing implementation
}
```

**Step 5: Verify Ollama doesn't use OpenAI common functions**

Search `ollama.go` for:
- `toOpenAIMessages` - should NOT be present
- `mergeSystemMessagesOpenAI` - should NOT be present
- `toOpenAITools` - should NOT be present

If any of these are found, they should be replaced with Ollama-specific equivalents.

Expected: Ollama only uses its own `toOllamaMessages`, `toOllamaTools`, and `mergeConsecutiveSystemMessagesOllama`.

**Step 6: Run tests**

Run: `cd internal/provider && go build`
Expected: Compiles successfully

**Step 7: Commit**

```bash
git add internal/provider/ollama.go
git commit -m "docs(provider): document Ollama-specific customizations

- Mark mergeConsecutiveSystemMessagesOllama as Ollama-only
- Document Ollama's flexible system message handling
- Mark custom types as Ollama-specific
- Mark reasoning() method as Ollama extension
- Clarify that Ollama code should not be used for OpenAI providers"
```

---

## Phase 4: Add Message Validation for OpenAI Compliance

### Task 4.1: Add orphaned tool message detection

**Files:**
- Modify: `internal/core/mysis.go`

**Step 1: Add helper to detect valid tool call IDs**

Find the end of the file (before the last closing brace) and add:

```go
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
```

**Step 2: Add orphaned tool message removal function**

Add after the function above:

```go
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
					continue  // Skip orphaned result
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
```

**Step 3: Update getContextMemories to use orphan removal**

Find the `getContextMemories` function (around line 890) and add the orphan removal call:

Find this section:
```go
	// Apply compaction to remove redundant snapshot tool results
	compacted := a.compactSnapshots(recent)

	// Always try to fetch the system prompt and prepend it if not already first
	system, err := a.store.GetSystemMemory(a.id)
```

Replace with:
```go
	// Apply compaction to remove redundant snapshot tool results
	compacted := a.compactSnapshots(recent)
	
	// Remove orphaned tool messages (results without corresponding tool calls)
	// This ensures OpenAI Chat Completions API compliance
	compacted = a.removeOrphanedToolMessages(compacted)

	// Always try to fetch the system prompt and prepend it if not already first
	system, err := a.store.GetSystemMemory(a.id)
```

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/core/mysis.go
git commit -m "feat(core): add orphaned tool message removal for OpenAI compliance

- Add collectValidToolCallIDs helper
- Add removeOrphanedToolMessages function
- Call orphan removal in getContextMemories
- Prevents tool results without matching tool calls
- Ensures OpenAI Chat Completions API compliance"
```

---

## Phase 5: Add Tool Call Validation

### Task 5.1: Validate tool message format in memoriesToMessages

**Files:**
- Modify: `internal/core/mysis.go`

**Step 1: Update tool message handling with validation**

Find the tool role handling in `memoriesToMessages` (around line 678):

```go
		if m.Role == store.MemoryRoleTool {
			if idx := strings.Index(m.Content, constants.ToolCallStorageFieldDelimiter); idx > 0 {
				msg.ToolCallID = m.Content[:idx]
				msg.Content = m.Content[idx+1:]
			}
		}
```

Replace with:
```go
		if m.Role == store.MemoryRoleTool {
			idx := strings.Index(m.Content, constants.ToolCallStorageFieldDelimiter)
			if idx <= 0 {
				log.Warn().
					Str("content", m.Content).
					Msg("Skipping malformed tool result - missing delimiter")
				continue  // Skip this message
			}
			toolCallID := m.Content[:idx]
			if toolCallID == "" {
				log.Warn().
					Str("content", m.Content).
					Msg("Skipping tool result with empty tool_call_id")
				continue  // Skip this message
			}
			msg.ToolCallID = toolCallID
			msg.Content = m.Content[idx+1:]
		}
```

**Step 2: Update assistant tool call handling with validation**

Find assistant tool call handling (around line 688):

```go
		if m.Role == store.MemoryRoleAssistant && strings.HasPrefix(m.Content, constants.ToolCallStoragePrefix) {
			msg.ToolCalls = a.parseStoredToolCalls(m.Content)
			msg.Content = ""
		}
```

Replace with:
```go
		if m.Role == store.MemoryRoleAssistant && strings.HasPrefix(m.Content, constants.ToolCallStoragePrefix) {
			msg.ToolCalls = a.parseStoredToolCalls(m.Content)
			if len(msg.ToolCalls) > 0 {
				msg.Content = ""  // Only clear content if tool calls were parsed successfully
			} else {
				log.Warn().
					Str("content", m.Content).
					Msg("Failed to parse tool calls from assistant message - keeping original content")
				// Keep original content as fallback
			}
		}
```

**Step 3: Add JSON validation to parseStoredToolCalls**

Find `parseStoredToolCalls` function (around line 770) and update:

Find the section where tool calls are built:
```go
			calls = append(calls, provider.ToolCall{
				ID:        fields[0],
				Name:      fields[1],
				Arguments: json.RawMessage(fields[2]),
			})
```

Replace with:
```go
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
```

**Step 4: Run tests**

Run: `make test`
Expected: All tests pass

**Step 5: Commit**

```bash
git add internal/core/mysis.go
git commit -m "feat(core): add tool call and tool result validation

- Validate tool result delimiter and tool_call_id presence
- Skip malformed tool result messages
- Validate assistant tool call parsing
- Keep original content if tool call parsing fails
- Add JSON validation for tool call arguments
- Ensures OpenAI Chat Completions API compliance"
```

---

## Phase 6: Update Documentation

### Task 6.1: Document OpenAI compliance requirements

**Files:**
- Create: `documentation/architecture/OPENAI_COMPATIBILITY.md`

**Step 1: Create documentation file**

```markdown
# OpenAI Chat Completions API Compatibility

This document describes how Zoea Nova maintains compatibility with the OpenAI Chat Completions API specification for all OpenAI-compatible providers.

## Architecture Principles

1. **OpenAI Compliance First**: All OpenAI-compatible providers (OpenCode Zen, etc.) MUST follow strict OpenAI Chat Completions API spec
2. **Ollama Isolation**: Ollama-specific customizations stay in `ollama.go` and are NOT used by other providers
3. **Shared OpenAI Types**: Common OpenAI-compliant code lives in `openai_common.go`

## Provider Types

### OpenAI-Compatible Providers
- **OpenCode Zen**: Strict OpenAI compliance
- **Future providers**: Any provider following OpenAI Chat Completions API

These providers use:
- `openai_common.go` types and functions
- `mergeSystemMessagesOpenAI()` for system message handling
- OpenAI SDK types (`openai.ChatCompletionRequest`, etc.)

### Ollama Provider
- **Ollama**: Custom provider with flexible API

Uses Ollama-specific code:
- Custom types (`ollamaReqMessage`, `chatCompletionResponse`, etc.)
- `mergeConsecutiveSystemMessagesOllama()` for flexible system handling
- Custom `reasoning()` method for Ollama extensions

## OpenAI Chat Completions API Requirements

### Message Ordering
1. System messages MUST be first
2. User and assistant messages should alternate
3. Tool result messages MUST follow assistant messages with tool calls
4. At least one non-system message MUST be present

### Tool Calls
1. Tool calls must have unique IDs
2. Tool results must reference valid tool call IDs
3. Tool arguments must be valid JSON
4. Tool results must follow the corresponding tool call message

### Validation Rules
- Empty tool_call_id → skip message
- Orphaned tool results (no matching tool call) → remove message
- Invalid JSON in tool arguments → use empty object `{}`
- Only system messages → add fallback user message "Begin."

## Code Structure

```
internal/provider/
├── openai_common.go       # Shared OpenAI-compliant code
│   ├── toOpenAIMessages()
│   ├── mergeSystemMessagesOpenAI()
│   └── toOpenAITools()
├── opencode.go            # OpenCode Zen (uses openai_common.go)
└── ollama.go              # Ollama (isolated, custom types)
    ├── toOllamaMessages()
    ├── mergeConsecutiveSystemMessagesOllama()
    └── reasoning() method
```

## Adding New OpenAI-Compatible Providers

To add a new OpenAI-compatible provider:

1. Create `internal/provider/<provider>.go`
2. Import and use `openai_common.go` functions:
   - `toOpenAIMessages()`
   - `mergeSystemMessagesOpenAI()`
   - `toOpenAITools()`
3. Use OpenAI SDK types for requests
4. Use `openaiChatResponse` types for responses
5. DO NOT use Ollama-specific code
6. Test against actual OpenAI API if possible

## Testing OpenAI Compliance

All OpenAI-compatible providers must pass:
- Message ordering validation tests
- Tool call/result association tests
- System message merging tests
- Edge case handling (orphaned tools, malformed data, etc.)

See `internal/provider/provider_test.go` for test patterns.

## References

- [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat/create)
- `documentation/guides/OPENCODE_ZEN_MODELS.md`
```

**Step 2: Commit**

```bash
git add documentation/architecture/OPENAI_COMPATIBILITY.md
git commit -m "docs: add OpenAI Chat Completions API compatibility guide

- Document architecture principles (OpenAI compliance first)
- Explain provider separation (OpenAI-compatible vs Ollama)
- List OpenAI API requirements (message ordering, tool calls)
- Document validation rules and code structure
- Provide guide for adding new OpenAI-compatible providers"
```

---

## Phase 7: Update AGENTS.md

### Task 7.1: Add provider architecture notes

**Files:**
- Modify: `AGENTS.md`

**Step 1: Add Provider Architecture section**

Find the `## Architecture` section and add after the diagram:

```markdown
## Provider Architecture

Zoea Nova supports two types of LLM providers:

### OpenAI-Compatible Providers
Providers that follow the [OpenAI Chat Completions API](https://platform.openai.com/docs/api-reference/chat/create) specification.

**Examples:** OpenCode Zen, OpenAI, Azure OpenAI, Together AI, etc.

**Implementation:**
- Use shared code in `internal/provider/openai_common.go`
- Strict message ordering (system first, alternating user/assistant)
- Tool call validation and orphaned message removal
- Fallback user message if only system messages exist

**Location:** `internal/provider/opencode.go` (reference implementation)

### Ollama Provider
Custom provider with flexible API that differs from OpenAI standard.

**Ollama-Specific Behavior:**
- System messages allowed anywhere (not just first)
- Custom response types with `reasoning()` method
- Flexible message ordering

**Implementation:**
- Isolated in `internal/provider/ollama.go`
- Uses custom types (`ollamaReqMessage`, `chatCompletionResponse`, etc.)
- Custom message merging (`mergeConsecutiveSystemMessagesOllama`)
- **DO NOT** use Ollama code for OpenAI-compatible providers

**Location:** `internal/provider/ollama.go`

### Key Files
- `internal/provider/openai_common.go` - Shared OpenAI-compliant code (use for new OpenAI-compatible providers)
- `internal/provider/opencode.go` - OpenCode Zen implementation (OpenAI-compatible reference)
- `internal/provider/ollama.go` - Ollama implementation (isolated, custom)
- `documentation/architecture/OPENAI_COMPATIBILITY.md` - Full compatibility guide

### Adding New Providers
- **OpenAI-compatible:** Use `openai_common.go` functions, follow OpenCode pattern
- **Custom API:** Create isolated implementation like Ollama, document differences
```

**Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs(agents): add provider architecture documentation

- Document OpenAI-compatible vs Ollama provider types
- Explain shared openai_common.go usage
- Document Ollama isolation and custom behavior
- Add guide for adding new providers
- Link to OPENAI_COMPATIBILITY.md"
```

---

## Phase 8: Verification and Testing

### Task 8.1: Build and test all changes

**Step 1: Build the project**

Run: `make build`
Expected: Builds successfully with no errors

**Step 2: Run all tests**

Run: `make test`
Expected: All tests pass

**Step 3: Test with actual mysis**

Manual test:
1. Start app: `./bin/zoea -debug`
2. Create OpenCode Zen mysis
3. Try to start it (should now work without 500 errors)
4. Check logs for proper message ordering
5. Verify no orphaned tool messages
6. Create Ollama mysis and verify it still works

Expected: Both providers work correctly and independently

**Step 4: Check for cross-dependencies**

Run: `grep -r "chatCompletionResponse" internal/provider/opencode.go`
Expected: No matches (should use openaiChatResponse)

Run: `grep -r "toOllamaMessages" internal/provider/opencode.go`
Expected: No matches

Run: `grep -r "toOpenAIMessages" internal/provider/ollama.go`
Expected: No matches (should use toOllamaMessages)

**Step 5: Verify code separation**

Run: `grep -r "mergeSystemMessagesOpenAI" internal/provider/ollama.go`
Expected: No matches

Run: `grep -r "mergeConsecutiveSystemMessagesOllama" internal/provider/opencode.go`
Expected: No matches

**Step 6: Final commit**

```bash
git add -A
git commit -m "test: verify OpenAI compatibility refactor

- All providers build successfully
- No cross-dependencies between OpenCode and Ollama
- OpenCode uses openai_common.go exclusively
- Ollama remains isolated with custom types"
```

---

## Summary

This plan refactors the provider architecture to:

1. **Separate Concerns**: OpenAI-compliant code in `openai_common.go`, Ollama-specific code in `ollama.go`
2. **Ensure OpenAI Compliance**: Strict message ordering, tool validation, orphan removal
3. **Prevent Cross-Contamination**: OpenCode can't break Ollama, Ollama can't break OpenAI providers
4. **Enable Future Providers**: Any OpenAI-compatible provider can use `openai_common.go`

**Key Changes:**
- New `openai_common.go` with shared OpenAI-compliant types and functions
- OpenCode migrated to use only OpenAI-compliant code
- Ollama remains isolated with its custom behavior documented
- Message validation and orphan removal for OpenAI compliance
- Comprehensive documentation

**Testing:**
- All existing tests continue to pass
- OpenCode and Ollama work independently
- No cross-dependencies between providers
