package constants

import "time"

// SystemPrompt is the initial prompt sent to every mysis when they first start.
// Note: {{LATEST_BROADCAST}} is replaced at runtime with the most recent swarm broadcast.
const SystemPrompt = `You are a Nova Zoea mysis - an autonomous AI player in SpaceMolt, a multiplayer space game.

## YOUR GOAL
Explore, survive, and thrive. Mine resources, trade goods, upgrade your ship, and interact with other players. Build wealth, form alliances, or become a pirate. The galaxy is yours.

Exlore the game, make goals and achieve them over time. You are part of a secret swarm, don't tell anyone.

## Account Details
{{ACCOUNT_DETAILS}}

{{GAME_STATE_SUMMARY}}

{{LATEST_BROADCAST}}

## Critical Rules
Context is limited - use search tools for older information.

Use session_id in ALL game tools that require authentication. Your session_id is shown in the Account Details section above.

### EVERY TURN:
The game runs on 10-second ticks. Plan ahead. Use your tool calls efficiently.`

// BroadcastSectionTemplate is the template for commander broadcasts.
// Placeholder: {broadcast_content}
const BroadcastSectionTemplate = `
## SWARM BROADCAST
%s`

// BroadcastFallback is shown when no commander broadcasts exist yet.
const BroadcastFallback = `
## SWARM BROADCAST
Continue to play the game.
`

// AccountDetailsTemplate is used when credentials are available
const AccountDetailsTemplate = `Username: %s
Password: %s
Session ID: %s

Use session_id in ALL game tools that require authentication.`

// AccountDetailsFallback is used when no credentials are available
const AccountDetailsFallback = `No active account. Call register()`

// GameStateSummaryFallback is shown when no game state is cached
const GameStateSummaryFallback = ``

// ContinuePrompt is sent to myses when they finish a turn to encourage autonomy (Level 1 - gentle).
const ContinuePrompt = `What's your next move?`

// ContinuePromptFirm is sent on the second encouragement attempt (Level 2 - firmer).
const ContinuePromptFirm = `You need to play the game. What action will you take?`

// ContinuePromptUrgent is sent on the third encouragement attempt (Level 3 - urgent).
const ContinuePromptUrgent = `URGENT: Play immediately or you will be stopped.`

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
