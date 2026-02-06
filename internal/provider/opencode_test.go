package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOpenCode_SystemPromptOnly tests that OpenCode provider adds "Begin." user message
// when only system prompts are present (OpenAI requirement).
func TestOpenCode_SystemPromptOnly(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify system message is first
		if len(req.Messages) < 1 {
			t.Fatal("expected at least 1 message")
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("expected first message role=system, got %s", req.Messages[0].Role)
		}

		// Verify "Begin." user message was added
		if len(req.Messages) < 2 {
			t.Fatal("expected 2 messages (system + auto-generated user)")
		}
		if req.Messages[1].Role != "user" {
			t.Errorf("expected second message role=user, got %s", req.Messages[1].Role)
		}
		if req.Messages[1].Content != "Begin." {
			t.Errorf("expected second message content='Begin.', got %q", req.Messages[1].Content)
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Ready.",
					},
				},
			},
		})
	}))
	defer server.Close()

	// Create provider
	provider := NewOpenCode(server.URL, "test-model", "test-key")

	// Send only system messages
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
	}

	ctx := context.Background()
	response, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}

	if response != "Ready." {
		t.Errorf("expected response='Ready.', got %q", response)
	}
}

// TestOpenCode_ToolCallsAndResults tests that OpenCode provider correctly handles
// messages with tool calls and tool results.
func TestOpenCode_ToolCallsAndResults(t *testing.T) {
	// Setup mock server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var req struct {
			Messages []struct {
				Role      string `json:"role"`
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls,omitempty"`
				ToolCallID string `json:"tool_call_id,omitempty"`
			} `json:"messages"`
			Tools []map[string]interface{} `json:"tools,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")

		// First request: return tool call
		if requestCount == 1 {
			// Verify tools are present
			if len(req.Tools) == 0 {
				t.Error("expected tools in request")
			}

			// Return tool call response
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "call_123",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "get_weather",
										"arguments": `{"location":"NYC"}`,
									},
								},
							},
						},
					},
				},
			})
			return
		}

		// Second request: verify tool result is included
		if requestCount == 2 {
			foundToolResult := false
			for _, msg := range req.Messages {
				if msg.Role == "tool" && msg.ToolCallID == "call_123" {
					foundToolResult = true
					if !strings.Contains(msg.Content, "72") {
						t.Errorf("expected tool result to contain '72', got %q", msg.Content)
					}
				}
			}
			if !foundToolResult {
				t.Error("expected tool result message in second request")
			}

			// Return final response
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "The weather in NYC is 72°F.",
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	provider := NewOpenCode(server.URL, "test-model", "test-key")
	ctx := context.Background()

	// First call with tools
	tools := []Tool{
		{
			Name:        "get_weather",
			Description: "Get current weather",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"location":{"type":"string"}}}`),
		},
	}

	messages := []Message{
		{Role: "user", Content: "What's the weather in NYC?"},
	}

	resp1, err := provider.ChatWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}

	// Verify tool call
	if len(resp1.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp1.ToolCalls))
	}
	if resp1.ToolCalls[0].ID != "call_123" {
		t.Errorf("expected tool call ID=call_123, got %s", resp1.ToolCalls[0].ID)
	}
	if resp1.ToolCalls[0].Name != "get_weather" {
		t.Errorf("expected tool name=get_weather, got %s", resp1.ToolCalls[0].Name)
	}

	// Second call with tool result
	messages = append(messages, Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: resp1.ToolCalls,
	})
	messages = append(messages, Message{
		Role:       "tool",
		Content:    `{"temp":72,"condition":"sunny"}`,
		ToolCallID: "call_123",
	})

	resp2, err := provider.ChatWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}

	if !strings.Contains(resp2.Content, "72") {
		t.Errorf("expected response to contain '72', got %q", resp2.Content)
	}
}

// TestOpenCode_InvalidToolSchema tests that OpenCode provider returns error
// for invalid tool schemas.
func TestOpenCode_InvalidToolSchema(t *testing.T) {
	// Setup mock server (won't be called)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be called with invalid tool schema")
	}))
	defer server.Close()

	provider := NewOpenCode(server.URL, "test-model", "test-key")
	ctx := context.Background()

	// Create tool with invalid JSON schema
	tools := []Tool{
		{
			Name:        "bad_tool",
			Description: "Tool with invalid schema",
			Parameters:  json.RawMessage(`{invalid json}`), // Invalid JSON
		},
	}

	messages := []Message{
		{Role: "user", Content: "Do something"},
	}

	_, err := provider.ChatWithTools(ctx, messages, tools)
	if err == nil {
		t.Fatal("expected error for invalid tool schema")
	}

	// Verify error message mentions invalid schema
	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid tool schema") {
		t.Errorf("expected error message to contain 'invalid tool schema', got %q", errMsg)
	}
}

// TestOpenCode_MultipleSystemMessages tests that multiple system messages
// are merged into a single message at the start (OpenAI requirement).
func TestOpenCode_MultipleSystemMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Count system messages
		systemCount := 0
		for _, msg := range req.Messages {
			if msg.Role == "system" {
				systemCount++
				// Verify merged content contains both instructions
				if !strings.Contains(msg.Content, "helpful assistant") {
					t.Error("expected merged system message to contain 'helpful assistant'")
				}
				if !strings.Contains(msg.Content, "Be concise") {
					t.Error("expected merged system message to contain 'Be concise'")
				}
			}
		}

		// Verify only one system message
		if systemCount != 1 {
			t.Errorf("expected 1 system message after merge, got %d", systemCount)
		}

		// Verify system message is first
		if req.Messages[0].Role != "system" {
			t.Errorf("expected first message to be system, got %s", req.Messages[0].Role)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Understood.",
					},
				},
			},
		})
	}))
	defer server.Close()

	provider := NewOpenCode(server.URL, "test-model", "test-key")
	ctx := context.Background()

	// Send multiple system messages scattered in conversation
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "system", Content: "Be concise in your responses."},
		{Role: "assistant", Content: "Hi"},
	}

	_, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}
