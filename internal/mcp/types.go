// Package mcp provides MCP (Model Context Protocol) client and proxy functionality.
package mcp

import (
	"context"
	"encoding/json"
)

// Request represents an MCP request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents an MCP error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a content block in tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// ListToolsResult is the result of tools/list.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams are parameters for tools/call.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// NewRequest creates a new MCP request.
func NewRequest(id interface{}, method string, params interface{}) (*Request, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		paramsJSON = data
	}

	return &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsJSON,
	}, nil
}

// NewResponse creates a new MCP response.
func NewResponse(id interface{}, result interface{}) (*Response, error) {
	var resultJSON json.RawMessage
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		resultJSON = data
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}, nil
}

// NewErrorResponse creates a new MCP error response.
func NewErrorResponse(id interface{}, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
}

// Standard MCP error codes
const (
	ErrorCodeParseError     = -32700
	ErrorCodeInvalidRequest = -32600
	ErrorCodeMethodNotFound = -32601
	ErrorCodeInvalidParams  = -32602
	ErrorCodeInternalError  = -32603
)

// UpstreamClient defines the interface for an upstream MCP server connection.
type UpstreamClient interface {
	Initialize(ctx context.Context, clientInfo map[string]interface{}) (*Response, error)
	ListTools(ctx context.Context) ([]Tool, error)
	CallTool(ctx context.Context, name string, arguments interface{}) (*ToolResult, error)
}
