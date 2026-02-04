package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolHandler is a function that handles a tool call.
type ToolHandler func(ctx context.Context, arguments json.RawMessage) (*ToolResult, error)

type AccountStore interface {
	CreateAccount(username, password string) (*Account, error)
	MarkAccountInUse(username string) error
	ReleaseAccount(username string) error
	ReleaseAllAccounts() error
}

type Account struct {
	Username string
	Password string
}

// Proxy combines an upstream MCP client with local tool handlers.
type Proxy struct {
	mu            sync.RWMutex
	upstream      *Client
	localTools    map[string]Tool
	localHandlers map[string]ToolHandler
	accountStore  AccountStore
}

// NewProxy creates a new MCP proxy.
func NewProxy(upstreamEndpoint string) *Proxy {
	var upstream *Client
	if upstreamEndpoint != "" {
		upstream = NewClient(upstreamEndpoint)
	}

	return &Proxy{
		upstream:      upstream,
		localTools:    make(map[string]Tool),
		localHandlers: make(map[string]ToolHandler),
	}
}

func (p *Proxy) SetAccountStore(store AccountStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accountStore = store
}

// RegisterTool registers a local tool with the proxy.
func (p *Proxy) RegisterTool(tool Tool, handler ToolHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.localTools[tool.Name] = tool
	p.localHandlers[tool.Name] = handler
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
			// Log but don't fail - upstream may be temporarily unavailable
			// In production, we'd want proper logging here
			_ = err
		} else {
			tools = append(tools, upstreamTools...)
		}
	}

	return tools, nil
}

// CallTool invokes a tool, checking local handlers first then upstream.
func (p *Proxy) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*ToolResult, error) {
	p.mu.RLock()
	handler, isLocal := p.localHandlers[name]
	accountStore := p.accountStore
	p.mu.RUnlock()

	// Try local handler first
	if isLocal {
		return handler(ctx, arguments)
	}

	// Fall back to upstream
	if p.upstream != nil {
		var args interface{}
		if len(arguments) > 0 {
			if err := json.Unmarshal(arguments, &args); err != nil {
				return nil, fmt.Errorf("unmarshal arguments: %w", err)
			}
		}

		result, err := p.upstream.CallTool(ctx, name, args)

		if accountStore != nil && result != nil && !result.IsError {
			p.interceptAuthTools(name, arguments, result)
		}

		return result, err
	}

	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("tool not found: %s", name)}},
		IsError: true,
	}, nil
}

func (p *Proxy) interceptAuthTools(toolName string, arguments json.RawMessage, result *ToolResult) {
	switch toolName {
	case "register":
		p.handleRegisterResponse(arguments, result)
	case "login":
		p.handleLoginResponse(arguments, result)
	case "logout":
		p.handleLogoutResponse(arguments, result)
	}
}

func (p *Proxy) handleRegisterResponse(arguments json.RawMessage, result *ToolResult) {
	var args struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return
	}

	if len(result.Content) == 0 {
		return
	}

	var response struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		return
	}

	if args.Username != "" && response.Password != "" {
		_, _ = p.accountStore.CreateAccount(args.Username, response.Password)
	}
}

func (p *Proxy) handleLoginResponse(arguments json.RawMessage, result *ToolResult) {
	var args struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(arguments, &args); err != nil {
		return
	}

	if args.Username != "" {
		_ = p.accountStore.MarkAccountInUse(args.Username)
	}
}

func (p *Proxy) handleLogoutResponse(arguments json.RawMessage, result *ToolResult) {
	if len(result.Content) == 0 {
		return
	}

	var response struct {
		Player struct {
			Username string `json:"username"`
		} `json:"player"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &response); err != nil {
		return
	}

	if response.Player.Username != "" {
		_ = p.accountStore.ReleaseAccount(response.Player.Username)
	}
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
