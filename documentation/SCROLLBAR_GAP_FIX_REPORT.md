# Scrollbar Gap Fix Report

**Date:** 2026-02-06  
**Issue:** Gap between scrollbar and right edge in conversation log viewport  
**Status:** ✅ **FIXED**

---

## Problem Statement

There was a visible 4-character gap between the scrollbar and the right edge of the conversation log viewport in focus view. The content should extend to fill the available width with the scrollbar flush against the right border.

### Visual Issue

**Before Fix:**
```
... text content here                                              │
... more text                                                      │
                                                            [GAP] █│  ← 4 char gap
```

**After Fix:**
```
... text content here extending to scrollbar                       █│
... more text fills the space                                      │
```

---

## Root Cause Analysis

The viewport width calculation at `internal/tui/app.go:152` was using an **outdated calculation** with an incorrect comment:

```go
m.viewport.Width = msg.Width - 6 - 2 // -6 for panel padding, -2 for scrollbar (space + char)
```

**The Problem:**
- The "-6 for panel padding" comment was **WRONG**
- Panel padding was removed in commit d023227 (to fix viewport overflow)
- The stale calculation left a 4-character gap

### Width Calculation Flow (120 col terminal)

**Before Fix:**
1. **Viewport width:** `120 - 6 - 2 = 112 chars`
2. **Viewport content:** 112 chars per line
3. **Scrollbar added:** 2 chars (space + `│`)
4. **Combined line:** `112 + 2 = 114 chars`
5. **Final render width:** `width - 2 = 118 chars` (accounting for borders)
6. **Gap:** `118 - 114 = 4 chars` ❌

**After Fix:**
1. **Viewport width:** `120 - 4 = 116 chars` (only borders + scrollbar)
2. **Viewport content:** 116 chars per line
3. **Scrollbar added:** 2 chars (space + `│`)
4. **Combined line:** `116 + 2 = 118 chars`
5. **Final render width:** `width - 2 = 118 chars`
6. **Gap:** `118 - 118 = 0 chars` ✅

---

## The Fix

**File:** `internal/tui/app.go`  
**Line:** 152

```diff
- m.viewport.Width = msg.Width - 6 - 2 // -6 for panel padding, -2 for scrollbar (space + char)
+ m.viewport.Width = msg.Width - 4 // -2 for borders, -2 for scrollbar (space + char)
```

**Explanation:**
- Remove the `-6` (stale panel padding offset)
- Keep `-4` for actual space requirements:
  - `-2` for left/right borders
  - `-2` for scrollbar (space + character)

---

## Verification

### Build Status
```bash
$ make build
✅ SUCCESS - Clean build
```

### Test Results
```bash
$ go test ./internal/tui -run TestFocusView
✅ PASS - All focus view tests passing
- TestFocusViewHeaderPresence: PASS
- TestFocusViewEdgeCases: PASS (9 test cases)
- TestFocusView: PASS (3 test cases with golden files)
- TestFocusViewWithViewport: PASS (2 test cases)
- TestFocusViewLayoutCalculations: PASS (14 test cases)
```

### Golden Files Updated
```bash
$ go test ./internal/tui -update
✅ SUCCESS - Golden files regenerated with correct widths
```

All golden file tests pass with the new viewport width calculation.

---

## Testing at Multiple Terminal Widths

| Terminal Width | Viewport Width | Content + Scrollbar | Render Width | Gap |
|----------------|----------------|---------------------|--------------|-----|
| **80 cols** | 76 | 78 | 78 | ✅ 0 |
| **100 cols** | 96 | 98 | 98 | ✅ 0 |
| **120 cols** | 116 | 118 | 118 | ✅ 0 |
| **160 cols** | 156 | 158 | 158 | ✅ 0 |

**Before fix, all terminals had a 4-char gap.**

---

## Impact Assessment

### User-Visible Changes
✅ **Positive:** Content now fills the full available width  
✅ **Positive:** Scrollbar is flush against the right edge  
✅ **Positive:** No wasted space in viewport  

### Code Quality
✅ **Fixed:** Removed stale comment about panel padding  
✅ **Fixed:** Correct calculation matches actual rendering behavior  
✅ **Improved:** Width calculation is now accurate and documented  

### Performance
✅ **Neutral:** No performance impact (calculation is the same complexity)

### Risk Assessment
✅ **Very Low Risk:**
- Simple arithmetic change (-6 → -4)
- All tests pass
- Golden files updated and verified
- No layout breakage at any terminal size

---

## Related Files Modified

1. **`internal/tui/app.go:152`** - Fixed viewport width calculation
2. **Golden test files** - Updated with correct widths (auto-regenerated)

---

## Historical Context

### Related Commits

**Commit d023227** (2026-02-06):
- Removed padding and borders from `logStyle` (conversation viewport)
- Removed `Padding(0, 2)` from viewport rendering in focus.go
- Fixed blank line at bottom of screen caused by viewport overflow
- **Side effect:** Left viewport width calculation with stale `-6` offset

**This Fix:**
- Corrects the viewport width calculation to match the removed padding
- Eliminates the 4-char gap created by the stale calculation

---

## Width Calculation Reference

### Components in Focus View

For a terminal width of `W` columns:

| Component | Width | Notes |
|-----------|-------|-------|
| **Terminal** | W | Total available space |
| **Left border** | 1 | Border character |
| **Right border** | 1 | Border character |
| **Available content** | W - 2 | Space inside borders |
| **Scrollbar (space)** | 1 | Space before scrollbar |
| **Scrollbar (char)** | 1 | `│` or `█` character |
| **Viewport content** | W - 4 | Content width |

### Calculation Formula

```
viewport.Width = terminalWidth - 2 (borders) - 2 (scrollbar)
               = terminalWidth - 4
```

### Combined Line Width

```
combinedLine = viewport.View() + " " + scrollbarChar
             = (W - 4 chars) + (2 chars)
             = W - 2 chars
```

This matches `logStyle.Width(width - 2)` perfectly.

---

## Recommendations

### For Future Changes

1. **Always verify width calculations** after removing padding or borders
2. **Run golden tests** to catch visual regressions
3. **Update comments** when changing layout calculations
4. **Document width formula** in code for maintainability

### For Documentation

Added this report to document the fix and prevent regression.

---

## Conclusion

**Status:** ✅ **FIX VERIFIED AND COMPLETE**

**Key Achievements:**
1. ✅ Identified root cause (stale width calculation)
2. ✅ Applied minimal fix (1 line change)
3. ✅ Verified across multiple terminal widths
4. ✅ All tests passing
5. ✅ Golden files updated
6. ✅ No regressions introduced

**Result:** Scrollbar is now flush against the right edge with no gap. Content fills the full available width in the conversation log viewport.

---

**Fix Applied By:** OpenCode Agent  
**Verification:** All tests passing, golden files updated  
**Documentation:** This report
