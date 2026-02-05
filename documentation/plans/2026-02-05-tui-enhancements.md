# TUI Enhancements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement all medium-priority TUI enhancements from KNOWN_ISSUES.md to improve user experience with reasoning display, account status visibility, JSON tree rendering, and scrollbar indicators.

**Architecture:** Extend existing TUI rendering logic in focus and dashboard views without breaking current styling. Add helper functions for JSON tree rendering and scrollbar calculations. Expose account information from core layer to TUI models.

**Tech Stack:** Go 1.22+, Bubble Tea TUI framework, lipgloss styling, Unicode box-drawing characters

---

## Task 1: Display Reasoning in Focus View

**Files:**
- Modify: `internal/tui/focus.go:172-249` (renderLogEntry function)
- Modify: `internal/tui/focus.go:252-265` (LogEntryFromMemory function)
- Modify: `internal/tui/focus.go:14-20` (LogEntry struct)
- Test: `internal/tui/focus_test.go`

**Context:** Reasoning content is stored in `store.Memory.Reasoning` but not displayed in TUI. We need to render it using the existing purple/magenta color scheme for assistant messages.

---

### Step 1: Update LogEntry struct to include Reasoning field

**Action:** Add Reasoning field to LogEntry struct

```go
// LogEntry represents a log entry for display.
type LogEntry struct {
	Role      string
	Source    string
	SenderID  string
	Content   string
	Reasoning string  // NEW: reasoning content from LLM
	Timestamp time.Time
}
```

Run: `make build`
Expected: Compile success

---

### Step 2: Update LogEntryFromMemory to populate Reasoning

**Action:** Modify LogEntryFromMemory to include reasoning field

```go
// LogEntryFromMemory converts a store.Memory to LogEntry.
func LogEntryFromMemory(m *store.Memory, currentMysisID string) LogEntry {
	source := string(m.Source)
	if m.Source == store.MemorySourceBroadcast && m.SenderID == currentMysisID {
		source = "broadcast_self"
	}
	return LogEntry{
		Role:      string(m.Role),
		Source:    source,
		SenderID:  m.SenderID,
		Content:   m.Content,
		Reasoning: m.Reasoning,  // NEW: copy reasoning field
		Timestamp: m.CreatedAt,
	}
}
```

Run: `make build`
Expected: Compile success

---

### Step 3: Write test for reasoning display

**Action:** Add test case for rendering log entries with reasoning

```go
func TestRenderLogEntryWithReasoning(t *testing.T) {
	// Force color output for testing
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	entry := LogEntry{
		Role:      "assistant",
		Source:    "llm",
		Content:   "I will mine the asteroid.",
		Reasoning: "The asteroid contains valuable ore. I have enough fuel and cargo space.",
		Timestamp: time.Now(),
	}

	maxWidth := 80
	lines := renderLogEntry(entry, maxWidth)

	// Should have content lines + reasoning section
	if len(lines) < 4 {
		t.Errorf("Expected at least 4 lines (padding + content + reasoning header + reasoning), got %d", len(lines))
	}

	// Join lines and check for reasoning content
	output := strings.Join(lines, "\n")
	if !strings.Contains(output, "REASONING:") {
		t.Error("Expected reasoning header 'REASONING:' in output")
	}
	if !strings.Contains(output, "The asteroid contains valuable ore") {
		t.Error("Expected reasoning content in output")
	}

	// Verify ANSI codes are present (styling is applied)
	if !strings.Contains(output, "\x1b[") {
		t.Error("Expected ANSI escape codes (styling) in output")
	}
}
```

Run: `go test ./internal/tui -run TestRenderLogEntryWithReasoning -v`
Expected: FAIL (reasoning not yet rendered)

---

### Step 4: Modify renderLogEntry to display reasoning

**Action:** Add reasoning rendering after content in renderLogEntry function

Find this section in `renderLogEntry` (around line 247):
```go
	}

	return result
}
```

Replace with:
```go
	}

	// Render reasoning if present
	if entry.Reasoning != "" {
		// Add spacing line
		emptyLine := contentStyle.Width(maxWidth).Render("")
		result = append(result, emptyLine)

		// Reasoning header in dimmed purple/magenta
		reasoningHeaderStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).  // Dimmed purple
			Background(logBgColor)

		reasoningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("213")).  // Lighter purple for reasoning text
			Background(logBgColor)

		reasoningHeader := "REASONING:"
		reasoningHeaderWidth := len(reasoningHeader) + 1 // +1 for space after
		reasoningContentWidth := maxWidth - reasoningHeaderWidth - padLeft - padRight
		if reasoningContentWidth < 20 {
			reasoningContentWidth = 20
		}

		// Wrap reasoning text
		wrappedReasoning := wrapText(entry.Reasoning, reasoningContentWidth)

		for i, line := range wrappedReasoning {
			lineLen := lipgloss.Width(line)
			remainingWidth := reasoningContentWidth - lineLen
			if remainingWidth < 0 {
				remainingWidth = 0
			}
			paddedLine := line + strings.Repeat(" ", remainingWidth+padRight)

			if i == 0 {
				// First line: left pad + styled header + space + content
				leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
				styledHeader := reasoningHeaderStyle.Render(reasoningHeader)
				styledContent := reasoningStyle.Render(" " + paddedLine)
				result = append(result, leftPad+styledHeader+styledContent)
			} else {
				// Continuation lines: left pad + indent + content
				leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
				indent := strings.Repeat(" ", reasoningHeaderWidth)
				styledIndent := contentStyle.Render(indent)
				styledContent := reasoningStyle.Render(paddedLine)
				result = append(result, leftPad+styledIndent+styledContent)
			}
		}
	}

	return result
}
```

Run: `make build`
Expected: Compile success

---

### Step 5: Run test to verify reasoning display works

**Action:** Run the test

Run: `go test ./internal/tui -run TestRenderLogEntryWithReasoning -v`
Expected: PASS

---

### Step 6: Test with full TUI test suite

**Action:** Run all TUI tests to ensure no regressions

Run: `make test`
Expected: All tests pass

---

### Step 7: Commit reasoning display feature

**Action:** Commit the changes

```bash
git add internal/tui/focus.go internal/tui/focus_test.go
git commit -m "feat(tui): display reasoning content in focus view

- Add Reasoning field to LogEntry struct
- Update LogEntryFromMemory to populate reasoning
- Render reasoning below content with purple styling
- Add test coverage for reasoning display"
```

---

## Task 2: Show Account Status in Views

**Files:**
- Modify: `internal/tui/dashboard.go:13-20` (MysisInfo struct)
- Modify: `internal/tui/dashboard.go:222-231` (MysisInfoFromCore function)
- Modify: `internal/tui/dashboard.go:160-220` (renderMysisLine function)
- Modify: `internal/tui/focus.go:63-124` (RenderFocusView function)
- Modify: `internal/tui/focus.go:126-167` (RenderFocusViewWithViewport function)
- Modify: `internal/core/mysis.go` (expose CurrentAccountUsername method)
- Test: `internal/tui/dashboard_test.go` (create if needed)

**Context:** Myses track which game account they're using via `currentAccountUsername` field, but this isn't exposed to the TUI layer. We need to surface this information in both focus view header and dashboard mysis list.

---

### Step 1: Add CurrentAccountUsername method to Mysis

**Action:** Expose the current account username from Mysis

In `internal/core/mysis.go`, find the getter methods section and add:

```go
// CurrentAccountUsername returns the username of the account this mysis is using.
func (m *Mysis) CurrentAccountUsername() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentAccountUsername
}
```

Run: `make build`
Expected: Compile success

---

### Step 2: Add AccountUsername field to MysisInfo

**Action:** Update MysisInfo struct in dashboard.go

```go
// MysisInfo holds display info for a mysis.
type MysisInfo struct {
	ID              string
	Name            string
	State           string
	Provider        string
	AccountUsername string    // NEW: game account username
	LastMessage     string    // Most recent message (user or assistant)
	CreatedAt       time.Time // When mysis was created
}
```

Run: `make build`
Expected: Compile success

---

### Step 3: Update MysisInfoFromCore to populate account username

**Action:** Modify MysisInfoFromCore function

```go
// MysisInfoFromCore converts a core.Mysis to MysisInfo.
func MysisInfoFromCore(m *core.Mysis) MysisInfo {
	return MysisInfo{
		ID:              m.ID(),
		Name:            m.Name(),
		State:           string(m.State()),
		Provider:        m.ProviderName(),
		AccountUsername: m.CurrentAccountUsername(),  // NEW: copy account username
		CreatedAt:       m.CreatedAt(),
	}
}
```

Run: `make build`
Expected: Compile success

---

### Step 4: Write test for account display in dashboard

**Action:** Create test for rendering mysis line with account info

Create `internal/tui/dashboard_test.go`:

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestRenderMysisLineWithAccount(t *testing.T) {
	// Force color output for testing
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	info := MysisInfo{
		ID:              "abc123",
		Name:            "test-mysis",
		State:           "running",
		Provider:        "ollama",
		AccountUsername: "crab_miner",
		LastMessage:     "Mining asteroid",
		CreatedAt:       time.Now(),
	}

	width := 100
	line := renderMysisLine(info, false, false, "⬡", width)

	// Should contain account username
	if !strings.Contains(line, "crab_miner") {
		t.Error("Expected account username 'crab_miner' in mysis line")
	}

	// Verify ANSI codes are present (styling is applied)
	if !strings.Contains(line, "\x1b[") {
		t.Error("Expected ANSI escape codes (styling) in output")
	}
}

func TestRenderMysisLineWithoutAccount(t *testing.T) {
	// Force color output
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	info := MysisInfo{
		ID:              "abc123",
		Name:            "test-mysis",
		State:           "idle",
		Provider:        "ollama",
		AccountUsername: "",  // No account claimed yet
		LastMessage:     "",
		CreatedAt:       time.Now(),
	}

	width := 100
	line := renderMysisLine(info, false, false, "⬡", width)

	// Should contain indicator for no account
	if !strings.Contains(line, "no account") {
		t.Error("Expected 'no account' indicator when AccountUsername is empty")
	}
}
```

Run: `go test ./internal/tui -run TestRenderMysisLine -v`
Expected: FAIL (account info not yet rendered)

---

### Step 5: Update renderMysisLine to show account username

**Action:** Modify renderMysisLine to include account info

Find this section in `renderMysisLine` (around line 188):
```go
	stateText := StateStyle(m.State).Render(fmt.Sprintf("%-8s", m.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", m.Provider))
```

Replace with:
```go
	stateText := StateStyle(m.State).Render(fmt.Sprintf("%-8s", m.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", m.Provider))
	
	// Account username display
	var accountText string
	if m.AccountUsername != "" {
		accountText = dimmedStyle.Render(fmt.Sprintf("@%s", m.AccountUsername))
	} else {
		accountText = dimmedStyle.Render("(no account)")
	}
```

Then update the firstPart line:
```go
	// First part: indicator + name + state + provider + account
	firstPart := fmt.Sprintf("%s %-16s %s %s %s", stateIndicator, name, stateText, provider, accountText)
```

And adjust the width calculation:
```go
	// Calculate remaining width for last message
	// Account for the prefix "│ " for the message
	// Use lipgloss.Width() for proper Unicode width calculation
	providerWidth := lipgloss.Width(m.Provider)
	accountTextWidth := lipgloss.Width(accountText)
	usedWidth := 2 + 16 + 1 + 8 + 1 + providerWidth + 2 + 1 + accountTextWidth + 4
	msgWidth := width - usedWidth - 8
	if msgWidth < 10 {
		msgWidth = 10
	}
```

Run: `make build`
Expected: Compile success

---

### Step 6: Run dashboard test to verify account display

**Action:** Run the test

Run: `go test ./internal/tui -run TestRenderMysisLine -v`
Expected: PASS

---

### Step 7: Update focus view header to show account username

**Action:** Modify RenderFocusViewWithViewport to include account in header

Find the info panel section (around line 140-146):
```go
	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("ID:"), valueStyle.Render(mysis.ID)),
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), stateDisplay),
		fmt.Sprintf("%s %s", labelStyle.Render("Provider:"), valueStyle.Render(mysis.Provider)),
	}
```

Replace with:
```go
	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("ID:"), valueStyle.Render(mysis.ID)),
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), stateDisplay),
		fmt.Sprintf("%s %s", labelStyle.Render("Provider:"), valueStyle.Render(mysis.Provider)),
	}
	
	// Add account info if available
	if mysis.AccountUsername != "" {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Account:"), valueStyle.Render(mysis.AccountUsername)))
	} else {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Account:"), dimmedStyle.Render("(not logged in)")))
	}
```

Do the same for RenderFocusView function (around line 76-80).

Run: `make build`
Expected: Compile success

---

### Step 8: Run all TUI tests

**Action:** Verify no regressions

Run: `make test`
Expected: All tests pass

---

### Step 9: Commit account status display feature

**Action:** Commit the changes

```bash
git add internal/core/mysis.go internal/tui/dashboard.go internal/tui/focus.go internal/tui/dashboard_test.go
git commit -m "feat(tui): display account status in dashboard and focus views

- Add CurrentAccountUsername() method to Mysis
- Add AccountUsername field to MysisInfo
- Show account username in dashboard mysis list
- Show account status in focus view header
- Add test coverage for account display"
```

---

## Task 3: Render JSON as Tree View

**Files:**
- Create: `internal/tui/json_tree.go`
- Create: `internal/tui/json_tree_test.go`
- Modify: `internal/tui/focus.go:172-249` (renderLogEntry function)
- Modify: `internal/tui/app.go:27-60` (Model struct - add verbose toggle)

**Context:** Tool results often contain large JSON payloads that are hard to read when rendered as plain text. We'll implement Unicode tree rendering with smart truncation (first 3 items, "[x more]", last 3 items) and a verbose toggle for full output.

---

### Step 1: Write test for JSON tree rendering

**Action:** Create test file with core rendering tests

Create `internal/tui/json_tree_test.go`:

```go
package tui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderJSONTree_SimpleObject(t *testing.T) {
	jsonStr := `{"name": "mysis-1", "state": "running", "id": "abc123"}`
	
	tree, err := renderJSONTree(jsonStr, false)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	// Should use tree structure
	if !strings.Contains(tree, "├─") && !strings.Contains(tree, "└─") {
		t.Error("Expected tree box characters in output")
	}

	// Should contain field names
	if !strings.Contains(tree, "name") || !strings.Contains(tree, "state") {
		t.Error("Expected field names in output")
	}
}

func TestRenderJSONTree_ArrayTruncation(t *testing.T) {
	items := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = map[string]interface{}{"id": i, "value": i * 100}
	}
	jsonBytes, _ := json.Marshal(items)
	
	tree, err := renderJSONTree(string(jsonBytes), false)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	// Should show truncation indicator
	if !strings.Contains(tree, "[4 more]") {
		t.Error("Expected '[4 more]' truncation indicator for 10-item array")
	}

	// Should show first 3 items (0, 1, 2)
	if !strings.Contains(tree, `"id": 0`) || !strings.Contains(tree, `"id": 1`) || !strings.Contains(tree, `"id": 2`) {
		t.Error("Expected first 3 items to be shown")
	}

	// Should show last 3 items (7, 8, 9)
	if !strings.Contains(tree, `"id": 7`) || !strings.Contains(tree, `"id": 8`) || !strings.Contains(tree, `"id": 9`) {
		t.Error("Expected last 3 items to be shown")
	}

	// Should NOT show middle items
	if strings.Contains(tree, `"id": 3`) || strings.Contains(tree, `"id": 4`) {
		t.Error("Middle items should be truncated")
	}
}

func TestRenderJSONTree_VerboseMode(t *testing.T) {
	items := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		items[i] = map[string]interface{}{"id": i}
	}
	jsonBytes, _ := json.Marshal(items)
	
	tree, err := renderJSONTree(string(jsonBytes), true)
	if err != nil {
		t.Fatalf("Failed to render JSON tree: %v", err)
	}

	// Should NOT truncate in verbose mode
	if strings.Contains(tree, "more") {
		t.Error("Should not show truncation in verbose mode")
	}

	// Should show all items
	for i := 0; i < 10; i++ {
		expected := strings.Replace(`"id": X`, "X", string(rune('0'+i)), 1)
		if !strings.Contains(tree, expected) {
			t.Errorf("Expected item %d to be shown in verbose mode", i)
		}
	}
}

func TestRenderJSONTree_InvalidJSON(t *testing.T) {
	jsonStr := `{invalid json`
	
	_, err := renderJSONTree(jsonStr, false)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}
```

Run: `go test ./internal/tui -run TestRenderJSONTree -v`
Expected: FAIL (renderJSONTree not yet implemented)

---

### Step 2: Implement JSON tree rendering function

**Action:** Create json_tree.go with tree rendering logic

Create `internal/tui/json_tree.go`:

```go
package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	// Tree box-drawing characters
	treeEdge   = "├─"
	treeLast   = "└─"
	treeVert   = "│ "
	treeSpace  = "  "
	
	// Truncation limits
	arrayTruncateThreshold = 6  // Show first 3 and last 3 if array has more than this
	arrayShowFirst         = 3
	arrayShowLast          = 3
)

// renderJSONTree renders JSON as a Unicode tree structure with smart truncation.
// If verbose is true, all items are shown. Otherwise, arrays with more than 6 items
// show first 3, "[x more]", and last 3.
func renderJSONTree(jsonStr string, verbose bool) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	var lines []string
	renderValue(data, "", true, verbose, &lines)
	return strings.Join(lines, "\n"), nil
}

// renderValue recursively renders a JSON value as tree lines
func renderValue(value interface{}, prefix string, isLast bool, verbose bool, lines *[]string) {
	switch v := value.(type) {
	case map[string]interface{}:
		renderObject(v, prefix, isLast, verbose, lines)
	case []interface{}:
		renderArray(v, prefix, isLast, verbose, lines)
	default:
		// Primitive value
		*lines = append(*lines, prefix+fmt.Sprintf("%v", v))
	}
}

// renderObject renders a JSON object
func renderObject(obj map[string]interface{}, prefix string, isLast bool, verbose bool, lines *[]string) {
	if len(obj) == 0 {
		*lines = append(*lines, prefix+"{}")
		return
	}

	*lines = append(*lines, prefix+"{")
	
	// Get keys in deterministic order (sorted)
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	// Note: Not sorting to preserve JSON order, but could add sort.Strings(keys) here
	
	for i, key := range keys {
		isLastField := i == len(keys)-1
		fieldPrefix := prefix
		if isLast {
			fieldPrefix += treeSpace
		} else {
			fieldPrefix += treeVert
		}
		
		var connector string
		if isLastField {
			connector = treeLast
		} else {
			connector = treeEdge
		}
		
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))  // Orange for keys
		valuePrefix := fieldPrefix + connector + keyStyle.Render(fmt.Sprintf("%q: ", key))
		
		val := obj[key]
		switch v := val.(type) {
		case map[string]interface{}:
			*lines = append(*lines, valuePrefix)
			renderObject(v, fieldPrefix+treeSpace, isLastField, verbose, lines)
		case []interface{}:
			*lines = append(*lines, valuePrefix)
			renderArray(v, fieldPrefix+treeSpace, isLastField, verbose, lines)
		default:
			*lines = append(*lines, valuePrefix+fmt.Sprintf("%v", v))
		}
	}
	
	closingPrefix := prefix
	if !isLast {
		closingPrefix += treeVert
	} else {
		closingPrefix += treeSpace
	}
	*lines = append(*lines, closingPrefix+"}")
}

// renderArray renders a JSON array with smart truncation
func renderArray(arr []interface{}, prefix string, isLast bool, verbose bool, lines *[]string) {
	if len(arr) == 0 {
		*lines = append(*lines, prefix+"[]")
		return
	}

	*lines = append(*lines, prefix+"[")
	
	shouldTruncate := !verbose && len(arr) > arrayTruncateThreshold
	
	itemsToShow := len(arr)
	if shouldTruncate {
		itemsToShow = arrayShowFirst + arrayShowLast
	}
	
	showItems := make([]int, 0, itemsToShow)
	if shouldTruncate {
		// First 3 items
		for i := 0; i < arrayShowFirst; i++ {
			showItems = append(showItems, i)
		}
		// Last 3 items
		for i := len(arr) - arrayShowLast; i < len(arr); i++ {
			showItems = append(showItems, i)
		}
	} else {
		// Show all items
		for i := 0; i < len(arr); i++ {
			showItems = append(showItems, i)
		}
	}
	
	truncatedCount := len(arr) - len(showItems)
	
	for idx, i := range showItems {
		isLastItem := idx == len(showItems)-1
		
		// Insert truncation indicator between first and last items
		if shouldTruncate && idx == arrayShowFirst {
			truncPrefix := prefix
			if isLast {
				truncPrefix += treeSpace
			} else {
				truncPrefix += treeVert
			}
			dimmedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			*lines = append(*lines, truncPrefix+treeEdge+dimmedStyle.Render(fmt.Sprintf("[%d more]", truncatedCount)))
		}
		
		itemPrefix := prefix
		if isLast {
			itemPrefix += treeSpace
		} else {
			itemPrefix += treeVert
		}
		
		var connector string
		if isLastItem && !shouldTruncate {
			connector = treeLast
		} else {
			connector = treeEdge
		}
		
		indexStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))  // Dim gray for indices
		itemLine := itemPrefix + connector + indexStyle.Render(fmt.Sprintf("[%d] ", i))
		
		val := arr[i]
		switch v := val.(type) {
		case map[string]interface{}:
			*lines = append(*lines, itemLine)
			renderObject(v, itemPrefix+treeSpace, isLastItem, verbose, lines)
		case []interface{}:
			*lines = append(*lines, itemLine)
			renderArray(v, itemPrefix+treeSpace, isLastItem, verbose, lines)
		default:
			*lines = append(*lines, itemLine+fmt.Sprintf("%v", v))
		}
	}
	
	closingPrefix := prefix
	if !isLast {
		closingPrefix += treeVert
	} else {
		closingPrefix += treeSpace
	}
	*lines = append(*lines, closingPrefix+"]")
}
```

Run: `make build`
Expected: Compile success

---

### Step 3: Run JSON tree tests

**Action:** Verify tree rendering works

Run: `go test ./internal/tui -run TestRenderJSONTree -v`
Expected: PASS

---

### Step 4: Add verbose toggle to TUI model

**Action:** Add verbose field to Model struct

In `internal/tui/app.go`, find the Model struct (around line 28) and add:

```go
type Model struct {
	commander *core.Commander
	store     *store.Store
	eventCh   <-chan core.Event

	view        View
	width       int
	height      int
	selectedIdx int
	showHelp    bool
	verboseJSON bool  // NEW: show full JSON trees without truncation

	input   InputModel
	// ... rest of fields
}
```

Add a new key binding at the end of the keys struct (around line 720):

```go
	End:       key.NewBinding(key.WithKeys("end", "G")),
	VerboseToggle: key.NewBinding(key.WithKeys("v")),  // NEW
}
```

Add the field to the keys struct definition (around line 687):

```go
var keys = struct {
	Quit      key.Binding
	// ... other bindings
	End       key.Binding
	VerboseToggle key.Binding  // NEW
}{
```

Run: `make build`
Expected: Compile success

---

### Step 5: Add key handler for verbose toggle

**Action:** Handle 'v' key to toggle verbose mode

In `internal/tui/app.go`, find `handleFocusKey` function and add before the viewport key handling:

```go
	case key.Matches(msg, keys.VerboseToggle):
		m.verboseJSON = !m.verboseJSON
		// Re-render viewport content with new verbose setting
		m.updateViewportContent()
		return m, nil
```

Run: `make build`
Expected: Compile success

---

### Step 6: Integrate JSON tree rendering into log entries

**Action:** Detect and render JSON content in tool messages

In `internal/tui/focus.go`, modify `renderLogEntry` function to detect JSON in tool role messages.

Find the content wrapping section (around line 216):
```go
	// Wrap the content
	wrappedLines := wrapText(entry.Content, contentWidth)
```

Replace with:
```go
	// Detect JSON content in tool messages and render as tree
	var wrappedLines []string
	if entry.Role == "tool" && isJSON(entry.Content) {
		// Render JSON as tree structure
		treeStr, err := renderJSONTree(entry.Content, false)  // TODO: pass verbose flag from model
		if err == nil {
			// Tree rendering succeeded
			wrappedLines = strings.Split(treeStr, "\n")
		} else {
			// Fall back to normal wrapping if JSON parsing fails
			wrappedLines = wrapText(entry.Content, contentWidth)
		}
	} else {
		// Normal text wrapping
		wrappedLines = wrapText(entry.Content, contentWidth)
	}
```

Add helper function at the end of focus.go:

```go
// isJSON checks if a string appears to be JSON
func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}
```

Run: `make build`
Expected: Compile success

---

### Step 7: Update renderLogEntry signature to accept verbose flag

**Action:** Pass verbose setting through rendering pipeline

In `internal/tui/focus.go`, update renderLogEntry signature:
```go
func renderLogEntry(entry LogEntry, maxWidth int, verbose bool) []string {
```

Update the JSON tree rendering call:
```go
		treeStr, err := renderJSONTree(entry.Content, verbose)
```

Update all callers in focus.go to pass verbose flag:
```go
	// In RenderFocusView (line 105)
	entryLines := renderLogEntry(entry, panelContentWidth, false)  // TODO: pass from model
	
	// In updateViewportContent (line 640) - this needs model access
	entryLines := renderLogEntry(entry, panelContentWidth, false)  // TODO: pass from model
```

Note: We need to pass the Model through to these functions to access verboseJSON. Let's refactor.

Actually, a cleaner approach: make renderLogEntry a method on Model so it can access m.verboseJSON.

In `internal/tui/app.go`, add method:
```go
// renderLogEntry is a method that can access model's verboseJSON setting
func (m *Model) renderLogEntry(entry LogEntry, maxWidth int) []string {
	return renderLogEntryImpl(entry, maxWidth, m.verboseJSON)
}
```

Rename the current renderLogEntry in focus.go to renderLogEntryImpl and keep it exported for testing.

Run: `make build`
Expected: Compile success

---

### Step 8: Update callers to use model method

**Action:** Replace renderLogEntry calls with model method calls

This requires passing the model reference or the verbose flag through. For simplicity, let's update the viewport rendering to accept a verbose parameter.

In `internal/tui/app.go`, modify updateViewportContent:

```go
func (m *Model) updateViewportContent() {
	if len(m.logs) == 0 {
		m.viewport.SetContent(dimmedStyle.Render("No conversation history."))
		return
	}

	panelContentWidth := m.width - 6

	var lines []string
	for _, entry := range m.logs {
		entryLines := renderLogEntryImpl(entry, panelContentWidth, m.verboseJSON)  // Pass verbose flag
		lines = append(lines, entryLines...)
	}

	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)

	if m.autoScroll {
		m.viewport.GotoBottom()
	}
}
```

Rename `renderLogEntry` to `renderLogEntryImpl` in focus.go and update signature:
```go
func renderLogEntryImpl(entry LogEntry, maxWidth int, verbose bool) []string {
```

Update tests to use renderLogEntryImpl.

Update RenderFocusView (the non-viewport version) to accept verbose parameter:
```go
func RenderFocusView(mysis MysisInfo, logs []LogEntry, width, height int, isLoading bool, spinnerView string, verbose bool) string {
```

And pass it through:
```go
		entryLines := renderLogEntryImpl(entry, panelContentWidth, verbose)
```

Update the caller in app.go View() method (line 238):
```go
		content = RenderFocusView(m.mysisByID(m.focusID), m.viewport, m.width, isLoading, m.spinner.View(), m.autoScroll, m.verboseJSON)
```

Wait, that's the viewport version. Let me check...

Actually, we only use RenderFocusViewWithViewport now. Update its signature:
```go
func RenderFocusViewWithViewport(mysis MysisInfo, vp viewport.Model, width int, isLoading bool, spinnerView string, autoScroll bool, verbose bool) string {
```

We don't need to update anything in that function since the viewport content is already rendered via Model.updateViewportContent().

But we should show the verbose toggle in the footer. Update the footer hint (around line 163):
```go
	// Footer with scroll hints
	verboseHint := ""
	if verbose {
		verboseHint = "  ·  [ v ] VERBOSE: ON"
	} else {
		verboseHint = "  ·  [ v ] VERBOSE: OFF"
	}
	hint := dimmedStyle.Render(fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM%s", verboseHint))
```

Update caller in app.go:
```go
		content = RenderFocusViewWithViewport(m.mysisByID(m.focusID), m.viewport, m.width, isLoading, m.spinner.View(), m.autoScroll, m.verboseJSON)
```

Run: `make build`
Expected: Compile success

---

### Step 9: Update focus.go to use verbose parameter in JSON rendering

**Action:** Apply the verbose flag in renderLogEntryImpl

Find the JSON tree rendering call:
```go
		treeStr, err := renderJSONTree(entry.Content, verbose)
```

This should now receive the verbose flag correctly.

Run: `make build`
Expected: Compile success

---

### Step 10: Update tests for new signatures

**Action:** Fix test calls to match new function signatures

In `internal/tui/focus_test.go`, update calls from `renderLogEntry` to `renderLogEntryImpl` and add verbose parameter:

```go
lines := renderLogEntryImpl(entry, maxWidth, false)
```

Do this for all test functions.

In `internal/tui/dashboard_test.go`, no changes needed (doesn't call renderLogEntry).

Run: `go test ./internal/tui -v`
Expected: All tests pass

---

### Step 11: Write integration test for JSON tree in focus view

**Action:** Add test that verifies JSON rendering in tool messages

Add to `internal/tui/focus_test.go`:

```go
func TestRenderLogEntryToolWithJSON(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	jsonPayload := `{"ship_id": "abc123", "cargo": [{"item": "iron", "quantity": 50}, {"item": "gold", "quantity": 10}], "fuel": 100}`

	entry := LogEntry{
		Role:      "tool",
		Source:    "tool",
		Content:   jsonPayload,
		Timestamp: time.Now(),
	}

	maxWidth := 80
	lines := renderLogEntryImpl(entry, maxWidth, false)

	output := strings.Join(lines, "\n")

	// Should have tree structure
	if !strings.Contains(output, "├─") && !strings.Contains(output, "└─") {
		t.Error("Expected tree box characters in tool JSON output")
	}

	// Should contain field names
	if !strings.Contains(output, "ship_id") || !strings.Contains(output, "cargo") {
		t.Error("Expected JSON field names in tree output")
	}
}
```

Run: `go test ./internal/tui -run TestRenderLogEntryToolWithJSON -v`
Expected: PASS

---

### Step 12: Run full test suite

**Action:** Verify all tests pass

Run: `make test`
Expected: All tests pass

---

### Step 13: Commit JSON tree rendering feature

**Action:** Commit the changes

```bash
git add internal/tui/json_tree.go internal/tui/json_tree_test.go internal/tui/focus.go internal/tui/focus_test.go internal/tui/app.go internal/tui/dashboard_test.go
git commit -m "feat(tui): add JSON tree rendering for tool results

- Implement Unicode tree rendering with smart truncation
- Show first 3 and last 3 items for large arrays
- Add verbose toggle (v key) for full JSON display
- Automatically detect and render JSON in tool messages
- Add comprehensive test coverage for tree rendering"
```

---

## Task 4: Add Scrollbar Indicator

**Files:**
- Create: `internal/tui/scrollbar.go`
- Create: `internal/tui/scrollbar_test.go`
- Modify: `internal/tui/focus.go:126-167` (RenderFocusViewWithViewport function)

**Context:** Focus view conversation log needs a visual scrollbar to help users understand scroll position and total content height. We'll add a minimal Unicode scrollbar on the right edge of the viewport.

---

### Step 1: Write test for scrollbar rendering

**Action:** Create test file for scrollbar logic

Create `internal/tui/scrollbar_test.go`:

```go
package tui

import (
	"strings"
	"testing"
)

func TestRenderScrollbar_AtTop(t *testing.T) {
	height := 10
	totalLines := 100
	scrollOffset := 0
	
	bar := renderScrollbar(height, totalLines, scrollOffset)
	
	// Should be height lines
	lines := strings.Split(bar, "\n")
	if len(lines) != height {
		t.Errorf("Expected %d lines, got %d", height, len(lines))
	}
	
	// First line should have thumb indicator
	if !strings.Contains(lines[0], "█") && !strings.Contains(lines[0], "▓") {
		t.Error("Expected thumb indicator in first line when at top")
	}
}

func TestRenderScrollbar_AtBottom(t *testing.T) {
	height := 10
	totalLines := 100
	scrollOffset := 90  // At bottom (90 + 10 = 100)
	
	bar := renderScrollbar(height, totalLines, scrollOffset)
	
	lines := strings.Split(bar, "\n")
	if len(lines) != height {
		t.Errorf("Expected %d lines, got %d", height, len(lines))
	}
	
	// Last line should have thumb indicator
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "█") && !strings.Contains(lastLine, "▓") {
		t.Error("Expected thumb indicator in last line when at bottom")
	}
}

func TestRenderScrollbar_Middle(t *testing.T) {
	height := 10
	totalLines := 100
	scrollOffset := 45  // Roughly middle
	
	bar := renderScrollbar(height, totalLines, scrollOffset)
	
	lines := strings.Split(bar, "\n")
	
	// Thumb should be somewhere in the middle lines (not first or last)
	hasThumbInMiddle := false
	for i := 2; i < len(lines)-2; i++ {
		if strings.Contains(lines[i], "█") || strings.Contains(lines[i], "▓") {
			hasThumbInMiddle = true
			break
		}
	}
	
	if !hasThumbInMiddle {
		t.Error("Expected thumb indicator in middle lines when scrolled to middle")
	}
}

func TestRenderScrollbar_NoScroll(t *testing.T) {
	height := 10
	totalLines := 5  // Content fits in viewport
	scrollOffset := 0
	
	bar := renderScrollbar(height, totalLines, scrollOffset)
	
	lines := strings.Split(bar, "\n")
	
	// All lines should show track (no thumb needed when everything fits)
	for i, line := range lines {
		if !strings.Contains(line, "│") && !strings.Contains(line, "║") {
			t.Errorf("Expected track character in line %d when no scroll needed", i)
		}
	}
}
```

Run: `go test ./internal/tui -run TestRenderScrollbar -v`
Expected: FAIL (renderScrollbar not yet implemented)

---

### Step 2: Implement scrollbar rendering function

**Action:** Create scrollbar.go with rendering logic

Create `internal/tui/scrollbar.go`:

```go
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	scrollbarThumb = "█"  // Solid block for thumb
	scrollbarTrack = "│"  // Thin vertical line for track
)

// renderScrollbar creates a vertical scrollbar indicator.
// height: viewport height in lines
// totalLines: total content lines
// scrollOffset: current scroll position (line number at top of viewport)
// Returns a multi-line string with one character per line.
func renderScrollbar(height int, totalLines int, scrollOffset int) string {
	if height <= 0 {
		return ""
	}

	// If content fits in viewport, show empty track
	if totalLines <= height {
		track := make([]string, height)
		trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))  // Dim gray
		for i := 0; i < height; i++ {
			track[i] = trackStyle.Render(scrollbarTrack)
		}
		return strings.Join(track, "\n")
	}

	// Calculate thumb position and size
	// Thumb size is proportional to viewport/content ratio, minimum 1 line
	thumbSize := (height * height) / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > height {
		thumbSize = height
	}

	// Calculate thumb position based on scroll offset
	// Position is proportional to scroll position
	scrollRatio := float64(scrollOffset) / float64(totalLines-height)
	if scrollRatio < 0 {
		scrollRatio = 0
	}
	if scrollRatio > 1 {
		scrollRatio = 1
	}
	
	maxThumbPos := height - thumbSize
	thumbPos := int(scrollRatio * float64(maxThumbPos))
	
	// Build scrollbar lines
	lines := make([]string, height)
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))   // Dim gray
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))   // Lighter gray
	
	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			lines[i] = thumbStyle.Render(scrollbarThumb)
		} else {
			lines[i] = trackStyle.Render(scrollbarTrack)
		}
	}
	
	return strings.Join(lines, "\n")
}
```

Run: `make build`
Expected: Compile success

---

### Step 3: Run scrollbar tests

**Action:** Verify scrollbar rendering works

Run: `go test ./internal/tui -run TestRenderScrollbar -v`
Expected: PASS

---

### Step 4: Integrate scrollbar into focus view

**Action:** Add scrollbar to viewport display

In `internal/tui/focus.go`, find RenderFocusViewWithViewport function.

Find the viewport rendering section (around line 157-160):
```go
	// Viewport content (scrollable)
	// Add horizontal padding (2 spaces each side) for content inside panel border
	vpView := logStyle.Width(width-2).Padding(0, 2).Render(vp.View())
	sections = append(sections, vpView)
```

Replace with:
```go
	// Viewport content (scrollable) with scrollbar
	// Calculate scrollbar
	totalLines := strings.Count(vp.View(), "\n") + 1
	scrollOffset := vp.YOffset
	scrollbar := renderScrollbar(vp.Height, totalLines, scrollOffset)
	
	// Split viewport content into lines
	vpLines := strings.Split(vp.View(), "\n")
	scrollbarLines := strings.Split(scrollbar, "\n")
	
	// Combine viewport lines with scrollbar
	combinedLines := make([]string, vp.Height)
	for i := 0; i < vp.Height; i++ {
		var vpLine string
		if i < len(vpLines) {
			vpLine = vpLines[i]
		}
		var scrollLine string
		if i < len(scrollbarLines) {
			scrollLine = scrollbarLines[i]
		}
		
		// Viewport content width accounts for scrollbar (1 char)
		// Panel is width-2 (borders), padding is 2 each side, scrollbar is 1 char
		// Content width = width - 2 (borders) - 4 (padding) - 1 (scrollbar) = width - 7
		contentWidth := width - 7
		if contentWidth < 20 {
			contentWidth = 20
		}
		
		// Truncate or pad viewport line to content width
		vpLineWidth := lipgloss.Width(vpLine)
		if vpLineWidth > contentWidth {
			// Truncate (shouldn't happen if wrapping is correct)
			vpLine = truncateToWidth(vpLine, contentWidth)
		} else if vpLineWidth < contentWidth {
			// Pad to align scrollbar
			vpLine += strings.Repeat(" ", contentWidth-vpLineWidth)
		}
		
		// Combine: left padding + content + scrollbar + right padding
		combinedLines[i] = "  " + vpLine + scrollLine + "  "
	}
	
	combinedContent := strings.Join(combinedLines, "\n")
	vpView := logStyle.Width(width-2).Height(vp.Height).Render(combinedContent)
	sections = append(sections, vpView)
```

Wait, this approach is getting complex. The viewport already renders its content. A simpler approach: render the scrollbar separately and position it absolutely.

Actually, lipgloss doesn't support absolute positioning. Let's use JoinHorizontal to place scrollbar next to viewport.

Better approach: render scrollbar as a separate column and use lipgloss.JoinHorizontal.

Let me revise:

```go
	// Viewport content (scrollable)
	vpContent := vp.View()
	
	// Calculate scrollbar based on viewport state
	// Total lines is the line count in viewport's full content
	totalLines := vp.TotalLineCount()  // This method exists in viewport.Model
	scrollOffset := vp.YOffset
	scrollbarColumn := renderScrollbar(vp.Height, totalLines, scrollOffset)
	
	// Style the viewport content with padding
	styledVP := logStyle.Width(width-2-2).Padding(0, 2).Render(vpContent)  // -2 for scrollbar width
	
	// Combine viewport and scrollbar horizontally
	combined := lipgloss.JoinHorizontal(lipgloss.Top, styledVP, scrollbarColumn)
	
	// Wrap in panel border
	vpView := logStyle.Width(width-2).Height(vp.Height).Render(combined)
	sections = append(sections, vpView)
```

Hmm, but we're double-wrapping with logStyle. Let me check the current implementation...

Looking at line 159, the viewport is already styled with logStyle. We need to adjust widths to make room for scrollbar.

Actually, the cleanest approach: modify updateViewportContent in app.go to render content with scrollbar already embedded per line. But that's complex.

Simpler: render scrollbar as overlay by modifying the final render.

Let's use a different approach: modify the viewport width to leave room for scrollbar, then overlay the scrollbar in the rendered output.

In `internal/tui/app.go`, modify viewport width calculation (line 129):
```go
		m.viewport.Width = msg.Width - 6 - 1  // -6 for existing padding, -1 for scrollbar
```

In `internal/tui/focus.go`, update RenderFocusViewWithViewport:

```go
	// Viewport content (scrollable)
	// Render scrollbar based on viewport state
	totalLines := vp.TotalLineCount()
	scrollOffset := vp.YOffset
	scrollbarStr := renderScrollbar(vp.Height, totalLines, scrollOffset)
	scrollbarLines := strings.Split(scrollbarStr, "\n")
	
	// Get viewport content lines
	vpContentLines := strings.Split(vp.View(), "\n")
	
	// Combine each line with scrollbar
	combinedLines := make([]string, vp.Height)
	for i := 0; i < vp.Height; i++ {
		var contentLine string
		if i < len(vpContentLines) {
			contentLine = vpContentLines[i]
		}
		var scrollLine string
		if i < len(scrollbarLines) {
			scrollLine = " " + scrollbarLines[i]  // Space before scrollbar
		} else {
			scrollLine = "  "  // Empty if no scrollbar line
		}
		combinedLines[i] = contentLine + scrollLine
	}
	
	combinedContent := strings.Join(combinedLines, "\n")
	vpView := logStyle.Width(width-2).Padding(0, 2).Render(combinedContent)
	sections = append(sections, vpView)
```

Run: `make build`
Expected: Compile success

---

### Step 5: Verify viewport.TotalLineCount() exists

**Action:** Check if viewport has this method

Actually, viewport.Model doesn't expose total line count directly. We need to get it from the content.

The viewport content is set via SetContent(). We can calculate line count from vp.View() but that only shows visible content...

Actually, we need access to the full content. The viewport stores this internally but doesn't expose it.

Solution: track totalLines in Model when we set viewport content.

In `internal/tui/app.go`, add field to Model:
```go
	viewport   viewport.Model
	autoScroll bool
	viewportTotalLines int  // NEW: total lines in viewport content
```

In updateViewportContent, set this field:
```go
	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	m.viewportTotalLines = len(lines)  // Track total lines
```

Pass this to RenderFocusViewWithViewport:
```go
		content = RenderFocusViewWithViewport(m.mysisByID(m.focusID), m.viewport, m.width, isLoading, m.spinner.View(), m.autoScroll, m.verboseJSON, m.viewportTotalLines)
```

Update RenderFocusViewWithViewport signature:
```go
func RenderFocusViewWithViewport(mysis MysisInfo, vp viewport.Model, width int, isLoading bool, spinnerView string, autoScroll bool, verbose bool, totalLines int) string {
```

Use totalLines in scrollbar rendering:
```go
	scrollbarStr := renderScrollbar(vp.Height, totalLines, vp.YOffset)
```

Run: `make build`
Expected: Compile success

---

### Step 6: Test scrollbar display manually

**Action:** Build and run to verify scrollbar appears

Run: `make build && ./bin/zoea`
Expected: Scrollbar appears on right side of focus view when content exceeds viewport height

Visual check:
- Scrollbar thumb should move as you scroll up/down
- Thumb should be at top when at top of conversation
- Thumb should be at bottom when at bottom of conversation

---

### Step 7: Write integration test for scrollbar in focus view

**Action:** Add test that verifies scrollbar is rendered

Add to `internal/tui/focus_test.go`:

```go
func TestRenderFocusViewWithScrollbar(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Create viewport with content
	vp := viewport.New(80, 10)
	longContent := strings.Repeat("line\n", 50)  // 50 lines
	vp.SetContent(longContent)
	vp.GotoTop()

	mysis := MysisInfo{
		ID:       "test-id",
		Name:     "test-mysis",
		State:    "running",
		Provider: "ollama",
	}

	width := 100
	totalLines := 50
	output := RenderFocusViewWithViewport(mysis, vp, width, false, "⬡", true, false, totalLines)

	// Should contain scrollbar characters
	if !strings.Contains(output, "█") && !strings.Contains(output, "│") {
		t.Error("Expected scrollbar characters in focus view output")
	}
}
```

Run: `go test ./internal/tui -run TestRenderFocusViewWithScrollbar -v`
Expected: PASS

---

### Step 8: Run full test suite

**Action:** Verify all tests pass

Run: `make test`
Expected: All tests pass

---

### Step 9: Commit scrollbar feature

**Action:** Commit the changes

```bash
git add internal/tui/scrollbar.go internal/tui/scrollbar_test.go internal/tui/focus.go internal/tui/focus_test.go internal/tui/app.go
git commit -m "feat(tui): add visual scrollbar indicator to focus view

- Implement Unicode scrollbar with proportional thumb
- Show scroll position and total content height
- Update viewport width to accommodate scrollbar
- Track total line count for accurate scrollbar sizing
- Add test coverage for scrollbar rendering"
```

---

## Task 5: Update Documentation

**Files:**
- Modify: `documentation/KNOWN_ISSUES.md`
- Modify: `README.md`

**Context:** Move completed TUI enhancements from Medium Priority section to Recently Resolved section with completion date.

---

### Step 1: Update KNOWN_ISSUES.md

**Action:** Mark TUI enhancements as resolved

In `documentation/KNOWN_ISSUES.md`, remove these items from Medium Priority section:

```markdown
- [ ] **Display reasoning in focus view** - Reasoning content is stored in database but not rendered in TUI
  - **Proposed:** Render reasoning messages using existing purple text color
  - **Location:** `internal/tui/focus.go`

- [ ] **Show account status in views** - Surface which game account username each mysis is currently using
  - **Locations:** Focus view header, commander dashboard
  - **Evidence:** Focus labels based on role only; account fields not present in TUI models

- [ ] **Render JSON as tree view** - Tool results with large JSON payloads should use Unicode tree rendering with smart truncation
  - **Format:** Show first 3 items, `[x more]`, last 3 items
  - **Enhancement:** Add verbose toggle for full output

- [ ] **Add scrollbar indicator** - Visual scrollbar for focus view conversation log
  - **Enhancement:** Improves navigation UX for long conversations
```

Add to Recently Resolved section:

```markdown
- [x] **TUI Enhancements** (2026-02-05) - Implemented display reasoning in focus view, account status in dashboard and focus header, JSON tree rendering with verbose toggle, and visual scrollbar indicator. Improves readability and navigation UX.
```

Run: `git diff documentation/KNOWN_ISSUES.md`
Expected: Shows the documented changes

---

### Step 2: Update README.md keyboard shortcuts

**Action:** Add new keyboard shortcut for verbose toggle

In `README.md`, find the Keyboard Shortcuts table and add:

```markdown
| `v`     | Toggle verbose JSON (focus) |
```

Alphabetically this should go near the end, after `s` and before `?`.

Run: `git diff README.md`
Expected: Shows the new shortcut

---

### Step 3: Commit documentation updates

**Action:** Commit doc changes

```bash
git add documentation/KNOWN_ISSUES.md README.md
git commit -m "docs: mark TUI enhancements as complete

- Move completed items from Medium Priority to Recently Resolved
- Add verbose toggle keyboard shortcut to README"
```

---

## Summary

This plan implements all four TUI enhancement tasks:

1. **Display Reasoning** - Shows LLM reasoning content below assistant messages in purple
2. **Account Status** - Displays game account username in dashboard list and focus header
3. **JSON Tree Rendering** - Renders tool results as Unicode trees with smart truncation and verbose toggle
4. **Scrollbar Indicator** - Adds visual scrollbar to show scroll position in focus view

**Testing Strategy:**
- Unit tests for each new rendering function
- Integration tests for TUI views
- Manual testing in live TUI

**Estimated Time:** 2-3 hours for implementation + testing

**Dependencies:** None - all changes are isolated to TUI layer

**Breaking Changes:** None - purely additive features
