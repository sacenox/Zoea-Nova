# Context Compression

Agents in Zoea Nova use a sliding window approach to manage LLM context size. Instead of sending the entire conversation history to the LLM on each turn, agents send only recent messages plus the system prompt. This keeps inference fast while agents retain access to older memories through search tools.

## Design

### The Problem

Without compression, agent context grows unbounded:
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
│  All messages since agent creation                  │
│  Searchable via zoea_search_messages tool           │
└─────────────────────────────────────────────────────┘
```

## Implementation

### Constants

```go
// internal/core/agent.go

// MaxContextMessages limits how many recent messages to include in LLM context.
// Value chosen to cover ~2 server ticks worth of activity.
const MaxContextMessages = 20
```

### Context Assembly

The `getContextMemories()` method assembles context for each LLM call:

1. Fetch the N most recent memories from SQLite
2. Fetch the system prompt (first system message for the agent)
3. If system prompt isn't already in recent memories, prepend it
4. Return the assembled context

```go
func (a *Agent) getContextMemories() ([]*store.Memory, error) {
    recent, err := a.store.GetRecentMemories(a.id, MaxContextMessages)
    if err != nil {
        return nil, err
    }

    system, err := a.store.GetSystemMemory(a.id)
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

// GetSystemMemory retrieves the initial system prompt for an agent.
func (s *Store) GetSystemMemory(agentID string) (*Memory, error)

// GetRecentMemories retrieves the most recent N memories for an agent.
func (s *Store) GetRecentMemories(agentID string, limit int) ([]*Memory, error)

// SearchMemories searches memories by content text (case-sensitive).
func (s *Store) SearchMemories(agentID, query string, limit int) ([]*Memory, error)

// SearchBroadcasts searches broadcast messages by content text (case-sensitive).
func (s *Store) SearchBroadcasts(query string, limit int) ([]*BroadcastMessage, error)
```

## Search Tools

Agents can retrieve older memories using MCP tools exposed through the orchestrator:

### zoea_search_messages

Search an agent's past messages by text content.

```json
{
  "agent_id": "abc123",
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

## System Prompt Guidance

The system prompt instructs agents to use their captain's log for persistent memory:

```
## Memory
Use captain's log (captains_log_add) to remember:
- Your password (CRITICAL)
- Discovered systems and their resources
- Player encounters (friendly or hostile)
- Current objectives and plans
```

This encourages agents to externalize important information to the game's built-in note system, which persists across context windows.

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
