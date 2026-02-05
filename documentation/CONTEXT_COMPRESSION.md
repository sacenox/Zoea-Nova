# Context Compression

Myses in Zoea Nova use a sliding window approach to manage LLM context size. Instead of sending the entire conversation history to the LLM on each turn, Myses send only recent messages plus the system prompt. This keeps inference fast while Myses retain access to older memories through search tools.

## Design

### The Problem

Without compression, Mysis context grows unbounded:
- Each tool call adds multiple messages (request + result)
- SpaceMolt gameplay involves frequent tool use (mining, trading, navigation)
- Large contexts slow inference and increase costs
- Eventually hits provider token limits

### The Solution

```
┌─────────────────────────────────────────────────────┐
│                   LLM Context                       │
├─────────────────────────────────────────────────────┤
│  System Prompt (always included)                    │
├─────────────────────────────────────────────────────┤
│  Recent Messages (last 20)                          │
│  - User messages                                    │
│  - Assistant responses                              │
│  - Tool calls and results                           │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│              Full Memory (SQLite)                   │
├─────────────────────────────────────────────────────┤
│  All messages since Mysis creation                  │
│  Searchable via zoea_search_messages tool           │
│  Reasoning searchable via zoea_search_reasoning     │
└─────────────────────────────────────────────────────┘
```

## Implementation

### Constants

```go
// internal/core/mysis.go

// MaxContextMessages limits how many recent messages to include in LLM context.
// Value chosen to cover ~2 server ticks worth of activity.
const MaxContextMessages = 20

// snapshotTools defines tools that return state snapshots.
// When multiple results from the same snapshot tool appear in context,
// only the most recent one is kept to prevent redundant state data.
var snapshotTools = map[string]bool{
    "get_ship":          true,
    "get_system":        true,
    "get_poi":           true,
    "get_nearby":        true,
    "get_cargo":         true,
    "zoea_swarm_status": true,
    "zoea_list_myses":   true,
}
```

### Context Assembly

The `getContextMemories()` method assembles context for each LLM call:

1. Fetch the N most recent memories from SQLite
2. Apply snapshot compaction to remove redundant state tool results
3. Fetch the system prompt (first system message for the Mysis)
4. If system prompt isn't already in compacted memories, prepend it
5. Return the assembled context

```go
func (m *Mysis) getContextMemories() ([]*store.Memory, error) {
    recent, err := m.store.GetRecentMemories(m.id, MaxContextMessages)
    if err != nil {
        return nil, err
    }

    // Apply compaction to remove redundant snapshot tool results
    compacted := m.compactSnapshots(recent)

    system, err := m.store.GetSystemMemory(m.id)
    if err != nil {
        // No system prompt - use compacted memories only
        return compacted, nil
    }

    // Check if system prompt is already first
    if len(compacted) > 0 && compacted[0].ID == system.ID {
        return compacted, nil
    }

    // Prepend system prompt
    result := make([]*store.Memory, 0, len(compacted)+1)
    result = append(result, system)
    result = append(result, compacted...)
    return result, nil
}
```

### Snapshot Compaction

The `compactSnapshots()` method removes redundant state tool results:

- Identifies tool results from snapshot tools (get_ship, get_system, etc.)
- Keeps only the most recent result for each snapshot tool
- Preserves all non-snapshot messages and tool results
- Maintains chronological order

Snapshot detection relies on tool call IDs recorded in assistant `[TOOL_CALLS]` memories. Tool results are compacted only when their tool call ID resolves to a snapshot tool name, which avoids misclassifying non-snapshot results that happen to include similar fields.

This prevents state-heavy tools from crowding out conversation history while ensuring the latest state is always available.

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
## Memory
Use captain's log to persist important information across sessions.

## Context & Memory Management
Your context window is limited. Recent state snapshots are kept, but older messages are removed.
If you need information from earlier in the conversation:
- Use zoea_search_messages to find past messages by keyword
- Use zoea_search_reasoning to find past reasoning by keyword
- Use captain's log for persistent notes across sessions
```

The continue prompt also reminds Myses about search tools:

```
CRITICAL REMINDERS:
- If you need past data, use zoea_search_messages or zoea_search_reasoning
```

This encourages Myses to:
1. Externalize important information to the game's built-in note system
2. Use search tools to retrieve older context when needed
3. Understand that their context window is limited and compacted

## Tradeoffs

| Aspect | Benefit | Cost |
|--------|---------|------|
| Inference speed | Faster responses with smaller context | May lose relevant older context |
| Token costs | Lower per-request costs | Search queries add overhead |
| Memory access | Full history searchable | Requires explicit search |
| Coherence | Recent context always available | Long-term plans may be forgotten |

## Tuning

The `MaxContextMessages` constant can be adjusted based on:

- **Model context window**: Larger models can handle more messages
- **Gameplay pace**: Faster tick rates may need larger windows
- **Tool call frequency**: Heavy tool use fills context faster

Current value of 20 messages covers approximately 2 server ticks of typical gameplay activity.
