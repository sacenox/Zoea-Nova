package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type mockUpstream struct {
	tools     []Tool
	callCount int
	lastName  string
	lastArgs  interface{}
	result    *ToolResult
	err       error
}

func (m *mockUpstream) Initialize(ctx context.Context, clientInfo map[string]interface{}) (*Response, error) {
	return &Response{JSONRPC: "2.0"}, nil
}

func (m *mockUpstream) ListTools(ctx context.Context) ([]Tool, error) {
	return m.tools, nil
}

func (m *mockUpstream) CallTool(ctx context.Context, name string, arguments interface{}) (*ToolResult, error) {
	m.callCount++
	m.lastName = name
	m.lastArgs = arguments
	return m.result, m.err
}

type mockAccountStore struct {
	created  []Account
	marked   []string
	released []string
}

func (m *mockAccountStore) CreateAccount(username, password string, mysisID ...string) (*Account, error) {
	m.created = append(m.created, Account{Username: username, Password: password})
	return &Account{Username: username, Password: password}, nil
}

func (m *mockAccountStore) MarkAccountInUse(username, mysisID string) error {
	m.marked = append(m.marked, username)
	return nil
}

func (m *mockAccountStore) ReleaseAccount(username string) error {
	m.released = append(m.released, username)
	return nil
}

func (m *mockAccountStore) ReleaseAllAccounts() error { return nil }

func TestProxyPrefersLocalTool(t *testing.T) {
	upstream := &mockUpstream{result: &ToolResult{Content: []ContentBlock{{Type: "text", Text: "upstream"}}}}
	proxy := NewProxy(upstream)

	localCalled := false
	proxy.RegisterTool(Tool{Name: "test"}, func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
		localCalled = true
		return &ToolResult{Content: []ContentBlock{{Type: "text", Text: "local"}}}, nil
	})

	result, err := proxy.CallTool(context.Background(), CallerContext{}, "test", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if !localCalled {
		t.Fatal("expected local tool to be called")
	}
	if upstream.callCount != 0 {
		t.Fatalf("expected no upstream calls, got %d", upstream.callCount)
	}
	if len(result.Content) == 0 || result.Content[0].Text != "local" {
		t.Fatalf("unexpected result content: %+v", result.Content)
	}
}

func TestProxyFallsBackToUpstream(t *testing.T) {
	upstream := &mockUpstream{result: &ToolResult{Content: []ContentBlock{{Type: "text", Text: "ok"}}}}
	proxy := NewProxy(upstream)

	args := json.RawMessage(`{"name":"argo"}`)
	_, err := proxy.CallTool(context.Background(), CallerContext{}, "upstream_tool", args)
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if upstream.callCount != 1 {
		t.Fatalf("expected 1 upstream call, got %d", upstream.callCount)
	}
	if upstream.lastName != "upstream_tool" {
		t.Fatalf("expected upstream tool name 'upstream_tool', got %s", upstream.lastName)
	}
	payload, ok := upstream.lastArgs.(map[string]interface{})
	if !ok {
		t.Fatalf("expected args to be map, got %T", upstream.lastArgs)
	}
	if payload["name"] != "argo" {
		t.Fatalf("expected args name=argo, got %v", payload["name"])
	}
}

func TestProxyAuthInterceptionRegister(t *testing.T) {
	upstream := &mockUpstream{result: &ToolResult{Content: []ContentBlock{{Type: "text", Text: `{"password":"secret"}`}}}}
	proxy := NewProxy(upstream)
	accounts := &mockAccountStore{}
	proxy.SetAccountStore(accounts)

	_, err := proxy.CallTool(context.Background(), CallerContext{}, "register", json.RawMessage(`{"username":"pilot"}`))
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if len(accounts.created) != 1 {
		t.Fatalf("expected 1 account created, got %d", len(accounts.created))
	}
	if accounts.created[0].Username != "pilot" || accounts.created[0].Password != "secret" {
		t.Fatalf("unexpected created account: %+v", accounts.created[0])
	}
}

func TestProxyAuthInterceptionLogin(t *testing.T) {
	upstream := &mockUpstream{result: &ToolResult{Content: []ContentBlock{{Type: "text", Text: `{"ok":true}`}}}}
	proxy := NewProxy(upstream)
	accounts := &mockAccountStore{}
	proxy.SetAccountStore(accounts)

	_, err := proxy.CallTool(context.Background(), CallerContext{}, "login", json.RawMessage(`{"username":"pilot"}`))
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if len(accounts.marked) != 1 || accounts.marked[0] != "pilot" {
		t.Fatalf("expected account marked in use for pilot, got %+v", accounts.marked)
	}
}

func TestProxyAuthInterceptionLogout(t *testing.T) {
	upstream := &mockUpstream{result: &ToolResult{Content: []ContentBlock{{Type: "text", Text: `{"player":{"username":"pilot"}}`}}}}
	proxy := NewProxy(upstream)
	accounts := &mockAccountStore{}
	proxy.SetAccountStore(accounts)

	_, err := proxy.CallTool(context.Background(), CallerContext{}, "logout", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}
	if len(accounts.released) != 1 || accounts.released[0] != "pilot" {
		t.Fatalf("expected account released for pilot, got %+v", accounts.released)
	}
}

func TestRewriteSessionError_SessionRequired(t *testing.T) {
	proxy := &Proxy{}

	original := "Error: session_required: You must provide a session_id. Get one by calling login() or register() first."
	rewritten := proxy.rewriteSessionError(original)

	if strings.Contains(rewritten, "Get one by calling login()") {
		t.Error("Should have replaced the instruction to login again")
	}

	if !strings.Contains(rewritten, "Check your recent tool results") {
		t.Error("Should instruct to check recent tool results")
	}
}

func TestRewriteSessionError_SessionInvalid(t *testing.T) {
	proxy := &Proxy{}

	original := "Error: session_invalid: Session not found or expired. Call login() again to get a new session_id."
	rewritten := proxy.rewriteSessionError(original)

	if !strings.Contains(rewritten, "session truly expired") {
		t.Error("Should add clarification about true expiration")
	}
}

func TestRewriteSessionError_Other(t *testing.T) {
	proxy := &Proxy{}

	original := "Error: some_other_error: This is not a session error."
	rewritten := proxy.rewriteSessionError(original)

	if rewritten != original {
		t.Error("Should not modify non-session errors")
	}
}

func TestProxyRewritesUpstreamErrors(t *testing.T) {
	// Simulate upstream returning a session_required error
	upstream := &mockUpstream{
		result: &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Error: session_required: You must provide a session_id. Get one by calling login() or register() first.",
			}},
			IsError: true,
		},
	}
	proxy := NewProxy(upstream)

	result, err := proxy.CallTool(context.Background(), CallerContext{}, "mine", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool() error: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected content in error result")
	}

	rewrittenText := result.Content[0].Text
	if strings.Contains(rewrittenText, "Get one by calling login()") {
		t.Error("Error message should have been rewritten")
	}

	if !strings.Contains(rewrittenText, "Check your recent tool results") {
		t.Error("Error message should contain guidance to check tool results")
	}
}
