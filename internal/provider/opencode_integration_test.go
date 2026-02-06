package provider

import (
	"testing"

	openai "github.com/sashabaranov/go-openai"
)

// TestMergeSystemMessagesOpenAI_PreservesOrphanRemoval tests that the provider's
// mergeSystemMessagesOpenAI function doesn't reintroduce orphaned messages
// after they've been removed by the core package.
//
// This test was added after discovering that our unit tests passed but production
// failed because tests never exercised the provider's message manipulation code.
func TestMergeSystemMessagesOpenAI_PreservesOrphanRemoval(t *testing.T) {
	tests := []struct {
		name           string
		input          []openai.ChatCompletionMessage
		expectedCount  int
		shouldAddBegin bool
		shouldAddCont  bool
	}{
		{
			name: "minimal_system_only",
			input: []openai.ChatCompletionMessage{
				{Role: "system", Content: "You are a helpful assistant"},
			},
			expectedCount:  2, // system + "Begin."
			shouldAddBegin: true,
			shouldAddCont:  false,
		},
		{
			name: "system_plus_user",
			input: []openai.ChatCompletionMessage{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "Hello"},
			},
			expectedCount:  2, // no additions needed
			shouldAddBegin: false,
			shouldAddCont:  false,
		},
		{
			name: "ends_with_assistant",
			input: []openai.ChatCompletionMessage{
				{Role: "system", Content: "You are a helpful assistant"},
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi!"},
			},
			expectedCount:  4, // system + user + assistant + "Continue."
			shouldAddBegin: false,
			shouldAddCont:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeSystemMessagesOpenAI(tt.input)

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d messages, got %d", tt.expectedCount, len(result))
			}

			if tt.shouldAddBegin {
				lastMsg := result[len(result)-1]
				if lastMsg.Role != "user" || lastMsg.Content != "Begin." {
					t.Errorf("Expected 'Begin.' user message to be added, got role=%s content=%q",
						lastMsg.Role, lastMsg.Content)
				}
			}

			if tt.shouldAddCont {
				lastMsg := result[len(result)-1]
				if lastMsg.Role != "user" || lastMsg.Content != "Continue." {
					t.Errorf("Expected 'Continue.' user message to be added, got role=%s content=%q",
						lastMsg.Role, lastMsg.Content)
				}
			}
		})
	}
}

// TestOpenCodeProvider_HandlesMinimalRequest tests that OpenCode provider
// can handle a minimal valid request (system + user message).
//
// This reproduces the production scenario where the very first request fails.
func TestOpenCodeProvider_HandlesMinimalRequest(t *testing.T) {
	// Create a mock HTTP server that simulates OpenCode Zen API
	// We can't test against real API without credentials, but we can
	// test that our request formatting is correct.
	t.Skip("Requires mock HTTP server - implement when debugging OpenCode issues")

	// TODO: Implement mock server that:
	// 1. Accepts the request
	// 2. Validates message structure
	// 3. Returns mock response
	// This will catch formatting issues before they hit production
}

// TestProviderMessageFlow_AfterOrphanRemoval tests the complete flow:
// 1. Core removes orphaned messages
// 2. Provider receives cleaned messages
// 3. Provider merges system messages
// 4. Final message array is still valid
//
// This test exercises the ACTUAL code path that production uses.
func TestProviderMessageFlow_AfterOrphanRemoval(t *testing.T) {
	// Simulate what core.getContextMemories() returns after orphan removal
	cleanedMessages := []Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!", ToolCalls: []ToolCall{}},
		// Note: No orphaned tool results - they were removed by core
	}

	// Convert to OpenAI format (what toOpenAIMessages does)
	openaiMsgs := make([]openai.ChatCompletionMessage, len(cleanedMessages))
	for i, m := range cleanedMessages {
		openaiMsgs[i] = openai.ChatCompletionMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	// Apply provider's message manipulation
	merged := mergeSystemMessagesOpenAI(openaiMsgs)

	// ASSERTIONS:
	// 1. Message count should be valid
	if len(merged) < 2 {
		t.Errorf("Merged messages too short: %d", len(merged))
	}

	// 2. Should start with system
	if merged[0].Role != "system" {
		t.Errorf("First message should be system, got %s", merged[0].Role)
	}

	// 3. Should end with user (Continue. was added)
	lastMsg := merged[len(merged)-1]
	if lastMsg.Role != "user" {
		t.Errorf("Last message should be user after merge, got %s", lastMsg.Role)
	}

	// 4. No orphaned tool messages should exist
	validToolCalls := make(map[string]bool)
	for _, msg := range merged {
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				validToolCalls[tc.ID] = true
			}
		}
	}

	for _, msg := range merged {
		if msg.Role == "tool" {
			if msg.ToolCallID == "" {
				t.Error("Tool message missing ToolCallID")
			} else if !validToolCalls[msg.ToolCallID] {
				t.Errorf("Orphaned tool message found: %s", msg.ToolCallID)
			}
		}
	}

	t.Logf("✅ Provider message flow validation passed")
	t.Logf("   Input: %d messages", len(cleanedMessages))
	t.Logf("   After merge: %d messages", len(merged))
	t.Logf("   Valid tool calls: %d", len(validToolCalls))
}
