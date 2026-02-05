package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/store"
)

// LogEntry represents a log entry for display.
type LogEntry struct {
	Role       string
	Source     string
	SenderID   string
	SenderName string
	Content    string
	Reasoning  string // NEW: reasoning content from LLM
	Timestamp  time.Time
}

// wrapText wraps text to fit within maxWidth display columns, preserving words.
// Uses lipgloss.Width() for proper Unicode character width calculation.
// Long words that exceed maxWidth are hard-wrapped to prevent overflow.
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := ""
		for _, word := range words {
			wordWidth := lipgloss.Width(word)

			// If word itself is too long, hard-wrap it
			if wordWidth > maxWidth {
				// Flush current line first
				if currentLine != "" {
					lines = append(lines, currentLine)
					currentLine = ""
				}
				// Hard-wrap the long word
				for len(word) > 0 {
					chunk := truncateToWidth(word, maxWidth)
					lines = append(lines, chunk)
					// Remove the chunk from word
					chunkRunes := []rune(chunk)
					wordRunes := []rune(word)
					if len(chunkRunes) < len(wordRunes) {
						word = string(wordRunes[len(chunkRunes):])
					} else {
						word = ""
					}
				}
				continue
			}

			// Normal word wrapping
			if currentLine == "" {
				currentLine = word
			} else {
				lineWidth := lipgloss.Width(currentLine)
				if lineWidth+1+wordWidth <= maxWidth {
					currentLine += " " + word
				} else {
					lines = append(lines, currentLine)
					currentLine = word
				}
			}
		}
		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}

// RenderFocusView renders the detailed mysis view (legacy, without viewport).
func RenderFocusView(mysis MysisInfo, logs []LogEntry, width, height int, isLoading bool, spinnerView string, verbose bool, focusIndex, totalMyses int) string {
	var sections []string

	// Header with mysis name - spans full width
	header := renderFocusHeader(mysis.Name, focusIndex, totalMyses, width)
	sections = append(sections, header)

	// Mysis info panel
	stateDisplay := StateStyle(mysis.State).Render(mysis.State)
	if isLoading {
		stateDisplay += " " + spinnerView + " thinking..."
	}

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

	createdText := dimmedStyle.Render("(unknown)")
	if !mysis.CreatedAt.IsZero() {
		createdText = valueStyle.Render(mysis.CreatedAt.Local().Format("2006-01-02 15:04"))
	}
	infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Created:"), createdText))

	if mysis.State == "errored" && mysis.LastError != "" {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Error:"), stateErroredStyle.Render(mysis.LastError)))
	}

	infoContent := strings.Join(infoLines, "  ")
	infoPanel := panelStyle.Width(width - 2).Render(infoContent)
	sections = append(sections, infoPanel)

	// Logs panel - use full-width section title
	logTitle := renderSectionTitle("CONVERSATION LOG", width)
	sections = append(sections, logTitle)

	// Calculate available height for logs
	usedHeight := lipgloss.Height(strings.Join(sections, "\n")) + 4 // +4 for footer
	logHeight := height - usedHeight - 2
	if logHeight < 5 {
		logHeight = 5
	}

	var logLines []string
	// Panel is rendered with logStyle.Width(width - 2).Padding(0, 2)
	// Content width = width - 2, minus 4 for padding (2 each side) = width - 6
	panelContentWidth := width - 6
	if len(logs) == 0 {
		logLines = append(logLines, dimmedStyle.Render("No conversation history."))
	} else {
		// Render all log entries to fill panel content area
		for _, entry := range logs {
			entryLines := renderLogEntryImpl(entry, panelContentWidth, verbose)
			logLines = append(logLines, entryLines...)
		}

		// Show most recent lines that fit
		if len(logLines) > logHeight {
			logLines = logLines[len(logLines)-logHeight:]
		}
	}

	logContent := strings.Join(logLines, "\n")
	logPanel := logStyle.Width(width-2).Height(logHeight).Padding(0, 2).Render(logContent)
	sections = append(sections, logPanel)

	// Footer
	hint := dimmedStyle.Render("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ r ] RELAUNCH  ·  [ s ] STOP")
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// RenderFocusViewWithViewport renders the detailed mysis view using a scrollable viewport.
func RenderFocusViewWithViewport(mysis MysisInfo, vp viewport.Model, width int, isLoading bool, spinnerView string, autoScroll bool, verbose bool, totalLines int, focusIndex, totalMyses int) string {
	var sections []string

	// Header with mysis name - spans full width
	header := renderFocusHeader(mysis.Name, focusIndex, totalMyses, width)
	sections = append(sections, header)

	// Mysis info panel
	stateDisplay := StateStyle(mysis.State).Render(mysis.State)
	if isLoading {
		stateDisplay += " " + spinnerView + " thinking..."
	}

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

	createdText := dimmedStyle.Render("(unknown)")
	if !mysis.CreatedAt.IsZero() {
		createdText = valueStyle.Render(mysis.CreatedAt.Local().Format("2006-01-02 15:04"))
	}
	infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Created:"), createdText))

	if mysis.State == "errored" && mysis.LastError != "" {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Error:"), stateErroredStyle.Render(mysis.LastError)))
	}

	infoContent := strings.Join(infoLines, "  ")
	infoPanel := panelStyle.Width(width - 2).Render(infoContent)
	sections = append(sections, infoPanel)

	// Conversation title with scroll indicator - spans full width
	scrollInfo := ""
	if !autoScroll && totalLines > 0 {
		currentLine := vp.YOffset + 1
		if currentLine > totalLines {
			currentLine = totalLines
		}
		scrollInfo = fmt.Sprintf("  LINE %d/%d", currentLine, totalLines)
	}
	logTitle := renderSectionTitleWithSuffix("CONVERSATION LOG", scrollInfo, width)
	sections = append(sections, logTitle)

	// Viewport content (scrollable) with scrollbar
	// Render scrollbar based on viewport state
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
			scrollLine = " " + scrollbarLines[i] // Space before scrollbar
		} else {
			scrollLine = "  " // Empty if no scrollbar line
		}
		combinedLines[i] = contentLine + scrollLine
	}

	combinedContent := strings.Join(combinedLines, "\n")
	vpView := logStyle.Width(width-2).Padding(0, 2).Render(combinedContent)
	sections = append(sections, vpView)

	// Footer with scroll hints and verbose toggle
	verboseHint := ""
	if verbose {
		verboseHint = "  ·  [ v ] VERBOSE: ON"
	} else {
		verboseHint = "  ·  [ v ] VERBOSE: OFF"
	}
	hint := dimmedStyle.Render(fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM%s", verboseHint))
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderLogEntryImpl(entry LogEntry, maxWidth int, verbose bool) []string {
	// Get the role's foreground color
	roleColor := RoleColor(entry.Role)
	prefixStyle := lipgloss.NewStyle().
		Foreground(roleColor)

	// Content style - no styling (transparent background)
	contentStyle := lipgloss.NewStyle()

	var rolePrefix string
	switch entry.Role {
	case "user":
		if entry.Source == "broadcast_self" {
			rolePrefix = "YOU (BROADCAST):"
		} else if entry.Source == "broadcast" {
			label := formatSenderLabel(entry.SenderID, entry.SenderName)
			if label != "" {
				rolePrefix = fmt.Sprintf("SWARM (%s):", label)
			} else {
				rolePrefix = "SWARM:"
			}
			roleColor = lipgloss.Color("214")
		} else {
			rolePrefix = "YOU:"
		}
	case "assistant":
		rolePrefix = "AI:"
	case "system":
		rolePrefix = "SYS:"
	case "tool":
		rolePrefix = "TOOL:"
	default:
		rolePrefix = "???:"
	}

	timePrefix := "--:--:--"
	if !entry.Timestamp.IsZero() {
		timePrefix = entry.Timestamp.Local().Format("15:04:05")
	}

	prefix := fmt.Sprintf("%s %s", timePrefix, rolePrefix)
	// Update prefix style with final role color
	prefixStyle = prefixStyle.Foreground(roleColor)

	// Inside padding: 1 space on left and right of all content
	const padLeft = 1
	const padRight = 1

	// Calculate content width (accounting for prefix, padding, and gap after prefix)
	prefixWidth := lipgloss.Width(prefix) + 1 // +1 for space after prefix
	contentWidth := maxWidth - prefixWidth - padLeft - padRight
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Detect JSON content in tool messages and render as tree
	var wrappedLines []string
	if entry.Role == "tool" && isJSON(entry.Content) {
		// Extract JSON content (strip tool call ID prefix if present)
		jsonContent := entry.Content
		if idx := strings.Index(jsonContent, ":"); idx > 0 && strings.HasPrefix(jsonContent, "call_") {
			jsonContent = jsonContent[idx+1:]
		}

		// Render JSON as tree structure with width constraint
		treeStr, err := renderJSONTree(jsonContent, verbose, contentWidth)
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

	var result []string
	indent := strings.Repeat(" ", prefixWidth)

	// Add top padding (empty line with background)
	emptyLine := contentStyle.Width(maxWidth).Render("")
	result = append(result, emptyLine)

	for i, line := range wrappedLines {
		// Pad the line content to fill remaining width
		lineLen := lipgloss.Width(line)
		remainingWidth := contentWidth - lineLen
		if remainingWidth < 0 {
			remainingWidth = 0
		}
		paddedLine := line + strings.Repeat(" ", remainingWidth+padRight)

		if i == 0 {
			// First line: left pad + styled prefix + space + content with background
			leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
			styledPrefix := prefixStyle.Render(prefix)
			styledContent := contentStyle.Render(" " + paddedLine)
			result = append(result, leftPad+styledPrefix+styledContent)
		} else {
			// Continuation lines: left pad + indent + content with background
			leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
			styledIndent := contentStyle.Render(indent)
			styledContent := contentStyle.Render(paddedLine)
			result = append(result, leftPad+styledIndent+styledContent)
		}
	}

	// Render reasoning if present
	if entry.Reasoning != "" {
		// Add spacing line
		emptyLine := contentStyle.Width(maxWidth).Render("")
		result = append(result, emptyLine)

		// Reasoning header in dimmed purple/magenta
		reasoningHeaderStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")) // Dimmed purple

		reasoningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("213")) // Lighter purple for reasoning text

		dimmedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dim gray for truncation indicator

		reasoningHeader := "REASONING:"
		reasoningHeaderWidth := lipgloss.Width(reasoningHeader) + 1 // +1 for space after
		reasoningContentWidth := maxWidth - reasoningHeaderWidth - padLeft - padRight
		if reasoningContentWidth < 20 {
			reasoningContentWidth = 20
		}

		// Wrap reasoning text
		wrappedReasoning := wrapText(entry.Reasoning, reasoningContentWidth)

		// Smart truncation: show first line, "[x more]", last 2 lines if verbose is false and > 3 lines
		const reasoningTruncateThreshold = 3
		const reasoningShowLast = 2
		shouldTruncate := !verbose && len(wrappedReasoning) > reasoningTruncateThreshold

		var linesToShow []int
		if shouldTruncate {
			// First line
			linesToShow = append(linesToShow, 0)
			// Last 2 lines
			for i := len(wrappedReasoning) - reasoningShowLast; i < len(wrappedReasoning); i++ {
				linesToShow = append(linesToShow, i)
			}
		} else {
			// Show all lines
			for i := 0; i < len(wrappedReasoning); i++ {
				linesToShow = append(linesToShow, i)
			}
		}

		truncatedCount := len(wrappedReasoning) - len(linesToShow)

		for idx, i := range linesToShow {
			line := wrappedReasoning[i]
			lineLen := lipgloss.Width(line)
			remainingWidth := reasoningContentWidth - lineLen
			if remainingWidth < 0 {
				remainingWidth = 0
			}
			paddedLine := line + strings.Repeat(" ", remainingWidth+padRight)

			// Insert truncation indicator after first line
			if shouldTruncate && idx == 1 {
				truncMsg := fmt.Sprintf("[%d more]", truncatedCount)
				truncMsgLen := lipgloss.Width(truncMsg)
				truncRemainingWidth := reasoningContentWidth - truncMsgLen
				if truncRemainingWidth < 0 {
					truncRemainingWidth = 0
				}
				truncPaddedLine := truncMsg + strings.Repeat(" ", truncRemainingWidth+padRight)

				leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
				indent := strings.Repeat(" ", reasoningHeaderWidth)
				styledIndent := contentStyle.Render(indent)
				styledTrunc := dimmedStyle.Render(truncPaddedLine)
				result = append(result, leftPad+styledIndent+styledTrunc)
			}

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

// LogEntryFromMemory converts a store.Memory to LogEntry.
func LogEntryFromMemory(m *store.Memory, currentMysisID, senderName string) LogEntry {
	source := string(m.Source)
	if m.Source == store.MemorySourceBroadcast && m.SenderID == currentMysisID {
		source = "broadcast_self"
	}
	return LogEntry{
		Role:       string(m.Role),
		Source:     source,
		SenderID:   m.SenderID,
		SenderName: senderName,
		Content:    m.Content,
		Reasoning:  m.Reasoning, // NEW: copy reasoning field
		Timestamp:  m.CreatedAt,
	}
}

// renderFocusHeader renders the focus view header spanning full width.
func renderFocusHeader(mysisName string, focusIndex, totalMyses int, width int) string {
	// Format:  ⬥─── ⬡ MYSIS: name ⬡ ───⬥ with dashes filling the remaining space
	// Build title with spaces
	countText := ""
	if totalMyses > 0 && focusIndex > 0 {
		countText = fmt.Sprintf(" (%d/%d)", focusIndex, totalMyses)
	}
	titleText := " ⬡ MYSIS: " + mysisName + countText + " ⬡ "
	titleDisplayWidth := lipgloss.Width(titleText)
	// Total fixed chars: space (1) + ⬥ (1) on left, ⬥ (1) on right = 3
	availableWidth := width - titleDisplayWidth - 3
	if availableWidth < 4 {
		availableWidth = 4
	}
	leftDashes := availableWidth / 2
	rightDashes := availableWidth - leftDashes

	line := " ⬥" + strings.Repeat("─", leftDashes) + titleText + strings.Repeat("─", rightDashes) + "⬥"
	return headerStyle.Width(width).Render(line)
}

// isJSON checks if a string appears to be JSON.
// Handles tool result format: "tool_call_id:json_content"
func isJSON(s string) bool {
	s = strings.TrimSpace(s)

	// Strip tool call ID prefix if present (format: "call_xxx:json")
	if idx := strings.Index(s, ":"); idx > 0 && strings.HasPrefix(s, "call_") {
		s = s[idx+1:]
		s = strings.TrimSpace(s)
	}

	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}
