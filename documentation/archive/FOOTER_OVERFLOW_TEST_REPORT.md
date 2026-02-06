# Footer Overflow Test Report

**Date:** 2026-02-06  
**Test File:** `internal/tui/footer_overflow_test.go`  
**Status:** ✅ Tests Created, ❌ Overflow Issues Identified

---

## Executive Summary

Created comprehensive overflow tests for footer keyboard hints in both dashboard and focus views. Tests **IDENTIFIED OVERFLOW ISSUES** at narrow terminal widths.

### Key Findings

| View | Footer Length | Minimum Width | Overflow At |
|------|---------------|---------------|-------------|
| **Dashboard** | 50 chars | 80 cols | **< 50 cols** |
| **Focus (verbose OFF)** | 88 chars | 80 cols | **< 88 cols (INCLUDING 80!)** |
| **Focus (verbose ON)** | 87 chars | 80 cols | **< 87 cols (INCLUDING 80!)** |

**CRITICAL:** Focus view footer exceeds the enforced minimum terminal width of 80 columns.

---

## Test Results

### Dashboard Footer Tests

**Text:** `"[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST"`  
**Length:** 50 characters

| Terminal Width | Result | Notes |
|----------------|--------|-------|
| 120 cols | ✅ PASS | 70 chars margin |
| 80 cols | ✅ PASS | 30 chars margin |
| 60 cols | ✅ PASS | 10 chars margin |
| **40 cols** | ❌ **FAIL** | **Overflows by 10 chars** |

**Verdict:** Dashboard footer is safe at minimum width (80 cols) but overflows at 40 cols.

---

### Focus View Footer Tests

**Text (verbose OFF):** `"[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM  ·  [ v ] VERBOSE: OFF"`  
**Length:** 88 characters

**Text (verbose ON):** `"[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM  ·  [ v ] VERBOSE: ON"`  
**Length:** 87 characters

| Terminal Width | Verbose OFF | Verbose ON | Notes |
|----------------|-------------|------------|-------|
| 120 cols | ✅ PASS | ✅ PASS | 32-33 chars margin |
| **80 cols** | ❌ **FAIL** | ❌ **FAIL** | **Overflows by 7-8 chars** |
| 60 cols | ❌ FAIL | ❌ FAIL | Overflows by 27-28 chars |
| 40 cols | ❌ FAIL | ❌ FAIL | Overflows by 47-48 chars |

**Verdict:** Focus footer exceeds the enforced minimum terminal width (80 cols) by **7-8 characters**.

---

## Impact Assessment

### User-Visible Issues

1. **Focus view footer wraps** - When terminal is at minimum width (80 cols), footer may wrap to a second line or get truncated awkwardly
2. **Layout corruption** - Wrapped footer steals space from viewport content
3. **Poor UX at narrow widths** - Users with 80-column terminals see broken layout

### Code Locations

- **Dashboard footer:** `internal/tui/dashboard.go:150`
- **Focus footer:** `internal/tui/focus.go:268`

Both use `dimmedStyle.Render()` with no width constraint.

---

## Test Coverage

### Tests Created

1. **`TestDashboardFooterOverflow`** - 4 width scenarios (40, 60, 80, 120 cols)
   - Tests single-line output (no wrapping)
   - Tests width doesn't exceed terminal width
   - Golden files: ANSI and Stripped variants

2. **`TestFocusFooterOverflow`** - 8 scenarios (4 widths × 2 verbose states)
   - Tests single-line output (no wrapping)
   - Tests width doesn't exceed terminal width
   - Golden files: ANSI and Stripped variants

3. **`TestFooterTextLengths`** - Documents actual character lengths
   - Dashboard: 50 chars
   - Focus (OFF): 88 chars
   - Focus (ON): 87 chars
   - Reports overflow risk at various widths

4. **`TestFooterTruncationBehavior`** - Tests current behavior at narrow widths
   - **Fails at 40, 60, and 80 cols for focus footer**
   - Documents need for truncation logic

### Test Statistics

- **Total test functions:** 4
- **Total test cases:** 16 (4 + 8 + 3 + 4)
- **Golden files generated:** 24 (12 ANSI + 12 Stripped)
- **Failures:** 9 test cases fail due to overflow

---

## Root Cause Analysis

### Why Footer Overflows

The footer rendering code uses `dimmedStyle.Render(text)` with no width constraint:

**Dashboard (line 150):**
```go
hint := dimmedStyle.Render("[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST")
```

**Focus (line 268):**
```go
hint := dimmedStyle.Render(fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM%s", verboseHint))
```

**Problem:** `lipgloss.Render()` does NOT truncate or wrap text to fit width. It renders the full string regardless of terminal width.

### Why This Wasn't Caught Earlier

1. **No width constraint tests** - Existing tests used `TestTerminalWidth = 120`, which fits all footers
2. **Golden tests at fixed width** - Never tested narrow terminal scenarios
3. **Manual testing at comfortable widths** - Developers likely use terminals > 120 cols
4. **Minimum width enforcement (80x20)** - Enforced but focus footer exceeds it

---

## Recommended Solutions

### Option 1: Truncate Footer Text (Quick Fix)

Add width constraint to footer rendering:

**Dashboard (line 150):**
```go
footerText := "[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST"
hint := dimmedStyle.Width(width).Render(truncateToWidth(footerText, width))
```

**Focus (line 268):**
```go
footerText := fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM%s", verboseHint)
hint := dimmedStyle.Width(width).Render(truncateToWidth(footerText, width))
```

**Pros:**
- Simple fix (2 lines changed per footer)
- Uses existing `truncateToWidth()` helper
- Prevents overflow at all widths

**Cons:**
- Footer may be unreadable at very narrow widths
- No intelligent shortening (e.g., abbreviations)

---

### Option 2: Progressive Abbreviation (Better UX)

Shorten footer text based on terminal width:

**Dashboard:**
```go
func renderDashboardFooter(width int) string {
    var footerText string
    switch {
    case width >= 120:
        footerText = "[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST"
    case width >= 80:
        footerText = "[ ? ] HELP  ·  [ n ] NEW  ·  [ b ] BROADCAST"
    case width >= 60:
        footerText = "[ ? ] HELP  ·  [ n ] NEW  ·  [ b ] BC"
    default:
        footerText = "[ ? ] HELP"
    }
    return dimmedStyle.Render(footerText)
}
```

**Focus:**
```go
func renderFocusFooter(width int, verbose bool) string {
    var footerText string
    verboseState := "OFF"
    if verbose {
        verboseState = "ON"
    }
    
    switch {
    case width >= 120:
        footerText = fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM  ·  [ v ] VERBOSE: %s", verboseState)
    case width >= 90:
        footerText = fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MSG  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM  ·  [ v ] VERBOSE: %s", verboseState)
    case width >= 80:
        footerText = fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MSG  ·  [ ↑↓ ]  ·  [ G ]  ·  [ v ] VERBOSE: %s", verboseState)
    case width >= 60:
        footerText = fmt.Sprintf("[ ESC ]  ·  [ m ]  ·  [ ↑↓ ]  ·  [ v ] VERBOSE: %s", verboseState)
    default:
        footerText = "[ ESC ] BACK"
    }
    return dimmedStyle.Render(footerText)
}
```

**Pros:**
- Always readable at all widths
- Intelligent abbreviations maintain usability
- Better UX than blind truncation

**Cons:**
- More complex code
- Requires design decisions on abbreviations
- More test cases needed

---

### Option 3: Increase Minimum Terminal Width (Not Recommended)

Raise minimum from 80x20 to 90x20.

**Pros:**
- No code changes to footer rendering

**Cons:**
- ❌ Breaks compatibility with 80-column terminals
- ❌ 80 cols is a standard (e.g., SSH, embedded systems)
- ❌ Doesn't solve dashboard overflow at <50 cols

---

## Recommended Action Plan

### Immediate (Required for RC)

1. **Fix focus footer overflow** (blocks RC release)
   - Focus footer MUST fit in 80 cols (current minimum)
   - Use Option 2 (progressive abbreviation) for focus view
   - Target: Focus footer ≤ 78 chars at 80 col width (2 char margin)

2. **Update tests to pass**
   - Adjust expectations in `TestFocusFooterOverflow` for abbreviated text
   - Regenerate golden files with `-update`

### Short-Term (Post-RC)

3. **Apply progressive abbreviation to dashboard footer** (optional)
   - Dashboard footer is safe at 80 cols but overflows at 40 cols
   - If supporting <80 col terminals is desired, use Option 2
   - Otherwise, enforce 50 col minimum or truncate

4. **Document minimum width requirements**
   - Update `AGENTS.md` and `README.md` to reflect footer width requirements
   - Note: Focus view requires ≥90 cols for full footer, ≥80 cols for abbreviated

### Long-Term (Future Enhancement)

5. **Consider dynamic help system**
   - Show footer hints only when space available
   - Prioritize most important hints at narrow widths
   - Use status bar for overflow hints

---

## Test Execution Summary

```bash
# Run footer overflow tests
go test ./internal/tui -run TestFooter -v

# Generate golden files (will fail until fix applied)
go test ./internal/tui -run TestDashboardFooterOverflow -update
go test ./internal/tui -run TestFocusFooterOverflow -update

# Check footer text lengths
go test ./internal/tui -run TestFooterTextLengths -v

# Test truncation behavior (expects failures)
go test ./internal/tui -run TestFooterTruncationBehavior -v
```

---

## Conclusion

**Status:** ✅ Overflow issues successfully identified and documented  
**Severity:** ❌ **CRITICAL** - Focus footer exceeds minimum terminal width (80 cols)  
**Recommendation:** Apply Option 2 (progressive abbreviation) to focus footer before RC release

The tests serve as both:
1. **Regression detection** - Will catch future footer overflow issues
2. **Documentation** - Golden files show expected truncation behavior at various widths
3. **Validation** - Confirms any fix actually resolves the overflow

---

**Test Created By:** OpenCode Agent  
**Test File:** `internal/tui/footer_overflow_test.go` (273 lines)  
**Documentation:** `documentation/FOOTER_OVERFLOW_TEST_REPORT.md`
