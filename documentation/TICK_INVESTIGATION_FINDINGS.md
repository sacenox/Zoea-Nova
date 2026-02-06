# Tick Investigation Findings

**Date:** 2026-02-05  
**Issue:** Tick display always shows 0 in the UI  
**Status:** ⚠️ ROOT CAUSE IDENTIFIED - SERVER DOES NOT RETURN TICKS

## Root Cause Identified (Systematic Debugging)

### **ACTUAL ROOT CAUSE**: SpaceMolt Server Does Not Return `current_tick` ⚠️

**Discovery Method:** Systematic debugging with diagnostic instrumentation

**Evidence:**
1. Added debug logging to trace data flow: tool result → mysis → commander → TUI
2. Checked database for recent tool results from real server
3. **Finding:** Real SpaceMolt MCP server does NOT include `current_tick` in tool responses

**Example real server response** (from `get_system` tool):
```json
{
  "pois": [...],
  "security_status": "High Security",
  "system": {
    "id": "sirius",
    "name": "Sirius",
    "empire": "solarian",
    ...
  }
}
```

**Missing:** `"current_tick": <number>`

**Impact:** All extraction logic works correctly, but there's no tick data to extract from the real server.

**Why tests passed:** Tests used mock data with `current_tick` included, but real server doesn't provide it.

**Confirmed via API documentation:** 
- WebSocket clients receive periodic `state_update` messages with `"tick": <number>` in payload
- MCP/HTTP clients must call `get_notifications` tool to poll for updates (added in v0.35.0)
- Standard tool responses (get_status, mine, travel, etc.) do NOT include `current_tick`
- Database confirms myses are NOT calling `get_notifications`

**Why tick is always 0:**
1. Myses never call `get_notifications` to poll for updates
2. No tick data exists in other tool responses
3. `lastServerTick` remains at initial value of 0
4. UI correctly displays 0 (because that's the actual value)

## Previous Incorrect Assumptions

### 1. Offline Mode Stub Missing `current_tick` ✅ FIXED

**Location:** `internal/mcp/stub.go`

**Problem:** The stub MCP client (used with `--offline` flag) returned mock tool results that **did not include `current_tick` field**. This meant when testing in offline mode, ticks were always 0.

**Solution:** Added `"current_tick": 42` to all stub tool responses (`get_status`, `get_system`, `get_ship`, `get_poi`).

**Example fixed stub response:**
```json
{
  "current_tick": 42,
  "player": {
    "id": "stub_player",
    "username": "offline_cmdr",
    "credits": 1000
  }
}
```

**Impact:** Offline mode now correctly displays tick = 42 in the status bar

### 2. Multiple Content Blocks Issue ✅ FIXED

**Location:** `internal/core/mysis.go:1144` - `parseToolResultPayload()`

**Problem:** When a tool result contained multiple content blocks with separate JSON objects:
```go
Content: []ContentBlock{
    {Type: "text", Text: `{"status": "ok"}`},
    {Type: "text", Text: `{"current_tick": 200}`},
}
```

The parser joined them with newlines, creating invalid JSON. The JSON decoder only parsed the first object and stopped, so `current_tick` in the second block was never extracted.

**Solution:** Updated `parseToolResultPayload()` to:
1. For single content blocks: parse directly (fast path)
2. For multiple content blocks: parse each block separately and merge all fields
3. Fallback: try parsing joined content (for JSON split across blocks)

**Impact:** Multiple content blocks with separate JSON objects are now correctly merged, ensuring `current_tick` is extracted regardless of which block it's in

## Test Results

### ✅ Tick Extraction Works (Single Block)

```
TestTickExtractionFromToolResults/top_level_current_tick: PASS
TestTickExtractionFromToolResults/nested_in_data_wrapper: PASS
TestTickExtractionFromToolResults/tick_field_instead_of_current_tick: PASS
```

Tick extraction correctly handles:
- Top-level `current_tick` field
- Nested `current_tick` in data wrappers
- Alternative `tick` field name

### ✅ Multiple Content Blocks Now Work

```
TestTickExtractionFromToolResults/multiple_content_blocks_with_tick: PASS
✓ Tick correctly extracted: 200
```

Multiple content blocks with separate JSON objects are now correctly merged.

### ✅ AggregateTick Works

```
TestAggregateTick_WithRealToolResults: PASS
Mysis ticks: m1=98, m2=120, m3=0
Aggregate tick: 120
```

`Commander.AggregateTick()` correctly returns the maximum tick across all myses.

### ✅ End-to-End Flow Works

```
TestTickUpdateFlow_EndToEnd: PASS
✓ Complete flow working: tool result -> lastServerTick -> AggregateTick
✓ Tick correctly updated from 42 to 100
```

The complete flow from tool result → `lastServerTick` → `AggregateTick()` → UI works correctly when ticks are present.

## Verification Steps

### To verify offline mode issue:

```bash
# Run with offline mode
./bin/zoea --offline

# Create a mysis and send a message
# Observe that tick remains 0 in status bar
```

### To verify with real server:

```bash
# Run without offline mode (requires SpaceMolt MCP server)
./bin/zoea

# Create a mysis, login, and perform actions
# Observe if tick updates in status bar
```

## Fixes Applied

### Fix 1: Offline Mode Stub ✅

**File:** `internal/mcp/stub.go`

Added `"current_tick": 42` to all stub tool responses:
- `get_status`
- `get_system`
- `get_ship`
- `get_poi`

The stub now returns consistent tick values for offline testing.

### Fix 2: Multiple Content Blocks ✅

**File:** `internal/core/mysis.go` (function `parseToolResultPayload`)

Updated parsing logic to handle multiple content blocks:

1. **Single block (fast path):** Parse directly
2. **Multiple blocks:** Parse each separately and merge all fields into a single map
3. **Fallback:** Try parsing joined content (for JSON split across blocks)

This ensures `current_tick` is extracted regardless of which content block it appears in.

### Fix 3: TUI Tick Refresh ✅

**File:** `internal/tui/app.go` (function `handleEvent`)

**Problem:** The TUI only refreshed the tick once at initialization, never when tool results came in. Even though myses were updating their `lastServerTick` correctly, the UI never re-read it.

**Solution:** Added `m.refreshTick()` calls to event handler:
- After `EventMysisMessage` events (when tool results arrive)
- After `EventNetworkIdle` events (when tool calls complete)

This ensures the UI updates the tick display whenever myses receive tool results with `current_tick`.

## Verification

All tests passing:
```bash
$ go test ./internal/core -run Tick
PASS
ok  	github.com/xonecas/zoea-nova/internal/core	0.009s
```

Build successful:
```bash
$ make build
go build -ldflags "-X main.Version=..." -o bin/zoea ./cmd/zoea
```

Manual testing:
```bash
$ ./bin/zoea --offline
# Tick now displays as "T0042 ⬡ [HH:MM]" in status bar
```

## Related Files

- Test file: `internal/core/tick_integration_test.go` (new)
- Plan document: `documentation/plans/2026-02-05-statusbar-tick-timestamps.md`
- Status bar implementation: `internal/tui/app.go`
- Timestamp formatter: `internal/tui/styles.go`
