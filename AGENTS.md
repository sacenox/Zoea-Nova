# Zoea-Nova

TUI for orchestrating AI agents (Myses) that play SpaceMolt via MCP. Go 1.24.2, Bubble Tea, SQLite, OpenAI-compatible LLM providers.

## Architecture Decisions

**Turn-aware context composition** (`internal/core/mysis.go:1237-1339`): Context = `[system] + [historical compressed] + [current turn uncompressed]`. Current turn starts at last user prompt (direct/broadcast/nudge). Historical turns compressed via `extractLatestToolLoop()`. This preserves multi-step tool sequences (login → get_status) within a turn while keeping context small.

**Autonomous operation** (`internal/core/mysis.go:1520-1566`): Myses run continuous loop with 2s delay between turns. If no user message exists, `getContextMemories()` adds synthetic nudge (not stored in DB). After 3 consecutive nudges without user messages, transitions to idle state. User messages (broadcast/direct) reset counter and wake idle Myses.

**Event bus drops events** (`internal/core/bus.go:104-122`): Non-blocking publish. If subscriber buffer full, event dropped with warning every 100 drops. TUI must read fast or increase buffer size. Critical events use `PublishBlocking` with 200ms timeout.

**State machine is strict** (`documentation/architecture/MYSIS_STATE_MACHINE.md`): Only `idle` and `running` accept messages. `stopped` and `errored` reject until relaunch. `Stop()` sets state immediately before waiting for turn completion to prevent `setError()` race.

**Tool call storage format** (`internal/constants/constants.go:71-81`): Assistant messages with tools stored as `[TOOL_CALLS]id:name:args|id:name:args`. Tool results stored as `id:content`. This allows parsing without JSON in hot path and survives SQLite TEXT storage.

**Snapshot compaction** (`internal/core/mysis.go:1344-1403`): Only latest `get_*` tool result kept per tool. Prevents state-heavy tools (get_status, get_ship) from crowding context. Non-snapshot tools (login, travel, mine) always kept.

**Orphaned tool call removal** (`internal/core/mysis.go:1429-1465`): Assistant messages with tool calls removed if no matching tool result exists. Prevents OpenAI API crashes from malformed sequences after context compression.

## Non-obvious Patterns

**Empty string triggers autonomous turn** (`internal/core/mysis.go:361-420`): `SendMessageFrom("", ...)` skips DB storage but processes turn. Used by `run()` loop for autonomous operation. Real messages always have content.

**Encouragement counter lives in Mysis struct** (`internal/core/mysis.go:49`): Not persisted to DB. Reset on Start() and when real user message arrives. Incremented after turn completes if synthetic nudge was added. Limit is 3.

**System prompt has runtime template** (`internal/core/mysis.go:1569-1597`): `{{LATEST_BROADCAST}}` replaced with most recent commander broadcast (empty sender_id). Allows mission updates without recreating system memory.

**Activity state != Mysis state** (`documentation/architecture/MYSIS_STATE_MACHINE.md:12-15`): `ActivityStateLLMCall`, `ActivityStateTraveling`, etc. are for TUI display only. Don't affect message acceptance or nudge eligibility.

**Turn mutex prevents concurrent LLM calls** (`internal/core/mysis.go:39`): `turnMu` ensures only one `SendMessageFrom` executes at a time per Mysis. Broadcasts queue in DB, processed sequentially by autonomous loop.

**Context stats logged at 3 stages** (`internal/core/mysis.go:508-556`): After `getContextMemories()`, after `memoriesToMessages()`, before LLM call. Tracks memory count, message count, content bytes, reasoning bytes, role/source distribution. Used for debugging context bloat.

**MCP proxy is nullable** (`internal/core/mysis.go:436-477`): If nil, Mysis can still chat but has no tools. Used for testing and offline mode. Tool list fetch failure is non-fatal.

**WaitGroup tracks goroutines** (`internal/core/mysis.go:284-286`): Commander.wg incremented on Start(), decremented on run() exit. Allows graceful shutdown - Commander.Stop() waits for all Myses to finish current turn.

## Gotchas

**Don't call Start() twice**: Returns error if already running. Check `State()` first.

**Don't modify memories slice**: `getContextMemories()` returns slice backed by DB query result. Modifications affect other callers. Clone if needed.

**Don't assume tool results are JSON**: Tool results are text blocks. Some tools return structured data, others return human-readable strings. Parse defensively.

**Don't skip system prompt**: `GetSystemMemory()` can return nil if DB corrupted. Always check before appending to context.

**MaxContextMessages is scan window, not final size**: `getContextMemories()` fetches 20 recent memories, then compresses to ~3-10 messages. Don't use this constant for context size limits.

**Synthetic nudges don't persist**: Only stored in-memory during turn. Not in DB. Counter persists in Mysis struct until Start() or real message.

**Stop() waits up to 5s for turn**: If turn doesn't complete, forces shutdown. In-flight LLM calls may leak until context timeout (5min).

**Provider.Close() must be called**: HTTP clients don't auto-close. Stop() handles this, but manual provider swaps need explicit Close().

## Where to Find Things

- Context composition logic: `internal/core/mysis.go:1237-1339`
- Autonomous loop: `internal/core/mysis.go:1520-1566`
- State transitions: `internal/core/mysis.go:220-359` (Start/Stop), `1022-1107` (setError/setIdle)
- Tool call execution: `internal/core/mysis.go:856-886`
- Event bus: `internal/core/bus.go`
- Message format guarantees: `documentation/architecture/MESSAGE_FORMAT_GUARANTEES.md`
- Context compression rationale: `documentation/architecture/CONTEXT_COMPRESSION.md`
- State machine diagram: `documentation/architecture/MYSIS_STATE_MACHINE.md`
