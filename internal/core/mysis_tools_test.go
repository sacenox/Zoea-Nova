package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

func TestMysisToolExecution(t *testing.T) {
	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, _ := s.CreateMysis("tool-mysis", "mock", "test-model")

	// Mock provider that returns a tool call first, then a text response
	mock := provider.NewMock("mock", "Initial response")

	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	// Setup MCP proxy with a local tool
	proxy := mcp.NewProxy("")
	proxy.RegisterTool(mcp.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}, func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
		return &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "tool result content"}},
		}, nil
	})
	mysis.SetMCP(proxy)

	events := bus.Subscribe()
	mysis.Start()

	// Wait for initial turn to finish
	timeout := time.After(5 * time.Second)
	initialFinished := false
	for !initialFinished {
		select {
		case e := <-events:
			if e.Type == EventMysisResponse {
				data := e.Data.(MessageData)
				if data.Content == "Initial response" {
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
				data := e.Data.(MessageData)
				if data.Role == "assistant" && data.Content == "Calling tools: test_tool" {
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

	stored, _ := s.CreateMysis("tool-err-mysis", "mock", "test-model")

	mock := provider.NewMock("mock", "Initial response")

	mysis := NewMysis(stored.ID, stored.Name, stored.CreatedAt, mock, s, bus)

	proxy := mcp.NewProxy("")
	proxy.RegisterTool(mcp.Tool{
		Name: "error_tool",
	}, func(ctx context.Context, args json.RawMessage) (*mcp.ToolResult, error) {
		return nil, errors.New("tool failed")
	})
	mysis.SetMCP(proxy)

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
			if m.Role == store.MemoryRoleTool && m.Content == "call_err:Error: tool failed" {
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
