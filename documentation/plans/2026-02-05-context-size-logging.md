# Context Size Logging Plan (Logs Only)

**Date:** 2026-02-05
**Status:** Ready for Implementation

## Goal
Add lightweight debug logging that reports LLM context size per turn. No new dependencies.

## Scope
- Log counts and byte sizes only.
- Never log message content.

## Implementation Steps

### 1) Add helper functions
**File:** `internal/core/mysis.go`

Create helpers to compute:
- Memory count
- Total bytes for `Memory.Content` and `Memory.Reasoning`
- Role counts (system/user/assistant/tool)
- Source counts (direct/broadcast/system/llm/tool)

Create helpers to compute:
- Message count
- Total bytes for `Message.Content`
- Tool call count

### 2) Add log points in the turn flow
**File:** `internal/core/mysis.go`

Insert debug logs in `SendMessage`:
- After `getContextMemories()`
- After `memoriesToMessages()`
- Right before `ChatWithTools` / `Chat`

Each log entry should include:
- `mysis_id`, `mysis_name`
- `memory_count`, `message_count`
- `content_bytes`, `reasoning_bytes`
- `role_counts`, `source_counts`
- `tool_call_count`

### 3) Keep logs lightweight
- Use counts and lengths only.
- Avoid allocations beyond slices/maps needed for counts.

## Tests
**File:** `internal/core/mysis_test.go`

Add unit tests for helper functions:
- Role/source counts are correct
- Content/reasoning byte totals are correct
- Tool call count is correct

## Verification
1. Run `make test`
2. Run `make build`
