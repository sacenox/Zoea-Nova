package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Client is an MCP client that communicates with an upstream server.
type Client struct {
	endpoint        string
	httpClient      *http.Client
	requestID       atomic.Int64
	sessionID       string // Session ID from server, included in subsequent requests
	protocolVersion string // Negotiated protocol version
}

// NewClient creates a new MCP client.
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // 30s timeout to prevent hanging requests
		},
		protocolVersion: "2024-11-05", // Default, may be updated during initialization
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
// Supports both JSON and SSE (Streamable HTTP) responses per MCP spec.
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
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	// Include session ID if we have one (required after initialization)
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	// Include protocol version header
	if c.protocolVersion != "" {
		httpReq.Header.Set("MCP-Protocol-Version", c.protocolVersion)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, &HTTPError{Status: httpResp.StatusCode, Body: string(respBody)}
	}

	// Capture session ID from response if present
	if sessionID := httpResp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
	}

	contentType := httpResp.Header.Get("Content-Type")

	// Handle SSE (Server-Sent Events) response
	if strings.HasPrefix(contentType, "text/event-stream") {
		return c.parseSSEResponse(httpResp.Body)
	}

	// Handle plain JSON response
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &resp, nil
}

// parseSSEResponse parses a Server-Sent Events stream for MCP responses.
func (c *Client) parseSSEResponse(body io.Reader) (*Response, error) {
	scanner := bufio.NewScanner(body)
	maxTokenSize := 2 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// SSE format: "data: {...json...}" or "event: message"
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		} else if line == "" && len(dataLines) > 0 {
			// Empty line marks end of event, try to parse accumulated data
			data := strings.Join(dataLines, "")
			dataLines = nil

			var resp Response
			if err := json.Unmarshal([]byte(data), &resp); err != nil {
				continue // Skip malformed events
			}

			// Return first valid response with our request ID
			if resp.ID != nil {
				return &resp, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read SSE stream: %w", err)
	}

	// If we collected data but no empty line terminated it
	if len(dataLines) > 0 {
		data := strings.Join(dataLines, "")
		var resp Response
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return nil, fmt.Errorf("unmarshal final SSE data: %w", err)
		}
		return &resp, nil
	}

	return nil, fmt.Errorf("no response in SSE stream")
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
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %s", resp.Error.Message)}},
			IsError: true,
		}, nil
	}

	var result ToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return &result, nil
}

// Initialize sends the initialize request to the server and completes the handshake.
func (c *Client) Initialize(ctx context.Context, clientInfo map[string]interface{}) (*Response, error) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      clientInfo,
	}

	var resp *Response
	var err error
	maxRetries := 3
	backoff := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			// Wait with backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		resp, err = c.Call(ctx, "initialize", params)
		if err == nil {
			break
		}
		// Log attempt (using fmt to avoid dependency on logger)
		fmt.Printf("MCP connection attempt %d/%d failed: %v\n", i+1, maxRetries, err)
	}

	if err != nil {
		return nil, fmt.Errorf("initialize failed after %d attempts: %w", maxRetries, err)
	}

	if resp.Error != nil {
		return resp, nil
	}

	// Send initialized notification to complete handshake
	if err := c.Notify(ctx, "notifications/initialized", nil); err != nil {
		return nil, fmt.Errorf("send initialized notification: %w", err)
	}

	return resp, nil
}

// Notify sends a notification (no response expected).
func (c *Client) Notify(ctx context.Context, method string, params interface{}) error {
	// Notifications have no ID
	req := &Request{
		JSONRPC: "2.0",
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal params: %w", err)
		}
		req.Params = data
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	// Include session ID if we have one
	if c.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	// Include protocol version header
	if c.protocolVersion != "" {
		httpReq.Header.Set("MCP-Protocol-Version", c.protocolVersion)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer httpResp.Body.Close()

	// Capture session ID from response if present
	if sessionID := httpResp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
	}

	// Notifications may return 200/202/204, we just check for success
	if httpResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("http error %d: %s", httpResp.StatusCode, string(respBody))
	}

	return nil
}

// Close closes idle HTTP connections
func (c *Client) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}
