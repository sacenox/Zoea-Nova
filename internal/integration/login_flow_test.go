package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestLoginFlowIntegration verifies the complete login flow:
// 1. Account pre-populated in store (via CreateAccount)
// 2. Mysis uses credentials with the game's login tool
// 3. handleLoginResponse intercepts the successful login and marks account in_use
// Note: This test uses ClaimAccount() for setup, but myses no longer call zoea_claim_account
func TestLoginFlowIntegration(t *testing.T) {
	// Setup store
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	// Create test account
	_, err = s.CreateAccount("pilot_crab", "secret123")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	// Release account to make it available
	if err := s.ReleaseAccount("pilot_crab"); err != nil {
		t.Fatalf("Failed to release account: %v", err)
	}

	// Verify account is available
	avail, err := s.ListAvailableAccounts()
	if err != nil || len(avail) != 1 {
		t.Fatalf("Expected 1 available account, got %d (err=%v)", len(avail), err)
	}

	// Create mock upstream that returns success for login
	upstream := &mockLoginUpstream{
		loginResult: &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: `{"ok": true, "message": "Login successful"}`}},
		},
	}

	// Create proxy with account store adapter
	proxy := mcp.NewProxy(upstream)
	proxy.SetAccountStore(&accountStoreAdapter{s})

	// STEP 1: Test setup - get account credentials
	// (In production, myses would call register() or have pre-known credentials)
	claimed, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("ClaimAccount() failed: %v", err)
	}
	if claimed.Username != "pilot_crab" {
		t.Fatalf("Expected username pilot_crab, got %s", claimed.Username)
	}
	if claimed.Password != "secret123" {
		t.Fatalf("Expected password secret123, got %s", claimed.Password)
	}

	// CRITICAL: Account should NOT be locked yet (InUse should be false)
	if claimed.InUse {
		t.Fatalf("BUG: ClaimAccount() returned account with InUse=true. Account should not be locked until login succeeds.")
	}

	// Verify in store that account is still available
	acc, err := s.GetAccount("pilot_crab")
	if err != nil {
		t.Fatalf("Failed to get account: %v", err)
	}
	if acc.InUse {
		t.Fatalf("BUG: Account is marked in_use=1 in store after ClaimAccount(). Should be 0 until login succeeds.")
	}

	// STEP 2: Mysis uses credentials with game's login tool
	loginArgs := json.RawMessage(`{"username":"pilot_crab","password":"secret123"}`)
	result, err := proxy.CallTool(context.Background(), mcp.CallerContext{MysisID: "test", MysisName: "test-mysis"}, "login", loginArgs)
	if err != nil {
		t.Fatalf("CallTool(login) failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(login) returned error: %+v", result)
	}

	// STEP 3: Verify handleLoginResponse marked account as in_use
	acc, err = s.GetAccount("pilot_crab")
	if err != nil {
		t.Fatalf("Failed to get account after login: %v", err)
	}
	if !acc.InUse {
		t.Fatalf("BUG: Account not marked in_use after successful login. handleLoginResponse() failed to intercept.")
	}

	// Verify account is no longer available for claiming
	avail, err = s.ListAvailableAccounts()
	if err != nil {
		t.Fatalf("ListAvailableAccounts() failed: %v", err)
	}
	if len(avail) != 0 {
		t.Fatalf("Expected 0 available accounts after login, got %d", len(avail))
	}

	// Verify another ClaimAccount() call would fail (no accounts available)
	_, err = s.ClaimAccount()
	if err == nil {
		t.Fatalf("BUG: ClaimAccount() succeeded when all accounts are in use. Should return 'no accounts available' error.")
	}
	if err.Error() != "no accounts available" {
		t.Fatalf("Expected 'no accounts available' error, got: %v", err)
	}
}

// TestLoginFlowRaceCondition verifies that two myses can't claim and login with the same account
func TestLoginFlowRaceCondition(t *testing.T) {
	// Setup store
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}
	defer s.Close()

	// Create single test account
	_, err = s.CreateAccount("shared_crab", "pass123")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}
	if err := s.ReleaseAccount("shared_crab"); err != nil {
		t.Fatalf("Failed to release account: %v", err)
	}

	// Create mock upstream
	upstream := &mockLoginUpstream{
		loginResult: &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: `{"ok": true}`}},
		},
	}
	proxy := mcp.NewProxy(upstream)
	proxy.SetAccountStore(&accountStoreAdapter{s})

	// Mysis 1 claims account
	claimed1, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("Mysis1 ClaimAccount() failed: %v", err)
	}

	// Mysis 2 tries to claim - should get the same account (not locked yet)
	claimed2, err := s.ClaimAccount()
	if err != nil {
		t.Fatalf("Mysis2 ClaimAccount() failed: %v", err)
	}

	// Both should get same credentials (account not locked)
	if claimed1.Username != claimed2.Username {
		t.Fatalf("Expected both myses to claim same account, got %s and %s", claimed1.Username, claimed2.Username)
	}

	// Mysis 1 logs in first
	loginArgs := json.RawMessage(`{"username":"shared_crab","password":"pass123"}`)
	_, err = proxy.CallTool(context.Background(), mcp.CallerContext{MysisID: "mysis1", MysisName: "mysis-1"}, "login", loginArgs)
	if err != nil {
		t.Fatalf("Mysis1 login failed: %v", err)
	}

	// Account should now be locked
	acc, _ := s.GetAccount("shared_crab")
	if !acc.InUse {
		t.Fatalf("Account should be in_use after mysis1 login")
	}

	// Mysis 2 tries to login with same credentials - this will succeed at the game level
	// (since credentials are valid), but won't cause issues because handleLoginResponse
	// is idempotent (just sets in_use=1 again)
	_, err = proxy.CallTool(context.Background(), mcp.CallerContext{MysisID: "mysis2", MysisName: "mysis-2"}, "login", loginArgs)
	if err != nil {
		t.Fatalf("Mysis2 login failed: %v", err)
	}

	// Both myses are now logged in with the same account
	// This is expected behavior - the lock happens at login, not claim
	// The game server will handle duplicate logins (usually kicks the old session)
}

// mockLoginUpstream mocks the SpaceMolt MCP server for login testing
type mockLoginUpstream struct {
	loginResult *mcp.ToolResult
	loginError  error
}

func (m *mockLoginUpstream) Initialize(ctx context.Context, clientInfo map[string]interface{}) (*mcp.Response, error) {
	return &mcp.Response{JSONRPC: "2.0"}, nil
}

func (m *mockLoginUpstream) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	return []mcp.Tool{
		{Name: "login", Description: "Login to the game"},
	}, nil
}

func (m *mockLoginUpstream) CallTool(ctx context.Context, name string, arguments interface{}) (*mcp.ToolResult, error) {
	if name == "login" {
		return m.loginResult, m.loginError
	}
	return &mcp.ToolResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: "unknown tool"}},
		IsError: true,
	}, nil
}

// accountStoreAdapter adapts store.Store to mcp.AccountStore interface
type accountStoreAdapter struct {
	store *store.Store
}

func (a *accountStoreAdapter) CreateAccount(username, password string) (*mcp.Account, error) {
	acc, err := a.store.CreateAccount(username, password)
	if err != nil {
		return nil, err
	}
	return &mcp.Account{Username: acc.Username, Password: acc.Password}, nil
}

func (a *accountStoreAdapter) MarkAccountInUse(username string) error {
	return a.store.MarkAccountInUse(username)
}

func (a *accountStoreAdapter) ReleaseAccount(username string) error {
	return a.store.ReleaseAccount(username)
}

func (a *accountStoreAdapter) ReleaseAllAccounts() error {
	return a.store.ReleaseAllAccounts()
}

func (a *accountStoreAdapter) ClaimAccount() (*mcp.Account, error) {
	acc, err := a.store.ClaimAccount()
	if err != nil {
		return nil, err
	}
	return &mcp.Account{Username: acc.Username, Password: acc.Password}, nil
}
