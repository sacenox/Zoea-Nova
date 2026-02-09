package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

type failingUpstream struct {
	tools   []mcp.Tool
	callErr error
}

func (f *failingUpstream) Initialize(ctx context.Context, clientInfo map[string]interface{}) (*mcp.Response, error) {
	return nil, nil
}

func (f *failingUpstream) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	return f.tools, nil
}

func (f *failingUpstream) CallTool(ctx context.Context, name string, arguments interface{}) (*mcp.ToolResult, error) {
	return nil, f.callErr
}

func TestMysisToolExecution(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("tool-mysis", "mock", "test-model", 0.7)

	// Mock provider that returns a tool call first, then a text response
	mock := provider.NewMock("mock", "Initial response")

	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	// Setup MCP proxy with a local tool
	proxy := mcp.NewProxy(nil)
	proxy.RegisterTool(mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}, func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
		return &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "tool result content"}},
		}, nil
	})
	// Manually set MCP proxy for test (normally done in initializeMCP)
	mysis.mcpProxy = proxy

	events := bus.Subscribe()
	mysis.Start()

	// Wait for initial turn to finish
	timeout := time.After(5 * time.Second)
	initialFinished := false
	for !initialFinished {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				if e.Message != nil && e.Message.Content == "Initial response" {
					initialFinished = true
				}
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial turn")
		}
	}

	// Now set tool calls for the next turn
	mock.WithResponse("Final response")
	mock.WithToolCalls([]provider.ToolCall{
		{
			ID:        "call_1",
			Name:      "test_tool",
			Arguments: json.RawMessage(`{"arg": "val"}`),
		},
	})

	// Send message to trigger the loop
	go mysis.SendMessage("Use the tool", store.MemorySourceDirect)

	// Wait for tool call event
	toolCalled := false
	for !toolCalled {
		select {
		case e := <-events:
			if e.Type == EventMysisMessage {
				if e.Message != nil && e.Message.Role == "assistant" && e.Message.Content == "Calling tools: test_tool" {
					toolCalled = true
					// Now update mock to return NO tool calls for the next iteration
					mock.WithToolCalls(nil)
				}
			}
		case <-timeout:
			t.Fatal("timeout waiting for tool call")
		}
	}

	// Wait for final response
	finalResponse := false
	for !finalResponse {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				finalResponse = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for final response")
		}
	}

	// Verify memories with retry
	deadline := time.Now().Add(2 * time.Second)
	foundToolResult := false
	for time.Now().Before(deadline) {
		memories, _ := s.GetMemories(stored.ID)
		for _, m := range memories {
			if m.Role == store.MemoryRoleTool && m.Content == "call_1:tool result content" {
				foundToolResult = true
				break
			}
		}
		if foundToolResult {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !foundToolResult {
		t.Error("tool result not found in memories")
	}
}

func TestMysisToolError(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("tool-err-mysis", "mock", "test-model", 0.7)

	mock := provider.NewMock("mock", "Initial response")

	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	proxy := mcp.NewProxy(nil)
	proxy.RegisterTool(mcp.Tool{
		Name: "error_tool",
	}, func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
		return nil, errors.New("tool failed")
	})
	// Manually set MCP proxy for test (normally done in initializeMCP)
	mysis.mcpProxy = proxy

	events := bus.Subscribe()
	mysis.Start()

	// Wait for initial turn
	timeout := time.After(5 * time.Second)
	initialFinished := false
	for !initialFinished {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				initialFinished = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial turn")
		}
	}

	// Now set tool calls
	mock.WithToolCalls([]provider.ToolCall{
		{
			ID:        "call_err",
			Name:      "error_tool",
			Arguments: json.RawMessage(`{}`),
		},
	})

	// Update mock after first call to stop the loop
	go func() {
		time.Sleep(100 * time.Millisecond)
		mock.WithToolCalls(nil)
	}()

	mysis.SendMessage("Trigger error", store.MemorySourceDirect)

	// Verify memory contains error with retry
	deadline := time.Now().Add(2 * time.Second)
	foundError := false
	for time.Now().Before(deadline) {
		memories, _ := s.GetMemories(stored.ID)
		for _, m := range memories {
			if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, "call_err:Error calling error_tool: tool failed") {
				foundError = true
				break
			}
		}
		if foundError {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !foundError {
		t.Error("tool error not found in memories")
	}
}

func TestMysisToolTimeoutSetsErrored(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("tool-timeout-mysis", "mock", "test-model", 0.7)

	mock := provider.NewMock("mock", "Initial response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	proxy := mcp.NewProxy(nil)
	proxy.RegisterTool(mcp.Tool{
		Name: "timeout_tool",
	}, func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
		return nil, context.DeadlineExceeded
	})
	// Manually set MCP proxy for test (normally done in initializeMCP)
	mysis.mcpProxy = proxy

	// Start and wait for initial turn
	events := bus.Subscribe()
	mysis.Start()

	timeout := time.After(5 * time.Second)
	initialFinished := false
	for !initialFinished {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				initialFinished = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial turn")
		}
	}

	// Trigger tool call that times out
	mock.WithToolCalls([]provider.ToolCall{
		{
			ID:        "call_timeout",
			Name:      "timeout_tool",
			Arguments: json.RawMessage(`{}`),
		},
	})

	go mysis.SendMessage("Trigger timeout", store.MemorySourceDirect)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mysis.State() == MysisStateErrored {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if state := mysis.State(); state != MysisStateErrored {
		t.Fatalf("expected state=errored, got %s", state)
	}

	// Verify memory contains timeout error
	foundTimeout := false
	memories, _ := s.GetMemories(stored.ID)
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, "call_timeout") && strings.Contains(m.Content, "timed out") {
			foundTimeout = true
			break
		}
	}

	if !foundTimeout {
		t.Error("tool timeout error not found in memories")
	}
}

func TestMysisToolRetryExhaustionSetsErrored(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("tool-retry-mysis", "mock", "test-model", 0.7)

	mock := provider.NewMock("mock", "Initial response")
	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus, "")

	upstream := &failingUpstream{
		tools:   []mcp.Tool{{Name: "upstream_tool"}},
		callErr: errors.New("upstream unavailable"),
	}
	// Manually set MCP proxy for test (normally done in initializeMCP)
	mysis.mcpProxy = mcp.NewProxy(upstream)

	// Start and wait for initial turn
	events := bus.Subscribe()
	mysis.Start()

	timeout := time.After(5 * time.Second)
	initialFinished := false
	for !initialFinished {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				initialFinished = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial turn")
		}
	}

	// Trigger upstream tool call
	mock.WithToolCalls([]provider.ToolCall{
		{
			ID:        "call_retry",
			Name:      "upstream_tool",
			Arguments: json.RawMessage(`{}`),
		},
	})

	go mysis.SendMessage("Trigger upstream retry", store.MemorySourceDirect)

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if mysis.State() == MysisStateErrored {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if state := mysis.State(); state != MysisStateErrored {
		t.Fatalf("expected state=errored, got %s", state)
	}

	// Verify memory contains retry exhaustion error
	foundRetry := false
	memories, _ := s.GetMemories(stored.ID)
	for _, m := range memories {
		if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, "call_retry") && strings.Contains(m.Content, "failed after retries") {
			foundRetry = true
			break
		}
	}

	if !foundRetry {
		t.Error("tool retry exhaustion error not found in memories")
	}
}
