package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// StubClient is an offline MCP client that returns mock data.
type StubClient struct{}

// NewStubClient creates a new stub MCP client.
func NewStubClient() *StubClient {
	return &StubClient{}
}

// Initialize simulates the MCP handshake.
func (c *StubClient) Initialize(ctx context.Context, clientInfo map[string]interface{}) (*Response, error) {
	return &Response{
		JSONRPC: "2.0",
		ID:      1,
		Result: json.RawMessage(`{
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"serverInfo": {
				"name": "spacemolt-stub",
				"version": "1.0.0"
			}
		}`),
	}, nil
}

// ListTools returns a list of mock tools.
func (c *StubClient) ListTools(ctx context.Context) ([]Tool, error) {
	return []Tool{
		{
			Name:        "get_status",
			Description: "Get player status (stub)",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"session_id": {"type": "string"}}}`),
		},
		{
			Name:        "get_system",
			Description: "Get system info (stub)",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"session_id": {"type": "string"}}}`),
		},
		{
			Name:        "get_ship",
			Description: "Get ship info (stub)",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"session_id": {"type": "string"}}}`),
		},
		{
			Name:        "get_poi",
			Description: "Get POI info (stub)",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {"session_id": {"type": "string"}}}`),
		},
	}, nil
}

// CallTool executes a mock tool call.
func (c *StubClient) CallTool(ctx context.Context, name string, arguments interface{}) (*ToolResult, error) {
	var content string

	switch name {
	case "get_status":
		content = `{
			"player": {
				"id": "stub_player",
				"username": "offline_cmdr",
				"credits": 1000,
				"faction_id": "stub_faction"
			},
			"ship": {
				"id": "stub_ship",
				"name": "Stub Ship",
				"health": 100,
				"fuel": 1000
			}
		}`
	case "get_system":
		content = `{
			"id": "stub_system",
			"name": "Stub System",
			"sector": "0,0",
			"security": 1.0
		}`
	case "get_ship":
		content = `{
			"id": "stub_ship",
			"name": "Stub Ship",
			"class": "scout",
			"modules": [],
			"cargo": []
		}`
	case "get_poi":
		content = `{
			"id": "stub_poi",
			"name": "Stub Station",
			"type": "station",
			"system_id": "stub_system"
		}`
	default:
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("tool %s not implemented in stub", name)}},
			IsError: true,
		}, nil
	}

	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: content}},
	}, nil
}
