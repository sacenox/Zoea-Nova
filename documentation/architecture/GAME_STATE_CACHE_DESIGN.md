# Game State Cache Design

## Problem

Myses are stuck in a loop of re-requesting game state every turn:

1. **Turn N:** Mysis calls `get_status`, `get_ship`, `get_map` → receives 190 KB of data
2. **Turn N+1:** Synthetic nudge starts new turn
   - Historical context contains old snapshots (compressed)
   - LLM doesn't trust "stale" data
   - Calls `get_status` again → another 190 KB
3. **Repeat forever:** Massive payloads every turn, hits 10-iteration limit

**Root cause:** LLMs can't tell if snapshot data is from 1 turn ago or 10 turns ago. They re-request to be safe.

## Solution: Incremental Game State Cache

Build and maintain game state incrementally as the mysis plays. Show state with recency metadata so LLM knows when to update.

### Core Concept

**Store game state snapshots per in-game username (not per mysis):**

- Mysis can switch accounts → state follows the account
- Each mysis has its own game account (SpaceMolt enforces one connection per account)
- State persists when mysis switches between different game accounts
- Account logout → clear that account's state

**Include state in context with delta metadata:**

```
Current Game State (updated 2 ticks ago):
  Player: VoidWanderer
  Credits: 5,000
  Location: Sol System > Asteroid Belt Alpha
  Ship: Cargo Hauler (Hull: 85%, Fuel: 60%)
  Cargo: 45/100 (Iron Ore x30, Copper Ore x15)

To update: call get_status, get_ship, get_notifications
```

**Benefits:**

- 190 KB payload appears ONCE when first fetched
- Subsequent turns show compact summary with recency
- LLM knows "2 ticks ago" vs "50 ticks ago" → makes informed decisions
- State persists across mysis restarts
- No pollution when switching accounts

## Architecture

### Database Schema

```sql
-- New table: game_state_snapshots
CREATE TABLE game_state_snapshots (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,           -- In-game username (e.g., "VoidWanderer")
    tool_name TEXT NOT NULL,          -- Tool that produced this state (e.g., "get_status")
    content TEXT NOT NULL,            -- Full JSON response from tool
    game_tick INTEGER,                -- Game tick when captured (from response)
    captured_at INTEGER NOT NULL,     -- Unix timestamp when stored
    UNIQUE(username, tool_name)       -- One snapshot per tool per username
);

CREATE INDEX idx_game_state_username ON game_state_snapshots(username);
```

### State Lifecycle

**1. Tool Call Interception**

- When mysis calls `get_status`, `get_ship`, `get_map`, etc.
- After receiving response, extract username from result
- Store full response in `game_state_snapshots` table
- Upsert: replace old snapshot for this username+tool

**2. Context Composition**

- Before building LLM context, check if mysis has active game account
- If logged in, fetch all snapshots for that username
- Build compact state summary with recency metadata
- Include summary in system prompt or as synthetic user message

**3. State Invalidation**

- On `logout`: Delete all snapshots for that username
- On `register`: Create new username entry
- On `login`: Switch to existing username's snapshots
- On action (travel, mine, attack): Mark affected snapshots as stale

### State Summary Format

```markdown
## Current Game State

**Account:** VoidWanderer (Solarian Empire)
**Last Updated:** 2 ticks ago (12 seconds)

### Status (get_status - 2 ticks ago)

- Credits: 5,000
- Location: Sol System > Asteroid Belt Alpha
- Ship: Cargo Hauler
- Docked: No

### Ship (get_ship - 2 ticks ago)

- Hull: 85/100
- Shield: 50/50
- Fuel: 60/100
- Cargo: 45/100
  - Iron Ore x30
  - Copper Ore x15

### System (get_system - 5 ticks ago)

- Sol System (Solarian Empire)
- POIs: Earth, Mars, Asteroid Belt Alpha, Station Alpha
- Connections: Alpha Centauri, Barnard's Star

### Map (get_map - 50 ticks ago)

- 487 systems cached
- Use get_map to refresh full galaxy data

**Note:** Call get_notifications to see events since last update.
**Stale data:** get_map is 50 ticks old - consider refreshing if planning long journey.
```

### Recency Calculation

```go
type GameStateSnapshot struct {
    ToolName   string
    Content    string
    GameTick   int       // From game response
    CapturedAt time.Time // When we stored it
}

func (s *GameStateSnapshot) RecencyMessage(currentTick int) string {
    if currentTick > 0 && s.GameTick > 0 {
        ticksAgo := currentTick - s.GameTick
        if ticksAgo == 0 {
            return "just now"
        }
        return fmt.Sprintf("%d ticks ago (%d seconds)", ticksAgo, ticksAgo*10)
    }

    // Fallback to wall clock time
    elapsed := time.Since(s.CapturedAt)
    if elapsed < time.Minute {
        return fmt.Sprintf("%.0f seconds ago", elapsed.Seconds())
    }
    return fmt.Sprintf("%.0f minutes ago", elapsed.Minutes())
}
```

## Implementation Plan

### Phase 1: Database & Storage

1. Add `game_state_snapshots` table to schema
2. Create `GameStateCache` struct in `internal/core/game_state.go`
3. Implement CRUD operations: Store, Get, Delete, List

### Phase 2: Tool Interception

1. Modify MCP proxy to intercept snapshot tool responses
2. Extract username from tool results (parse JSON)
3. Store snapshots in database with game tick
4. Handle login/logout/register to manage username association [Dont break exisitng account re-use logic!]

### Phase 3: Context Integration

1. Add `GetGameStateSummary(username)` method
2. Modify `getContextMemories()` to include state summary
3. Format summary with recency metadata
4. Add to system prompt or as synthetic message

### Phase 4: Staleness Detection

1. Track which actions invalidate which snapshots
2. Mark snapshots as "stale" after relevant actions
3. Prompt LLM to refresh stale data
4. Example: After `travel`, mark `get_status` as stale

### Phase 5: Partial Updates

1. Implement merge logic for partial state updates
2. Handle tools that return partial state (e.g., `get_notifications`)
3. Merge new data into existing snapshots without replacing entire snapshot
4. Track which fields were updated and when

### Phase 6: Testing & Refinement

1. Test with account switching (mysis changes between different game accounts)
2. Verify state persists across mysis restarts
3. Verify state isolation between different game accounts
4. Test partial update merging
5. Measure context size reduction
6. Tune staleness thresholds

## Edge Cases

### Account Switching (Single Mysis, Multiple Accounts)

- Mysis logs out of Account A, logs into Account B
- Account A's state preserved in DB
- Account B's state loaded (or empty if new)
- No pollution between accounts

### Stale Data After Actions

- After `travel`: `get_status` location is stale
- After `mine`: `get_ship` cargo is stale
- After `buy`: `get_status` credits are stale
- Mark affected snapshots, prompt refresh

### No Active Account

- Mysis not logged in → no game state available
- Context shows: "Not logged in. Use register or login."
- After login, state appears

### Tick Synchronization

- Game tick comes from tool responses
- If tick not in response, fall back to wall clock time
- Recency still useful: "30 seconds ago" vs "5 minutes ago"

## Expected Impact

### Context Size Reduction

- **Before:** 190 KB `get_map` every turn
- **After:** 190 KB once, then 2 KB summary per turn
- **Savings:** ~95% reduction in repeated data

### Iteration Limit

- **Before:** 10 iterations spent processing map data
- **After:** 1-2 iterations for decision making
- **Result:** Myses can complete multi-step plans

### LLM Decision Quality

- **Before:** "I don't know current state, better check"
- **After:** "State is 2 ticks old, still fresh, proceed"
- **Result:** More decisive, less redundant tool calls

### State Persistence

- Mysis restart → state preserved for its game account
- Account switch → correct state loaded for new account
- Each mysis operates independently with its own game account
- State keyed by username ensures proper isolation

## Migration Path

1. No backwards migration, new db required.

## Success Metrics

- **Tool call frequency:** Reduce `get_*` calls by 80%+
- **Iteration limits:** Reduce "Max tool iterations" warnings by 90%+
- **Context size:** Reduce average context from 191 KB to <20 KB
- **Decision speed:** Myses complete actions in 2-3 iterations vs 10
- **State accuracy:** LLMs make decisions with correct recency awareness
