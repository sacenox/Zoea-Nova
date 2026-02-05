package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Register mock providers
	reg.RegisterFactory(NewMockFactory("provider1", "response1"))
	reg.RegisterFactory(NewMockFactory("provider2", "response2"))

	// Get existing provider
	p, err := reg.Create("provider1", "model", 0.7)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if p.Name() != "provider1" {
		t.Errorf("expected name=provider1, got %s", p.Name())
	}

	// Get non-existent provider
	_, err = reg.Create("nonexistent", "model", 0.7)
	if !errors.Is(err, ErrProviderNotFound) {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}

	// List providers
	names := reg.List()
	if len(names) != 2 {
		t.Errorf("expected 2 providers, got %d", len(names))
	}
}

func TestMockProviderChat(t *testing.T) {
	mock := NewMock("test", "Hello, World!")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	response, err := mock.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	if response != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %s", response)
	}
}

func TestMockProviderChatDelay(t *testing.T) {
	delay := 30 * time.Millisecond
	mock := NewMock("test", "ok").SetDelay(delay)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	start := time.Now()
	_, err := mock.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < delay {
		t.Fatalf("expected delay >= %v, got %v", delay, elapsed)
	}
}

func TestMockProviderChatWithToolsDelay(t *testing.T) {
	delay := 25 * time.Millisecond
	mock := NewMock("test", "ok").SetDelay(delay)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	start := time.Now()
	_, err := mock.ChatWithTools(ctx, messages, nil)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < delay {
		t.Fatalf("expected delay >= %v, got %v", delay, elapsed)
	}
}

func TestMockProviderChatDelayContextCancel(t *testing.T) {
	mock := NewMock("test", "ok").SetDelay(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	messages := []Message{{Role: "user", Content: "Hi"}}
	_, err := mock.Chat(ctx, messages)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestMockProviderChatError(t *testing.T) {
	expectedErr := errors.New("chat error")
	mock := NewMock("test", "").WithChatError(expectedErr)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	_, err := mock.Chat(ctx, messages)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestMockProviderStream(t *testing.T) {
	mock := NewMock("test", "Streamed response")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	ch, err := mock.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	var content string
	var done bool
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("Stream chunk error: %v", chunk.Err)
		}
		if chunk.Done {
			done = true
			continue
		}
		content += chunk.Content
	}

	if content != "Streamed response" {
		t.Errorf("expected 'Streamed response', got %s", content)
	}
	if !done {
		t.Error("expected done=true")
	}
}

func TestMockProviderStreamError(t *testing.T) {
	expectedErr := errors.New("stream error")
	mock := NewMock("test", "").WithStreamError(expectedErr)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	_, err := mock.Stream(ctx, messages)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestOllamaProviderName(t *testing.T) {
	p := NewOllama("http://localhost:11434", "llama3")
	if p.Name() != "ollama" {
		t.Errorf("expected name=ollama, got %s", p.Name())
	}
}

func TestOpenCodeProviderName(t *testing.T) {
	p := NewOpenCode("https://api.opencode.ai/v1", "zen-default", "test-key")
	if p.Name() != "opencode_zen" {
		t.Errorf("expected name=opencode_zen, got %s", p.Name())
	}
}

func TestToOpenAIMessages(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	result := toOpenAIMessages(messages)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("expected role=system, got %s", result[0].Role)
	}
	if result[1].Content != "Hello" {
		t.Errorf("expected content=Hello, got %s", result[1].Content)
	}
}

func TestToOpenAIMessagesWithToolCalls(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "What's the weather?"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "call_123", Name: "get_weather", Arguments: json.RawMessage(`{"location":"NYC"}`)},
			},
		},
		{Role: "tool", Content: `{"temp": 72}`, ToolCallID: "call_123"},
	}

	result := toOpenAIMessages(messages)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Check assistant message has tool calls
	assistantMsg := result[1]
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].ID != "call_123" {
		t.Errorf("expected tool call ID=call_123, got %s", assistantMsg.ToolCalls[0].ID)
	}
	if assistantMsg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected function name=get_weather, got %s", assistantMsg.ToolCalls[0].Function.Name)
	}

	// Check tool result message has ToolCallID
	toolMsg := result[2]
	if toolMsg.ToolCallID != "call_123" {
		t.Errorf("expected ToolCallID=call_123, got %s", toolMsg.ToolCallID)
	}
}

func TestMockProviderChatWithTools(t *testing.T) {
	toolCalls := []ToolCall{
		{ID: "call_abc", Name: "test_tool", Arguments: json.RawMessage(`{}`)},
	}
	mock := NewMock("test", "thinking...").WithToolCalls(toolCalls)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Do something"}}
	tools := []Tool{{Name: "test_tool", Description: "A test tool"}}

	resp, err := mock.ChatWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}

	if resp.Content != "thinking..." {
		t.Errorf("expected content='thinking...', got %s", resp.Content)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "test_tool" {
		t.Errorf("expected tool name=test_tool, got %s", resp.ToolCalls[0].Name)
	}
}

func TestMockProviderChatWithToolsError(t *testing.T) {
	expectedErr := errors.New("tools error")
	mock := NewMock("test", "").WithChatError(expectedErr)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Do something"}}
	tools := []Tool{{Name: "test_tool", Description: "A test tool"}}

	_, err := mock.ChatWithTools(ctx, messages, tools)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestMockProviderWithResponse(t *testing.T) {
	mock := NewMock("test", "initial")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	// Initial response
	resp, _ := mock.Chat(ctx, messages)
	if resp != "initial" {
		t.Errorf("expected 'initial', got %s", resp)
	}

	// Change response
	mock.WithResponse("updated")
	resp, _ = mock.Chat(ctx, messages)
	if resp != "updated" {
		t.Errorf("expected 'updated', got %s", resp)
	}
}

func TestMockProviderWithReasoning(t *testing.T) {
	reasoning := "thinking..."
	mock := NewMock("test", "response").WithReasoning(reasoning)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}
	tools := []Tool{{Name: "test_tool", Description: "A test tool"}}

	resp, err := mock.ChatWithTools(ctx, messages, tools)
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}
	if resp.Reasoning != reasoning {
		t.Errorf("expected reasoning=%q, got %q", reasoning, resp.Reasoning)
	}
}

func TestMockProviderConcurrentAccess(t *testing.T) {
	mock := NewMock("test", "response")
	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	// Run concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			mock.Chat(ctx, messages)
			done <- true
		}()
		go func() {
			mock.WithResponse("new response")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestToOpenAIMessagesEmpty(t *testing.T) {
	result := toOpenAIMessages([]Message{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d messages", len(result))
	}
}

func TestToOpenAIMessagesMultipleToolCalls(t *testing.T) {
	messages := []Message{
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{ID: "call_1", Name: "tool_a", Arguments: json.RawMessage(`{"a":1}`)},
				{ID: "call_2", Name: "tool_b", Arguments: json.RawMessage(`{"b":2}`)},
			},
		},
	}

	result := toOpenAIMessages(messages)

	if len(result[0].ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result[0].ToolCalls))
	}
	if result[0].ToolCalls[0].Function.Arguments != `{"a":1}` {
		t.Errorf("expected arguments={\"a\":1}, got %s", result[0].ToolCalls[0].Function.Arguments)
	}
	if result[0].ToolCalls[1].Function.Arguments != `{"b":2}` {
		t.Errorf("expected arguments={\"b\":2}, got %s", result[0].ToolCalls[1].Function.Arguments)
	}
}

func TestToOllamaMessagesSerializesEmptyContent(t *testing.T) {
	messages := []Message{{Role: "assistant", Content: ""}}

	result := toOllamaMessages(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	data, err := json.Marshal(result[0])
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	if !strings.Contains(string(data), `"content":""`) {
		t.Fatalf("expected content field to be serialized, got %s", string(data))
	}
}

func TestMockProviderRateLimit(t *testing.T) {
	limiter := rate.NewLimiter(5, 1)
	mock := NewMock("test", "ok").WithLimiter(limiter)

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hi"}}

	start := time.Now()
	for i := 0; i < 2; i++ {
		if _, err := mock.Chat(ctx, messages); err != nil {
			t.Fatalf("Chat() error: %v", err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 180*time.Millisecond {
		t.Fatalf("expected rate limiting delay, got %v", elapsed)
	}
}
