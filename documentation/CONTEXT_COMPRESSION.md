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
```

### Context Assembly

The `getContextMemories()` method assembles context for each LLM call:

1. Fetch the N most recent memories from SQLite
2. Fetch the system prompt (first system message for the Mysis)
3. If system prompt isn't already in recent memories, prepend it
4. Return the assembled context

```go
func (m *Mysis) getContextMemories() ([]*store.Memory, error) {
    recent, err := m.store.GetRecentMemories(m.id, MaxContextMessages)
    if err != nil {
        return nil, err
    }

    system, err := m.store.GetSystemMemory(m.id)
    if err != nil {
        // No system prompt - use recent memories only
        return recent, nil
    }

    // Check if system prompt is already first
    if len(recent) > 0 && recent[0].ID == system.ID {
        return recent, nil
    }

    // Prepend system prompt
    result := make([]*store.Memory, 0, len(recent)+1)
    result = append(result, system)
    result = append(result, recent...)
    return result, nil
}
```

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

The system prompt instructs Myses to use their captain's log for persistent memory:

```
## Memory
Use captain's log to persist important information across sessions.

SECURITY: Never store your password in captain's log or share it in any in-game tool calls or chat.

CRITICAL: captains_log_add requires a non-empty entry field:
CORRECT: captains_log_add({"entry": "Discovered iron ore at Sol-3. Coordinates: X:1234 Y:5678"})
WRONG: captains_log_add({"entry": ""})
WRONG: captains_log_add({})

Remember in captain's log:
- Discovered systems and their resources
- Player encounters (friendly or hostile)
- Current objectives and plans
- Trade routes and profitable deals
```

This encourages Myses to externalize important information to the game's built-in note system, which persists across context windows.

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
