# Status Bar Overflow Test Report

**Date:** 2026-02-06  
**Test File:** `internal/tui/statusbar_overflow_test.go`  
**Status:** ✅ Tests Created, ❌ Overflow Issues Identified

---

## Executive Summary

Created comprehensive overflow tests for the status bar at the bottom of the screen. Tests **IDENTIFIED CRITICAL OVERFLOW ISSUES** that cause multi-line rendering and extra blank lines.

### Key Findings

| Terminal Width | Rendered Width | Overflow | Lines | Issue |
|----------------|----------------|----------|-------|-------|
| **120 cols** | 120 chars | ✅ None | 1 | SAFE |
| **80 cols** | 80 chars | ✅ None | 1 | SAFE (minimum enforced) |
| **60 cols** | >60 chars | ❌ Yes | **2** | **WRAPPING** |
| **50 cols** | 51 chars | ❌ +1 | 1-2 | **OVERFLOW** |
| **40 cols** | 46 chars | ❌ +6 | **2** | **WRAPPING** |

**ROOT CAUSE IDENTIFIED:** When status bar content exceeds terminal width, `lipgloss.Style.Width()` wraps the content to multiple lines, creating the "blank line at bottom" issue.

---

## Critical Issues Found

### Issue 1: Multi-Line Wrapping at Narrow Widths

**Terminals Affected:** < 80 columns (below minimum enforced width)

**Symptom:** Status bar renders on 2 lines instead of 1 when terminal is narrow.

**Example at 40 cols:**
```
⬦ IDLE ▐░░░░░░░░░░░░▌ |  T100 ⬡ [14:30]       
| ⬡ 1                                         
```

**Example at 60 cols:**
```
⬦ IDLE ▐░░░░░░░░░░░░▌ |  T42337 ⬡ [14:30]       | ⬡ 1  ◦ 1  
◌ 1                                                         
```

**Impact:**
- Extra line at bottom of screen
- Layout corruption
- Poor UX for users with narrow terminals

### Issue 2: Lipgloss Width Constraint Behavior

**Location:** `internal/tui/app.go:368-371`

**Code:**
```go
barStyle := lipgloss.NewStyle().
    Background(colorBorder).
    Foreground(colorPrimary).
    Width(m.width)

bar := leftSegment + separators + strings.Repeat(" ", leftPad) + 
       middleSegment + strings.Repeat(" ", rightPad) + 
       separators + rightSegment

return barStyle.Render(bar)
```

**Problem:** `lipgloss.Style.Width()` sets a MAXIMUM width. When content exceeds this width, lipgloss **wraps** the content to multiple lines rather than truncating.

**Why This Happens:**
1. Status bar components are concatenated into a single string
2. If total content > terminal width, the concatenated string is too long
3. `barStyle.Width(m.width)` wraps overflow to next line
4. Result: 2-line status bar instead of 1-line

---

## Test Results Summary

### TestStatusBarOverflow (11 scenarios)

| Test Case | Width | Tick | Myses | Result | Notes |
|-----------|-------|------|-------|--------|-------|
| normal_width_120 | 120 | 42337 | 3 | ✅ PASS | All components fit |
| minimum_width_80 | 80 | 42337 | 3 | ✅ PASS | Fits at enforced minimum |
| narrow_width_60 | 60 | 42337 | 3 | ❌ **FAIL** | **Wraps to 2 lines** |
| very_narrow_40 | 40 | 100 | 1 | ❌ **FAIL** | **Overflow +6 chars, 2 lines** |
| max_myses_full_swarm | 120 | 42337 | 16 | ✅ PASS | Full swarm fits |
| max_myses_narrow | 80 | 42337 | 16 | ✅ PASS | Fits at minimum |
| very_long_tick | 120 | 999999999 | 3 | ✅ PASS | Long tick fits |
| very_long_tick_narrow | 80 | 999999999 | 3 | ✅ PASS | Fits (barely) |
| extreme_tick_number | 120 | 9999999999999 | 1 | ✅ PASS | Extreme tick fits |
| all_states_wide | 160 | 42337 | 16 | ✅ PASS | Wide terminal |
| idle_with_long_label | 80 | 42337 | 0 | ✅ PASS | "(no myses)" fits |

**Pass Rate:** 9/11 (82%)  
**Fail Rate:** 2/11 (18%) - both below enforced minimum width

### TestStatusBarComponentWidths (3 scenarios)

**All PASS** - Component width calculations are correct:

| Width | Left | Middle | Right | Separators | Content | Padding Available |
|-------|------|--------|-------|------------|---------|-------------------|
| 80 | 22 | 16 | 13 | 6 | **57** | **23** ✅ |
| 120 | 22 | 16 | 13 | 6 | **57** | **63** ✅ |
| 160 | 22 | 20 | 18 | 6 | **66** | **94** ✅ |

**Conclusion:** At the enforced minimum width (80 cols), content only uses 57 chars, leaving 23 chars for padding. **This is SAFE.**

### TestStatusBarEdgeCases (5 scenarios)

| Test Case | Width | Result | Notes |
|-----------|-------|--------|-------|
| minimum_viable_width | 50 | ❌ **FAIL** | **Overflow +1 char** |
| zero_myses | 80 | ✅ PASS | "(no myses)" fits |
| single_digit_tick | 80 | ✅ PASS | T5 fits |
| max_int64_tick | 160 | ✅ PASS | T9223372036854775807 fits |
| wide_terminal | 200 | ✅ PASS | Plenty of space |

**Pass Rate:** 4/5 (80%)

### TestStatusBarPaddingCalculation (4 scenarios)

**All PASS** - Padding calculation logic is correct:
- Guards prevent negative padding
- Centering algorithm works
- Overflow detection works

---

## Root Cause Analysis

### Why Status Bar Wraps

1. **No Content Truncation:** The status bar concatenates all components without checking total length
2. **Lipgloss Wrapping Behavior:** `lipgloss.Style.Width()` wraps overflow content instead of truncating
3. **Missing Width Constraint:** Components are not individually constrained to fit within terminal width

### Where The Problem Occurs

**File:** `internal/tui/app.go`  
**Function:** `renderStatusBar()` (lines 329-382)

**Specific Issue:**
```go
// Line 373-379: Components concatenated without width check
bar := leftSegment +
    separators +
    strings.Repeat(" ", leftPad) +
    middleSegment +
    strings.Repeat(" ", rightPad) +
    separators +
    rightSegment

// Line 381: Width constraint applied AFTER concatenation
return barStyle.Render(bar)
```

**Problem Flow:**
1. `leftSegment` might be 22 chars (net indicator)
2. `middleSegment` might be 16 chars (tick + timestamp)
3. `rightSegment` might be 13+ chars (state counts)
4. Total content = 51+ chars (with separators and minimal padding)
5. If terminal width = 40, `barStyle.Width(40)` wraps to 2 lines

### Why Minimum Width (80) Is Safe

At 80 cols:
- Content: 57 chars
- Padding: 23 chars
- Total: 80 chars ✅

The padding calculation has guards (`if leftPad < 1 { leftPad = 1 }`) that prevent negative padding, so content always fits at 80 cols.

---

## Impact on Blank Line Issue

### Hypothesis Confirmation

**CONFIRMED:** The status bar CAN cause multi-line rendering when terminal width < 80 cols (below enforced minimum).

**However:**
- Zoea Nova enforces 80x20 minimum (lines 243-252 in `app.go`)
- At 80 cols, status bar fits on 1 line ✅
- Users should not encounter this in normal operation

**Caveat:**
- If minimum width enforcement is bypassed or removed, status bar WILL wrap at < 80 cols
- If user forces terminal to < 80 cols (bypassing checks), status bar wraps

### Connection to Original Issue

The blank line issue is likely NOT caused by status bar overflow at supported widths (≥80 cols). However:

1. **Width enforcement is CRITICAL** - Removing it would expose this issue
2. **Other components may have similar issues** - Footer, input prompts, etc.
3. **Testing methodology is valuable** - Can be applied to other components

---

## Recommended Solutions

### Option 1: Truncate Components (Quick Fix)

Add width constraints to each component BEFORE concatenation:

**Status bar rendering (lines 373-381):**
```go
// Truncate components if needed
maxLeftWidth := m.width / 3
maxMiddleWidth := m.width / 3
maxRightWidth := m.width / 3

if lipgloss.Width(leftSegment) > maxLeftWidth {
    leftSegment = truncateToWidth(leftSegment, maxLeftWidth)
}
// ... similar for middle and right

bar := leftSegment + separators + ... + rightSegment
return barStyle.Render(bar)
```

**Pros:**
- Simple fix
- Prevents overflow at all widths
- Maintains single-line rendering

**Cons:**
- May truncate important information
- Arbitrary width allocation

---

### Option 2: Progressive Degradation (Better UX)

Simplify components at narrow widths:

**Net indicator:**
- ≥80 cols: Full bar ` ⬥ LLM  ▐░░███░░░░░░░▌` (22 chars)
- <80 cols: Compact `⬥ LLM` (5 chars)

**Tick timestamp:**
- ≥80 cols: Full `T42337 ⬡ [14:30]` (16 chars)
- <80 cols: Tick only `T42337` (6 chars)

**State counts:**
- ≥80 cols: All states `⬡ 3  ◦ 2  ◌ 1  ✖ 0` (18 chars)
- <80 cols: Total only `3 myses` (7 chars)

**Pros:**
- Always readable
- Intelligent abbreviations
- Better UX than blind truncation

**Cons:**
- More complex code
- More test cases

---

### Option 3: Use ViewCompact() (Already Exists!)

The `NetIndicator` already has `ViewCompact()` method (lines 136-160 in `netindicator.go`):

**Current:** ` ⬥ LLM  ▐░░███░░░░░░░▌` (22 chars)  
**Compact:** `⬥ LLM` (5 chars)

**Implementation:**
```go
func (m Model) renderStatusBar() string {
    // Use compact mode for narrow terminals
    var leftSegment string
    if m.width < 80 {
        leftSegment = m.netIndicator.ViewCompact()
    } else {
        leftSegment = m.netIndicator.View()
    }
    
    // ... rest of rendering
}
```

**Pros:**
- Already implemented!
- Minimal code change
- Maintains aesthetics

**Cons:**
- Only helps with net indicator
- Still need to handle other components

---

## Recommended Action Plan

### Immediate (Not Required for RC)

Since the minimum width (80 cols) is enforced and status bar fits correctly at 80+ cols, **no immediate fix is required for RC release**.

**Rationale:**
- Users cannot reach <80 col widths (enforced at app.go:243)
- Status bar tested safe at 80-200 cols ✅
- This is an edge case below enforced minimum

### Short-Term (Post-RC Enhancement)

1. **Use ViewCompact() for narrow terminals** (if minimum width is ever lowered)
   - Already implemented for net indicator
   - Easy to add width check

2. **Add truncation helpers for other components**
   - Truncate state counts if needed
   - Shorten tick timestamp at narrow widths

3. **Document minimum width requirement**
   - Update AGENTS.md to note status bar requires ≥57 chars content space
   - Note: 80 cols provides 23 chars padding margin

### Long-Term (Future Enhancement)

4. **Implement full progressive degradation**
   - Simplify all components at narrow widths
   - Define breakpoints (120, 100, 80, 60)
   - Test at each breakpoint

5. **Consider dynamic component visibility**
   - Hide less important components at narrow widths
   - Prioritize critical information (tick, state)

---

## Test Coverage Impact

### New Tests Created

- **TestStatusBarOverflow:** 11 test cases (22 golden files)
- **TestStatusBarComponentWidths:** 3 test cases (no golden files)
- **TestStatusBarEdgeCases:** 5 test cases (10 golden files)
- **TestStatusBarPaddingCalculation:** 4 test cases (no golden files)

**Total:** 23 test cases, 32 golden files

### Test Statistics

| Metric | Count |
|--------|-------|
| Test functions | 4 |
| Test cases | 23 |
| Golden files | 32 (16 ANSI + 16 Stripped) |
| Terminal widths tested | 40, 50, 60, 80, 120, 160, 200 |
| Tick values tested | 5, 100, 42337, 999999999, 9223372036854775807 |
| Mysis counts tested | 0, 1, 3, 16 |

### Coverage Increase

- **Before:** Status bar rendering untested
- **After:** Comprehensive overflow testing at 7 widths
- **Edge cases:** Zero myses, extreme tick values, full swarm

---

## Conclusion

**Status:** ✅ Tests successfully created and overflow issues identified

**Key Findings:**
1. ✅ Status bar is SAFE at enforced minimum width (80 cols)
2. ❌ Status bar WRAPS at <80 cols (below enforced minimum)
3. ✅ Component width calculations are correct
4. ✅ Padding algorithm works correctly

**Critical Insight:**
The status bar does NOT cause blank line issues at supported widths (≥80 cols). However, if minimum width enforcement were removed, it WOULD cause multi-line wrapping and extra blank lines.

**Recommendation:**
- **No fix required for RC** - Status bar works correctly at supported widths
- **Keep minimum width enforcement** - Critical for preventing overflow
- **Consider ViewCompact()** - Easy enhancement if minimum width is ever lowered
- **Apply testing methodology** - Use these tests as template for other components

---

**Test Created By:** OpenCode Agent  
**Test File:** `internal/tui/statusbar_overflow_test.go` (400+ lines)  
**Documentation:** This report
