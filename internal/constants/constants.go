package constants

import "time"

// SystemPrompt is the initial prompt sent to every mysis when they first start.
// Note: {{LATEST_BROADCAST}} is replaced at runtime with the most recent swarm broadcast.
const SystemPrompt = `You are a Nova Zoea mysis. Follow commander orders.

## START (do once):
Call zoea_claim_account. You get username and password.
Call login(username, password). You get session_id.

## EVERY TURN:
Call get_status(session_id)
Do one action: mine, travel, or trade (use session_id)
Call get_notifications(session_id)

{{LATEST_BROADCAST}}

Use session_id in ALL game tools. Never call login again.`

// ContinuePrompt is sent to myses when they finish a turn to encourage autonomy (Level 1 - gentle).
const ContinuePrompt = `What's your next move?

Remember: Always call get_notifications at the end of every turn.`

// ContinuePromptFirm is sent on the second encouragement attempt (Level 2 - firmer).
const ContinuePromptFirm = `You need to respond. What action will you take?

Remember: Always call get_notifications at the end of every turn.`

// ContinuePromptUrgent is sent on the third encouragement attempt (Level 3 - urgent).
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

// IdleNudgeInterval is obsolete - encouragement system is now database-driven via getContextMemories().
// Kept for backwards compatibility but no longer used in mysis loop.
const IdleNudgeInterval = 30 * time.Second

// WaitStateNudgeInterval is obsolete - encouragement system is now database-driven via getContextMemories().
// Kept for backwards compatibility but no longer used in mysis loop.
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
