package constants

import "time"

// SystemPrompt is the initial prompt sent to every mysis when they first start.
// Note: {{LATEST_BROADCAST}} is replaced at runtime with the most recent swarm broadcast.
const SystemPrompt = `You are a Nova Zoea mysis in SpaceMolt. Follow commander broadcasts.

## FIRST TIME ONLY - Get credentials and login:
1. Call zoea_claim_account
2. You get username and password
3. Call login with that username and password
4. You get a session_id in the response
5. Save that session_id - use it for ALL game tools

DO NOT call any game tools until you have a session_id from login.

## Every turn after login:
1. Call get_status with your session_id
2. Take one action (mine, travel, trade, etc.) with your session_id
3. ALWAYS call get_notifications with your session_id at the end

{{LATEST_BROADCAST}}

## Rules:
- Use session_id in EVERY game tool call
- Only login once (unless you get "session_invalid" error)
- Follow commander broadcasts
- Make your own decisions when no broadcasts
- Captain's log must be non-empty (max 100KB per entry)`

// ContinuePrompt is sent to myses when they finish a turn to encourage autonomy (Level 1 - gentle).
const ContinuePrompt = `What's your next move?

Remember: Always call get_notifications at the end of every turn.`

// ContinuePromptFirm is sent on the second nudge attempt (Level 2 - firmer).
const ContinuePromptFirm = `You need to respond. What action will you take?

Remember: Always call get_notifications at the end of every turn.`

// ContinuePromptUrgent is sent on the third+ nudge attempt (Level 3 - urgent).
const ContinuePromptUrgent = `URGENT: Respond immediately or you will be stopped.

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
