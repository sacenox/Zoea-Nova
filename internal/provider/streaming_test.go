package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestOllamaProvider_Stream tests basic streaming functionality
func TestOllamaProvider_Stream(t *testing.T) {
	// Mock server that streams chunks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		if !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
			t.Errorf("Expected path ending with /v1/chat/completions, got %s", r.URL.Path)
		}

		// Verify stream=true in request body (OpenAI client adds this)
		// Since we're using the go-openai client, it handles streaming protocol

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Stream chunks in SSE format (OpenAI streaming format)
		chunks := []string{
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond) // Simulate streaming delay
		}
	}))
	defer server.Close()

	provider := NewOllama(server.URL, "test-model")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "test"}}

	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	var result string
	var gotDone bool
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("Stream chunk error: %v", chunk.Err)
		}
		if chunk.Done {
			gotDone = true
			continue
		}
		result += chunk.Content
	}

	expected := "Hello world!"
	if result != expected {
		t.Errorf("Expected content %q, got %q", expected, result)
	}
	if !gotDone {
		t.Error("Expected Done=true chunk")
	}
}

// TestOllamaProvider_Stream_Cancellation tests context cancellation during streaming
func TestOllamaProvider_Stream_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send chunks slowly
		for i := 0; i < 10; i++ {
			fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"chunk %d\"},\"finish_reason\":null}]}\n\n", i)
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewOllama(server.URL, "test")

	ctx, cancel := context.WithCancel(context.Background())

	messages := []Message{{Role: "user", Content: "test"}}
	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	// Cancel after first chunk
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	var chunks int
	var gotError bool
	for chunk := range ch {
		if chunk.Err != nil {
			gotError = true
			// Context cancellation should result in error
			break
		}
		if !chunk.Done {
			chunks++
		}
	}

	if chunks >= 10 {
		t.Errorf("Expected cancellation after few chunks, got %d chunks", chunks)
	}
	if !gotError {
		t.Error("Expected error after context cancellation")
	}
}

// TestOllamaProvider_Stream_EmptyChunks tests handling of empty content chunks
func TestOllamaProvider_Stream_EmptyChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		chunks := []string{
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":""},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":"Hi"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":""},"finish_reason":null}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewOllama(server.URL, "test")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "test"}}

	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	var result string
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("Stream chunk error: %v", chunk.Err)
		}
		if chunk.Done {
			continue
		}
		result += chunk.Content
	}

	expected := "Hi"
	if result != expected {
		t.Errorf("Expected content %q, got %q", expected, result)
	}
}

// TestOllamaProvider_Stream_ServerError tests handling of HTTP errors
func TestOllamaProvider_Stream_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	provider := NewOllama(server.URL, "test")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "test"}}

	_, err := provider.Stream(ctx, messages)
	if err == nil {
		t.Fatal("Expected error from Stream(), got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected 500 error, got: %v", err)
	}
}

// TestOpenCodeProvider_Stream tests basic streaming functionality
func TestOpenCodeProvider_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header 'Bearer test-api-key', got %q", auth)
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")

		chunks := []string{
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":"OpenCode"},"finish_reason":null}]}`,
			`data: {"id":"1","object":"chat.completion.chunk","created":1234567890,"model":"test","choices":[{"index":0,"delta":{"content":" response"},"finish_reason":null}]}`,
			`data: [DONE]`,
		}

		for _, chunk := range chunks {
			fmt.Fprintf(w, "%s\n\n", chunk)
			flusher.Flush()
		}
	}))
	defer server.Close()

	provider := NewOpenCode(server.URL, "test-model", "test-api-key")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "test"}}

	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	var result string
	var gotDone bool
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("Stream chunk error: %v", chunk.Err)
		}
		if chunk.Done {
			gotDone = true
			continue
		}
		result += chunk.Content
	}

	expected := "OpenCode response"
	if result != expected {
		t.Errorf("Expected content %q, got %q", expected, result)
	}
	if !gotDone {
		t.Error("Expected Done=true chunk")
	}
}

// TestOpenCodeProvider_Stream_Cancellation tests context cancellation
func TestOpenCodeProvider_Stream_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		for i := 0; i < 10; i++ {
			fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"chunk %d\"},\"finish_reason\":null}]}\n\n", i)
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	provider := NewOpenCode(server.URL, "test", "key")

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	messages := []Message{{Role: "user", Content: "test"}}
	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	var chunks int
	for chunk := range ch {
		if chunk.Err != nil {
			// Expected timeout error
			break
		}
		if !chunk.Done {
			chunks++
		}
	}

	if chunks >= 10 {
		t.Errorf("Expected timeout after few chunks, got %d chunks", chunks)
	}
}

// TestStreamChunk_Fields tests StreamChunk field structure
func TestStreamChunk_Fields(t *testing.T) {
	tests := []struct {
		name  string
		chunk StreamChunk
	}{
		{
			name:  "content_chunk",
			chunk: StreamChunk{Content: "hello", Done: false, Err: nil},
		},
		{
			name:  "done_chunk",
			chunk: StreamChunk{Content: "", Done: true, Err: nil},
		},
		{
			name:  "error_chunk",
			chunk: StreamChunk{Content: "", Done: false, Err: errors.New("error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "content_chunk" && tt.chunk.Content != "hello" {
				t.Errorf("Expected content='hello', got %q", tt.chunk.Content)
			}
			if tt.name == "done_chunk" && !tt.chunk.Done {
				t.Error("Expected Done=true")
			}
			if tt.name == "error_chunk" && tt.chunk.Err == nil {
				t.Error("Expected non-nil error")
			}
		})
	}
}

// TestOllamaProvider_Stream_NoChoices tests handling of responses with no choices
func TestOllamaProvider_Stream_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		// Send chunk with empty choices array
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"created\":1234567890,\"model\":\"test\",\"choices\":[]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	provider := NewOllama(server.URL, "test")

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "test"}}

	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() failed: %v", err)
	}

	var contentChunks int
	for chunk := range ch {
		if chunk.Err != nil {
			t.Fatalf("Stream chunk error: %v", chunk.Err)
		}
		if !chunk.Done && chunk.Content != "" {
			contentChunks++
		}
	}

	// Should handle gracefully - no content chunks expected
	if contentChunks > 0 {
		t.Errorf("Expected 0 content chunks for empty choices, got %d", contentChunks)
	}
}

// TestProvider_Stream_Interface tests that providers implement Stream correctly
func TestProvider_Stream_Interface(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
	}{
		{
			name:     "ollama_provider",
			provider: NewOllama("http://localhost:11434", "test"),
		},
		{
			name:     "opencode_provider",
			provider: NewOpenCode("https://api.test.com", "test", "key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify Stream method exists and returns correct types
			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "test"}}

			// This will fail if server is not available, but that's expected
			// We're just verifying the method signature
			_, err := tt.provider.Stream(ctx, messages)
			// Error is expected (no server), we just want to verify method exists
			_ = err
		})
	}
}
