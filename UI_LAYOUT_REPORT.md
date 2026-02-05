# Zoea Nova UI Layout Report

Generated: 2026-02-05

## Dashboard View (Main Screen)

### Layout Structure (Top to Bottom)

```
┌─────────────────────────────────────────────────────────┐
│ 1. HEADER (3 lines)                                     │
│ 2. STATUS BAR (1 line)                                  │
│ 3. SWARM BROADCAST SECTION (header + variable content)  │
│ 4. MYSIS SWARM SECTION (header + bordered panel)        │
│ 5. FOOTER (1 line)                                      │
└─────────────────────────────────────────────────────────┘
```

---

## Panel Details

### 1. HEADER (Lines 1-3)
**Location:** `dashboard.go` lines 33-56  
**Rendering:** `headerStyle.Width(width).Render(headerText)`

```
 ⬥═══════════════════════════════════════════════════════⬥
          ⬡ Z O E A   N O V A ⬡   COMMAND CENTER          
 ⬥═══════════════════════════════════════════════════════⬥
```

**Components:**
- Top border: ` ⬥` + `═` repeated + `⬥`
- Title: Centered text with hexagonal motifs (`⬡`)
- Bottom border: Same as top border
- **Style:** `headerStyle` (defined in `styles.go`)

---

### 2. STATUS BAR (Line 5)
**Location:** `dashboard.go` lines 58-89  
**Rendering:** `statusBarStyle.Width(width).Render(stats)`

```
  ∙  1  ◦  1  ◌  0  ✖  0
```

**Components:**
- **Running count:** `∙` (bullet operator U+2219) + count
- **Idle count:** `◦` (white bullet U+25E6) + count
- **Stopped count:** `◌` (dotted circle) + count
- **Errored count:** `✖` (heavy multiplication X) + count
- **Loading indicator:** (optional) spinner + count if any myses are loading

**Calculation:**
- Running: Counts myses with `state == "running"`
- Idle: `len(myses) - running - stopped - errored`
- Stopped: Counts myses with `state == "stopped"`
- Errored: Counts myses with `state == "errored"`

**Style:** `statusBarStyle` with colored icons:
- Running: `stateRunningStyle` (green)
- Idle: `stateIdleStyle` (yellow)
- Stopped: `stateStoppedStyle` (dimmed)
- Errored: `stateErroredStyle` (red)

---

### 3. SWARM BROADCAST SECTION (Lines 6-8)
**Location:** `dashboard.go` lines 91-122  
**Always visible:** Yes (shows placeholder when empty)

#### Header (Line 6)
```
⬧────────────────── SWARM BROADCAST ──────────────────⬧
```
**Rendering:** `renderSectionTitle("SWARM BROADCAST", width)`

#### Content (Lines 7-8, variable)
**When empty:**
```
No broadcasts yet. Press 'b' to broadcast.
```

**When populated (up to 10 messages):**
```
11:00:00 All units: proceed to sector 7
11:05:00 Target rich environment detected
```

**Format per message:**
- Timestamp: `HH:MM:SS` (dimmed style)
- Content: Single line, truncated if > `width - 15` characters
- Newlines replaced with spaces
- Truncation: `...` suffix if content too long

**Max messages:** 10 (constant `maxSwarmMessages`)  
**Order:** Most recent first

---

### 4. MYSIS SWARM SECTION (Lines 9-40)
**Location:** `dashboard.go` lines 124-161

#### Header (Line 9)
```
⬧──────────────────── MYSIS SWARM ────────────────────⬧
```
**Rendering:** `renderSectionTitle("MYSIS SWARM", width)`

#### Panel (Lines 10-40, bordered box)
**Border style:** Double-line box (`╔═╗║╚═╝`)  
**Rendering:** `mysisListStyle.Width(width - 2).Height(mysisListHeight).Render(content)`

**Height calculation:**
```
usedHeight = 6 (header + stats + mysis header + footer)
           + 1 (swarm header)
           + len(msgLines) (swarm content)
           + 2 (panel borders)
mysisListHeight = height - usedHeight
```

**When empty:**
```
╔════════════════════════════════════════════════════╗
║No myses. Press 'n' to create one.                 ║
║                                                    ║
╚════════════════════════════════════════════════════╝
```

**When populated:**
```
╔════════════════════════════════════════════════════╗
║ ⠋ alpha    running  [ollama] @crab_warrior │ ...  ║
║ ◦ beta     idle     [zen] @crab_trader │ ...      ║
╚════════════════════════════════════════════════════╝
```

#### Mysis Line Format
**Location:** `dashboard.go` lines 170-243  
**Function:** `renderMysisLine()`

**Structure:**
```
[space] [indicator] [space] [styled content]
```

**Indicator (outside styled area):**
- Running: Animated spinner (e.g., `⠋`)
- Idle: `◦` (white bullet)
- Stopped: `◌` (dotted circle)
- Errored: `✖` (heavy X)
- Loading: Animated spinner

**Content (inside styled area):**
```
[name(16)] [state(8)] [provider] [account] │ [last message]
```

**Fields:**
- **Name:** Max 16 chars, truncated with `...` if longer
- **State:** 8 chars, padded, colored by state
- **Provider:** `[ollama]` or `[opencode_zen]`, dimmed
- **Account:** `@username` or `(no account)`, dimmed
- **Last message:** Remaining width, truncated with `...`, dimmed, prefixed with ` │ `

**Selection styling:**
- Selected: `mysisItemSelectedStyle` (background color applied)
- Unselected: `mysisItemStyle` (no background)

**Note:** Indicator is rendered OUTSIDE the styled content to prevent background color from applying to it (fix from commit fb1d0b6).

---

### 5. FOOTER (Line 41)
**Location:** `dashboard.go` lines 163-165  
**Rendering:** `dimmedStyle.Render(hint)`

```
[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST
```

**Components:**
- Help hint: `[ ? ] HELP`
- New mysis hint: `[ n ] NEW MYSIS`
- Broadcast hint: `[ b ] BROADCAST`
- Separator: ` · ` (middle dot)

**Style:** `dimmedStyle` (gray/dimmed color)

---

## Focus View (Detailed Mysis View)

### Layout Structure

```
┌─────────────────────────────────────────────────────────┐
│ 1. HEADER (mysis name + account + state)                │
│ 2. CONVERSATION VIEWPORT (scrollable)                   │
│ 3. FOOTER (help hints)                                  │
└─────────────────────────────────────────────────────────┘
```

### Components

#### 1. Header
**Location:** `focus.go` `renderFocusHeader()`

```
⬧──────────────────── MYSIS: alpha ────────────────────⬧
@crab_warrior · running
```

**Components:**
- Title bar: `⬧` + `─` repeated + `MYSIS: [name]` + `─` repeated + `⬧`
- Account + state: `@username · state` (dimmed)

#### 2. Conversation Viewport
**Location:** `focus.go` `RenderFocusViewWithViewport()`

**Scrollable content showing log entries:**

**Entry types:**
- **System messages:** Cyan color, `[SYSTEM]` prefix
- **User messages:** Green color, `[USER]` or `[BROADCAST]` prefix
- **Assistant messages:** Magenta color, `[AI]` prefix
- **Tool calls:** Yellow color, `[TOOL_CALL]` prefix
- **Tool results:** Yellow color, `[TOOL_RESULT]` prefix

**Broadcast labels:**
- Self broadcast: `[BROADCAST (self)]`
- Swarm broadcast: `[SWARM BROADCAST]`

**Reasoning display:**
- When verbose mode enabled (`v` key)
- Shows reasoning content in dimmed style
- Prefix: `[REASONING]`

**JSON rendering:**
- Tool results with JSON are rendered as tree structure
- Collapsible/expandable (when verbose mode enabled)
- Syntax highlighting with colors

**Scrollbar:**
- Visual indicator on right edge
- Shows scroll position
- Characters: `█` (thumb), `│` (track)

#### 3. Footer
```
[ ESC ] BACK  ·  [ v ] VERBOSE  ·  [ ? ] HELP
```

---

## Key Observations

### Current Issues Identified

1. **Status Bar Position:**
   - Currently at line 5 (between header and swarm broadcast)
   - Styled with `statusBarStyle` (full-width panel)

2. **No Duplicate Status Icons:**
   - Status icons appear ONLY in the status bar (line 5)
   - Footer only shows keyboard hints
   - No duplication found in current code

3. **Swarm Broadcast Section:**
   - Always visible (shows placeholder when empty)
   - Takes variable height based on message count (max 10)
   - Positioned between status bar and mysis list

4. **Mysis List Panel:**
   - Bordered box with double-line style
   - Height dynamically calculated to fill remaining space
   - Each mysis line shows: indicator, name, state, provider, account, last message

---

## Code References

### Main Files
- `internal/tui/dashboard.go` - Dashboard rendering
- `internal/tui/focus.go` - Focus view rendering
- `internal/tui/styles.go` - Style definitions
- `internal/tui/app.go` - Model and update logic

### Key Functions
- `RenderDashboard()` - Main dashboard render (line 30)
- `renderMysisLine()` - Individual mysis line (line 170)
- `renderSectionTitle()` - Section headers (in `styles.go`)
- `RenderFocusViewWithViewport()` - Focus view with scrolling

### Test Files
- `internal/tui/testdata/TestDashboard/with_swarm_messages/Stripped.golden`
- `internal/tui/testdata/TestDashboard/empty_swarm/Stripped.golden`

---

## Summary

The current UI has **5 main sections** in the dashboard:

1. **Header** (3 lines) - Retro-futuristic banner
2. **Status Bar** (1 line) - Mysis state counts with icons
3. **Swarm Broadcast** (variable) - Recent broadcast messages
4. **Mysis Swarm** (variable) - Bordered list of myses
5. **Footer** (1 line) - Keyboard hints

**Total fixed height:** 6 lines (header + status + footer + 2 section headers)  
**Variable height:** Swarm messages + Mysis list (fills remaining space)
