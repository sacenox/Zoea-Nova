package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// Client is an MCP client that communicates with an upstream server.
type Client struct {
	endpoint   string
	httpClient *http.Client
	requestID  atomic.Int64
}

// NewClient creates a new MCP client.
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 0, // No timeout, MCP requests can be long-running
		},
	}
}

// nextID returns the next request ID.
func (c *Client) nextID() int64 {
	return c.requestID.Add(1)
}

// Call makes an MCP request and returns the response.
func (c *Client) Call(ctx context.Context, method string, params interface{}) (*Response, error) {
	req, err := NewRequest(c.nextID(), method, params)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	return c.send(ctx, req)
}

// send sends a request and receives a response.
func (c *Client) send(ctx context.Context, req *Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// ListTools requests the list of available tools from the server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	resp, err := c.Call(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result ListToolsResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}

	return result.Tools, nil
}

// CallTool invokes a tool on the server.
func (c *Client) CallTool(ctx context.Context, name string, arguments interface{}) (*ToolResult, error) {
	var argsJSON json.RawMessage
	if arguments != nil {
		data, err := json.Marshal(arguments)
		if err != nil {
			return nil, fmt.Errorf("marshal arguments: %w", err)
		}
		argsJSON = data
	}

	params := CallToolParams{
		Name:      name,
		Arguments: argsJSON,
	}

	resp, err := c.Call(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("mcp error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	var result ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &result, nil
}

// Initialize sends the initialize request to the server.
func (c *Client) Initialize(ctx context.Context, clientInfo map[string]interface{}) (*Response, error) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      clientInfo,
	}
	return c.Call(ctx, "initialize", params)
}
