package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/constants"
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
func RenderFocusView(mysis MysisInfo, logs []LogEntry, width, height int, isLoading bool, spinnerView string, verbose bool, focusIndex, totalMyses int, currentTick int64, err error) string {
	var sections []string

	// Header with mysis name - spans full width (2 lines)
	header := renderFocusHeader(mysis, focusIndex, totalMyses, width, spinnerView)
	sections = append(sections, header)

	// Mysis info panel (State, Provider, Account only - ID/Created/Error moved to header)
	stateDisplay := StateStyle(mysis.State).Render(mysis.State)
	if isLoading {
		stateDisplay += " " + spinnerView + " thinking..."
	}

	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), stateDisplay),
		fmt.Sprintf("%s %s", labelStyle.Render("Provider:"), valueStyle.Render(mysis.Provider)),
	}

	// Add account info if available
	if mysis.AccountUsername != "" {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Account:"), valueStyle.Render(mysis.AccountUsername)))
	} else {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Account:"), dimmedStyle.Render("(not logged in)")))
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
			entryLines := renderLogEntryImpl(entry, panelContentWidth, verbose, currentTick)
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

	// Footer with hint and error status
	hint := renderHintWithError("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ r ] RELAUNCH  ·  [ s ] STOP", err, width)
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// RenderFocusViewWithViewport renders the detailed mysis view using a scrollable viewport.
func RenderFocusViewWithViewport(mysis MysisInfo, vp viewport.Model, width int, isLoading bool, spinnerView string, verbose bool, totalLines int, focusIndex, totalMyses int, currentTick int64, gameStateSnapshots []*store.GameStateSnapshot, err error) string {
	var sections []string

	// Header with mysis name - spans full width (2 lines)
	header := renderFocusHeader(mysis, focusIndex, totalMyses, width, spinnerView)
	sections = append(sections, header)

	// Mysis info panel (State, Provider, Account only - ID/Created/Error moved to header)
	stateDisplay := StateStyle(mysis.State).Render(mysis.State)
	if isLoading {
		stateDisplay += " " + spinnerView + " thinking..."
	}

	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), stateDisplay),
		fmt.Sprintf("%s %s", labelStyle.Render("Provider:"), valueStyle.Render(mysis.Provider)),
	}

	// Add account info if available
	if mysis.AccountUsername != "" {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Account:"), valueStyle.Render(mysis.AccountUsername)))
	} else {
		infoLines = append(infoLines, fmt.Sprintf("%s %s", labelStyle.Render("Account:"), dimmedStyle.Render("(not logged in)")))
	}

	infoContent := strings.Join(infoLines, "  ")
	infoPanel := panelStyle.Width(width - 2).Render(infoContent)
	sections = append(sections, infoPanel)

	// Two-column layout: Game State Sidebar | Conversation Log
	const sidebarWidth = 24 // Fixed width for game state sidebar (total including border)
	const columnGap = 2     // Gap between columns
	conversationWidth := width - sidebarWidth - columnGap

	// Conversation title with scroll indicator
	scrollInfo := ""
	if !vp.AtBottom() && totalLines > 0 {
		currentLine := vp.YOffset + 1
		if currentLine > totalLines {
			currentLine = totalLines
		}
		scrollInfo = fmt.Sprintf("  LINE %d/%d", currentLine, totalLines)
	}
	logTitle := renderSectionTitleWithSuffix("CONVERSATION LOG", scrollInfo, conversationWidth)

	// Game state title
	gameStateTitle := renderSectionTitle("GAME STATE", sidebarWidth)

	// Render game state sidebar content (returns lines without border)
	gameStateSidebarContent := renderGameStateSidebar(gameStateSnapshots, currentTick, sidebarWidth)

	// Render sidebar with border using lipgloss
	// Width() sets CONTENT width, padding is added on top, border is INCLUDED in width
	// So: content_width + padding(2) = sidebarWidth
	// Therefore: content_width = sidebarWidth - 2
	sidebarStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Width(sidebarWidth-2). // -2 for padding (border is included in width)
		Padding(0, 1)          // 1 space padding on left/right

	sidebarRendered := sidebarStyle.Render(strings.Join(gameStateSidebarContent, "\n"))
	gameStateSidebarLines := strings.Split(sidebarRendered, "\n")

	// Render scrollbar based on viewport state
	scrollOffset := vp.YOffset
	scrollbarStr := renderScrollbar(vp.Height, totalLines, scrollOffset)
	scrollbarLines := strings.Split(scrollbarStr, "\n")

	// Get viewport content lines
	vpContentLines := strings.Split(vp.View(), "\n")

	// Combine viewport content with scrollbar
	conversationLines := make([]string, vp.Height)
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
		conversationLines[i] = contentLine + scrollLine
	}

	// Render conversation log (no border, just content)
	// The conversation log doesn't have a border, so we use the full conversationWidth
	conversationContent := strings.Join(conversationLines, "\n")
	conversationView := logStyle.Width(conversationWidth).Render(conversationContent)
	conversationViewLines := strings.Split(conversationView, "\n")

	// Combine titles
	titleRow := lipgloss.JoinHorizontal(lipgloss.Top,
		gameStateTitle,
		strings.Repeat(" ", columnGap),
		logTitle,
	)
	sections = append(sections, titleRow)

	// Combine sidebar and conversation content side by side
	maxLines := len(gameStateSidebarLines)
	if len(conversationViewLines) > maxLines {
		maxLines = len(conversationViewLines)
	}

	for i := 0; i < maxLines; i++ {
		var leftLine, rightLine string

		if i < len(gameStateSidebarLines) {
			leftLine = gameStateSidebarLines[i]
		} else {
			leftLine = strings.Repeat(" ", sidebarWidth)
		}

		if i < len(conversationViewLines) {
			rightLine = conversationViewLines[i]
		} else {
			rightLine = strings.Repeat(" ", conversationWidth)
		}

		combinedLine := lipgloss.JoinHorizontal(lipgloss.Top,
			leftLine,
			strings.Repeat(" ", columnGap),
			rightLine,
		)
		sections = append(sections, combinedLine)
	}

	// Bottom border for conversation log - matches top border style
	logBottomBorder := renderSectionTitle("", conversationWidth)
	sections = append(sections, strings.Repeat(" ", sidebarWidth)+strings.Repeat(" ", columnGap)+logBottomBorder)

	// Footer with scroll hints, verbose toggle, and error status
	verboseHint := ""
	if verbose {
		verboseHint = "  ·  [ v ] VERBOSE: ON"
	} else {
		verboseHint = "  ·  [ v ] VERBOSE: OFF"
	}
	hintText := fmt.Sprintf("[ ESC ] BACK  ·  [ m ] MESSAGE  ·  [ ↑↓ ] SCROLL  ·  [ G ] BOTTOM%s", verboseHint)
	hint := renderHintWithError(hintText, err, width)
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderToolCallsCompact formats tool calls in compact format.
// Format: ⚙ tool_name(args...) - truncated, no huge JSON
func renderToolCallsCompact(content string, contentWidth int) []string {
	// Parse tool calls from storage format
	stored := strings.TrimPrefix(content, constants.ToolCallStoragePrefix)
	if stored == "" {
		return []string{dimmedStyle.Render("⚠ Empty tool call")}
	}

	var result []string
	toolIconStyle := lipgloss.NewStyle().Foreground(colorTool)
	toolNameStyle := lipgloss.NewStyle().Foreground(colorTool).Bold(true)
	toolArgStyle := lipgloss.NewStyle().Foreground(colorMuted)

	// Parse each tool call record
	parts := strings.Split(stored, constants.ToolCallStorageRecordDelimiter)
	for _, part := range parts {
		fields := strings.SplitN(part, constants.ToolCallStorageFieldDelimiter, constants.ToolCallStorageFieldCount)
		if len(fields) < constants.ToolCallStorageFieldCount {
			continue
		}

		toolName := fields[1]
		argsJSON := fields[2]

		// Format: ⚙ tool_name(args...)
		var argsDisplay string
		if argsJSON == "{}" {
			argsDisplay = "()"
		} else {
			// Parse JSON and create compact inline format
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(argsJSON), &args); err == nil && len(args) > 0 {
				// Sort keys for deterministic output
				var keys []string
				for k := range args {
					keys = append(keys, k)
				}
				// Simple lexicographic sort
				for i := 0; i < len(keys); i++ {
					for j := i + 1; j < len(keys); j++ {
						if keys[i] > keys[j] {
							keys[i], keys[j] = keys[j], keys[i]
						}
					}
				}

				var argParts []string
				for _, k := range keys {
					v := args[k]
					// Format value based on type
					var valStr string
					switch v := v.(type) {
					case string:
						// Truncate long strings
						if len(v) > 20 {
							valStr = fmt.Sprintf("%q", v[:20]+"...")
						} else {
							valStr = fmt.Sprintf("%q", v)
						}
					case float64, int, bool:
						valStr = fmt.Sprintf("%v", v)
					default:
						// Complex types: use ellipsis
						valStr = "{...}"
					}
					argParts = append(argParts, fmt.Sprintf("%s=%s", k, valStr))
				}
				argsDisplay = "(" + strings.Join(argParts, ", ") + ")"
			} else {
				argsDisplay = "(...)"
			}
		}

		// Truncate if too long
		toolLine := toolIconStyle.Render("⚙") + " " + toolNameStyle.Render(toolName) + toolArgStyle.Render(argsDisplay)
		if lipgloss.Width(toolLine) > contentWidth {
			// Truncate args
			maxArgsWidth := contentWidth - lipgloss.Width(toolIconStyle.Render("⚙")+" "+toolName) - 5
			if maxArgsWidth < 5 {
				maxArgsWidth = 5
			}
			argsDisplay = truncateWithEllipsis(argsDisplay, maxArgsWidth)
			toolLine = toolIconStyle.Render("⚙") + " " + toolNameStyle.Render(toolName) + toolArgStyle.Render(argsDisplay)
		}

		result = append(result, toolLine)
	}

	return result
}

// renderToolResultCompact formats tool results compactly.
// Success: just ✓ Success
// Error: ✗ Error: message
func renderToolResultCompact(content string, contentWidth int, verbose bool) []string {
	// Strip tool call ID prefix if present (format: "call_xxx:json")
	jsonContent := content
	if idx := strings.Index(jsonContent, ":"); idx > 0 && strings.HasPrefix(jsonContent, "call_") {
		jsonContent = jsonContent[idx+1:]
	}

	successStyle := lipgloss.NewStyle().Foreground(colorSuccess)
	errorStyle := lipgloss.NewStyle().Foreground(colorError)

	// Check if it's an error
	if strings.Contains(jsonContent, "\"error\"") || strings.Contains(jsonContent, "\"isError\"") {
		// Parse error message
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonContent), &data); err == nil {
			if errMsg, ok := data["error"].(string); ok {
				// Show error message
				errorLine := errorStyle.Render("✗") + " " + errorStyle.Render("Error: "+errMsg)
				if verbose {
					// In verbose mode, show full error
					return wrapText(errorLine, contentWidth)
				}
				// Truncate error message
				if lipgloss.Width(errorLine) > contentWidth {
					maxErrWidth := contentWidth - 10
					if maxErrWidth < 10 {
						maxErrWidth = 10
					}
					errMsg = truncateWithEllipsis(errMsg, maxErrWidth)
					errorLine = errorStyle.Render("✗") + " " + errorStyle.Render("Error: "+errMsg)
				}
				return []string{errorLine}
			}
		}
		// Fallback: generic error
		return []string{errorStyle.Render("✗ Error")}
	}

	// Success: just show checkmark
	return []string{successStyle.Render("✓ Success")}
}

func renderLogEntryImpl(entry LogEntry, maxWidth int, verbose bool, currentTick int64) []string {
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

	timePrefix := "T0 ⬡ [--:--]"
	if !entry.Timestamp.IsZero() {
		timePrefix = formatTickTimestamp(currentTick, entry.Timestamp)
	}

	prefix := fmt.Sprintf("%s %s", timePrefix, rolePrefix)
	// Update prefix style with final role color
	prefixStyle = prefixStyle.Foreground(roleColor)

	// Inside padding: 1 space on left and right of all content
	const padLeft = 1
	const padRight = 1
	const contentIndent = 4 // 4 spaces indent for content below header

	// Calculate content width (accounting for indent and padding)
	contentWidth := maxWidth - contentIndent - padLeft - padRight
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Detect special content types and render appropriately
	var wrappedLines []string

	// Check for tool calls (assistant messages with [TOOL_CALLS] prefix)
	if entry.Role == "assistant" && strings.HasPrefix(entry.Content, constants.ToolCallStoragePrefix) {
		// Render tool calls with compact format
		wrappedLines = renderToolCallsCompact(entry.Content, contentWidth)
	} else if entry.Role == "tool" {
		// Tool results: only show errors, success is just a checkmark
		wrappedLines = renderToolResultCompact(entry.Content, contentWidth, verbose)
	} else {
		// Normal text wrapping (user messages, assistant replies)
		wrappedLines = wrapText(entry.Content, contentWidth)
	}

	var result []string

	// Add top padding (empty line with background)
	emptyLine := contentStyle.Width(maxWidth).Render("")
	result = append(result, emptyLine)

	// Render header line: timestamp + role + horizontal separator
	leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
	styledPrefix := prefixStyle.Render(prefix)

	// Calculate remaining width for separator after prefix
	prefixDisplayWidth := lipgloss.Width(prefix)
	separatorWidth := maxWidth - padLeft - prefixDisplayWidth - 1 - padRight // -1 for space after prefix
	if separatorWidth < 0 {
		separatorWidth = 0
	}

	// Create dimmed horizontal separator
	dimmedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray
	separator := strings.Repeat("─", separatorWidth)
	styledSeparator := dimmedStyle.Render(" " + separator)

	// Pad to full width
	headerPadding := strings.Repeat(" ", padRight)
	headerLine := leftPad + styledPrefix + styledSeparator + headerPadding
	result = append(result, headerLine)

	// Render content lines (4 spaces indented below header)
	for _, line := range wrappedLines {
		// Pad the line content to fill remaining width
		lineLen := lipgloss.Width(line)
		remainingWidth := maxWidth - padLeft - contentIndent - lineLen - padRight
		if remainingWidth < 0 {
			remainingWidth = 0
		}
		paddedLine := line + strings.Repeat(" ", remainingWidth)

		// All content lines: left pad + 4 space indent + content
		contentLeftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
		indent := contentStyle.Render(strings.Repeat(" ", contentIndent))
		styledContent := contentStyle.Render(paddedLine)
		rightPad := contentStyle.Render(strings.Repeat(" ", padRight))
		result = append(result, contentLeftPad+indent+styledContent+rightPad)
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

		// Format reasoning header with timestamp (same format as main message)
		reasoningLabel := "REASONING:"
		reasoningHeader := fmt.Sprintf("%s %s", timePrefix, reasoningLabel)
		reasoningHeaderWidth := lipgloss.Width(reasoningHeader) + 1 // +1 for space after
		reasoningContentWidth := maxWidth - reasoningHeaderWidth - padLeft - padRight
		if reasoningContentWidth < 20 {
			reasoningContentWidth = 20
		}

		// Render header line with separator (matching main message pattern)
		leftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
		styledReasoningHeader := reasoningHeaderStyle.Render(reasoningHeader)
		reasoningPrefixDisplayWidth := lipgloss.Width(reasoningHeader)
		reasoningSeparatorWidth := maxWidth - padLeft - reasoningPrefixDisplayWidth - 1 - padRight // -1 for space after prefix
		if reasoningSeparatorWidth < 0 {
			reasoningSeparatorWidth = 0
		}
		reasoningSeparator := strings.Repeat("─", reasoningSeparatorWidth)
		styledReasoningSeparator := dimmedStyle.Render(" " + reasoningSeparator)
		reasoningHeaderPadding := strings.Repeat(" ", padRight)
		reasoningHeaderLine := leftPad + styledReasoningHeader + styledReasoningSeparator + reasoningHeaderPadding
		result = append(result, reasoningHeaderLine)

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
				truncPaddedLine := truncMsg + strings.Repeat(" ", truncRemainingWidth)

				contentLeftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
				indent := contentStyle.Render(strings.Repeat(" ", contentIndent))
				styledTrunc := dimmedStyle.Render(truncPaddedLine)
				rightPad := contentStyle.Render(strings.Repeat(" ", padRight))
				result = append(result, contentLeftPad+indent+styledTrunc+rightPad)
			}

			// All lines: left pad + 4 space indent + content (matches main message rendering)
			contentLeftPad := contentStyle.Render(strings.Repeat(" ", padLeft))
			indent := contentStyle.Render(strings.Repeat(" ", contentIndent))
			styledContent := reasoningStyle.Render(paddedLine)
			rightPad := contentStyle.Render(strings.Repeat(" ", padRight))
			result = append(result, contentLeftPad+indent+styledContent+rightPad)
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

// renderFocusHeader renders the focus view header spanning full width (2 lines).
// Line 1: Mysis name and position
// Line 2: ID and Created timestamp
func renderFocusHeader(mysis MysisInfo, focusIndex, totalMyses int, width int, spinnerView string) string {
	// Line 1: Mysis name with position
	countText := ""
	if totalMyses > 0 && focusIndex > 0 {
		countText = fmt.Sprintf(" (%d/%d)", focusIndex, totalMyses)
	}
	titleText := " ⬡ MYSIS: " + mysis.Name + countText + " ⬡ "
	titleDisplayWidth := lipgloss.Width(titleText)
	// Total fixed chars: space (1) + ⬥ (1) on left, ⬥ (1) on right = 3
	availableWidth := width - titleDisplayWidth - 3
	if availableWidth < 4 {
		availableWidth = 4
	}
	leftDashes := availableWidth / 2
	rightDashes := availableWidth - leftDashes

	line1 := " ⬥" + strings.Repeat("─", leftDashes) + titleText + strings.Repeat("─", rightDashes) + "⬥"

	// Line 2: ID and Created (or error)
	var line2 string
	if mysis.State == "errored" && mysis.LastError != "" {
		// Show simplified error with animated red icon
		errorIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render(spinnerView)
		line2 = "  " + labelStyle.Render("ERROR") + " " + errorIcon
	} else {
		// Show ID and Created
		idText := fmt.Sprintf("%s %s", labelStyle.Render("ID:"), dimmedStyle.Render(mysis.ID))
		createdText := dimmedStyle.Render("(unknown)")
		if !mysis.CreatedAt.IsZero() {
			createdText = dimmedStyle.Render(mysis.CreatedAt.Local().Format("2006-01-02 15:04"))
		}
		createdDisplay := fmt.Sprintf("%s %s", labelStyle.Render("Created:"), createdText)
		line2 = "  " + idText + "    " + createdDisplay
	}

	// Combine both lines
	header := line1 + "\n" + line2
	return headerStyle.Width(width).Render(header)
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

// renderGameStateSidebar renders the game state sidebar showing all snapshots.
func renderGameStateSidebar(snapshots []*store.GameStateSnapshot, currentTick int64, width int) []string {
	var lines []string

	if len(snapshots) == 0 {
		lines = append(lines, dimmedStyle.Render("No game state"))
		lines = append(lines, dimmedStyle.Render("available"))
		return lines
	}

	// Group snapshots by category (same as buildGameStateSummary)
	statusTools := []string{"get_status", "get_player"}
	shipTools := []string{"get_ship", "get_cargo"}
	locationTools := []string{"get_system", "get_location", "get_poi"}

	// Helper to render a snapshot group
	renderGroup := func(toolNames []string) {
		for _, snapshot := range snapshots {
			for _, toolName := range toolNames {
				if snapshot.ToolName == toolName {
					// Tool name with recency
					recency := snapshot.RecencyMessage(currentTick)
					toolHeader := labelStyle.Render(toolName)
					recencyText := dimmedStyle.Render(fmt.Sprintf(" (%s)", recency))
					lines = append(lines, toolHeader+recencyText)

					// Extract and show compact info
					compactInfo := extractCompactInfoForTUI(snapshot.Content, toolName, width-4)
					lines = append(lines, compactInfo...)
					lines = append(lines, "") // Empty line between groups
				}
			}
		}
	}

	renderGroup(statusTools)
	renderGroup(shipTools)
	renderGroup(locationTools)

	// Remove trailing empty line if present
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

// extractCompactInfoForTUI extracts key info from snapshot for TUI display.
func extractCompactInfoForTUI(content, toolName string, maxWidth int) []string {
	var lines []string

	// Parse JSON content
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return lines
	}

	switch toolName {
	case "get_status", "get_player":
		// Extract player data
		if player, ok := data["player"].(map[string]interface{}); ok {
			if credits, ok := player["credits"].(float64); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Credits: %d", int(credits))))
			}
			if location, ok := player["current_system"].(string); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  System: %s", truncateWithEllipsis(location, maxWidth-10))))
			}
		}
		// Extract ship data from get_status
		if ship, ok := data["ship"].(map[string]interface{}); ok {
			if name, ok := ship["name"].(string); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Ship: %s", truncateWithEllipsis(name, maxWidth-8))))
			}
			if fuel, ok := ship["fuel"].(float64); ok {
				if maxFuel, ok := ship["max_fuel"].(float64); ok {
					lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Fuel: %d/%d", int(fuel), int(maxFuel))))
				}
			}
			if hull, ok := ship["hull"].(float64); ok {
				if maxHull, ok := ship["max_hull"].(float64); ok {
					lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Hull: %d/%d", int(hull), int(maxHull))))
				}
			}
		}

	case "get_ship":
		if cargoUsed, ok := data["cargo_used"].(float64); ok {
			if cargoMax, ok := data["cargo_max"].(float64); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Cargo: %d/%d", int(cargoUsed), int(cargoMax))))
			}
		}
		if class, ok := data["class"].(map[string]interface{}); ok {
			if className, ok := class["name"].(string); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Class: %s", truncateWithEllipsis(className, maxWidth-9))))
			}
		}

	case "get_cargo":
		if cargo, ok := data["cargo"].([]interface{}); ok {
			cargoUsed := 0
			for _, item := range cargo {
				if itemMap, ok := item.(map[string]interface{}); ok {
					name, _ := itemMap["name"].(string)
					qty, _ := itemMap["quantity"].(float64)
					if name != "" {
						cargoUsed += int(qty)
						lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  • %s x%d", truncateWithEllipsis(name, maxWidth-10), int(qty))))
					}
				}
			}
			if capacity, ok := data["capacity"].(float64); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Total: %d/%d", cargoUsed, int(capacity))))
			}
		}

	case "get_system":
		if pois, ok := data["pois"].([]interface{}); ok {
			poiCount := len(pois)
			lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  POIs: %d", poiCount)))
		}

	case "get_poi":
		if poi, ok := data["poi"].(map[string]interface{}); ok {
			if name, ok := poi["name"].(string); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  %s", truncateWithEllipsis(name, maxWidth-2))))
			}
			if poiType, ok := poi["type"].(string); ok {
				lines = append(lines, dimmedStyle.Render(fmt.Sprintf("  Type: %s", poiType)))
			}
		}
	}

	return lines
}
