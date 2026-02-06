# Zoea Nova UI Layout Report

Generated: 2026-02-05 (Updated after v1 completion)

**Note on Unicode Characters:** This report documents the actual Unicode characters used in the code. Terminal font rendering may display these characters differently. For example, `⬧` (BLACK MEDIUM LOZENGE) may appear as `◈` (WHITE DIAMOND CONTAINING BLACK SMALL DIAMOND) depending on your terminal font.

## Dashboard View (Main Screen)

### Layout Structure (Top to Bottom)

```
┌─────────────────────────────────────────────────────────┐
│ 1. HEADER (3 lines)                                     │
│ 2. SWARM BROADCAST SECTION (header + variable content)  │
│ 3. MYSIS SWARM SECTION (header + list)                  │
│ 4. INPUT PROMPT (1 line, when active)                   │
│ 5. FOOTER (1 line)                                      │
│ 6. STATUS BAR (1 line)                                  │
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
- Top border: `⬥` + `═` repeated + `⬥`
- Title: Centered text with hexagonal motifs (`⬡`)
- Bottom border: Same as top border
- **Style:** `headerStyle` (defined in `styles.go`)
- **Character:** `⬥` (U+2B25 BLACK MEDIUM DIAMOND)

---

### 2. SWARM BROADCAST SECTION (Lines 6-8)
**Location:** `dashboard.go` lines 68-108  
**Always visible:** Yes (shows placeholder when empty)

#### Header (Line 6)
```
⬧────────────────── SWARM BROADCAST ──────────────────⬧
```
**Rendering:** `renderSectionTitle("SWARM BROADCAST", width)`
**Character:** `⬧` (U+2B27 BLACK MEDIUM LOZENGE)

#### Content (Lines 7-8, variable)
**When empty:**
```
No broadcasts yet. Press 'b' to broadcast.
```

**When populated (up to 10 messages):**
```
11:00:00 [mysis-1] All units: proceed to sector 7
11:05:00 [alpha] Target rich environment detected
```

**Format per message:**
- Timestamp: `HH:MM:SS` (dimmed style)
- Sender label: `[sender_name]` (highlighted style) - NEW in v1
- Content: Single line, truncated if > `width - 15 - senderTextWidth` characters
- Newlines replaced with spaces
- Truncation: `...` suffix if content too long

**Max messages:** 10 (constant `maxSwarmMessages`)  
**Order:** Most recent first

**Sender display logic:**
- Shows sender mysis name in brackets
- Uses `formatSenderLabel(senderID, senderName)` helper
- Empty label if sender info unavailable

---

### 3. MYSIS SWARM SECTION (Lines 9-40)
**Location:** `dashboard.go` lines 110-147

#### Header (Line 9)
```
⬧──────────────────── MYSIS SWARM ────────────────────⬧
```
**Rendering:** `renderSectionTitle("MYSIS SWARM", width)`
**Character:** `⬧` (U+2B27 BLACK MEDIUM LOZENGE)

#### Panel (Lines 10-40, bordered box)
**Border style:** Double-line box (`╔═╗║╚═╝`)
**Rendering:** `mysisListStyle.Width(width - 2).Height(mysisListHeight).Render(content)`
**Style:** `mysisListStyle` with `lipgloss.DoubleBorder()` (defined in `styles.go:61-63`)
**Border color:** `colorBrandDim` (#6B00B3) - Changed in Phase 1 from `colorBorder` (#2A2A55) for improved contrast (2.85:1 → ~3.0:1)

**Height calculation:**
```
usedHeight = 5 (header + mysis header + footer)
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
║ ⠋ alpha    running  [ollama] @crab_warrior │ 15:04:05 Mining ore...  ║
║ ◦ beta     idle     [zen] @crab_trader │ 15:03:12 Ready to trade    ║
╚════════════════════════════════════════════════════╝
```

#### Mysis Line Format
**Location:** `dashboard.go` lines 156-247  
**Function:** `renderMysisLine()`

**Structure:**
```
[space] [indicator] [space] [styled content]
```

**Indicator (outside styled area):**
- Running: Animated spinner - cycles through `⬡`, `⬢`, `⬦`, `⬥` (hexagonal theme)
- Idle: `◦` (white bullet)
- Stopped: `◌` (dotted circle)
- Errored: `✖` (heavy X)
- Loading: Same animated spinner as running state

**Spinner details:**
- Frames: `["⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"]`
- Speed: 8 FPS (125ms per frame)
- Style: Brand purple color
- Location: `app.go:87`

**Content (inside styled area):**
```
[name(16)] [state(8)] [provider] [account] │ [timestamp] [last message]
```

**Fields:**
- **Name:** Max 16 chars, truncated with `...` if longer
- **State:** 8 chars, padded, colored by state
- **Provider:** `[ollama]` or `[opencode_zen]`, dimmed
- **Account:** `@username` or `(no account)`, dimmed
- **Last message timestamp:** `HH:MM:SS` format, dimmed - NEW in v1
- **Last message:** Remaining width, truncated with `...`, dimmed, prefixed with ` │ `

**Error state handling:**
- If state is `errored` and `LastMessage` is empty, shows `LastError` instead
- Format: `Error: [error message]`

**Selection styling:**
- Selected: `mysisItemSelectedStyle` (background color applied)
- Unselected: `mysisItemStyle` (no background)

**Note:** Indicator is rendered OUTSIDE the styled content to prevent background color from applying to it (fix from commit fb1d0b6).

---

### 4. INPUT PROMPT (when active)
**Location:** `input.go` `InputModel` (lines 36-237)  
**Rendering:** Bordered text input box with mode-specific indicator

```
⬧  Attack sector 7|                    (Broadcast mode)
⬥  Check your cargo|                   (Message mode)
```

**Mode Indicators:**
- **Broadcast:** `⬧` (U+2B27 BLACK MEDIUM LOZENGE) - Purple/brand color
- **Message:** `⬥` (U+2B25 BLACK MEDIUM DIAMOND) - Purple/brand color
- **New Mysis:** `⬡` (U+2B21 WHITE HEXAGON) - Purple/brand color
- **Config (Provider):** `⚙` (gear icon) - Purple/brand color
- **Config (Model):** `cfg` (text) - Purple/brand color

**Behavior:**
- **Active state:** Shows actual text input with mode indicator
- **Sending state:** Shows animated spinner with "Broadcasting..." or "Sending..." label
- **Inactive state:** Shows placeholder "Press 'm' to message, 'b' to broadcast..."

**Features:**
- History navigation with up/down arrows (message and broadcast modes)
- Stores last 100 messages
- Character limit: 1000 characters
- Width adapts to terminal size
- Placeholder text varies by mode

**Style:** 
- Border: Rounded, teal color (`colorTeal`)
- Prompt: Brand purple (`colorBrand`), bold
- Placeholder: Dimmed/muted color

**Tests:** `internal/tui/input_test.go` (8 test functions, 14 golden files)

---

### 5. FOOTER (keyboard hints)
**Location:** `dashboard.go` lines 150-151  
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

### 6. STATUS BAR (bottom line)
**Location:** `app.go` status rendering
**Rendering:** Bottom status line with LLM indicator, view name, and mysis count

```
◈LLM [████████░░] DASHBOARD                    Myses: 1/3 running
```

**Components:**
- **LLM indicator:** `⬥LLM` with progress bar showing active LLM calls
- **View name:** `DASHBOARD` or `FOCUS: [mysis-id]`
- **Mysis count:** Shows running/total myses (e.g., `1/3 running`)
- **Character:** `⬥` (U+2B25 BLACK MEDIUM DIAMOND)

**Style:** Spans full width, dimmed colors

---

## Focus View (Detailed Mysis View)

### Layout Structure

```
┌─────────────────────────────────────────────────────────┐
│ 1. HEADER (mysis name + position in swarm)              │
│ 2. INFO PANEL (id, state, provider, account, created)   │
│ 3. CONVERSATION SECTION HEADER (with scroll position)   │
│ 4. CONVERSATION VIEWPORT (scrollable with scrollbar)    │
│ 5. INPUT PROMPT (1 line, when active)                   │
│ 6. FOOTER (help hints + verbose toggle)                 │
│ 7. STATUS BAR (1 line)                                  │
└─────────────────────────────────────────────────────────┘
```

### Components

#### 1. Header
**Location:** `focus.go` `renderFocusHeader()` (lines 621+)

```
 ⬥─── ⬡ MYSIS: alpha (2/16) ⬡ ───⬥
```

**Components:**
- Title bar: ` ⬥` + `─` repeated + ` ⬡ MYSIS: [name] ([index]/[total]) ⬡ ` + `─` repeated + `⬥`
- Mysis count: Shows position in swarm (e.g., `(2/16)`)
- Centered title with balanced dashes on both sides
- **Character:** `⬥` (U+2B25 BLACK MEDIUM DIAMOND)

#### 2. Info Panel
**Location:** `focus.go` `RenderFocusViewWithViewport()` (lines 180+)

```
ID: 6b152b72-09e4-4695-aaa2-9a529147d3d7  State: running  Provider: ollama
Account: prawn_trader  Created: 2026-02-05 20:29
```

**Fields displayed:**
- **ID:** Mysis unique identifier (full UUID)
- **State:** Current state with color coding (running/idle/stopped/errored)
- **Provider:** LLM provider name (ollama/opencode_zen)
- **Account:** Game account username or "(not logged in)"
- **Created:** Creation timestamp in `YYYY-MM-DD HH:MM` format
- **Error:** Last error message (only shown if state is `errored`)

**Layout:** Single line with all fields separated by spaces, no visible border

**Spinner indicator:**
- Shows animated spinner next to state when mysis is thinking
- Format: `running ⠋ thinking...`

#### 3. Conversation Viewport
**Location:** `focus.go` `RenderFocusViewWithViewport()` (lines 180+)

**Section header:**
```
⬧──────────────────── CONVERSATION LOG ────────────────────⬧
```
- **Character:** `⬧` (U+2B27 BLACK MEDIUM LOZENGE)
- Scroll position shown when not at bottom: `LINE [current]/[total]` (appears on right side)

**Scrollable content showing log entries:**

**Entry format:**
```
15:04:05 AI: Response text here...
```
- **Timestamp:** `HH:MM:SS` format at start of each entry - NEW in v1
- **Role prefix:** `YOU:`, `AI:`, `SYS:`, `TOOL:`
- **Content:** Wrapped text with proper Unicode width handling

**Entry types:**
- **System messages:** Cyan color, `SYS:` prefix
- **User messages:** Green color, `YOU:` prefix
- **Broadcast (self):** Green color, `YOU (BROADCAST):` prefix - NEW in v1
- **Broadcast (swarm):** Orange color, `SWARM (sender_name):` prefix - NEW in v1
- **Assistant messages:** Magenta color, `AI:` prefix
- **Tool calls:** Yellow color, `TOOL:` prefix
- **Tool results:** Yellow color, `TOOL:` prefix

**Reasoning display:**
- When verbose mode enabled (`v` key)
- Shows reasoning content in dimmed purple style
- Prefix: `REASONING:`
- Smart truncation when verbose OFF: shows first line, `[x more]`, last 2 lines - NEW in v1

**JSON rendering:**
- Tool results with JSON are rendered as tree structure - NEW in v1
- Collapsible/expandable (when verbose mode enabled)
- Syntax highlighting with colors
- Width-constrained to fit viewport

**Scrollbar:**
- Visual indicator on right edge - NEW in v1
- Shows scroll position
- Characters: `█` (thumb), `│` (track)
- Rendered with `renderScrollbar()` helper

#### 4. Input Prompt
**Location:** `input.go` `InputModel` (lines 36-237)  
**Rendering:** Same as dashboard - bordered text input box with mode indicator

```
⬥  Message to mysis...|
```

**In focus view:**
- Typically used for direct messages to the focused mysis
- Shows `⬥` (message) indicator
- Same features as dashboard input (history, character limit, width adaptation)

**Visibility:** Shows when user is entering text for message
**Style:** Cyan/teal bordered box with brand purple prompt indicator

---

#### 5. Footer
**Location:** `focus.go` `RenderFocusViewWithViewport()` (lines 180+)

```
[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM  ·  [ v ] VERBOSE: OFF
```

**Components:**
- Navigation: `[ ESC ] BACK`
- Actions: `[ m ] MESSAGE`
- Scrolling: `[ ↑↓ ] SCROLL`, `[ G ] BOTTOM`
- Verbose toggle: `[ v ] VERBOSE: ON/OFF` - Shows current state

---

#### 6. Status Bar
**Location:** `app.go` status rendering
**Rendering:** Bottom status line (same as dashboard)

```
◈LLM [████████░░] FOCUS: 6b152b72              Myses: 1/3 running
```

**Components:**
- **LLM indicator:** `⬥LLM` with progress bar
- **View name:** `FOCUS: [mysis-id-prefix]` (shows first 8 chars of ID)
- **Mysis count:** Shows running/total myses
- **Character:** `⬥` (U+2B25 BLACK MEDIUM DIAMOND)

---

## Key Features (v1 Enhancements)

### Dashboard Improvements

1. **Swarm Broadcast Section:**
   - Always visible (shows placeholder when empty)
   - Takes variable height based on message count (max 10)
   - Positioned between header and mysis list
   - **NEW:** Shows sender name in brackets for each broadcast

2. **Mysis List Panel:**
   - Bordered box with double-line style
   - Height dynamically calculated to fill remaining space
   - Each mysis line shows: indicator, name, state, provider, account, last message
   - **NEW:** Last message includes timestamp (HH:MM:SS)
   - **NEW:** Error state shows last error message when no last message available

### Focus View Improvements

1. **Info Panel:**
   - **NEW:** Shows game account username or "(not logged in)"
   - **NEW:** Shows creation timestamp
   - **NEW:** Single-line compact layout with full UUID

2. **Conversation Log:**
   - **NEW:** Each entry shows timestamp (HH:MM:SS)
   - **NEW:** Broadcast entries show sender name
   - **NEW:** Reasoning display with smart truncation
   - **NEW:** JSON tree rendering for tool results
   - **NEW:** Visual scrollbar indicator
   - **NEW:** Scroll position display (LINE x/y)
   - **NEW:** Verbose mode toggle affects reasoning and JSON display

---

## Code References

### Main Files
- `internal/tui/dashboard.go` - Dashboard rendering (268 lines)
- `internal/tui/focus.go` - Focus view rendering (655 lines)
- `internal/tui/styles.go` - Style definitions
- `internal/tui/app.go` - Model and update logic
- `internal/tui/json_tree.go` - JSON tree rendering - NEW in v1
- `internal/tui/scrollbar.go` - Scrollbar rendering - NEW in v1

### Key Functions
- `RenderDashboard()` - Main dashboard render (line 34)
- `renderMysisLine()` - Individual mysis line (line 156)
- `renderSectionTitle()` - Section headers (in `styles.go`)
- `RenderFocusViewWithViewport()` - Focus view with scrolling (line 180)
- `renderLogEntryImpl()` - Log entry rendering with timestamps (line 376)
- `renderJSONTree()` - JSON tree structure rendering (in `json_tree.go`)
- `renderScrollbar()` - Scrollbar indicator rendering (in `scrollbar.go`)
- `formatSenderLabel()` - Broadcast sender label formatting (in `styles.go`)

### Test Files
- `internal/tui/testdata/TestDashboard/` - Dashboard golden files
- `internal/tui/testdata/TestFocusView/` - Focus view golden files
- `internal/tui/testdata/TestLogEntry/` - Log entry golden files
- `internal/tui/testdata/TestJSONTree/` - JSON tree golden files
- `internal/tui/testdata/TestScrollbar/` - Scrollbar golden files

---

## Summary

### Dashboard View

The current UI has **6 main sections** in the dashboard:

1. **Header** (3 lines) - Retro-futuristic banner with diamond (`⬥`) corners
2. **Swarm Broadcast** (variable) - Recent broadcast messages with sender labels
3. **Mysis Swarm** (variable) - Bordered list of myses with timestamps
4. **Input Prompt** (1 line, when active) - Text input for messages/broadcasts
5. **Footer** (1 line) - Keyboard hints
6. **Status Bar** (1 line) - LLM activity, view name, mysis count

**Total fixed height:** 6 lines (header + footer + status + 2 section headers)  
**Variable height:** Swarm messages + Mysis list (fills remaining space)

### Focus View

The current UI has **7 main sections** in the focus view:

1. **Header** (1 line) - Mysis name with position indicator and diamond (`⬥`) decoration
2. **Info Panel** (1 line) - Mysis details (ID, state, provider, account, created)
3. **Conversation Section Header** (1 line) - Title with lozenge (`⬧`) decoration
4. **Conversation Viewport** (variable) - Scrollable log entries with scrollbar
5. **Input Prompt** (1 line, when active) - Text input for messages
6. **Footer** (1 line) - Keyboard hints with verbose toggle
7. **Status Bar** (1 line) - LLM activity, mysis ID, mysis count

**Total fixed height:** 5 lines (header + info + section header + footer + status)  
**Variable height:** Conversation viewport (fills remaining space)

### v1 Enhancements Summary

- **Timestamps:** All messages and log entries show HH:MM:SS timestamps
- **Sender labels:** Broadcasts show sender mysis name
- **Account display:** Shows game account username throughout UI
- **Reasoning display:** LLM reasoning visible with smart truncation (e.g., `[448 more]`)
- **JSON tree rendering:** Tool results rendered as collapsible tree structure
- **Scrollbar indicator:** Visual scroll position feedback (right edge of viewport)
- **Verbose mode:** Toggle for detailed reasoning and JSON display
- **Status bar:** Shows LLM activity, current view, and mysis count
- **Input prompt:** Bordered text input box for messages and broadcasts
- **Error handling:** Errored myses show last error in dashboard
- **Unicode decorations:** 
  - `⬥` (U+2B25 BLACK MEDIUM DIAMOND) for header corners and status indicators
  - `⬧` (U+2B27 BLACK MEDIUM LOZENGE) for section title borders
  - `⬡` (U+2B21 WHITE HEXAGON) for title decorations

---

## UI Fixes & Improvements (2026-02-05)

### Phase 1: Border Rendering Improvements

**Issue:** Border color had insufficient contrast (1.48:1) against background.

**Fix Applied:**
- Changed border color from `colorBorder` (#2A2A55) to `colorBrandDim` (#6B00B3)
- Contrast improved from 1.48:1 to ~3.0:1 (2x improvement)
- Meets WCAG 3.0:1 UI component standard
- Maintains brand purple aesthetic

**Files Modified:**
- `internal/tui/styles.go` - Updated `mysisListStyle` and `logStyle` border colors

**Testing:**
- Manual verification in Ghostty terminal
- Golden file tests updated and passing
- Verified in TrueColor (24-bit RGB) environments

**References:** `documentation/TERMINAL_COMPATIBILITY.md`

### Phase 2: Focus View Header Verification

**Investigation:** Comprehensive audit of focus view header rendering.

**Finding:** **No bug found** - header renders correctly.

**Verification:**
- Created `focus_header_test.go` with 3 test cases (80x20, 120x40, 160x60)
- All 5 existing golden tests show header on line 1
- Code trace confirms header is first section in output
- Viewport calculation includes header height
- No code path can scroll header off-screen

**Evidence:**
- Header always renders on line 1 of all outputs
- Header contains all expected Unicode decorations (⬥, ⬡, ─)
- Header width scales correctly to terminal width
- Header is never scrolled off-screen

**Files Added:**
- `internal/tui/focus_header_test.go` (119 lines, 3 test cases)

**References:** `documentation/PHASE_2_HEADER_INVESTIGATION.md`

### Phase 3: Unicode Character Safety

**Audit:** All Unicode characters verified for East Asian Ambiguous Width safety.

**Characters Verified Safe:**
- `⬥` (U+2B25 BLACK MEDIUM DIAMOND) - Spinner, headers, indicators
- `⬧` (U+2B27 BLACK MEDIUM LOZENGE) - Section borders
- `⬡` (U+2B21 WHITE HEXAGON) - Title decorations, spinner
- `⬢` (U+2B22 BLACK HEXAGON) - Spinner frame
- `⬦` (U+2B26 WHITE MEDIUM DIAMOND) - Spinner frame, idle indicator
- `◦` (U+25E6 WHITE BULLET) - Idle state
- `◌` (U+25CC DOTTED CIRCLE) - Stopped state
- `✖` (U+2716 HEAVY MULTIPLICATION X) - Error state
- `⚙` (U+2699 GEAR) - Config prompt

**Previous Unsafe Characters Replaced:**
- `●` → `∙` (U+2219 BULLET OPERATOR)
- `○` → `◦` (U+25E6 WHITE BULLET)
- `◆` → `⬥` (U+2B25 BLACK MEDIUM DIAMOND)
- `◈` → `⬧` (U+2B27 BLACK MEDIUM LOZENGE)
- `◇` → `⬦` (U+2B26 WHITE MEDIUM DIAMOND)

**Testing:**
- `TestUnicodeAmbiguousWidthSafety` - Verifies all characters are non-ambiguous
- `TestUnicodeCharacterInventory` - Documents all characters with codepoints
- `TestUnicodeWidthConsistency` - Verifies width calculations

**Terminal Compatibility:** Tested in Ghostty (TrueColor), verified rendering.

### Terminal Requirements

**Minimum Dimensions:**
- **Width:** 80 columns
- **Height:** 20 lines
- **Enforced at:** `internal/tui/app.go:243-252`

**Warning Message:**
When terminal is too small, displays:
```
Terminal too small!

Minimum size: 80x20
Current size: {width}x{height}

Please resize your terminal.
```

**Layout Safety:**
- Dashboard mysis list height: minimum 3 lines, never negative
- Focus view viewport height: minimum 5 lines, never negative
- Content width: minimum 20 chars, never negative
- All calculations verified with 65 test cases (Phase 6)

**Recommended Terminals:**
- Alacritty (excellent Unicode support)
- Kitty (full Unicode support)
- WezTerm (native font fallback)
- iTerm2 (good with proper font)
- Windows Terminal (requires Nerd Font)

**Font Recommendations:**
- Nerd Fonts: FiraCode Nerd Font, JetBrains Mono Nerd Font
- Unicode fonts: Cascadia Code, Ubuntu Mono, Inconsolata
- Fallback: DejaVu Sans Mono

**References:** `documentation/TERMINAL_COMPATIBILITY.md`, `documentation/TUI_TESTING.md`
