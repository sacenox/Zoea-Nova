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
go test ./internal/tui -update
```

### Verify Build
```bash
make build
make test
```

## Test Infrastructure

### Files
- `testhelpers_test.go` - Test utilities and constants
- `golden_test.go` - Golden file tests (22 tests, 44 golden files)
- `integration_test.go` - Integration tests (18 tests)
- `tui_test.go` - Unit tests (22 tests)
- `focus_test.go` - Focus view tests (7 tests)
- `json_tree_test.go` - JSON rendering tests (4 tests)
- `scrollbar_test.go` - Scrollbar tests (4 tests)

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
│   ├── TestFocusView/
│   ├── TestHelp/
│   ├── TestLogEntry/
│   ├── TestJSONTree/
│   ├── TestScrollbar/
│   ├── TestMysisLine/
│   └── TestBroadcastLabels/
├── testhelpers_test.go          # Test utilities
├── golden_test.go               # Golden file tests
├── integration_test.go          # Integration tests
├── tui_test.go                  # Unit tests
├── focus_test.go                # Focus view tests
├── json_tree_test.go            # JSON rendering tests
└── scrollbar_test.go            # Scrollbar tests
```

## Metrics

| Metric | Value |
|--------|-------|
| Total tests | 48 |
| Test files | 7 |
| Unit tests | 22 |
| Golden tests | 22 |
| Integration tests | 18 (1 passing, 17 need timeout fix) |
| Golden files | 44 (ANSI + Stripped variants) |
| Test coverage | 83% |

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
