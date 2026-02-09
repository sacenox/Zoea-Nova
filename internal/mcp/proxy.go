package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ToolHandler is a function that handles a tool call.
type ToolHandler func(ctx context.Context, arguments json.RawMessage) (*ToolResult, error)

// CallerContext provides information about who is calling a tool.
type CallerContext struct {
	MysisID   string
	MysisName string
}

// ToolHandlerWithContext is a function that handles a tool call with caller context.
type ToolHandlerWithContext func(ctx context.Context, caller CallerContext, arguments json.RawMessage) (*ToolResult, error)

type AccountStore interface {
	CreateAccount(username, password string, mysisID ...string) (*Account, error)
	GetAccountByMysisID(mysisID string) (*Account, error)
	AssignAccount(username, mysisID string) error
	ReleaseAccount(username string) error
	ReleaseAccountByMysisID(mysisID string) error
	ReleaseAllAccounts() error
}

type Account struct {
	Username string
	Password string
}

type GameStateStore interface {
	StoreGameStateSnapshot(username, toolName, content string, gameTick int64) error
	DeleteGameStateSnapshotsForUsername(username string) error
}

// Proxy combines an upstream MCP client with local tool handlers.
type Proxy struct {
	mu              sync.RWMutex
	upstream        UpstreamClient
	localTools      map[string]Tool
	localHandlers   map[string]ToolHandler
	contextHandlers map[string]ToolHandlerWithContext
	accountStore    AccountStore
	gameStateStore  GameStateStore
}

var (
	ErrToolRetryExhausted = errors.New("mcp tool call failed after retries")
)

// Retry delays increased to respect SpaceMolt's "Try again in 5 seconds" rate limit
var toolRetryDelays = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// retryAfterRegex matches "Retry-After: N" in error messages
var retryAfterRegex = regexp.MustCompile(`Retry-After:\s*(\d+)`)

// parseRetryAfter extracts retry delay from error message containing "Retry-After: N seconds"
func parseRetryAfter(err error) (time.Duration, bool) {
	if err == nil {
		return 0, false
	}

	errMsg := err.Error()

	// Try to extract from "Retry-After: N" header format
	if matches := retryAfterRegex.FindStringSubmatch(errMsg); len(matches) > 1 {
		if seconds, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
			return time.Duration(seconds) * time.Second, true
		}
	}

	// Try to extract from "Try again in N seconds" message format
	if strings.Contains(errMsg, "Try again in") {
		re := regexp.MustCompile(`Try again in (\d+) seconds?`)
		if matches := re.FindStringSubmatch(errMsg); len(matches) > 1 {
			if seconds, parseErr := strconv.Atoi(matches[1]); parseErr == nil {
				return time.Duration(seconds) * time.Second, true
			}
		}
	}

	return 0, false
}

// NewProxy creates a new MCP proxy.
func NewProxy(upstream UpstreamClient) *Proxy {
	return &Proxy{
		upstream:        upstream,
		localTools:      make(map[string]Tool),
		localHandlers:   make(map[string]ToolHandler),
		contextHandlers: make(map[string]ToolHandlerWithContext),
	}
}

func (p *Proxy) SetAccountStore(store AccountStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accountStore = store
}

func (p *Proxy) SetGameStateStore(store GameStateStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gameStateStore = store
}

// RegisterTool registers a local tool with the proxy.
func (p *Proxy) RegisterTool(tool Tool, handler ToolHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.localTools[tool.Name] = tool
	p.localHandlers[tool.Name] = handler
}

// RegisterToolWithContext registers a tool handler that receives caller context.
func (p *Proxy) RegisterToolWithContext(tool Tool, handler ToolHandlerWithContext) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.localTools[tool.Name] = tool
	p.contextHandlers[tool.Name] = handler
}

// ListTools returns all available tools (local + upstream).
func (p *Proxy) ListTools(ctx context.Context) ([]Tool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Start with local tools
	tools := make([]Tool, 0, len(p.localTools))
	for _, t := range p.localTools {
		tools = append(tools, t)
	}

	// Add upstream tools if available
	if p.upstream != nil {
		upstreamTools, err := p.upstream.ListTools(ctx)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("failed to list upstream tools")
		} else {
			tools = append(tools, upstreamTools...)
		}
	}

	return tools, nil
}

// CallTool invokes a tool, checking local handlers first then upstream.
func (p *Proxy) CallTool(ctx context.Context, caller CallerContext, name string, arguments json.RawMessage) (*ToolResult, error) {
	p.mu.RLock()
	handler, isLocal := p.localHandlers[name]
	contextHandler, hasContext := p.contextHandlers[name]
	accountStore := p.accountStore
	p.mu.RUnlock()

	if hasContext {
		return contextHandler(ctx, caller, arguments)
	}

	// Try local handler first
	if isLocal {
		return handler(ctx, arguments)
	}

	// Fall back to upstream
	if p.upstream != nil {
		// Intercept register - check if mysis already has an assigned account
		if name == "register" && accountStore != nil {
			// Check if mysis already has a permanently assigned account
			if assignedAccount, err := accountStore.GetAccountByMysisID(caller.MysisID); err == nil && assignedAccount != nil {
				// Mysis already has an account - return error
				errorMsg := fmt.Sprintf("Already have account. Use login(username=\"%s\", password=\"...\").", assignedAccount.Username)
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: errorMsg}},
					IsError: true,
				}, nil
			}

			// Try to claim an available account from pool
			if poolAccount := p.tryClaimPoolAccount(caller.MysisID); poolAccount != nil {
				// Account claimed - login with pool account to get real session
				loginArgs := map[string]interface{}{
					"username": poolAccount.Username,
					"password": poolAccount.Password,
				}
				result, err := p.callUpstreamWithRetry(ctx, "login", loginArgs)
				if result != nil && !result.IsError {
					// Transform login response to look like register response
					// (inject password field so mysis can extract it)
					p.injectPasswordIntoResult(result, poolAccount.Password)
				}
				return result, err
			}
		}

		// Intercept login - substitute credentials with assigned account if mysis has one
		if name == "login" && accountStore != nil {
			if assignedAccount, err := accountStore.GetAccountByMysisID(caller.MysisID); err == nil && assignedAccount != nil {
				// Mysis has assigned account - substitute credentials
				loginArgs := map[string]interface{}{
					"username": assignedAccount.Username,
					"password": assignedAccount.Password,
				}
				result, err := p.callUpstreamWithRetry(ctx, "login", loginArgs)
				return result, err
			}
		}

		var args interface{}
		if len(arguments) > 0 {
			if err := json.Unmarshal(arguments, &args); err != nil {
				return nil, fmt.Errorf("unmarshal arguments: %w", err)
			}
		}

		result, err := p.callUpstreamWithRetry(ctx, name, args)

		if result != nil {
			if !result.IsError && accountStore != nil {
				p.interceptAuthTools(name, arguments, result, caller.MysisID)
			}
			if result.IsError {
				// Rewrite error messages to guide myses toward correct behavior
				for i := range result.Content {
					if result.Content[i].Type == "text" {
						result.Content[i].Text = p.rewriteSessionError(result.Content[i].Text)
					}
				}
			}
		}

		return result, err
	}

	errorMsg := fmt.Sprintf("tool not found: %s", name)
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: p.rewriteSessionError(errorMsg)}},
		IsError: true,
	}, nil
}

func (p *Proxy) callUpstreamWithRetry(ctx context.Context, name string, args interface{}) (*ToolResult, error) {
	var lastErr error
	for attempt := 0; attempt <= len(toolRetryDelays); attempt++ {
		if attempt > 0 {
			delay := toolRetryDelays[attempt-1]

			// Check if error is a 429 rate limit
			is429 := lastErr != nil && (strings.Contains(lastErr.Error(), "429") || strings.Contains(lastErr.Error(), "Rate limited"))

			// Try to parse Retry-After from error message
			if retryAfter, ok := parseRetryAfter(lastErr); ok {
				// Use server-specified delay, but cap at 30 seconds for safety
				if retryAfter > 30*time.Second {
					retryAfter = 30 * time.Second
				}
				delay = retryAfter

				log.Warn().
					Str("tool", name).
					Int("attempt", attempt).
					Dur("delay", delay).
					Dur("server_requested", retryAfter).
					Err(lastErr).
					Msg("MCP tool rate limited - respecting Retry-After")
			} else if is429 {
				log.Warn().
					Str("tool", name).
					Int("attempt", attempt).
					Dur("delay", delay).
					Err(lastErr).
					Msg("MCP tool rate limited - waiting before retry")
			} else {
				log.Warn().
					Str("tool", name).
					Int("attempt", attempt).
					Dur("delay", delay).
					Msg("Retrying MCP tool call after error")
			}

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		result, err := p.upstream.CallTool(ctx, name, args)
		if err == nil {
			// Log successful call at Info level for visibility
			if attempt > 0 {
				log.Info().
					Str("tool", name).
					Int("attempt", attempt+1).
					Msg("MCP tool call succeeded after retry")
			}
			return result, nil
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		lastErr = err
	}

	// Log final failure with more context
	log.Error().
		Str("tool", name).
		Int("total_attempts", len(toolRetryDelays)+1).
		Err(lastErr).
		Msg("MCP tool call failed after all retries")

	return nil, fmt.Errorf("%w: %v", ErrToolRetryExhausted, lastErr)
}

func (p *Proxy) interceptAuthTools(toolName string, arguments json.RawMessage, result *ToolResult, mysisID string) {
	switch toolName {
	case "register":
		p.handleRegisterResponse(arguments, result, mysisID)
	case "login":
		p.handleLoginResponse(arguments, result, mysisID)
	case "logout":
		p.handleLogoutResponse(arguments, result)
	}
}

func (p *Proxy) handleRegisterResponse(arguments json.RawMessage, result *ToolResult, mysisID string) {
	var args struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return
	}

	payload, ok := parseToolResultPayload(result)
	if !ok {
		return
	}

	password, ok := findStringField(payload, "password", "token")
	if args.Username != "" && ok && password != "" {
		_, _ = p.accountStore.CreateAccount(args.Username, password, mysisID)
	}
}

func (p *Proxy) handleLoginResponse(arguments json.RawMessage, result *ToolResult, mysisID string) {
	var args struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return
	}

	if args.Username != "" {
		_ = p.accountStore.AssignAccount(args.Username, mysisID)
	}
}

func (p *Proxy) handleLogoutResponse(arguments json.RawMessage, result *ToolResult) {
	payload, ok := parseToolResultPayload(result)
	if !ok {
		return
	}

	username, ok := findStringFieldAtPath(payload, "player", "username")
	if !ok {
		username, _ = findStringField(payload, "username")
	}

	if username != "" {
		// Clear game state cache on logout (but keep account assignment)
		if p.gameStateStore != nil {
			_ = p.gameStateStore.DeleteGameStateSnapshotsForUsername(username)
		}
	}
}

// injectPasswordIntoResult adds the password field to a tool result payload.
// This is used when substituting register with pool account login, since login
// responses don't include the password but the mysis expects it (to match register behavior).
func (p *Proxy) injectPasswordIntoResult(result *ToolResult, password string) {
	if result == nil || len(result.Content) == 0 {
		return
	}

	// Find the text content block with JSON payload
	for i := range result.Content {
		if result.Content[i].Type == "text" {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(result.Content[i].Text), &data); err == nil {
				// Add password to the payload
				data["password"] = password
				// Re-marshal and update the content
				if updatedJSON, err := json.Marshal(data); err == nil {
					result.Content[i].Text = string(updatedJSON)
				}
				return
			}
		}
	}
}

// tryClaimPoolAccount attempts to claim an available account from the pool.
// Returns the account if one is available, nil otherwise.
func (p *Proxy) tryClaimPoolAccount(mysisID string) *Account {
	p.mu.RLock()
	accountStore := p.accountStore
	p.mu.RUnlock()

	if accountStore == nil {
		return nil
	}

	// Try to claim an account from the pool
	type accountClaimer interface {
		ClaimAccount(mysisID string) (*Account, error)
	}

	if claimer, ok := accountStore.(accountClaimer); ok {
		account, err := claimer.ClaimAccount(mysisID)
		if err == nil && account != nil {
			return account
		}
	}

	return nil
}

func parseToolResultPayload(result *ToolResult) (interface{}, bool) {
	if result == nil {
		return nil, false
	}

	content := strings.TrimSpace(joinToolResultText(result))
	if content == "" {
		return nil, false
	}
	if !strings.HasPrefix(content, "{") && !strings.HasPrefix(content, "[") {
		return nil, false
	}

	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	var payload interface{}
	if err := decoder.Decode(&payload); err != nil {
		return nil, false
	}

	return payload, true
}

func joinToolResultText(result *ToolResult) string {
	if result == nil {
		return ""
	}

	var parts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func findStringField(payload interface{}, keys ...string) (string, bool) {
	queue := []interface{}{payload}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		switch value := current.(type) {
		case map[string]interface{}:
			for _, key := range keys {
				if raw, ok := value[key]; ok {
					if str, ok := raw.(string); ok {
						return str, true
					}
				}
			}

			for _, child := range value {
				queue = append(queue, child)
			}
		case []interface{}:
			queue = append(queue, value...)
		}
	}

	return "", false
}

func findStringFieldAtPath(payload interface{}, path ...string) (string, bool) {
	current, ok := payload.(map[string]interface{})
	if !ok {
		return "", false
	}

	for i, key := range path {
		value, exists := current[key]
		if !exists {
			return "", false
		}
		if i == len(path)-1 {
			str, ok := value.(string)
			return str, ok
		}
		next, ok := value.(map[string]interface{})
		if !ok {
			return "", false
		}
		current = next
	}

	return "", false
}

// rewriteSessionError improves session-related error messages to guide myses
// toward correct behavior instead of causing claim→login loops.
func (p *Proxy) rewriteSessionError(errorMsg string) string {
	// Handle session_required errors
	if strings.Contains(errorMsg, "session_required") {
		// Original: "You must provide a session_id. Get one by calling login() or register() first."
		// Problem: Tells mysis to login again even if they already have session_id
		return strings.Replace(errorMsg,
			"Get one by calling login() or register() first.",
			"Check your recent tool results for session_id from login/register and use it as a parameter.",
			1)
	}

	// Handle session_invalid errors
	if strings.Contains(errorMsg, "session_invalid") {
		// Original: "Call login() again to get a new session_id."
		// This is actually correct - session truly expired
		// But add clarity about when this happens
		if strings.Contains(errorMsg, "Session not found or expired") {
			return errorMsg + " This means your session truly expired (server restart, timeout, or duplicate login)."
		}
	}

	return errorMsg
}

// Initialize initializes the upstream connection if available.
func (p *Proxy) Initialize(ctx context.Context) error {
	if p.upstream == nil {
		return nil
	}

	clientInfo := map[string]interface{}{
		"name":    "zoea-nova",
		"version": "0.1.0",
	}

	resp, err := p.upstream.Initialize(ctx, clientInfo)
	if err != nil {
		return fmt.Errorf("initialize upstream: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("upstream error: %s", resp.Error.Message)
	}

	return nil
}

// HasUpstream returns true if an upstream client is configured.
func (p *Proxy) HasUpstream() bool {
	return p.upstream != nil
}

// LocalToolCount returns the number of registered local tools.
func (p *Proxy) LocalToolCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.localTools)
}

// Close closes the upstream client connection if available.
func (p *Proxy) Close() error {
	p.mu.RLock()
	upstream := p.upstream
	p.mu.RUnlock()

	if upstream != nil {
		if closer, ok := upstream.(interface{ Close() error }); ok {
			return closer.Close()
		}
	}
	return nil
}
