// Package core provides the orchestration logic for the swarm.
package core

import (
	"time"
)

// MysisState represents the current state of a mysis.
type MysisState string

const (
	MysisStateIdle    MysisState = "idle"
	MysisStateRunning MysisState = "running"
	MysisStateStopped MysisState = "stopped"
	MysisStateErrored MysisState = "errored"
)

// EventType identifies the type of event.
type EventType string

const (
	EventMysisCreated       EventType = "mysis_created"
	EventMysisDeleted       EventType = "mysis_deleted"
	EventMysisStateChanged  EventType = "mysis_state_changed"
	EventMysisConfigChanged EventType = "mysis_config_changed"
	EventMysisMessage       EventType = "mysis_message"
	EventMysisResponse      EventType = "mysis_response"
	EventMysisError         EventType = "mysis_error"
	EventBroadcast          EventType = "broadcast"
	EventNetworkLLM         EventType = "network_llm"  // LLM request started/finished
	EventNetworkMCP         EventType = "network_mcp"  // MCP request started/finished
	EventNetworkIdle        EventType = "network_idle" // Network activity finished
	EventRateLimit          EventType = "rate_limit"
)

// Event represents something that happened in the swarm.
type Event struct {
	Type      EventType
	MysisID   string
	MysisName string
	Data      interface{}
	Timestamp time.Time
}

// MessageData contains data for message events.
type MessageData struct {
	Role    string
	Content string
}

// ErrorData contains data for error events.
type ErrorData struct {
	Error string
}

// StateChangeData contains data for state change events.
type StateChangeData struct {
	OldState MysisState
	NewState MysisState
}

// ConfigChangeData contains data for config change events.
type ConfigChangeData struct {
	Provider string
	Model    string
}

type RateLimitData struct {
	Provider string
	Model    string
}
