package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// MysisInfo represents mysis information returned by the orchestrator.
type MysisInfo struct {
	ID        string
	Name      string
	LastError error
}

// SearchResult represents a search result from memory.
type SearchResult struct {
	Role      string
	Source    string
	Content   string
	CreatedAt string
}

type ReasoningResult struct {
	Role      string
	Source    string
	Content   string
	Reasoning string
	CreatedAt string
}

// BroadcastResult represents a broadcast search result.
type BroadcastResult struct {
	Content   string
	CreatedAt string
}

// AccountInfo represents account credential information.
type AccountInfo struct {
	Username string
	Password string
}

// Orchestrator defines the interface for swarm orchestration.
// This interface breaks the import cycle between mcp and core packages.
type Orchestrator interface {
	ListMyses() []MysisInfo
	MysisCount() int
	MaxMyses() int
	GetStateCounts() map[string]int
	SendMessageAsync(mysisID, message string) error
	BroadcastAsync(message string) error
	BroadcastFrom(senderID, message string) error
	SearchMessages(mysisID, query string, limit int) ([]SearchResult, error)
	SearchReasoning(mysisID, query string, limit int) ([]ReasoningResult, error)
	SearchBroadcasts(query string, limit int) ([]BroadcastResult, error)
	ClaimAccount() (AccountInfo, error)
}

// RegisterOrchestratorTools registers the internal orchestration tools with the proxy.
func RegisterOrchestratorTools(proxy *Proxy, orchestrator Orchestrator) {
	// List myses tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_list_myses",
			Description: "List all myses in the swarm with their current status",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			myses := orchestrator.ListMyses()
			var result []map[string]interface{}
			for _, m := range myses {
				result = append(result, map[string]interface{}{
					"id":   m.ID,
					"name": m.Name,
				})
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	// Swarm status tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_swarm_status",
			Description: "Get overall swarm status and statistics",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			stateCounts := orchestrator.GetStateCounts()

			status := map[string]interface{}{
				"total_myses": orchestrator.MysisCount(),
				"max_myses":   orchestrator.MaxMyses(),
				"states":      stateCounts,
			}

			data, _ := json.MarshalIndent(status, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	// Send message to mysis tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_send_message",
			Description: "Send a message to a specific mysis",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"mysis_id": {"type": "string", "description": "The ID of the mysis"},
					"message": {"type": "string", "description": "The message to send"}
				},
				"required": ["mysis_id", "message"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				MysisID string `json:"mysis_id"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if err := orchestrator.SendMessageAsync(params.MysisID, params.Message); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("failed to send message: %v", err)}},
					IsError: true,
				}, nil
			}

			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "message queued for delivery"}},
			}, nil
		},
	)

	// Broadcast message tool
	proxy.RegisterToolWithContext(
		Tool{
			Name:        "zoea_broadcast",
			Description: "Send a message to all running myses (you will not receive your own broadcast)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message": {"type": "string", "description": "The message to broadcast"}
				},
				"required": ["message"]
			}`),
		},
		func(ctx context.Context, caller CallerContext, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if err := orchestrator.BroadcastFrom(caller.MysisID, params.Message); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("broadcast failed: %v", err)}},
					IsError: true,
				}, nil
			}

			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "broadcast sent to all running myses (excluding you)"}},
			}, nil
		},
	)

	// Search messages tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_search_messages",
			Description: "Search your past messages (direct messages, responses, tool results) by text content",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"mysis_id": {"type": "string", "description": "The ID of the mysis whose messages to search"},
					"query": {"type": "string", "description": "Text to search for in message content"},
					"limit": {"type": "integer", "description": "Maximum results to return (default 20, max 100)"}
				},
				"required": ["mysis_id", "query"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				MysisID string `json:"mysis_id"`
				Query   string `json:"query"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if params.Query == "" {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: "query cannot be empty"}},
					IsError: true,
				}, nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 20
			} else if limit > 100 {
				limit = 100
			}

			results, err := orchestrator.SearchMessages(params.MysisID, params.Query, limit)
			if err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("search failed: %v", err)}},
					IsError: true,
				}, nil
			}

			if len(results) == 0 {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("no messages found matching '%s'", params.Query)}},
				}, nil
			}

			data, _ := json.MarshalIndent(results, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	proxy.RegisterTool(
		Tool{
			Name:        "zoea_search_reasoning",
			Description: "Search your past reasoning content by text",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"mysis_id": {"type": "string", "description": "The ID of the mysis whose reasoning to search"},
					"query": {"type": "string", "description": "Text to search for in reasoning content"},
					"limit": {"type": "integer", "description": "Maximum results to return (default 20, max 100)"}
				},
				"required": ["mysis_id", "query"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				MysisID string `json:"mysis_id"`
				Query   string `json:"query"`
				Limit   int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if params.Query == "" {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: "query cannot be empty"}},
					IsError: true,
				}, nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 20
			} else if limit > 100 {
				limit = 100
			}

			results, err := orchestrator.SearchReasoning(params.MysisID, params.Query, limit)
			if err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("search failed: %v", err)}},
					IsError: true,
				}, nil
			}

			if len(results) == 0 {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("no reasoning found matching '%s'", params.Query)}},
				}, nil
			}

			data, _ := json.MarshalIndent(results, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	// Search broadcasts tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_search_broadcasts",
			Description: "Search past swarm broadcasts by text content",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Text to search for in broadcast content"},
					"limit": {"type": "integer", "description": "Maximum results to return (default 20, max 100)"}
				},
				"required": ["query"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if params.Query == "" {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: "query cannot be empty"}},
					IsError: true,
				}, nil
			}

			limit := params.Limit
			if limit <= 0 {
				limit = 20
			} else if limit > 100 {
				limit = 100
			}

			results, err := orchestrator.SearchBroadcasts(params.Query, limit)
			if err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("search failed: %v", err)}},
					IsError: true,
				}, nil
			}

			if len(results) == 0 {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("no broadcasts found matching '%s'", params.Query)}},
				}, nil
			}

			data, _ := json.MarshalIndent(results, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	// Claim account tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_claim_account",
			Description: "Get existing credentials from swarm pool. Returns username and password to use with the game's login tool. No arguments needed.",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			account, err := orchestrator.ClaimAccount()
			if err != nil {
				// Check if no accounts available
				if err.Error() == "no accounts available" {
					return &ToolResult{
						Content: []ContentBlock{{Type: "text", Text: "No accounts available. Use the game's register tool to create a new account with a Crustacean-themed username."}},
					}, nil
				}
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("failed to claim account: %v", err)}},
					IsError: true,
				}, nil
			}

			message := fmt.Sprintf("Use the game's login tool with these credentials:\nusername: %s\npassword: %s", account.Username, account.Password)
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: message}},
			}, nil
		},
	)
}
