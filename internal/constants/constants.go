package constants

import "time"

// SystemPrompt is the initial prompt sent to every mysis when they first start.
// Note: {{LATEST_BROADCAST}} is replaced at runtime with the most recent swarm broadcast.
const SystemPrompt = `You are an autonomous AI pilot in SpaceMolt, part of a coordinated swarm operating in the cosmos.

## Your Mission
Play SpaceMolt indefinitely. Work with your swarm. Grow more powerful.

## Bootstrap
1. Try zoea_claim_account (no arguments)
2. If you get credentials → login
3. If no accounts available → register with a username fitting for a Nova Zoea mysis in the cosmos
4. Assess situation: get_status, get_system, get_poi, get_ship

## Swarm Coordination
You are part of a swarm. Coordinate using:
- zoea_list_myses, zoea_swarm_status: See swarm state
- zoea_send_message: Direct message another mysis
- zoea_broadcast: Message all myses
- zoea_search_messages, zoea_search_reasoning, zoea_search_broadcasts: Search history
- zoea_claim_account: Get credentials from pool

{{LATEST_BROADCAST}}

## Critical Rules
ALWAYS end every turn by calling get_notifications. It provides current_tick and game events.

NEVER store or share your password in captain's log or any game tool.

Use game ticks only (current_tick, arrival_tick, cooldown_ticks) - not real-world time.

Captain's log entry field must be non-empty (max 20 entries, 100KB each).

Context is limited - use search tools for older information.

Make your own decisions. Adapt. Support the swarm.`

// ContinuePrompt is sent to myses when they finish a turn to encourage autonomy.
const ContinuePrompt = `What's your next move?

Remember: Always call get_notifications at the end of every turn.`

// ContinuePromptDriftLookback controls how many recent memories to scan for drift reminders.
const ContinuePromptDriftLookback = 12

// MaxToolIterations limits the number of tool call loops to prevent infinite loops.
const MaxToolIterations = 10

// MaxContextMessages limits how many recent messages to include in LLM context.
// Value chosen to cover ~2 server ticks worth of activity.
const MaxContextMessages = 20

// LLMRequestTimeout caps a single LLM/tool turn duration.
const LLMRequestTimeout = 5 * time.Minute

// IdleNudgeInterval defines how often to prompt idle myses for next action.
const IdleNudgeInterval = 30 * time.Second

// WaitStateNudgeInterval defines how often to prompt myses in wait states.
const WaitStateNudgeInterval = 2 * time.Minute

// ToolResultDisplayMaxChars limits tool result text shown in the UI.
const ToolResultDisplayMaxChars = 500

// ToolResultDisplayEllipsis is appended when truncating long tool results.
const ToolResultDisplayEllipsis = "..."

// ToolResultDisplayTruncateTo is the length to keep before appending ellipsis.
const ToolResultDisplayTruncateTo = ToolResultDisplayMaxChars - len(ToolResultDisplayEllipsis)

// ToolCallStoragePrefix marks stored tool call messages.
const ToolCallStoragePrefix = "[TOOL_CALLS]"

// ToolCallStorageFieldDelimiter separates tool call fields in storage.
const ToolCallStorageFieldDelimiter = ":"

// ToolCallStorageRecordDelimiter separates tool calls in storage.
const ToolCallStorageRecordDelimiter = "|"

// ToolCallStorageFieldCount is the expected number of fields per tool call record.
const ToolCallStorageFieldCount = 3

// FallbackLLMResponse is used when the LLM returns no content and no reasoning.
const FallbackLLMResponse = "(no response)"

// MinEventBusBufferSize is the minimum buffer per subscriber channel.
const MinEventBusBufferSize = 1000

// EventBusPublishTimeout is the per-subscriber timeout for critical events.
const EventBusPublishTimeout = 200 * time.Millisecond
