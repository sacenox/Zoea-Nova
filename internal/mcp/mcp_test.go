package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRequest(t *testing.T) {
	req, err := NewRequest(1, "tools/list", nil)
	if err != nil {
		t.Fatalf("NewRequest() error: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", req.JSONRPC)
	}
	if req.Method != "tools/list" {
		t.Errorf("expected method=tools/list, got %s", req.Method)
	}
}

func TestNewResponse(t *testing.T) {
	result := map[string]string{"foo": "bar"}
	resp, err := NewResponse(1, result)
	if err != nil {
		t.Fatalf("NewResponse() error: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", resp.JSONRPC)
	}
	if resp.Error != nil {
		t.Error("expected no error")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(1, ErrorCodeMethodNotFound, "method not found")

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != ErrorCodeMethodNotFound {
		t.Errorf("expected code=%d, got %d", ErrorCodeMethodNotFound, resp.Error.Code)
	}
}

func TestProxyLocalTools(t *testing.T) {
	proxy := NewProxy("") // No upstream

	// Register a local tool
	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: json.RawMessage(`{"type": "object"}`),
	}
	handler := func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
		return &ToolResult{
			Content: []ContentBlock{{Type: "text", Text: "test result"}},
		}, nil
	}

	proxy.RegisterTool(tool, handler)

	if proxy.LocalToolCount() != 1 {
		t.Errorf("expected 1 local tool, got %d", proxy.LocalToolCount())
	}

	// List tools
	ctx := context.Background()
	tools, err := proxy.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	// Call tool
	result, err := proxy.CallTool(ctx, "test_tool", nil)
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if result.IsError {
		t.Error("expected no error in result")
	}
	if len(result.Content) != 1 || result.Content[0].Text != "test result" {
		t.Errorf("unexpected result content: %+v", result.Content)
	}
}

func TestProxyToolNotFound(t *testing.T) {
	proxy := NewProxy("") // No upstream

	ctx := context.Background()
	result, err := proxy.CallTool(ctx, "nonexistent", nil)
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error in result")
	}
}

func TestProxyHasUpstream(t *testing.T) {
	proxyNoUpstream := NewProxy("")
	if proxyNoUpstream.HasUpstream() {
		t.Error("expected no upstream")
	}

	proxyWithUpstream := NewProxy("http://example.com/mcp")
	if !proxyWithUpstream.HasUpstream() {
		t.Error("expected upstream")
	}
}

func TestClientWithMockServer(t *testing.T) {
	// Create a mock MCP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		var resp Response
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "tools/list":
			result := ListToolsResult{
				Tools: []Tool{
					{Name: "mock_tool", Description: "A mock tool"},
				},
			}
			data, _ := json.Marshal(result)
			resp.Result = data
		case "tools/call":
			result := ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "mock response"}},
			}
			data, _ := json.Marshal(result)
			resp.Result = data
		default:
			resp.Error = &Error{Code: ErrorCodeMethodNotFound, Message: "method not found"}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	// Test ListTools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools() error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "mock_tool" {
		t.Errorf("unexpected tools: %+v", tools)
	}

	// Test CallTool
	result, err := client.CallTool(ctx, "mock_tool", nil)
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "mock response" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// mockOrchestrator is a test implementation of the Orchestrator interface.
type mockOrchestrator struct {
	myses []MysisInfo
}

func (m *mockOrchestrator) ListMyses() []MysisInfo {
	return m.myses
}

func (m *mockOrchestrator) GetMysis(id string) (MysisInfo, error) {
	for _, mysis := range m.myses {
		if mysis.ID == id {
			return mysis, nil
		}
	}
	return MysisInfo{}, errors.New("mysis not found")
}

func (m *mockOrchestrator) MysisCount() int {
	return len(m.myses)
}

func (m *mockOrchestrator) MaxMyses() int {
	return 16
}

func (m *mockOrchestrator) SendMessageAsync(mysisID, message string) error {
	for _, mysis := range m.myses {
		if mysis.ID == mysisID {
			return nil
		}
	}
	return errors.New("mysis not found")
}

func (m *mockOrchestrator) BroadcastAsync(message string) error {
	return nil
}

func (m *mockOrchestrator) SearchMessages(mysisID, query string, limit int) ([]SearchResult, error) {
	return []SearchResult{}, nil
}

func (m *mockOrchestrator) SearchBroadcasts(query string, limit int) ([]BroadcastResult, error) {
	return []BroadcastResult{}, nil
}

func TestOrchestratorTools(t *testing.T) {
	// Create mock orchestrator
	orchestrator := &mockOrchestrator{
		myses: []MysisInfo{
			{ID: "mysis-1", Name: "test-mysis", State: "running", Provider: "mock"},
		},
	}

	// Create proxy and register tools
	proxy := NewProxy("")
	RegisterOrchestratorTools(proxy, orchestrator)

	// Should have 7 orchestrator tools registered
	if proxy.LocalToolCount() != 7 {
		t.Errorf("expected 7 local tools, got %d", proxy.LocalToolCount())
	}

	ctx := context.Background()

	// Test zoea_swarm_status
	result, err := proxy.CallTool(ctx, "zoea_swarm_status", nil)
	if err != nil {
		t.Fatalf("CallTool(zoea_swarm_status) error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content[0].Text)
	}

	// Test zoea_list_myses
	result, err = proxy.CallTool(ctx, "zoea_list_myses", nil)
	if err != nil {
		t.Fatalf("CallTool(zoea_list_myses) error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content[0].Text)
	}

	// Test zoea_get_mysis
	args, _ := json.Marshal(map[string]string{"mysis_id": "mysis-1"})
	result, err = proxy.CallTool(ctx, "zoea_get_mysis", args)
	if err != nil {
		t.Fatalf("CallTool(zoea_get_mysis) error: %v", err)
	}
	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content[0].Text)
	}

	// Test zoea_get_mysis with invalid ID
	args, _ = json.Marshal(map[string]string{"mysis_id": "nonexistent"})
	result, _ = proxy.CallTool(ctx, "zoea_get_mysis", args)
	if !result.IsError {
		t.Error("expected error for nonexistent mysis")
	}
}
