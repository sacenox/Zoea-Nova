package provider

import (
	"context"
	"errors"
	"testing"
)

func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	// Register mock providers
	mock1 := NewMock("provider1", "response1")
	mock2 := NewMock("provider2", "response2")

	reg.Register(mock1)
	reg.Register(mock2)

	// Get existing provider
	p, err := reg.Get("provider1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if p.Name() != "provider1" {
		t.Errorf("expected name=provider1, got %s", p.Name())
	}

	// Get non-existent provider
	_, err = reg.Get("nonexistent")
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
