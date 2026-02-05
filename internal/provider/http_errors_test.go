package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaChatReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("model not found"))
	}))
	defer server.Close()

	provider := NewOllama("http://unused", "missing-model")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 404") || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOllamaChatReturnsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("overloaded"))
	}))
	defer server.Close()

	provider := NewOllama("http://unused", "any")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 503") || !strings.Contains(err.Error(), "overloaded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenCodeChatReturnsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid api key"))
	}))
	defer server.Close()

	provider := NewOpenCode("http://unused", "model", "bad-key")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOllamaChatWithToolsParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"test_tool","arguments":"{\"a\":1}"}}]}}]}`))
	}))
	defer server.Close()

	provider := NewOllama("http://unused", "any")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	resp, err := provider.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, []Tool{{Name: "test_tool"}})
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "test_tool" {
		t.Fatalf("expected tool name test_tool, got %s", resp.ToolCalls[0].Name)
	}
	if string(resp.ToolCalls[0].Arguments) != `{"a":1}` {
		t.Fatalf("expected arguments {\"a\":1}, got %s", string(resp.ToolCalls[0].Arguments))
	}
}

func TestOpenCodeChatWithToolsParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"call_2","type":"function","function":{"name":"other_tool","arguments":"{\"b\":\"c\"}"}}]}}]}`))
	}))
	defer server.Close()

	provider := NewOpenCode("http://unused", "model", "key")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	resp, err := provider.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, []Tool{{Name: "other_tool"}})
	if err != nil {
		t.Fatalf("ChatWithTools() error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "other_tool" {
		t.Fatalf("expected tool name other_tool, got %s", resp.ToolCalls[0].Name)
	}
	if string(resp.ToolCalls[0].Arguments) != `{"b":"c"}` {
		t.Fatalf("expected arguments {\"b\":\"c\"}, got %s", string(resp.ToolCalls[0].Arguments))
	}
}

func TestOllamaChatWithToolsReturnsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	provider := NewOllama("http://unused", "any")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	_, err := provider.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, []Tool{{Name: "test_tool"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 401") || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenCodeChatWithToolsReturnsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid api key"))
	}))
	defer server.Close()

	provider := NewOpenCode("http://unused", "model", "bad-key")
	provider.baseURL = server.URL
	provider.httpClient = server.Client()

	_, err := provider.ChatWithTools(context.Background(), []Message{{Role: "user", Content: "hi"}}, []Tool{{Name: "test_tool"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "status 401") || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("unexpected error: %v", err)
	}
}
