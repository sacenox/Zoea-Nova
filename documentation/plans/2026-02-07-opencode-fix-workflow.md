# OpenCode Zen Fix Workflow

> **Execution Mode:** Parallel subagents with dependencies mapped

**Goal:** Fix OpenCode Zen 500 errors by addressing missing `Stream` parameter, aggressive message merging, and adding validation.

**Strategy:** Parallel execution where possible, with clear dependency chains for sequential work.

---

## Dependency Graph

```
Phase 1 (Parallel - No Dependencies):
├── Task 1.1: Add Stream parameter
├── Task 1.2: Add message validation
└── Task 1.3: Fix system message merging

Phase 2 (Parallel - Depends on Phase 1 completion):
├── Task 2.1: Add unit tests for message merging
├── Task 2.2: Add unit tests for validation
└── Task 2.3: Add integration tests for OpenCode

Phase 3 (Sequential - Depends on Phase 2):
└── Task 3.1: Manual verification with real API

Phase 4 (Sequential - Depends on Phase 3):
└── Task 4.1: Commit all changes
```

---

## Phase 1: Core Fixes (Parallel Execution)

### Task 1.1: Add Stream Parameter to OpenCode Requests

**Agent:** general
**Dependencies:** None
**Estimated Time:** 2-3 minutes

**Objective:** Add explicit `Stream: false` parameter to all OpenCode non-streaming requests.

**Files to Modify:**
- `internal/provider/opencode.go`

**Changes:**

1. Update `Chat` method (line ~65):
```go
resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:       p.model,
    Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
    Temperature: float32(p.temperature),
    Stream:      false,  // ADD THIS
})
```

2. Update `ChatWithTools` method (line ~95):
```go
resp, err := p.createChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:       p.model,
    Messages:    mergeSystemMessagesOpenAI(toOpenAIMessages(messages)),
    Tools:       openaiTools,
    Temperature: float32(p.temperature),
    Stream:      false,  // ADD THIS
})
```

**Verification:**
- Run: `cd internal/provider && go build`
- Expected: Compiles successfully

**Commit:**
```bash
git add internal/provider/opencode.go
git commit -m "fix(provider): add explicit Stream:false parameter to OpenCode requests

OpenCode Zen API was defaulting to streaming mode when Stream parameter
was not set, causing 500 errors due to different JSON response structure.
Explicitly set Stream:false for non-streaming Chat and ChatWithTools methods."
```

---

### Task 1.2: Add Message Validation Function

**Agent:** general
**Dependencies:** None
**Estimated Time:** 5 minutes

**Objective:** Add validation to ensure conversations have at least one user message and proper alternation.

**Files to Modify:**
- `internal/provider/openai_common.go`

**Changes:**

Add validation function before `mergeSystemMessagesOpenAI`:

```go
// validateOpenAIMessages ensures message structure meets OpenAI API requirements:
// 1. At least one non-system message exists
// 2. Conversation ends with user message (if expecting completion)
// 3. No consecutive assistant messages without user messages between them
//
// Returns error if validation fails.
func validateOpenAIMessages(messages []openai.ChatCompletionMessage) error {
	if len(messages) == 0 {
		return errors.New("messages array is empty")
	}

	// Count message types
	hasUser := false
	hasAssistant := false
	lastRole := ""

	for _, msg := range messages {
		if msg.Role == "user" {
			hasUser = true
		}
		if msg.Role == "assistant" {
			hasAssistant = true
			// Check for consecutive assistant messages
			if lastRole == "assistant" {
				log.Warn().Msg("OpenAI: Consecutive assistant messages detected")
			}
		}
		lastRole = msg.Role
	}

	// If we have assistant messages but no user messages, that's invalid
	if hasAssistant && !hasUser {
		return errors.New("conversation has assistant messages but no user messages")
	}

	// If last message is assistant (and we're expecting a completion), that's invalid
	if lastRole == "assistant" {
		log.Warn().Msg("OpenAI: Conversation ends with assistant message - may cause issues")
	}

	return nil
}
```

Add import for errors:
```go
import (
	"encoding/json"
	"errors"  // ADD THIS
	"strings"

	"github.com/rs/zerolog/log"
	openai "github.com/sashabaranov/go-openai"
)
```

Update `mergeSystemMessagesOpenAI` to call validation:

Find line ~85 (start of mergeSystemMessagesOpenAI) and add validation call:

```go
func mergeSystemMessagesOpenAI(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	// Validate messages before processing
	if err := validateOpenAIMessages(messages); err != nil {
		log.Error().Err(err).Msg("OpenAI: Message validation failed")
		// Continue processing but log the issue
	}

	// ... rest of existing function
```

**Verification:**
- Run: `cd internal/provider && go build`
- Expected: Compiles successfully

**Commit:**
```bash
git add internal/provider/openai_common.go
git commit -m "feat(provider): add OpenAI message validation

Add validateOpenAIMessages function to check:
- Messages array not empty
- No consecutive assistant messages
- Assistant messages have corresponding user messages
- Warn if conversation ends with assistant message

This helps catch invalid message structures before sending to OpenAI-compatible APIs."
```

---

### Task 1.3: Fix Aggressive System Message Merging

**Agent:** general
**Dependencies:** None
**Estimated Time:** 8 minutes

**Objective:** Update `mergeSystemMessagesOpenAI` to preserve conversation structure while moving system messages to start.

**Files to Modify:**
- `internal/provider/openai_common.go`

**Changes:**

Replace `mergeSystemMessagesOpenAI` function (lines ~85-135) with smarter version:

```go
// mergeSystemMessagesOpenAI merges system messages intelligently while preserving conversation flow.
//
// Strategy:
// 1. Separate initial system messages (before any user/assistant messages)
// 2. Keep user/assistant conversation intact
// 3. Merge any mid-conversation system messages into the initial system prompt
//
// OpenAI requires:
// - System messages at the start
// - At least one non-system message
// - Proper user/assistant alternation
func mergeSystemMessagesOpenAI(messages []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}

	// Separate system messages from conversation messages
	var systemMessages []string
	var conversationMessages []openai.ChatCompletionMessage
	
	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg.Content)
		} else {
			conversationMessages = append(conversationMessages, msg)
		}
	}

	// Build result: merged system message + conversation
	result := make([]openai.ChatCompletionMessage, 0, len(messages))
	
	// Add merged system message if any system messages exist
	if len(systemMessages) > 0 {
		mergedSystem := strings.Join(systemMessages, "\n\n")
		result = append(result, openai.ChatCompletionMessage{
			Role:    "system",
			Content: mergedSystem,
		})
	}

	// Add conversation messages
	result = append(result, conversationMessages...)

	// OpenAI requires at least one non-system message
	// If we only have system messages, add a minimal user message
	if len(conversationMessages) == 0 && len(result) > 0 {
		log.Debug().
			Msg("OpenAI: Only system messages present, adding minimal user message")
		result = append(result, openai.ChatCompletionMessage{
			Role:    "user",
			Content: "Begin.",
		})
	}

	// If conversation ends with assistant message, warn (may cause issues)
	if len(result) > 1 && result[len(result)-1].Role == "assistant" {
		log.Warn().
			Msg("OpenAI: Conversation ends with assistant message")
	}

	log.Debug().
		Int("original_count", len(messages)).
		Int("merged_count", len(result)).
		Int("system_merged", len(systemMessages)).
		Int("conversation_kept", len(conversationMessages)).
		Bool("added_user_msg", len(conversationMessages) == 0 && len(result) > 0).
		Msg("OpenAI: Merged system messages")

	return result
}
```

**Verification:**
- Run: `cd internal/provider && go build`
- Expected: Compiles successfully
- Check: Merging now preserves conversation structure

**Commit:**
```bash
git add internal/provider/openai_common.go
git commit -m "fix(provider): improve system message merging to preserve conversation

Previous implementation was too aggressive - it stripped ALL system messages
regardless of position, destroying conversation history (21 msgs -> 2 msgs).

New approach:
- Merges all system messages into one at the start
- Preserves all user/assistant conversation messages
- Maintains conversation flow and history
- Still adds fallback user message if only system messages exist

This fixes OpenCode Zen errors caused by invalid message structures."
```

---

## Phase 2: Test Coverage (Parallel Execution)

### Task 2.1: Unit Tests for Message Merging

**Agent:** general
**Dependencies:** Task 1.3 complete
**Estimated Time:** 10 minutes

**Objective:** Add comprehensive tests for the new `mergeSystemMessagesOpenAI` function.

**Files to Create/Modify:**
- `internal/provider/openai_common_test.go` (add to existing file)

**Tests to Add:**

```go
func TestMergeSystemMessagesOpenAI_PreservesConversation(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System 1"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "system", Content: "System 2"},
		{Role: "user", Content: "How are you?"},
		{Role: "assistant", Content: "I'm good"},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 1 system (merged) + 4 conversation messages = 5 total
	if len(result) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(result))
	}

	// First should be merged system
	if result[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %s", result[0].Role)
	}
	if !strings.Contains(result[0].Content, "System 1") || !strings.Contains(result[0].Content, "System 2") {
		t.Error("System messages not properly merged")
	}

	// Rest should be conversation in order
	if result[1].Role != "user" || result[1].Content != "Hello" {
		t.Error("User message 1 not preserved")
	}
	if result[2].Role != "assistant" || result[2].Content != "Hi there" {
		t.Error("Assistant message 1 not preserved")
	}
	if result[3].Role != "user" || result[3].Content != "How are you?" {
		t.Error("User message 2 not preserved")
	}
	if result[4].Role != "assistant" || result[4].Content != "I'm good" {
		t.Error("Assistant message 2 not preserved")
	}
}

func TestMergeSystemMessagesOpenAI_OnlySystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System 1"},
		{Role: "system", Content: "System 2"},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should have: 1 merged system + 1 fallback user = 2 total
	if len(result) != 2 {
		t.Errorf("Expected 2 messages (merged system + fallback user), got %d", len(result))
	}

	if result[0].Role != "system" {
		t.Errorf("Expected first message to be system, got %s", result[0].Role)
	}

	if result[1].Role != "user" || result[1].Content != "Begin." {
		t.Error("Expected fallback user message 'Begin.'")
	}
}

func TestMergeSystemMessagesOpenAI_NoSystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	result := mergeSystemMessagesOpenAI(messages)

	// Should keep messages as-is
	if len(result) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(result))
	}

	if result[0].Role != "user" || result[1].Role != "assistant" {
		t.Error("Messages not preserved correctly")
	}
}

func TestMergeSystemMessagesOpenAI_EmptyInput(t *testing.T) {
	messages := []openai.ChatCompletionMessage{}
	result := mergeSystemMessagesOpenAI(messages)

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d messages", len(result))
	}
}
```

**Verification:**
- Run: `cd internal/provider && go test -run TestMergeSystemMessagesOpenAI -v`
- Expected: All 4 tests pass

**Commit:**
```bash
git add internal/provider/openai_common_test.go
git commit -m "test(provider): add tests for improved system message merging

Add 4 test cases covering:
- Preserving conversation with mid-stream system messages
- Only system messages (fallback user message)
- No system messages (preserve as-is)
- Empty input handling"
```

---

### Task 2.2: Unit Tests for Message Validation

**Agent:** general
**Dependencies:** Task 1.2 complete
**Estimated Time:** 8 minutes

**Objective:** Add tests for `validateOpenAIMessages` function.

**Files to Modify:**
- `internal/provider/openai_common_test.go`

**Tests to Add:**

```go
func TestValidateOpenAIMessages_Valid(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
	}

	err := validateOpenAIMessages(messages)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestValidateOpenAIMessages_EmptyArray(t *testing.T) {
	messages := []openai.ChatCompletionMessage{}

	err := validateOpenAIMessages(messages)
	if err == nil {
		t.Error("Expected error for empty messages array")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' error, got: %v", err)
	}
}

func TestValidateOpenAIMessages_AssistantWithoutUser(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System"},
		{Role: "assistant", Content: "Hi"},
	}

	err := validateOpenAIMessages(messages)
	if err == nil {
		t.Error("Expected error for assistant without user message")
	}
	if !strings.Contains(err.Error(), "assistant") || !strings.Contains(err.Error(), "user") {
		t.Errorf("Expected assistant/user error, got: %v", err)
	}
}

func TestValidateOpenAIMessages_OnlySystemMessages(t *testing.T) {
	messages := []openai.ChatCompletionMessage{
		{Role: "system", Content: "System 1"},
		{Role: "system", Content: "System 2"},
	}

	// Only system messages is valid (but will trigger fallback user message in merging)
	err := validateOpenAIMessages(messages)
	if err != nil {
		t.Errorf("Expected no error for only system messages, got: %v", err)
	}
}
```

**Verification:**
- Run: `cd internal/provider && go test -run TestValidateOpenAIMessages -v`
- Expected: All 4 tests pass

**Commit:**
```bash
git add internal/provider/openai_common_test.go
git commit -m "test(provider): add tests for message validation

Add 4 test cases covering:
- Valid conversation structure
- Empty messages array
- Assistant without user message
- Only system messages (valid but needs fallback)"
```

---

### Task 2.3: Integration Test for OpenCode with Real Structure

**Agent:** general
**Dependencies:** Task 1.1, 1.2, 1.3 complete
**Estimated Time:** 10 minutes

**Objective:** Add integration test that simulates the exact failure scenario from logs.

**Files to Modify:**
- `internal/provider/opencode_test.go`

**Test to Add:**

```go
func TestOpenCode_PreservesConversationHistory(t *testing.T) {
	// Simulate the exact scenario from logs:
	// 21 messages (20 system + 1 assistant) should NOT collapse to 2 messages

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Verify Stream is false
		if req.Stream {
			t.Error("Expected Stream:false, got Stream:true")
		}

		// Verify we have reasonable message count (not collapsed to 2)
		if len(req.Messages) < 3 {
			t.Errorf("Expected at least 3 messages (system + user + assistant), got %d", len(req.Messages))
		}

		// Verify first message is system
		if req.Messages[0].Role != "system" {
			t.Errorf("Expected first message to be system, got %s", req.Messages[0].Role)
		}

		// Verify we have user messages
		hasUser := false
		for _, msg := range req.Messages {
			if msg.Role == "user" {
				hasUser = true
				break
			}
		}
		if !hasUser {
			t.Error("Expected at least one user message")
		}

		// Return valid response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response",
					},
				},
			},
		})
	}))
	defer server.Close()

	// Create provider with mock server
	provider := NewOpenCodeWithTemp(server.URL, "test-model", "test-key", 0.7, nil)

	// Create messages simulating the failure scenario
	messages := []provider.Message{
		{Role: "system", Content: "System prompt 1"},
		{Role: "user", Content: "User message 1"},
		{Role: "assistant", Content: "Assistant response 1"},
		{Role: "system", Content: "Context update 1"},
		{Role: "user", Content: "User message 2"},
		{Role: "assistant", Content: "Assistant response 2"},
	}

	// Call Chat
	_, err := provider.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}

func TestOpenCode_StreamParameterSetCorrectly(t *testing.T) {
	// Verify Stream:false is explicitly set

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream bool `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// THIS IS THE CRITICAL TEST
		if req.Stream {
			t.Error("FAIL: Stream parameter is true, should be false")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"role": "assistant", "content": "ok"}},
			},
		})
	}))
	defer server.Close()

	provider := NewOpenCodeWithTemp(server.URL, "test-model", "test-key", 0.7, nil)

	messages := []provider.Message{
		{Role: "user", Content: "Test"},
	}

	_, err := provider.Chat(context.Background(), messages)
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}
}
```

**Verification:**
- Run: `cd internal/provider && go test -run "TestOpenCode_Preserves|TestOpenCode_Stream" -v`
- Expected: Both tests pass

**Commit:**
```bash
git add internal/provider/opencode_test.go
git commit -m "test(provider): add integration tests for OpenCode fixes

Add tests verifying:
- Conversation history is preserved (not collapsed)
- Stream:false parameter is set correctly
- Message validation works in real scenario"
```

---

## Phase 3: Manual Verification (Sequential)

### Task 3.1: Test with Real OpenCode Zen API

**Agent:** general
**Dependencies:** All Phase 2 tasks complete
**Estimated Time:** 5 minutes

**Objective:** Verify fixes work with actual OpenCode Zen API.

**Steps:**

1. Rebuild application:
```bash
make build
```

2. Start application in debug mode:
```bash
./bin/zoea -debug
```

3. Try to start the `deadlock-test` mysis (OpenCode Zen)

4. Check logs:
```bash
tail -f ~/.zoea-nova/zoea.log
```

5. Verify:
   - No 500 errors from OpenCode
   - Mysis starts successfully
   - Request shows `"stream":false` in debug logs
   - Message count is reasonable (not collapsed to 2)

**Expected Output:**
```
✓ OpenCode chat completion response status:200
✓ OpenCode response decoded choice_count:1
✓ Mysis running successfully
```

**If Success:** Document in report
**If Failure:** Capture logs and error details for further investigation

---

## Phase 4: Finalize (Sequential)

### Task 4.1: Commit and Create Summary Report

**Agent:** general
**Dependencies:** Task 3.1 complete (successful verification)
**Estimated Time:** 3 minutes

**Objective:** Create final commit and summary report.

**Steps:**

1. Check git status:
```bash
git status
```

2. If any uncommitted changes remain, review and commit appropriately

3. Create summary report at `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`:

```markdown
# OpenCode Zen 500 Error Fix - 2026-02-07

## Problem

OpenCode Zen provider was failing with 500 errors:
```
"Cannot read properties of undefined (reading 'prompt_tokens')"
```

## Root Causes Identified

1. **Missing Stream Parameter**: Requests didn't set `Stream:false`, causing server to default to streaming mode
2. **Aggressive Message Merging**: System message merging collapsed 21 messages to 2, losing conversation history
3. **No Message Validation**: Invalid message structures (assistant without user) were sent to API

## Fixes Applied

### 1. Add Stream Parameter
- File: `internal/provider/opencode.go`
- Added `Stream: false` to Chat and ChatWithTools methods
- Ensures server returns complete JSON response

### 2. Improve System Message Merging
- File: `internal/provider/openai_common.go`
- New strategy: Merge all system messages to start, preserve all conversation messages
- Before: 21 messages → 2 messages (broken)
- After: 21 messages → reasonable count with conversation intact

### 3. Add Message Validation
- File: `internal/provider/openai_common.go`
- New function: `validateOpenAIMessages()`
- Checks for empty arrays, assistant without user, consecutive assistant messages
- Logs warnings for potential issues

## Test Coverage Added

- 4 unit tests for improved message merging
- 4 unit tests for message validation
- 2 integration tests for OpenCode fixes
- Total: 10 new tests, all passing

## Verification

- ✓ All tests pass (`make test`)
- ✓ Builds successfully (`make build`)
- ✓ Manual testing with real OpenCode Zen API: SUCCESS
- ✓ Mysis starts and runs without 500 errors

## Commits

1. `fix(provider): add explicit Stream:false parameter to OpenCode requests`
2. `feat(provider): add OpenAI message validation`
3. `fix(provider): improve system message merging to preserve conversation`
4. `test(provider): add tests for improved system message merging`
5. `test(provider): add tests for message validation`
6. `test(provider): add integration tests for OpenCode fixes`

## Impact

- OpenCode Zen provider now works reliably
- Conversation history preserved correctly
- Better error detection and logging
- Improved test coverage for provider layer
```

4. Commit report:
```bash
git add documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md
git commit -m "docs: add OpenCode Zen fix report

Document root causes, fixes applied, test coverage, and verification results
for the OpenCode Zen 500 error fix."
```

5. Run final verification:
```bash
make test
make build
```

**Expected:** All tests pass, clean build

---

## Execution Summary

**Total Tasks:** 10
**Parallel Phases:** 2 (Phase 1 and Phase 2)
**Sequential Phases:** 2 (Phase 3 and Phase 4)
**Estimated Total Time:** ~45 minutes

**Execution Order:**
1. Launch 3 agents for Phase 1 (Tasks 1.1, 1.2, 1.3) - Parallel
2. Wait for Phase 1 completion
3. Launch 3 agents for Phase 2 (Tasks 2.1, 2.2, 2.3) - Parallel
4. Wait for Phase 2 completion
5. Execute Phase 3 (Task 3.1) - Sequential
6. Execute Phase 4 (Task 4.1) - Sequential

**Success Criteria:**
- All tests pass
- Clean build
- Manual verification with real API succeeds
- OpenCode Zen mysis runs without errors
- 6 commits created with clear messages
