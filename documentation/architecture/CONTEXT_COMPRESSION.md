# Context Compression

Zoea Nova uses a loop-based context composition strategy to keep LLM context small and stable. Instead of sending large conversation windows, each LLM request includes only three components: system prompt, a single prompt source, and the most recent tool loop. This guarantees bounded context size (typically 2-10 messages) while the full conversation history remains searchable.

## Design

### The Problem

Without compression, Mysis context grows unbounded:
- Each tool call adds multiple messages (request + result)
- SpaceMolt gameplay involves frequent tool use (mining, trading, navigation)
- Large contexts slow inference and increase costs
- Eventually hits provider token limits
- Traditional sliding windows create orphaned tool results and broken message sequences

### The Solution: Loop Slices

```
┌─────────────────────────────────────────────────────┐
│                   LLM Context                       │
│              (Loop-Based Composition)               │
├─────────────────────────────────────────────────────┤
│  [1] System Prompt (always first)                   │
├─────────────────────────────────────────────────────┤
│  [2] Current Turn Start                              │
│      Selected by recency (most recent wins):        │
│      • Commander direct message                     │
│      • Last commander broadcast                     │
│      • Last swarm broadcast                         │
│      • Synthetic nudge (if no prompts found)        │
├─────────────────────────────────────────────────────┤
│  [3] Latest Tool Loop (if any)                      │
│      • Tool call message (assistant with tools)     │
│      • All subsequent tool results                  │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│              Full Memory (SQLite)                   │
├─────────────────────────────────────────────────────┤
│  All messages since Mysis creation                  │
│  Searchable via zoea_search_messages tool           │
│  Reasoning searchable via zoea_search_reasoning     │
└─────────────────────────────────────────────────────┘
```

## Architecture

### Loop Slice Model

A **loop slice** is the atomic unit of tool interaction:
1. **Tool call message** (assistant role, contains `[TOOL_CALLS]` prefix)
2. **Tool results** (tool role, consecutive messages following the call)

The most recent loop slice is always included in context to maintain tool-calling coherence.

### Prompt Source Priority

Prompt sources are user messages that drive Mysis behavior and mark the start of the current turn. Selection order (most recent message wins):

1. **Commander direct message** (`source="direct"`) - Direct command from user
2. **Last commander broadcast** (`source="broadcast"`, `sender_id=""`) - Commander identified by empty sender_id
3. **Last swarm broadcast** (`source="broadcast"`, `sender_id!=""`) - Most recent broadcast from another Mysis
4. **Synthetic nudge** (generated, not stored) - Fallback when no prompt sources exist

See [Terminology](#terminology) for definitions of turn boundaries and context composition.

### Context Composition Algorithm

```go
// internal/core/mysis.go:1374-1421

func (m *Mysis) getContextMemories() ([]*store.Memory, error) {
    // Fetch recent memories from DB
    allMemories := GetRecentMemories(m.id, MaxContextMessages)
    
    result := []
    
    // Step 1: Add system prompt (if available)
    system := GetSystemMemory(m.id)
    if system != nil {
        result.append(system)
    }
    
    // Step 2: Find current turn boundary (most recent user message)
    turnStart := selectPromptSource(allMemories)
    if turnStart == nil {
        // Generate synthetic nudge (not stored in DB)
        nudge := createEphemeralNudge()
        result.append(nudge)
    } else {
        result.append(turnStart)
    }
    
    // Step 3: Extract latest tool loop
    toolLoop := extractLatestToolLoop(allMemories)
    result.append(toolLoop...)
    
    return result
}
```

### Loop Extraction

```go
// internal/core/mysis.go:1326-1372

func extractLatestToolLoop(memories []*store.Memory) []*store.Memory {
    // Scan backwards (newest first) for most recent tool call
    toolCallIdx := findLastToolCallMessage(memories)
    if toolCallIdx == -1 {
        return nil
    }
    
    // Collect tool call + all consecutive tool results
    loop := [memories[toolCallIdx]]
    for i := toolCallIdx + 1; i < len(memories); i++ {
        if memories[i].Role == "tool" {
            loop.append(memories[i])
        } else {
            break  // Stop at first non-tool message
        }
    }
    
    return loop
}
```

Tool call messages are identified by:
- Role: `assistant`
- Content prefix: `[TOOL_CALLS]`

## Bounded Size Guarantees

The loop slice model provides strict size bounds:

| Component | Typical Size | Max Size |
|-----------|--------------|----------|
| System prompt | 1 message | 1 message |
| Prompt source | 1 message | 1 message |
| Tool loop | 1-8 messages | ~15 messages (1 call + max results) |
| **Total** | **3-10 messages** | **~17 messages** |

This is dramatically smaller than traditional sliding windows (20-50 messages) and eliminates orphaned tool sequencing issues.

## Synthetic Nudge Behavior

When no prompt source is found in recent memory, a synthetic nudge is generated:

```go
// internal/core/mysis.go:1398-1409

nudgeContent := "Continue your mission. Check notifications and coordinate with the swarm."
nudgeMemory := &store.Memory{
    Role:      "user",
    Source:    "system",
    Content:   nudgeContent,
    SenderID:  "",
    CreatedAt: time.Now(),
}
```

**Key properties:**
- **NOT stored in database** - Ephemeral automation scaffolding
- **Counted for circuit breaker** - Nudge counter increments in ticker loop (see `internal/core/mysis.go:1624-1641`)
- **Escalating prompts** - Content changes based on `encouragementCount` to increase urgency:
  - Attempt 1: "Continue your mission. Check notifications and coordinate with the swarm." (gentle)
  - Attempt 2: "You need to respond. Check your notifications and status." (firm)
  - Attempt 3: "URGENT: Respond immediately. Check your system status." (urgent)
  - After 3 failures: Mysis transitions to idle state with error "Failed to respond after 3 encouragements"
- **Escalation intervals** - Nudges sent every 30 seconds for idle myses, 2 minutes for wait states (see `internal/constants/constants.go:69-73`)

### Nudge Circuit Breaker

```go
// internal/core/mysis.go:1624-1641

if shouldNudge(time.Now()) {
    encouragementCount++
    
    if encouragementCount >= 3 {
        setIdle("Failed to respond after 3 encouragements")
        return
    }
    
    nudgeCh <- struct{}{}  // Trigger nudge processing
}
```

**Counter reset:**
- Reset to 0 on successful LLM response (line 616)
- Prevents idle Myses from looping indefinitely
- Forces transition to idle state after 3 failed nudges

For state machine transitions related to nudge failures, see [MYSIS_STATE_MACHINE.md](MYSIS_STATE_MACHINE.md#state-transitions).

### Nudge Intervals

```go
// internal/constants/constants.go:69-73

IdleNudgeInterval = 30 * time.Second      // Default idle nudging
WaitStateNudgeInterval = 2 * time.Minute  // Nudging during wait states
```

## Search Tools

Myses can retrieve older memories using MCP tools exposed through the orchestrator:

### zoea_search_messages

Search a Mysis's past messages by text content.

```json
{
  "mysis_id": "abc123",
  "query": "iron ore",
  "limit": 20
}
```

Returns matching messages with role, source, content, and timestamp.

### zoea_search_broadcasts

Search past swarm broadcasts by text content.

```json
{
  "query": "enemy spotted",
  "limit": 20
}
```

Returns matching broadcasts with content and timestamp.

### zoea_search_reasoning

Search a Mysis's past reasoning content by text.

```json
{
  "mysis_id": "abc123",
  "query": "travel cooldown",
  "limit": 20
}
```

Returns matching reasoning entries with role, source, content, reasoning, and timestamp.

## System Prompt Guidance

The system prompt instructs Myses to use their captain's log for persistent memory and search tools for older context:

```
## Critical Rules
Context is limited - use search tools for older information.
```

The continue prompt also reminds Myses about search tools when generated by nudging logic.

This encourages Myses to:
1. Externalize important information to the game's built-in note system
2. Use search tools to retrieve older context when needed
3. Understand that their context window is minimal and focused

## Benefits Over Sliding Windows

| Aspect | Loop Slices | Traditional Sliding Window |
|--------|-------------|---------------------------|
| Size | 2-10 messages | 20-50 messages |
| Stability | Always same structure | Variable, depends on history |
| Tool coherence | Complete loops guaranteed | Often broken mid-sequence |
| Orphaned messages | Never | Common (tool results without calls) |
| Context bloat | None | Redundant state snapshots accumulate |
| Inference speed | Fast (minimal tokens) | Slower (large context) |

## Implementation Details

### Constants

```go
// internal/constants/constants.go:62-64

MaxContextMessages = 20  // Window for scanning recent memories
```

Note: `MaxContextMessages` is the scanning window, NOT the final context size. The loop slice extraction typically produces 2-10 messages regardless of this value.

### Store Methods

```go
// internal/store/memories.go

// GetSystemMemory retrieves the initial system prompt for a Mysis.
func (s *Store) GetSystemMemory(mysisID string) (*Memory, error)

// GetRecentMemories retrieves the most recent N memories for a Mysis.
func (s *Store) GetRecentMemories(mysisID string, limit int) ([]*Memory, error)

// SearchMemories searches memories by content text (case-sensitive).
func (s *Store) SearchMemories(mysisID, query string, limit int) ([]*Memory, error)

// SearchBroadcasts searches broadcast messages by content text (case-sensitive).
func (s *Store) SearchBroadcasts(query string, limit int) ([]*BroadcastMessage, error)
```

## Tradeoffs

| Aspect | Benefit | Cost |
|--------|---------|------|
| Inference speed | Very fast with minimal context | May lose relevant older context |
| Token costs | Minimal per-request costs | Search queries add overhead |
| Memory access | Full history searchable | Requires explicit search |
| Coherence | Recent tool loops always complete | Long-term plans may be forgotten |
| Predictability | Same structure every time | Less conversational continuity |

## Tuning

The loop slice model has minimal tuning surface:

- **`MaxContextMessages`**: Scanning window for finding turn boundaries and tool loops. Larger values increase DB query cost but provide more history to search. Default: 20 messages (~2 server ticks).
- **Turn boundary selection**: Most recent user message wins (commander direct > commander broadcast > swarm broadcast > nudge).
- **Nudge intervals**: Control how often idle Myses are prompted. Faster intervals increase responsiveness but may interrupt LLM processing.

## Migration Notes

This architecture replaced the previous snapshot compaction model (v0.4.x) which used a 20-message sliding window with snapshot deduplication. The old model:
- Sent 20 recent messages per LLM request
- Deduplicated redundant `get_*` tool results
- Often broke tool call sequences mid-loop
- Created orphaned tool results

The loop slice model eliminates these issues by composing context from atomic units (system + prompt + loop) rather than arbitrary time windows.

See `documentation/archive/` for historical compaction implementation details.

## Turn-Aware Context Composition (v0.5.0+)

As of v0.5.0, context composition distinguishes between historical turns and the current turn.

### Architecture

**Turn Boundary:** Detected by finding the most recent user-initiated prompt (direct message, broadcast, or system nudge).

**Historical Turns:** All messages before the turn boundary
- Compressed using `extractLatestToolLoop()`
- Only the most recent complete tool loop is included
- Saves context space for older conversations

**Current Turn:** All messages from turn boundary to present
- Included WITHOUT compression
- Preserves complete multi-step tool sequences
- Essential for multi-step reasoning (e.g., login → get_status → get_notifications)

### Example

```
Historical:
  [user: "old question"]
  [assistant: tool_call_1]
  [tool: result_1]
  [assistant: tool_call_2]  ← Only latest loop included
  [tool: result_2]           ← Only latest loop included

Current Turn:
  [user: "new question"]      ← Turn boundary
  [assistant: login_call]     ← All included
  [tool: session_id: abc123]  ← All included
  [assistant: status_call]    ← All included
  [tool: status_result]       ← All included
```

### Benefits

1. **Multi-step reasoning:** LLM can reference earlier tool results within the same turn
2. **Session persistence:** Session IDs from login remain visible throughout the turn
3. **Context efficiency:** Historical turns compressed, current turn preserved
4. **Orphan prevention:** Complete tool loops stay together

### Implementation

**Function:** `getContextMemories()` in `internal/core/mysis.go`

**Turn Detection:** `findLastUserPromptIndex()` scans backwards to find the most recent user prompt

**Composition:**
1. System prompt (always first)
2. Historical context (compressed via `extractLatestToolLoop()`)
3. Current turn (uncompressed, from turn boundary to present)

### Migration from v0.4.x

**Before (Loop-based):**
- Context: `[system] + [prompt source] + [latest tool loop only]`
- Issue: Multi-step tool sequences truncated (login result lost)

**After (Turn-aware):**
- Context: `[system] + [historical compressed] + [current turn complete]`
- Fixed: Complete current turn preserved, enabling multi-step reasoning
