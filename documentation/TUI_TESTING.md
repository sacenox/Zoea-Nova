# TUI Testing Guide

## Overview

Zoea Nova's TUI test suite uses three complementary testing approaches:
1. **Unit tests** - Model state and business logic
2. **Golden file tests** - Visual regression testing
3. **Integration tests** - End-to-end user flows with teatest

This document describes testing practices, patterns, and lessons learned.

## Test Categories

### 1. Unit Tests (`tui_test.go`, `focus_test.go`, etc.)

Test model state transitions and business logic without rendering concerns.

**What to test:**
- Model initialization and state
- Navigation logic (up/down, view switching)
- Input mode transitions
- Help toggle, history navigation
- Error handling
- Business logic (truncation, formatting)

**Pattern:**
```go
func TestModelNavigation(t *testing.T) {
    m, cleanup := setupTestModel(t)
    defer cleanup()
    
    m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
    if m.selectedIdx != 1 {
        t.Errorf("expected selectedIdx=1, got %d", m.selectedIdx)
    }
}
```

**DO NOT test:**
- Width arithmetic (use golden files instead)
- ANSI codes directly (use golden files instead)
- Implementation details (padding, spacing calculations)

### 2. Golden File Tests (`golden_test.go`)

Visual regression tests using snapshot comparison. Each test has ANSI and Stripped variants.

**What to test:**
- Dashboard rendering
- Focus view layouts
- Log entry formatting
- JSON tree rendering
- Scrollbar positioning
- Any visual output

**Pattern:**
```go
func TestDashboard(t *testing.T) {
    defer setupGoldenTest(t)()  // Force TrueColor output
    
    output := renderDashboard(...)
    
    t.Run("ANSI", func(t *testing.T) {
        golden.RequireEqual(t, []byte(output))
    })
    
    t.Run("Stripped", func(t *testing.T) {
        stripped := stripANSIForGolden(output)
        golden.RequireEqual(t, []byte(stripped))
    })
}
```

**Update golden files:**
```bash
go test ./internal/tui -update
```

**Golden files location:** `internal/tui/testdata/`

**Golden file naming convention:**
- Directory: `testdata/Test<FunctionName>/<test_case_name>/`
- Files: `ANSI.golden` (with ANSI codes) and `Stripped.golden` (content only)
- Example: `testdata/TestDashboard/empty_swarm/ANSI.golden`

**Why two variants:**
- **ANSI** - Catches color and style regressions (RGB codes, bold, etc.)
- **Stripped** - Catches content and layout regressions (easier to read diffs)

### 3. Integration Tests (`integration_test.go`)

End-to-end tests using teatest to simulate full user interactions.

**What to test:**
- Complete user flows (create mysis, send broadcast, etc.)
- Async event handling
- Window resize behavior
- Viewport scrolling
- Multi-step interactions

**Pattern:**
```go
func TestIntegration_Example(t *testing.T) {
    m, cleanup := setupTestModel(t)
    defer cleanup()
    
    tm := teatest.NewTestModel(t, m,
        teatest.WithInitialTermSize(120, 40))
    defer tm.Quit()
    
    // Wait for initial render
    teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
        return bytes.Contains(bts, []byte("expected"))
    }, teatest.WithDuration(2*time.Second))
    
    // Send input
    tm.Send(tea.KeyMsg{Type: tea.KeyDown})
    
    // Wait briefly
    time.Sleep(100 * time.Millisecond)
    
    // Send quit to allow FinalModel to complete
    tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
    
    // Verify final state
    fm := tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
    finalModel := fm.(Model)
    // ... assertions
}
```

**Known Issue:** Integration tests must send quit command before calling `FinalModel()` to avoid timeouts.

## Testing Guidelines

### DO
- Test model state and logic with unit tests
- Test visual output with golden files
- Test user flows with integration tests
- Use `lipgloss.Width()` for display width calculations
- Use `setupGoldenTest(t)` for consistent ANSI output
- Update golden files when intentionally changing UI

### DO NOT
- Test width arithmetic (false positives, environment-dependent)
- Test ANSI codes directly (use golden files)
- Use `len()` on styled strings (use `lipgloss.Width()`)
- Test implementation details (padding calculations, internal spacing)
- Skip golden file updates after UI changes

## Lipgloss Testing Notes

### Width Calculations
Multi-byte Unicode characters cause width bugs:

- **`len()` returns BYTES, not display width**: Characters like `◈`, `◆`, `╭`, `─` are 3 bytes each but display as 1 column
- **ALWAYS use `lipgloss.Width()`**: Correctly calculates display width for Unicode and ANSI-styled strings
- **Test with Unicode-heavy content**: Section titles and borders use Unicode box-drawing characters

### Style Padding and Alignment
- `lipgloss.Style.Padding(0, 1)` adds 1 space on each side
- If one element has padding and another doesn't, decorations won't align
- `lipgloss.Width()` sets CONTENT width - borders and padding are added on top
- Example: `style.Width(98)` with border produces total width 100

### Background Color Rendering
**Critical lesson from Unicode character overlap fix:**

When rendering elements with selection backgrounds, content inside the styled area gets the background applied. To prevent background from extending to decorative elements:

**WRONG:**
```go
// Indicator is part of styled content - gets background
line := fmt.Sprintf("%s  %s", indicator, content)
return mysisItemSelectedStyle.Render(line)
```

**RIGHT:**
```go
// Indicator is outside styled content - no background
line := content
return " " + indicator + " " + mysisItemSelectedStyle.Render(line)
```

**Pattern:** Render decorative elements (icons, indicators, spacing) OUTSIDE the styled area to prevent background color from applying to them.

## Unicode Character Safety

### East Asian Ambiguous Width
Characters like `●`, `○`, `◆`, `◈`, `◇` are "East Asian Ambiguous Width":
- Render as 1 cell in Western locales
- Render as 2 cells in East Asian locales (Chinese, Japanese, Korean)

This causes visual overlap when width calculations assume 1 cell but terminals render 2 cells.

### Safe Replacements
| Ambiguous | Safe | Unicode | Name |
|-----------|------|---------|------|
| `●` | `∙` | U+2219 | Bullet Operator |
| `○` | `◦` | U+25E6 | White Bullet |
| `◆` | `⬥` | U+2B25 | Black Medium Diamond |
| `◈` | `⬧` | U+2B27 | Black Medium Lozenge |
| `◇` | `⬦` | U+2B26 | White Medium Diamond |

### Testing for Ambiguous Width
Use `TestUnicodeAmbiguousWidthSafety` to verify all Unicode characters are non-ambiguous:

```go
func TestUnicodeAmbiguousWidthSafety(t *testing.T) {
    chars := map[string]string{
        "filled_circle": "∙",
        "empty_circle":  "◦",
        // ...
    }
    
    for name, char := range chars {
        t.Run(name, func(t *testing.T) {
            r := []rune(char)[0]
            
            runewidth.DefaultCondition.EastAsianWidth = false
            narrowWidth := runewidth.RuneWidth(r)
            
            runewidth.DefaultCondition.EastAsianWidth = true
            wideWidth := runewidth.RuneWidth(r)
            
            if narrowWidth != wideWidth {
                t.Errorf("Character %q is ambiguous width", char)
            }
        })
    }
}
```

## Edge Case Testing

### Dashboard Edge Cases (`golden_edge_cases_test.go`)

The `TestDashboardEdgeCases` test covers:
- **Empty states** - No myses, no messages
- **Full states** - 16 myses (max swarm), 10 broadcasts (max display)
- **Error states** - All myses errored, stopped, idle
- **Mixed states** - Combination of running, idle, stopped, errored
- **Long content** - Very long broadcast messages (truncation)
- **Unicode content** - Emoji, CJK characters, box-drawing
- **Extreme dimensions** - 60 cols (narrow), 240 cols (wide), 100 lines (tall)
- **Loading states** - All myses loading simultaneously
- **Selection positions** - Top, middle, bottom of list

### Focus View Edge Cases

The `TestFocusViewEdgeCases` test covers:
- **Empty states** - No logs, fresh mysis
- **Long content** - Very long log entries, reasoning (wrapping)
- **Unicode content** - Emoji in logs, CJK text
- **Large JSON** - Huge tool results (truncation)
- **Many logs** - 100+ log entries (scrolling)
- **Narrow views** - 60 col focus view (wrapping)
- **All role types** - System, user (direct), user (broadcast), assistant, tool

### Net Indicator Edge Cases

The `TestNetIndicatorEdgeCases` test covers:
- **Activity states** - Idle, LLM, MCP
- **Animation positions** - Start, middle, end of bar
- **View modes** - Full bar, compact
- **All combinations** - 8 test cases covering state × position × mode

### Edge Case Coverage Summary

| Component | Test Cases | Edge Cases Covered |
|-----------|------------|-------------------|
| Dashboard | 15 | Empty, full, errors, Unicode, dimensions, selection |
| Focus View | 9 | Empty, long content, Unicode, JSON, many logs, narrow |
| Net Indicator | 8 | Idle, LLM, MCP, positions, compact |
| **Total** | **32** | **All major edge cases** |

## Unicode Character Testing

### Character Inventory Test

The `TestUnicodeCharacterInventory` test documents all Unicode characters used in the TUI with their codepoints, names, and usage locations. This test serves as a comprehensive reference and ensures all characters are properly documented.

**Verified characters:**
- **⬥** (U+2B25 BLACK MEDIUM DIAMOND) - Header corners, status bar, message prompt, spinner
- **⬧** (U+2B27 BLACK MEDIUM LOZENGE) - Section borders, broadcast prompt
- **⬡** (U+2B21 WHITE HEXAGON) - Title decoration, new mysis prompt, spinner frame 0/2
- **⬢** (U+2B22 BLACK HEXAGON) - Spinner frame 1/3
- **⬦** (U+2B26 WHITE MEDIUM DIAMOND) - Spinner frame 4/6, idle indicator
- **◦** (U+25E6 WHITE BULLET) - Idle state indicator
- **◌** (U+25CC DOTTED CIRCLE) - Stopped state indicator
- **✖** (U+2716 HEAVY MULTIPLICATION X) - Errored state indicator
- **⚙** (U+2699 GEAR) - Config provider prompt

### Spinner Animation Tests

The `TestSpinnerFrameRendering` test verifies all 8 spinner frames:

1. Frame 0: ⬡ (U+2B21)
2. Frame 1: ⬢ (U+2B22)
3. Frame 2: ⬡ (U+2B21)
4. Frame 3: ⬢ (U+2B22)
5. Frame 4: ⬦ (U+2B26)
6. Frame 5: ⬥ (U+2B25)
7. Frame 6: ⬦ (U+2B26)
8. Frame 7: ⬥ (U+2B25)

**Animation speed:** 125ms per frame (8 FPS)  
**Pattern:** Hexagonal theme alternating between filled/hollow shapes

### Width Consistency Tests

The `TestUnicodeWidthConsistency` test verifies that all characters:
- Have width of 1 via `lipgloss.Width()`
- Have width of 1 via `runewidth.RuneWidth()`
- Are NOT ambiguous width (render consistently in East Asian locales)

This prevents the Unicode overlap bug previously fixed in commit fb1d0b6.

### Terminal Compatibility Testing

**Manual testing checklist:**

1. **Test in primary terminal** (Alacritty, Kitty, WezTerm recommended):
   ```bash
   ./bin/zoea --offline
   ```

2. **Verify spinner animation:**
   - Create a mysis (press `n`)
   - Observe running state indicator animates smoothly
   - All 8 frames should be visible and distinct
   - No frame causes layout shift

3. **Verify state indicators:**
   - Idle: ◦ (white bullet)
   - Stopped: ◌ (dotted circle)
   - Errored: ✖ (heavy X)
   - All indicators should align vertically

4. **Verify section decorations:**
   - Header corners: ⬥
   - Section borders: ⬧
   - Title decorations: ⬡
   - All decorations should render clearly

5. **Verify input prompts:**
   - Press `b` for broadcast: ⬧
   - Press `m` for message: ⬥
   - Press `n` for new mysis: ⬡
   - Press `c` for config: ⚙

**Terminal compatibility notes:**

| Terminal | Status | Notes |
|----------|--------|-------|
| Alacritty | ✅ Excellent | Best rendering, all characters clear |
| Kitty | ✅ Excellent | Full Unicode support |
| WezTerm | ✅ Excellent | Native font fallback |
| iTerm2 | ✅ Good | May vary with font |
| Terminal.app | ⚠️ Fair | Font-dependent |
| Windows Terminal | ✅ Good | Requires Nerd Font or Unicode font |
| gnome-terminal | ✅ Good | Most fonts work well |
| xterm | ⚠️ Limited | May not render all characters |

**Font recommendations:**
- **Nerd Fonts:** FiraCode Nerd Font, JetBrains Mono Nerd Font
- **Unicode fonts:** Cascadia Code, Ubuntu Mono, Inconsolata
- **Fallback:** DejaVu Sans Mono (decent Unicode coverage)

### Character Replacement Strategy

If a character renders poorly in a specific terminal:

1. **Check font first:** Install a Nerd Font or Unicode-compatible font
2. **Test with `go test`:** Run `TestCharacterRenderingMatrix` to see all characters
3. **Report compatibility:** Document in KNOWN_ISSUES.md with terminal/font details
4. **Consider fallback:** Add configuration option for ASCII-only mode (future enhancement)

### Adding New Unicode Characters

When adding new Unicode characters to the TUI:

1. **Choose non-ambiguous width characters:** Use `runewidth` to verify
2. **Update `TestUnicodeCharacterInventory`:** Add to inventory map with usage
3. **Verify width:** Run `TestUnicodeWidthConsistency` with new character
4. **Update golden files:** Run `go test ./internal/tui -update`
5. **Test manually:** Verify in multiple terminals (see checklist above)

## Spinner Testing Guidelines

### Animation Testing

The spinner animation is tested via `TestSpinnerFrameRendering` which verifies:
- All 8 frames render correctly
- Each frame uses the correct Unicode character
- Frame sequence follows expected pattern (hexagonal theme)
- Animation speed matches specification (8 FPS, 125ms per frame)

**Test Pattern:**
```go
func TestSpinnerFrameRendering(t *testing.T) {
    frames := []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"}
    
    for i, expectedChar := range frames {
        t.Run(fmt.Sprintf("frame_%d", i), func(t *testing.T) {
            actual := getSpinnerFrame(i)
            if actual != expectedChar {
                t.Errorf("frame %d: expected %q, got %q", i, expectedChar, actual)
            }
        })
    }
}
```

**Manual Verification:**
1. Run `./bin/zoea --offline`
2. Create a mysis (press `n`)
3. Observe running state indicator animates smoothly
4. All frames should be visible and distinct
5. No frame should cause layout shift

**Spinner Specifications:**
- Frames: 8 total
- Pattern: Hexagonal theme alternating between filled/hollow shapes
- Speed: 8 FPS (125ms per frame)
- Unicode characters: All verified non-ambiguous width

## Golden File Update Process

### When to Update

Update golden files when:
1. **Intentionally changing UI** - Color changes, layout changes, new elements
2. **After Phase 1 border color fix** - Border RGB codes changed
3. **After Unicode character replacements** - Character codepoints changed
4. **After adding new golden tests** - Initial generation

### How to Update

```bash
# Update all golden files
go test ./internal/tui -update

# Update specific test golden files
go test ./internal/tui -run TestDashboard -update
go test ./internal/tui -run TestFocusView -update
```

### Verification After Update

```bash
# 1. Verify all tests pass
go test ./internal/tui

# 2. Review changes to golden files
git diff internal/tui/testdata/

# 3. Check for expected changes
# - Phase 1: RGB color codes changed from 42,42,85 to 107,0,179
# - Phase 3: Unicode codepoints changed for replaced characters
# - Phase 4: Spinner frames use new characters

# 4. Commit golden file updates with descriptive message
git add internal/tui/testdata/
git commit -m "test: update golden files after [phase description]"
```

### Golden File Structure

Each test has two variants:
- **ANSI.golden** - Full output with ANSI escape codes (RGB colors, styles)
- **Stripped.golden** - Content only, ANSI codes removed

**Example:**
```
internal/tui/testdata/
├── TestDashboard/
│   ├── empty_swarm/
│   │   ├── ANSI.golden       # Full ANSI with RGB codes
│   │   └── Stripped.golden   # Content only
│   └── with_swarm_messages/
│       ├── ANSI.golden
│       └── Stripped.golden
```

**Why Two Variants:**
- **ANSI** - Catches color and style regressions
- **Stripped** - Catches content and layout regressions (easier to read diffs)

## Layout Testing Guidelines

### Dashboard Layout

The dashboard layout calculation is tested via `TestDashboardLayoutCalculations` and `TestDashboardHeightCalculation`:

**What to test:**
- Terminal sizes from 80x20 to 200x100
- Various mysis counts (0, 1, 5, 10, 20)
- Various swarm message counts (0, 5, 10, 20)
- No panics during rendering
- Minimum height guards enforced

**Test Pattern:**
```go
func TestDashboardLayoutCalculations(t *testing.T) {
    testCases := []struct {
        width, height int
        mysisCount    int
        messageCount  int
    }{
        {80, 20, 0, 0},
        {120, 40, 5, 5},
        {200, 100, 20, 10},
    }
    
    for _, tc := range testCases {
        t.Run(fmt.Sprintf("%dx%d_m%d_msg%d", tc.width, tc.height, tc.mysisCount, tc.messageCount), func(t *testing.T) {
            // Render and verify no panics
            output := RenderDashboard(...)
            if output == "" {
                t.Error("empty output")
            }
        })
    }
}
```

### Focus View Layout

The focus view layout calculation is tested via `TestFocusViewLayoutCalculations` and `TestViewportHeightCalculation`:

**What to test:**
- Terminal sizes from 80x10 to 200x100
- Various log entry counts (0, 5, 20, 50, 100, 500, 1000)
- Viewport height minimum enforcement (5 lines)
- No negative heights
- No panics during rendering

**Test Pattern:**
```go
func TestFocusViewLayoutCalculations(t *testing.T) {
    testCases := []struct {
        width, height int
        logCount      int
    }{
        {80, 10, 0},
        {120, 40, 50},
        {200, 100, 1000},
    }
    
    for _, tc := range testCases {
        t.Run(fmt.Sprintf("%dx%d_logs%d", tc.width, tc.height, tc.logCount), func(t *testing.T) {
            // Render and verify no panics
            output := RenderFocusViewWithViewport(...)
            if output == "" {
                t.Error("empty output")
            }
        })
    }
}
```

### Minimum Dimension Guards

**Critical safety checks:**
- Dashboard mysis list height: `if mysisListHeight < 3 { mysisListHeight = 3 }`
- Focus view viewport height: `if vpHeight < 5 { vpHeight = 5 }`
- Content width: `if contentWidth < 20 { contentWidth = 20 }`

**Test Coverage:**
- `TestLayoutNoNegativeHeights` verifies no negative heights at extreme small sizes
- All guards tested with terminal heights from 5 to 100 lines
- All guards tested with terminal widths from 10 to 200 columns

## Width Calculation Testing Guidelines

### Lipgloss Width vs len()

**Critical Rule:** NEVER use `len()` for width calculations with Unicode or styled strings.

**Test Pattern:**
```go
func TestWidthCalculations(t *testing.T) {
    // Unicode character that is 3 bytes but displays as 1 column
    unicodeStr := "⬥"
    
    // WRONG: len() returns 3 (bytes)
    bytesWidth := len(unicodeStr)
    
    // RIGHT: lipgloss.Width() returns 1 (display width)
    displayWidth := lipgloss.Width(unicodeStr)
    
    if bytesWidth == displayWidth {
        t.Error("len() should NOT equal lipgloss.Width() for Unicode")
    }
    
    if displayWidth != 1 {
        t.Errorf("expected display width 1, got %d", displayWidth)
    }
}
```

### Content Width with Borders

When calculating content width with borders, account for border characters:

**Pattern:**
```go
// Double-line border adds 4 chars total (2 on each side)
contentWidth := terminalWidth - 4

// Minimum guard
if contentWidth < 20 {
    contentWidth = 20
}
```

**Test:**
```go
func TestContentWidthCalculation(t *testing.T) {
    widths := []int{10, 20, 40, 80, 120, 160, 200}
    
    for _, w := range widths {
        t.Run(fmt.Sprintf("width_%d", w), func(t *testing.T) {
            contentWidth := w - 4
            if contentWidth < 20 {
                contentWidth = 20
            }
            
            if contentWidth < 0 {
                t.Error("content width must never be negative")
            }
        })
    }
}
```

## Golden File Update Process

### When to Update

Update golden files when:
1. **Intentionally changing UI** - Color changes, layout changes, new elements
2. **After Phase 1 border color fix** - Border RGB codes changed
3. **After Unicode character replacements** - Character codepoints changed
4. **After adding new golden tests** - Initial generation

### How to Update

```bash
# Update all golden files
go test ./internal/tui -update

# Update specific test golden files
go test ./internal/tui -run TestDashboard -update
go test ./internal/tui -run TestFocusView -update
```

### Verification After Update

```bash
# 1. Verify all tests pass
go test ./internal/tui

# 2. Review changes to golden files
git diff internal/tui/testdata/

# 3. Check for expected changes
# - Phase 1: RGB color codes changed from 42,42,85 to 107,0,179
# - Phase 3: Unicode codepoints changed for replaced characters
# - Phase 4: Spinner frames use new characters

# 4. Commit golden file updates with descriptive message
git add internal/tui/testdata/
git commit -m "test: update golden files after [phase description]"
```

### Golden File Structure

Each test has two variants:
- **ANSI.golden** - Full output with ANSI escape codes (RGB colors, styles)
- **Stripped.golden** - Content only, ANSI codes removed

**Example:**
```
internal/tui/testdata/
├── TestDashboard/
│   ├── empty_swarm/
│   │   ├── ANSI.golden       # Full ANSI with RGB codes
│   │   └── Stripped.golden   # Content only
│   └── with_swarm_messages/
│       ├── ANSI.golden
│       └── Stripped.golden
```

**Why Two Variants:**
- **ANSI** - Catches color and style regressions
- **Stripped** - Catches content and layout regressions (easier to read diffs)

## Test Execution

### Run All Tests
```bash
go test ./internal/tui
```

### Run Specific Test Types
```bash
# Unit tests only
go test ./internal/tui -run TestModel

# Golden file tests only
go test ./internal/tui -run TestDashboard

# Integration tests only
go test ./internal/tui -run TestIntegration

# Unicode safety tests
go test ./internal/tui -run TestUnicodeAmbiguousWidthSafety
```

### Update Golden Files
```bash
# Update all golden files
go test ./internal/tui -update

# Update specific test golden files
go test ./internal/tui -run TestDashboard -update
go test ./internal/tui -run TestFocusView -update

# Verify changes after update
git diff internal/tui/testdata/
```

### Verify Build
```bash
make build
make test
```

## Test Infrastructure

### Files
- `testhelpers_test.go` - Test utilities and constants
- `golden_test.go` - Golden file tests (basic rendering)
- `golden_edge_cases_test.go` - Edge case golden tests (empty states, full states, Unicode, etc.)
- `integration_test.go` - Integration tests (18 tests)
- `tui_test.go` - Unit tests (22 tests)
- `focus_test.go` - Focus view tests (7 tests)
- `json_tree_test.go` - JSON rendering tests (4 tests)
- `scrollbar_test.go` - Scrollbar tests (4 tests)
- `spinner_test.go` - Spinner animation tests
- `statusbar_test.go` - Status bar tests
- `input_test.go` - Input prompt tests
- `unicode_test.go` - Unicode safety tests
- `width_test.go` - Width calculation tests
- `layout_test.go` - Layout calculation tests

### Dependencies
- `github.com/charmbracelet/x/exp/teatest` - Integration testing
- `github.com/charmbracelet/x/exp/golden` - Golden file comparison
- `github.com/mattn/go-runewidth` - Unicode width testing

### Directory Structure
```
internal/tui/
├── testdata/                    # Golden files
│   ├── TestDashboard/
│   │   ├── empty_swarm/
│   │   │   ├── ANSI.golden
│   │   │   └── Stripped.golden
│   │   └── with_swarm_messages/
│   │       ├── ANSI.golden
│   │       └── Stripped.golden
│   ├── TestDashboardEdgeCases/  # Edge case tests
│   │   ├── full_swarm_16_myses/
│   │   ├── all_myses_errored/
│   │   ├── unicode_emoji_in_messages/
│   │   └── ...
│   ├── TestFocusView/
│   ├── TestFocusViewEdgeCases/  # Edge case tests
│   │   ├── no_logs_empty_state/
│   │   ├── very_long_log_entry/
│   │   ├── unicode_emoji_logs/
│   │   └── ...
│   ├── TestNetIndicatorEdgeCases/
│   ├── TestHelp/
│   ├── TestLogEntry/
│   ├── TestJSONTree/
│   ├── TestScrollbar/
│   ├── TestMysisLine/
│   └── TestBroadcastLabels/
├── testhelpers_test.go          # Test utilities
├── golden_test.go               # Golden file tests
├── golden_edge_cases_test.go    # Edge case golden tests
├── integration_test.go          # Integration tests
├── tui_test.go                  # Unit tests
├── focus_test.go                # Focus view tests
├── json_tree_test.go            # JSON rendering tests
└── scrollbar_test.go            # Scrollbar tests
```

## Metrics

| Metric | Value |
|--------|-------|
| Total tests | 80+ |
| Test files | 14 |
| Unit tests | 22 |
| Golden tests | 54 (basic + edge cases) |
| Integration tests | 18 (1 passing, 17 need timeout fix) |
| Golden files | 218 (ANSI + Stripped variants) |
| Test coverage | 85.7% |

## Key Lessons

### 1. Golden Files Catch Visual Bugs
Golden files caught the Unicode character overlap issue that unit tests couldn't detect because they don't render backgrounds.

### 2. Background Styling Requires Careful Rendering
Decorative elements (icons, indicators) must be rendered OUTSIDE styled areas to prevent background color from applying to them.

### 3. Unicode Width Is Complex
- Never use `len()` for width calculations
- Always use `lipgloss.Width()`
- Test for East Asian Ambiguous Width characters
- Use `TestUnicodeAmbiguousWidthSafety` to prevent regressions

### 4. Tests Don't Replace Manual Verification
Always run the actual TUI application to verify visual changes. Tests validate logic and catch regressions, but human eyes catch visual issues tests miss.

Use offline mode for safe UI testing without connecting to the live game server:
```bash
./bin/zoea -offline
```

### 5. Integration Tests Need Quit Pattern
Integration tests must send quit command before calling `FinalModel()` to avoid timeouts. See `TestIntegration_DashboardNavigation` for the correct pattern.

## Future Improvements

1. Fix remaining 17 integration test timeouts
2. Add golden file tests for new UI components
3. Increase test coverage to 90%+
4. Add performance benchmarks for rendering
5. Document visual testing workflow for contributors

## References

- [Bubble Tea Testing Guide](https://github.com/charmbracelet/bubbletea/tree/master/tutorials/testing)
- [Golden File Testing](https://github.com/charmbracelet/x/tree/main/exp/golden)
- [Lipgloss Documentation](https://github.com/charmbracelet/lipgloss)
- [Unicode Width Testing](https://github.com/mattn/go-runewidth)
