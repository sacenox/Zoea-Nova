// Package core provides the orchestration logic for the swarm.
package core

import (
	"time"
)

// AgentState represents the current state of an agent.
type AgentState string

const (
	AgentStateIdle    AgentState = "idle"
	AgentStateRunning AgentState = "running"
	AgentStateStopped AgentState = "stopped"
	AgentStateErrored AgentState = "errored"
)

// EventType identifies the type of event.
type EventType string

const (
	EventAgentCreated       EventType = "agent_created"
	EventAgentDeleted       EventType = "agent_deleted"
	EventAgentStateChanged  EventType = "agent_state_changed"
	EventAgentConfigChanged EventType = "agent_config_changed"
	EventAgentMessage       EventType = "agent_message"
	EventAgentResponse      EventType = "agent_response"
	EventAgentError         EventType = "agent_error"
	EventBroadcast          EventType = "broadcast"
)

// Event represents something that happened in the swarm.
type Event struct {
	Type      EventType
	AgentID   string
	AgentName string
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
	OldState AgentState
	NewState AgentState
}

// ConfigChangeData contains data for config change events.
type ConfigChangeData struct {
	Provider string
	Model    string
}
