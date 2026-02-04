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

// Orchestrator defines the interface for swarm orchestration.
// This interface breaks the import cycle between mcp and core packages.
type Orchestrator interface {
	ListAgents() []AgentInfo
	GetAgent(id string) (AgentInfo, error)
	AgentCount() int
	MaxAgents() int
	SendMessage(agentID, message string) error
	Broadcast(message string) error
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

			if err := orchestrator.SendMessage(params.AgentID, params.Message); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("failed to send message: %v", err)}},
					IsError: true,
				}, nil
			}

			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "message sent successfully"}},
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

			if err := orchestrator.Broadcast(params.Message); err != nil {
				return &ToolResult{
					Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("broadcast partially failed: %v", err)}},
					IsError: true,
				}, nil
			}

			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "broadcast sent successfully"}},
			}, nil
		},
	)
}
