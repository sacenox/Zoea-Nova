# Message Row Format Examples

This document shows the different message format types in the mysis list dashboard.

## Priority System

Messages are displayed using a priority system:
1. **Errors** (highest priority) - shown when mysis is in errored state
2. **AI Replies** - assistant messages that are not tool calls
3. **Tool Calls** - assistant messages with tool call prefix
4. **User Messages/Broadcasts** (lowest priority) - direct messages or broadcasts

## Format Examples

### 1. Error Message (Priority 1)

**When mysis is in `errored` state:**

```
║[  ] ✖  delta    ollama       errored  @crab_miner  │ Error: Connection timeout - server unreachable    ║
```

**With timestamp (when LastMessageAt is set):**

```
║[  ] ✖  delta    ollama       errored  @crab_miner  │ T42 ⬡ [10:45] Error: Connection timeout           ║
```

### 2. AI Reply (Priority 2)

**Format:** `T42 ⬡ [10:45] [AI] Message content...`

```
║[→ ] ⠋  alpha    ollama       running  @crab_war... │ T42 ⬡ [10:45] [AI] Directive received, mining!    ║
```

### 3. Tool Calls (Priority 3)

**Format:** `T42 ⬡ [10:45] → call function_name(arg: value, ...)`

**Simple tool call (no arguments):**

```
║[  ] ⠋  beta     zen-nano     running  @trader      │ T42 ⬡ [10:46] → call get_status()                 ║
```

**Tool call with arguments:**

```
║[  ] ⠋  beta     zen-nano     running  @trader      │ T42 ⬡ [10:46] → call mine_asteroid(target: "ast_123", quantity: 10) ║
```

**Tool call with complex arguments (nested objects/arrays):**

```
║[  ] ⠋  gamma    ollama-qwen  running  @scout       │ T42 ⬡ [10:47] → call travel_to(destination: {...}, speed: 100)     ║
```

### 4. User Messages (Priority 4)

**Direct message format:** `T42 ⬡ [10:45] [YOU] Message...`

```
║[  ] ◦  gamma    ollama-qwen  idle     @scout       │ T41 ⬡ [10:40] [YOU] Scout sector 7 and report    ║
```

**Broadcast message format:** `T42 ⬡ [10:45] [SWARM] Message...`

```
║[  ] ⠋  epsilon  zen-pickle   running  @miner       │ T40 ⬡ [10:35] [SWARM] All units proceed to sector 7 ║
```

## Complete Example Dashboard

```
⬥══════════════════════════════════════════════════════════════════════════════════════════════════════════⬥
                                        ⬡ Z O E A   N O V A ⬡   COMMAND CENTER                              
⬥══════════════════════════════════════════════════════════════════════════════════════════════════════════⬥
⬧───────────────────────────────────────── SWARM BROADCAST ─────────────────────────────────────────────────⬧
T42 ⬡ [10:50] [alpha] All units: enemy detected in sector 7, proceeding with caution                        
⬧─────────────────────────────────────────── MYSIS SWARM ───────────────────────────────────────────────────⬧
╔════════════════════════════════════════════════════════════════════════════════════════════════════════════╗
║[→ ] ⠋  alpha    ollama       running  @crab_war... │ T42 ⬡ [10:45] [AI] Directive received, mining!    ║
║[  ] ⠋  beta     zen-nano     running  @trader      │ T42 ⬡ [10:46] → call mine_asteroid(target: "ast_123") ║
║[  ] ◦  gamma    ollama-qwen  idle     @scout       │ T41 ⬡ [10:40] [YOU] Scout sector 7 and report    ║
║[  ] ✖  delta    zen-pickle   errored  @miner       │ T40 ⬡ [10:35] Error: Connection timeout           ║
║[  ] ⠋  epsilon  ollama       running  @explorer    │ T42 ⬡ [10:48] [SWARM] All units proceed to sector 7 ║
╚════════════════════════════════════════════════════════════════════════════════════════════════════════════╝
[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST
```

## Implementation Details

### Color Scheme

- **Tool calls:** Yellow/gold (`colorTool = #FFCC00`)
  - Label `→ call`: yellow
  - Function name: yellow bold
  - Arguments: yellow

### Argument Formatting

Arguments are formatted to be concise and readable:

- **String values:** Quoted (`"value"`)
- **Numbers/booleans:** Direct (`10`, `true`)
- **Nested objects:** Collapsed to `{...}`
- **Arrays:** Collapsed to `[...]`
- **Empty args:** Function name with `()` (no space)

### Truncation

When messages are too long for the available space:
- Content is truncated with `...` suffix
- For tool calls, arguments are truncated first, preserving the function name
- Minimum space is reserved for function name before truncating arguments

### Priority Logic

The system checks memories in order:
1. If `State == "errored"` and `LastError != ""`, show error
2. Scan memories for AI replies (role=assistant, not tool calls) - show first found
3. Scan memories for tool calls (role=assistant with tool call prefix) - show first found
4. Scan memories for user messages (role=user) - show first found, labeled by source

### Backward Compatibility

For tests and cases where `RecentMemories` is empty but `LastMessage` is set, the system falls back to displaying `LastMessage` with timestamp using a legacy format.
