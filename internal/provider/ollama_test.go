package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOllama_SystemMessagesAnywhere tests that Ollama provider correctly handles
// system messages anywhere in the conversation (not just at the start).
func TestOllama_SystemMessagesAnywhere(t *testing.T) {
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

		// Verify message order: system, user, system (merged), assistant
		// Ollama allows system messages anywhere, but consecutive ones should be merged
		expectedRoles := []string{"system", "user", "system", "assistant"}
		if len(req.Messages) != len(expectedRoles) {
			t.Errorf("expected %d messages, got %d", len(expectedRoles), len(req.Messages))
		}

		for i, expected := range expectedRoles {
			if i >= len(req.Messages) {
				break
			}
			if req.Messages[i].Role != expected {
				t.Errorf("message %d: expected role=%s, got %s", i, expected, req.Messages[i].Role)
			}
		}

		// Verify first system message
		if !strings.Contains(req.Messages[0].Content, "helpful assistant") {
			t.Error("expected first system message to contain 'helpful assistant'")
		}

		// Verify third message merges two consecutive system messages
		if !strings.Contains(req.Messages[2].Content, "Be concise") {
			t.Error("expected merged system message to contain 'Be concise'")
		}
		if !strings.Contains(req.Messages[2].Content, "Use simple language") {
			t.Error("expected merged system message to contain 'Use simple language'")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Got it!",
					},
				},
			},
		})
	}))
	defer server.Close()

	// Remove /v1 suffix since NewOllama adds it
	baseURL := strings.TrimSuffix(server.URL, "/v1")
	provider := NewOllama(baseURL, "test-model")
	ctx := context.Background()

	// System messages at different positions, including consecutive ones
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "system", Content: "Be concise in your responses."},
		{Role: "system", Content: "Use simple language."},
		{Role: "assistant", Content: "Hi there!"},
	}

	_, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

// TestOllama_ConsecutiveSystemMessagesMerge tests that consecutive system messages
// are merged in place (Ollama-specific behavior).
func TestOllama_ConsecutiveSystemMessagesMerge(t *testing.T) {
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

		// Verify: 3 system messages should be merged into 1
		systemCount := 0
		for i, msg := range req.Messages {
			if msg.Role == "system" {
				systemCount++
				// Should be first message
				if i != 0 {
					t.Errorf("expected system message at position 0, got %d", i)
				}
				// Should contain all three system prompts merged
				if !strings.Contains(msg.Content, "instruction one") {
					t.Error("expected merged system to contain 'instruction one'")
				}
				if !strings.Contains(msg.Content, "instruction two") {
					t.Error("expected merged system to contain 'instruction two'")
				}
				if !strings.Contains(msg.Content, "instruction three") {
					t.Error("expected merged system to contain 'instruction three'")
				}
			}
		}

		if systemCount != 1 {
			t.Errorf("expected 1 system message after merge, got %d", systemCount)
		}

		// Total messages should be 2 (1 merged system + 1 user)
		if len(req.Messages) != 2 {
			t.Errorf("expected 2 messages total, got %d", len(req.Messages))
		}

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

	baseURL := strings.TrimSuffix(server.URL, "/v1")
	provider := NewOllama(baseURL, "test-model")
	ctx := context.Background()

	// Three consecutive system messages followed by user message
	messages := []Message{
		{Role: "system", Content: "This is instruction one."},
		{Role: "system", Content: "This is instruction two."},
		{Role: "system", Content: "This is instruction three."},
		{Role: "user", Content: "Hello"},
	}

	_, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
}

// TestOllama_ToolCalls tests that Ollama provider correctly handles
// messages with tool calls.
func TestOllama_ToolCalls(t *testing.T) {
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

			// Verify tool structure matches Ollama format
			tool := req.Tools[0]
			if tool["type"] != "function" {
				t.Errorf("expected tool type=function, got %v", tool["type"])
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "",
							"tool_calls": []map[string]interface{}{
								{
									"id":   "call_abc",
									"type": "function",
									"function": map[string]interface{}{
										"name":      "calculate",
										"arguments": `{"operation":"add","a":5,"b":3}`,
									},
								},
							},
						},
					},
				},
			})
			return
		}

		// Second request: verify tool result message
		if requestCount == 2 {
			foundToolResult := false
			for _, msg := range req.Messages {
				if msg.Role == "tool" && msg.ToolCallID == "call_abc" {
					foundToolResult = true
					if msg.Content != "8" {
						t.Errorf("expected tool result='8', got %q", msg.Content)
					}
				}
			}
			if !foundToolResult {
				t.Error("expected tool result message in second request")
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "The result is 8.",
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	baseURL := strings.TrimSuffix(server.URL, "/v1")
	provider := NewOllama(baseURL, "test-model")
	ctx := context.Background()

	// First call with tools
	tools := []Tool{
		{
			Name:        "calculate",
			Description: "Perform calculations",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"operation":{"type":"string"},"a":{"type":"number"},"b":{"type":"number"}}}`),
		},
	}

	messages := []Message{
		{Role: "user", Content: "What is 5 plus 3?"},
	}

	resp1, err := provider.ChatWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}

	// Verify tool call
	if len(resp1.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp1.ToolCalls))
	}
	if resp1.ToolCalls[0].Name != "calculate" {
		t.Errorf("expected tool name=calculate, got %s", resp1.ToolCalls[0].Name)
	}

	// Second call with tool result
	messages = append(messages, Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: resp1.ToolCalls,
	})
	messages = append(messages, Message{
		Role:       "tool",
		Content:    "8",
		ToolCallID: "call_abc",
	})

	resp2, err := provider.ChatWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}

	if !strings.Contains(resp2.Content, "8") {
		t.Errorf("expected response to contain '8', got %q", resp2.Content)
	}
}

// TestOllama_ReasoningField tests that Ollama provider extracts reasoning
// from both "reasoning" and "reasoning_content" fields (Ollama extension).
func TestOllama_ReasoningField(t *testing.T) {
	tests := []struct {
		name          string
		responseField string
		responseValue string
	}{
		{
			name:          "reasoning field",
			responseField: "reasoning",
			responseValue: "I need to think about this...",
		},
		{
			name:          "reasoning_content field",
			responseField: "reasoning_content",
			responseValue: "Let me analyze this step by step.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				// Build response with specified reasoning field
				response := map[string]interface{}{
					"choices": []map[string]interface{}{
						{
							"message": map[string]interface{}{
								"role":           "assistant",
								"content":        "Answer",
								tt.responseField: tt.responseValue,
							},
						},
					},
				}

				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			baseURL := strings.TrimSuffix(server.URL, "/v1")
			provider := NewOllama(baseURL, "test-model")
			ctx := context.Background()

			messages := []Message{
				{Role: "user", Content: "Question?"},
			}

			tools := []Tool{
				{Name: "dummy", Description: "Dummy tool"},
			}

			resp, err := provider.ChatWithTools(ctx, messages, tools)
			if err != nil {
				t.Fatalf("ChatWithTools() error: %v", err)
			}

			if resp.Reasoning != tt.responseValue {
				t.Errorf("expected reasoning=%q, got %q", tt.responseValue, resp.Reasoning)
			}
		})
	}
}

// TestOllama_SystemMessagesAtEnd tests that system messages at the end
// are preserved (Ollama allows this, unlike OpenAI).
func TestOllama_SystemMessagesAtEnd(t *testing.T) {
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

		// Verify last message is system
		lastMsg := req.Messages[len(req.Messages)-1]
		if lastMsg.Role != "system" {
			t.Errorf("expected last message role=system, got %s", lastMsg.Role)
		}
		if !strings.Contains(lastMsg.Content, "final instruction") {
			t.Error("expected last system message to contain 'final instruction'")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Understood final instruction.",
					},
				},
			},
		})
	}))
	defer server.Close()

	baseURL := strings.TrimSuffix(server.URL, "/v1")
	provider := NewOllama(baseURL, "test-model")
	ctx := context.Background()

	// System message at the end
	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi!"},
		{Role: "system", Content: "Here's a final instruction."},
	}

	resp, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}

	if !strings.Contains(resp, "final instruction") {
		t.Errorf("expected response to acknowledge final instruction, got %q", resp)
	}
}
