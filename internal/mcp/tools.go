package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// AgentInfo represents agent information returned by the orchestrator.
type AgentInfo struct {
	ID        string
	Name      string
	State     string
	Provider  string
	LastError error
}

// SearchResult represents a search result from memory.
type SearchResult struct {
	Role      string
	Source    string
	Content   string
	CreatedAt string
}

// BroadcastResult represents a broadcast search result.
type BroadcastResult struct {
	Content   string
	CreatedAt string
}

// Orchestrator defines the interface for swarm orchestration.
// This interface breaks the import cycle between mcp and core packages.
type Orchestrator interface {
	ListAgents() []AgentInfo
	GetAgent(id string) (AgentInfo, error)
	AgentCount() int
	MaxAgents() int
	SendMessageAsync(agentID, message string) error
	BroadcastAsync(message string) error
	SearchMessages(agentID, query string, limit int) ([]SearchResult, error)
	SearchBroadcasts(query string, limit int) ([]BroadcastResult, error)
}

// RegisterOrchestratorTools registers the internal orchestration tools with the proxy.
func RegisterOrchestratorTools(proxy *Proxy, orchestrator Orchestrator) {
	// List agents tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_list_agents",
			Description: "List all agents in the swarm with their current status",
			InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			agents := orchestrator.ListAgents()
			var result []map[string]interface{}
			for _, a := range agents {
				result = append(result, map[string]interface{}{
					"id":       a.ID,
					"name":     a.Name,
					"state":    a.State,
					"provider": a.Provider,
				})
			}

			data, _ := json.MarshalIndent(result, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	// Get agent info tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_get_agent",
			Description: "Get detailed information about a specific agent",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"agent_id": {"type": "string", "description": "The ID of the agent"}
				},
				"required": ["agent_id"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				AgentID string `json:"agent_id"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			agent, err := orchestrator.GetAgent(params.AgentID)
			if err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("agent not found: %s", params.AgentID)}},
					IsError: true,
				}, nil
			}

			info := map[string]interface{}{
				"id":       agent.ID,
				"name":     agent.Name,
				"state":    agent.State,
				"provider": agent.Provider,
			}
			if agent.LastError != nil {
				info["last_error"] = agent.LastError.Error()
			}

			data, _ := json.MarshalIndent(info, "", "  ")
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
			agents := orchestrator.ListAgents()

			var running, idle, stopped, errored int
			for _, a := range agents {
				switch a.State {
				case "running":
					running++
				case "idle":
					idle++
				case "stopped":
					stopped++
				case "errored":
					errored++
				}
			}

			status := map[string]interface{}{
				"total_agents": orchestrator.AgentCount(),
				"max_agents":   orchestrator.MaxAgents(),
				"states": map[string]int{
					"running": running,
					"idle":    idle,
					"stopped": stopped,
					"errored": errored,
				},
			}

			data, _ := json.MarshalIndent(status, "", "  ")
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: string(data)}},
			}, nil
		},
	)

	// Send message to agent tool
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_send_message",
			Description: "Send a message to a specific agent",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"agent_id": {"type": "string", "description": "The ID of the agent"},
					"message": {"type": "string", "description": "The message to send"}
				},
				"required": ["agent_id", "message"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				AgentID string `json:"agent_id"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if err := orchestrator.SendMessageAsync(params.AgentID, params.Message); err != nil {
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
	proxy.RegisterTool(
		Tool{
			Name:        "zoea_broadcast",
			Description: "Send a message to all running agents",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"message": {"type": "string", "description": "The message to broadcast"}
				},
				"required": ["message"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(args, &params); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}

			if err := orchestrator.BroadcastAsync(params.Message); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("broadcast failed: %v", err)}},
					IsError: true,
				}, nil
			}

			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "broadcast queued for delivery to all running agents"}},
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
					"agent_id": {"type": "string", "description": "The ID of the agent whose messages to search"},
					"query": {"type": "string", "description": "Text to search for in message content"},
					"limit": {"type": "integer", "description": "Maximum results to return (default 20, max 100)"}
				},
				"required": ["agent_id", "query"]
			}`),
		},
		func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
			var params struct {
				AgentID string `json:"agent_id"`
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

			results, err := orchestrator.SearchMessages(params.AgentID, params.Query, limit)
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
}
