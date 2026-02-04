package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientInitialize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method == "initialize" {
			result := map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo":      map[string]string{"name": "test", "version": "1.0"},
			}
			data, _ := json.Marshal(result)
			resp := &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  data,
			}
			json.NewEncoder(w).Encode(resp)
		} else if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusNoContent)
		} else {
			resp := NewErrorResponse(req.ID, 0, "not implemented")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Initialize(context.Background(), map[string]interface{}{"name": "test-client"})
	if err != nil {
		t.Fatalf("Initialize() error: %v", err)
	}
}

func TestClientNotify(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "notifications/initialized" {
			received = true
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Notify(context.Background(), "notifications/initialized", nil)
	if err != nil {
		t.Fatalf("Notify() error: %v", err)
	}
	if !received {
		t.Error("notification not received by server")
	}
}

func TestClientParseSSEResponse(t *testing.T) {
	client := NewClient("")

	sseData := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"foo\":\"bar\"}}\n\n"
	resp, err := client.parseSSEResponse(strings.NewReader(sseData))
	if err != nil {
		t.Fatalf("parseSSEResponse() error: %v", err)
	}
	if resp.ID != float64(1) {
		t.Errorf("expected ID=1, got %v", resp.ID)
	}

	// Test multiline data
	sseData = "data: {\"jsonrpc\":\"2.0\",\ndata: \"id\":2,\"result\":{}}\n\n"
	resp, err = client.parseSSEResponse(strings.NewReader(sseData))
	if err != nil {
		t.Fatalf("parseSSEResponse() error: %v", err)
	}
	if resp.ID != float64(2) {
		t.Errorf("expected ID=2, got %v", resp.ID)
	}
}
