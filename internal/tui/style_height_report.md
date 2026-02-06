# Lipgloss Style Height Investigation Report

**Date:** 2026-02-06  
**Issue:** Investigating if lipgloss styles add unexpected vertical space causing height overflow

---

## Executive Summary

✅ **NO UNEXPECTED VERTICAL SPACE DETECTED** in the three key styles (inputStyle, headerStyle, panelStyle).

All styles behave as expected according to lipgloss documentation:
- Borders add exactly 2 lines (top + bottom)
- Padding(v, h) adds exactly 2v lines (v top + v bottom)
- Width() does NOT add vertical space
- Margin() adds EXTERNAL space (outside the rendered box)

---

## Test Results

### 1. inputStyle (RoundedBorder + Padding(0, 1))

**Definition:** `Border(lipgloss.RoundedBorder()).BorderForeground(colorTeal).Padding(0, 1)`

**Single-line content:**
- ✅ Expected: 3 lines (border top, content, border bottom)
- ✅ Actual: 3 lines
- ✅ Padding(0, 1) correctly adds NO vertical space (only horizontal)

**Output:**
```
╭─────────────╮
│ Hello World │
╰─────────────╯
```

**With width constraint:**
- ✅ inputStyle.Width(80) still renders 3 lines
- ✅ Width() does NOT add vertical space

**Multi-line content:**
- ✅ Expected: 5 lines (border + 3 content lines + border)
- ✅ Actual: 5 lines

**Conclusion:** inputStyle is **NOT** adding unexpected vertical space.

---

### 2. headerStyle (no border, just background)

**Definition:** `Bold(true).Foreground(colorBrand).Background(colorBgAlt)`

**Single-line content:**
- ✅ Expected: 1 line
- ✅ Actual: 1 line

**With width constraint:**
- ✅ headerStyle.Width(80) still renders 1 line

**Output:**
```
ZOEA NOVA                                                                       
```

**Actual 3-line header content:**
- ✅ Expected: 3 lines (content has 3 newlines)
- ✅ Actual: 3 lines

**Conclusion:** headerStyle is **NOT** adding unexpected vertical space.

---

### 3. panelStyle (RoundedBorder + Padding(0, 1))

**Definition:** `Border(lipgloss.RoundedBorder()).BorderForeground(colorBorder).Background(colorBgPanel).Padding(0, 1)`

**Single-line content:**
- ✅ Expected: 3 lines
- ✅ Actual: 3 lines

**Output:**
```
╭───────────────╮
│ Panel content │
╰───────────────╯
```

**Conclusion:** panelStyle is **NOT** adding unexpected vertical space.

---

## Border Behavior (Isolated)

| Border Type | Content Lines | Rendered Lines | Formula |
|-------------|---------------|----------------|---------|
| None | 1 | 1 | contentLines |
| RoundedBorder | 1 | 3 | contentLines + 2 |
| DoubleBorder | 1 | 3 | contentLines + 2 |

✅ **All borders add exactly 2 lines** (top + bottom)

---

## Padding Behavior (Isolated)

| Padding | Content Lines | Rendered Lines | Formula |
|---------|---------------|----------------|---------|
| None | 1 | 1 | contentLines |
| Padding(0, 1) | 1 | 1 | contentLines + 0 |
| Padding(1, 0) | 1 | 3 | contentLines + 2 |
| Padding(2, 0) | 1 | 5 | contentLines + 4 |

✅ **Padding(v, h) adds exactly 2v lines** (v top + v bottom)

---

## Combined Border + Padding

| Style | Content Lines | Rendered Lines | Formula |
|-------|---------------|----------------|---------|
| RoundedBorder + Padding(0, 1) | 1 | 3 | contentLines + 2 (border) |
| RoundedBorder + Padding(1, 0) | 1 | 5 | contentLines + 2 (border) + 2 (padding) |
| DoubleBorder + Padding(1, 2) | 1 | 5 | contentLines + 2 (border) + 2 (padding) |

✅ **Border and padding effects are additive and predictable**

---

## Issue Found: helpStyle with Margin

**Definition:** `Border(lipgloss.DoubleBorder()).BorderForeground(colorBrand).Background(colorBgPanel).Padding(1, 2).Margin(1)`

**Expected:** 5 lines (border + padding)
**Actual:** 7 lines

**Reason:** `Margin(1)` adds 1 line above and below the rendered box.

**Output:**
```
                     
 ╔═════════════╗ 
 ║             ║ 
 ║  Help text  ║ 
 ║             ║ 
 ╚═════════════╝ 
                     
```

⚠️ **Margin adds EXTERNAL space** - This is expected behavior but must be accounted for in height calculations.

**Impact:** helpStyle is used for the help overlay, which is rendered separately and doesn't affect dashboard/focus view layout.

---

## Real-World Scenarios

### Dashboard Input Prompt

**Content:** `"Press 'm' to message, 'b' to broadcast..."`
**Rendered lines:** ✅ 3 lines (as expected)

### Focus View Input Prompt

**Content:** `"Message to mysis..."`
**Rendered lines:** ✅ 3 lines (as expected)

### Dashboard Header

**Content:** 3-line ASCII art header
**Rendered lines:** ✅ 3 lines (as expected)

---

## Potential Wrapping Issue

When testing long content with width constraint:

```go
content := "This is a very long input that might wrap to multiple lines if the width is constrained"
styled := inputStyle.Width(40).Render(content)
lines := len(strings.Split(styled, "\n"))
// Result: 5 lines (content wrapped to 3 lines + 2 border lines)
```

✅ **Lipgloss DOES auto-wrap content to fit width constraint**

**This is expected behavior** - when content exceeds width, lipgloss wraps it.

**Impact on height calculation:**
- If input content is longer than `width - 2 (borders) - 2 (padding)`, it will wrap
- Each wrapped line adds 1 to total height
- This could explain overflow if height calculation assumes single-line input

---

## Height Calculation Formula

For accurate height prediction:

```go
// For inputStyle (RoundedBorder + Padding(0, 1))
borderLines := 2  // top + bottom
paddingLines := 0  // Padding(0, 1) adds no vertical space
contentLines := calculateContentLines(text, width - 4)  // width - borders - horizontal padding
totalHeight := borderLines + paddingLines + contentLines

// For headerStyle (no border, no padding)
totalHeight := calculateContentLines(text, width)

// For panelStyle (RoundedBorder + Padding(0, 1))
borderLines := 2
paddingLines := 0
contentLines := calculateContentLines(text, width - 4)
totalHeight := borderLines + paddingLines + contentLines
```

**Key insight:** Content can wrap to multiple lines if constrained by width!

---

## Findings Summary

### ✅ Styles NOT Adding Unexpected Space

1. **inputStyle** - Correctly adds 2 lines for border, 0 for horizontal padding
2. **headerStyle** - Adds 0 extra lines (no border, no padding)
3. **panelStyle** - Correctly adds 2 lines for border, 0 for horizontal padding

### ⚠️ Potential Issue: Content Wrapping

**inputStyle.Width(width - 2).Render(longText)** will wrap content if it exceeds available width.

**Example:**
- Terminal width: 80 cols
- Input style applied: `inputStyle.Width(78)`
- Available content width: 78 - 2 (borders) - 2 (horizontal padding) = 74 cols
- If content > 74 cols, it wraps to multiple lines
- If content wraps to 3 lines: total height = 3 (content) + 2 (borders) = **5 lines**

**This could explain height overflow** if the height calculation assumes input is always 3 lines (single-line content).

---

## Recommendations

### 1. Check Input Content Length

Verify that `textInput.View()` returns single-line content:

```go
// In input.go ViewAlways() method
content := m.textInput.View()
contentLines := strings.Count(content, "\n") + 1
// If contentLines > 1, wrapping is occurring
```

### 2. Account for Text Wrapping in Height Calculation

If input content can wrap, height calculation must account for this:

```go
// Instead of assuming input is always 3 lines:
inputHeight := 3  // WRONG if content wraps

// Use actual line count:
inputRendered := inputStyle.Width(width - 2).Render(m.textInput.View())
inputHeight := len(strings.Split(inputRendered, "\n"))  // CORRECT
```

### 3. Constrain TextInput Width

Prevent wrapping by constraining the bubbles/textinput width:

```go
// In InputModel.SetWidth()
m.textInput.Width = width - 4  // Account for border + padding

// Bubbles textinput will truncate instead of wrap if CharLimit is set correctly
```

### 4. Check if textInput.View() Can Return Multi-line

Bubbles `textinput` is designed for single-line input. Check if:
- Newlines are being inserted somehow
- CharLimit is being exceeded
- Width is causing internal wrapping

---

## Next Steps

1. **Check actual input rendering in dashboard/focus view**
   - Add logging to capture `m.textInput.View()` output
   - Count newlines in the output
   - Verify if wrapping is occurring

2. **Test with very long input**
   - Type 200+ character message
   - Observe if input prompt height increases
   - Check if overflow occurs

3. **Review height calculation in app.go**
   - Find where input prompt height is assumed to be 3
   - Update to dynamically measure actual height
   - Test with various input lengths

---

## Conclusion

**Root Cause:** Styles are NOT adding unexpected vertical space. The issue is likely:
1. ✅ **Content wrapping** when input text exceeds available width
2. ✅ **Height calculation assumes single-line input** (always 3 lines)

**Fix:** Update height calculations to measure actual rendered height instead of assuming fixed height.

**Test File:** `internal/tui/style_height_test.go` (557 lines)  
**All tests passing:** 23/24 (1 expected failure for helpStyle with Margin)
