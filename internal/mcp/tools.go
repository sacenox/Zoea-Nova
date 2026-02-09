package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

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

// Orchestrator defines the interface for swarm orchestration.
// This interface breaks the import cycle between mcp and core packages.
type Orchestrator interface {
	MysisCount() int
	MaxMyses() int
	GetStateCounts() map[string]int
	SendMessageAsync(mysisID, message string) error
	BroadcastAsync(message string) error
	BroadcastFrom(senderID, message string) error
	SearchMessages(mysisID, query string, limit int) ([]SearchResult, error)
	SearchReasoning(mysisID, query string, limit int) ([]ReasoningResult, error)
}

// RegisterOrchestratorTools registers the internal orchestration tools with the proxy.
func RegisterOrchestratorTools(proxy *Proxy, orchestrator Orchestrator) {
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
}
