# TOOL_CALLS Visual Design Proposal

**Date:** 2026-02-06  
**Status:** Proposal for implementation

---

## Current Behavior

Tool calls are currently rendered as **plain text** with no special visual treatment:

```
T42000 ⬡ [14:30] AI: [TOOL_CALLS]call_abc123:get_ship:{}
```

**Issues with current rendering:**
1. **Not visually distinct** - Looks like any other assistant message
2. **Raw storage format** - Exposes internal format (`[TOOL_CALLS]`, pipe delimiters, call IDs)
3. **Hard to read** - Multiple tool calls are concatenated with `|` separator
4. **No semantic information** - Tool name and arguments blend together
5. **Poor UX** - User must mentally parse the storage format

---

## Test Coverage

Created comprehensive test suite with **46 golden files** covering:

### Basic Rendering (12 test cases)
- Single tool call
- Multiple tool calls (up to 3)
- Various parameter types: string, number, array, object, nested JSON
- Long tool names (e.g., `zoea_search_messages`)
- Empty arguments (`{}`)
- Narrow terminal widths (60, 80, 120 columns)
- Verbose mode toggle

### Tool Calls with Reasoning (4 test cases)
- Single tool + reasoning (verbose OFF)
- Single tool + reasoning (verbose ON)
- Multiple tools + long reasoning
- Tool + multiline reasoning

### Edge Cases (7 test cases)
- Malformed: missing `[TOOL_CALLS]` prefix
- Empty content after prefix
- Incomplete tool call records (1 field, 2 fields)
- Invalid JSON arguments
- Unicode characters in arguments
- Very long arguments (500+ chars)

**Total test cases:** 23  
**Total golden files:** 46 (ANSI + Stripped variants)

---

## Proposed Visual Design

### Design Principles

1. **Clear tool identification** - Tool name prominently displayed
2. **Distinct visual style** - Different from regular messages
3. **Parseable format** - Arguments shown as formatted JSON
4. **Compact when needed** - Collapses details in non-verbose mode
5. **Professional aesthetic** - Matches retro-futuristic brand

### Visual Treatment Option 1: Compact List (Recommended)

```
T42000 ⬡ [14:30] AI: ⚡ Calling tools:
                      • get_ship()
                      • get_system(system_id: "sol")
                      • mine()
```

**Style details:**
- Prefix: `⚡ Calling tools:` (or `⚡ Tool calls:`) in yellow/gold color
- Each tool on its own line
- Indented with bullet (`•`) or diamond (`⬥`)
- Tool name in **bold yellow/gold**
- Arguments shown inline with simplified format
- Empty args `{}` shown as `()`

**Verbose mode:**
```
T42000 ⬡ [14:30] AI: ⚡ Calling tools:
                      ┌─ get_ship()
                      ├─ get_system
                      │  └─ system_id: "sol"
                      └─ mine()
```

### Visual Treatment Option 2: Inline Badges

```
T42000 ⬡ [14:30] AI: ⚡ get_ship  ⚡ get_system  ⚡ mine
```

**Style details:**
- Each tool as an inline badge
- Lightning bolt (`⚡`) or wrench (`🔧`) icon prefix
- Tool name in yellow/gold, bold
- Space-separated for multiple tools
- Arguments hidden in non-verbose mode

**Verbose mode:**
```
T42000 ⬡ [14:30] AI: ⚡ get_ship()
                      ⚡ get_system(system_id: "sol")
                      ⚡ mine()
```

### Visual Treatment Option 3: Bordered Section (Most Distinct)

```
T42000 ⬡ [14:30] AI: ⚡ Calling 3 tools
                      ╭─────────────────────╮
                      │ • get_ship()        │
                      │ • get_system(...)   │
                      │ • mine()            │
                      ╰─────────────────────╯
```

**Style details:**
- Header: `⚡ Calling N tools` (count shown)
- Bordered box with rounded corners
- Each tool listed inside
- Border in yellow/gold color
- Arguments truncated with `...` in non-verbose mode

**Verbose mode:**
```
T42000 ⬡ [14:30] AI: ⚡ Calling 3 tools
                      ╭─────────────────────────────────────────╮
                      │ • get_ship()                            │
                      │ • get_system                            │
                      │   - system_id: "sol"                    │
                      │ • mine()                                │
                      ╰─────────────────────────────────────────╯
```

---

## Recommendation: Option 1 (Compact List)

**Why Option 1?**
1. **Readability** - Clear hierarchy without extra visual clutter
2. **Flexibility** - Works well with 1 or many tools
3. **Consistency** - Matches reasoning display style (indented, structured)
4. **Terminal-friendly** - Uses simple Unicode chars (•) that render well everywhere
5. **Compact** - Doesn't take excessive vertical space
6. **Verbose toggle** - Natural expansion to show JSON tree structure

**Color scheme:**
- Lightning bolt: Yellow/gold (`colorTool` = `#FFCC00`)
- "Calling tools:" label: Yellow/gold, bold
- Tool names: Yellow/gold, bold
- Arguments: Dimmed yellow/gold
- Bullets: Yellow/gold

---

## Implementation Plan

### Phase 1: Detection and Parsing
1. Detect `[TOOL_CALLS]` prefix in `renderLogEntryImpl()`
2. Parse stored format using existing `parseStoredToolCalls()` helper
3. Extract tool names and arguments

### Phase 2: Formatting Logic
1. Create `renderToolCallsEntry()` helper function
2. Format tool calls based on verbose mode
3. Handle edge cases (malformed, empty, etc.)

### Phase 3: Visual Styling
1. Add tool call header ("⚡ Calling tools:")
2. Render each tool call as bulleted list item
3. Format arguments (simplified or JSON tree)
4. Apply yellow/gold color scheme

### Phase 4: Testing
1. Update golden files with new rendering
2. Verify all 46 golden tests pass
3. Manual testing in TUI (offline mode)

---

## Code Changes Required

### Files to Modify

**`internal/tui/focus.go`** (main rendering logic)
- Modify `renderLogEntryImpl()` to detect `[TOOL_CALLS]` prefix
- Add call to new `renderToolCallsEntry()` helper

**New function:** `renderToolCallsEntry()`
```go
func renderToolCallsEntry(content string, maxWidth int, verbose bool) []string {
    // Parse tool calls from storage format
    // Format as compact list
    // Return formatted lines
}
```

### Dependencies

Uses existing helpers:
- `parseStoredToolCalls()` from `internal/core/mysis.go` (or create TUI-local version)
- `wrapText()` for argument wrapping
- `renderJSONTree()` for verbose mode argument display (optional)

---

## Example Transformations

### Before (current)
```
T42000 ⬡ [14:30] AI: [TOOL_CALLS]call_1:get_ship:{}|call_2:get_system:{"system_id":"sol"}|call_3:mine:{}
```

### After (proposed)
```
T42000 ⬡ [14:30] AI: ⚡ Calling tools:
                      • get_ship()
                      • get_system(system_id: "sol")
                      • mine()
```

### After (verbose mode)
```
T42000 ⬡ [14:30] AI: ⚡ Calling tools:
                      • get_ship()
                      • get_system
                        {
                          "system_id": "sol"
                        }
                      • mine()
```

---

## Alternative Icons

If `⚡` (lightning bolt) doesn't render well on some terminals:
- `⚙` (gear) - mechanical/technical
- `▸` (play/execute) - action-oriented  
- `⬢` (hexagon) - matches brand hexagonal theme
- `◆` (diamond) - matches existing UI elements
- `◉` (target) - focus/execution

**Recommendation:** Stick with `⚡` (lightning bolt) as it:
- Conveys action/execution
- Is well-supported in Unicode fonts
- Is visually distinct from other UI elements
- Has thematic fit (energy, power, execution)

---

## Edge Case Handling

### Malformed Tool Calls

If tool call parsing fails, fall back to displaying raw content with warning:

```
T42000 ⬡ [14:30] AI: ⚠️ Malformed tool call data
                      [TOOL_CALLS]call_bad:incomplete
```

### Empty Tool Calls

If no tool calls are parsed after removing prefix:

```
T42000 ⬡ [14:30] AI: ⚠️ Empty tool call record
```

### Very Long Arguments

Truncate in non-verbose mode:
```
• configure(settings: {...})  [230 chars]
```

Expand in verbose mode:
```
• configure
  {
    "settings": {
      "speed": "fast",
      ...
    }
  }
```

---

## Success Criteria

1. ✅ Tool calls visually distinct from regular messages
2. ✅ Raw storage format hidden from user
3. ✅ Multiple tool calls clearly separated
4. ✅ Tool names prominently displayed
5. ✅ Arguments readable (simplified or JSON tree)
6. ✅ Verbose mode toggle works
7. ✅ All 46 golden tests pass with new rendering
8. ✅ Works at various terminal widths (60-200 cols)
9. ✅ Edge cases handled gracefully

---

## Next Steps

1. **Review and approve** visual design (Option 1 recommended)
2. **Implement** rendering logic in `focus.go`
3. **Update** golden files with new rendering
4. **Test** manually in TUI (offline mode)
5. **Document** in UI_LAYOUT_REPORT.md
6. **Commit** with clear description

---

**Status:** Ready for implementation  
**Estimated effort:** 2-3 hours (implementation + testing)  
**Risk:** Low (comprehensive test coverage in place)
