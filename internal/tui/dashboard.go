package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/constants"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/store"
)

const (
	// Mysis list row prefix widths
	cursorDisplayWidth         = 4 // "[→ ]" with backgrounds
	stateIndicatorDisplayWidth = 1 // Single Unicode character
	rowPrefixSpacing           = 3 // Spaces around indicator (1 before + 2 after)
)

// MysisInfo holds display info for a mysis.
type MysisInfo struct {
	ID              string
	Name            string
	State           string
	Activity        string // Current activity (idle, llm_call, mcp_call, traveling, etc.)
	Provider        string
	AccountUsername string          // NEW: game account username
	LastMessage     string          // Most recent message (user or assistant) - DEPRECATED, kept for compatibility
	LastMessageAt   time.Time       // Timestamp of most recent message
	RecentMemories  []*store.Memory // Recent memories for message row formatting
	CreatedAt       time.Time       // When mysis was created
	LastError       string          // Last error string (if errored)
}

// SwarmMessageInfo holds display info for a broadcast message.
type SwarmMessageInfo struct {
	SenderID   string
	SenderName string
	Content    string
	CreatedAt  time.Time
}

// RenderDashboard renders the main dashboard view.
func RenderDashboard(myses []MysisInfo, swarmMessages []SwarmMessageInfo, selectedIdx int, width, height int, loadingSet map[string]bool, spinnerView string, currentTick int64) string {
	var sections []string

	// Header - retro-futuristic command center banner with hexagonal motif (matching logo)
	if width < 20 {
		width = 20
	}

	// Define custom border with empty sides but diamonds in corners
	// We enable all borders to get the corners, but set sides to empty strings
	headerBorder := lipgloss.Border{
		Top:         "═",
		Bottom:      "═",
		Left:        " ",
		Right:       " ",
		TopLeft:     "⬥",
		TopRight:    "⬥",
		BottomLeft:  "⬥",
		BottomRight: "⬥",
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorBrand).
		Background(colorBgAlt).
		Width(width-2). // Subtract 2 for the corner characters
		Align(lipgloss.Center).
		Border(headerBorder, true, true, true, true).
		BorderForeground(colorBrand)

	titleText := "⬡ Z O E A   N O V A ⬡   COMMAND CENTER"
	header := headerStyle.Render(titleText)
	sections = append(sections, header)

	// Swarm message history - always visible with fixed height
	swarmHeader := renderSectionTitle("SWARM BROADCAST", width)
	sections = append(sections, swarmHeader)

	const maxSwarmMessages = 10
	var msgLines []string
	if len(swarmMessages) == 0 {
		// Show placeholder when empty
		msgLines = append(msgLines, dimmedStyle.Render("No broadcasts yet. Press 'b' to broadcast."))
	} else {
		// Show up to maxSwarmMessages (most recent first)
		displayCount := len(swarmMessages)
		if displayCount > maxSwarmMessages {
			displayCount = maxSwarmMessages
		}
		for i := 0; i < displayCount; i++ {
			msg := swarmMessages[i]
			timeStr := formatTickTimestamp(currentTick, msg.CreatedAt)
			senderLabel := formatSenderLabel(msg.SenderID, msg.SenderName)
			senderText := ""
			if senderLabel != "" {
				senderText = " [" + senderLabel + "]"
			}
			content := strings.ReplaceAll(msg.Content, "\n", " ")
			maxLen := width - 15 - lipgloss.Width(senderText)
			if maxLen < 1 {
				maxLen = 1
			}
			if lipgloss.Width(content) > maxLen {
				if maxLen > 3 {
					content = truncateToWidth(content, maxLen-3) + "..."
				} else {
					content = truncateToWidth(content, maxLen)
				}
			}
			line := fmt.Sprintf("%s%s %s", dimmedStyle.Render(timeStr), highlightStyle.Render(senderText), content)
			msgLines = append(msgLines, line)
		}
	}
	swarmContent := strings.Join(msgLines, "\n")
	sections = append(sections, swarmContent)

	// Mysis list header
	mysisHeader := renderSectionTitle("MYSIS SWARM", width)
	sections = append(sections, mysisHeader)

	// Calculate height used by other elements to fill remaining space
	// Header: 3 lines + margin, Swarm: header + content, Mysis header: 1 line, Footer: 1 line
	usedHeight := 5 // header (3 + margin) + mysis header (1) + footer (1)
	// Swarm section: header (1) + content lines (at least 1 for placeholder or messages)
	usedHeight += 1 + len(msgLines)
	// Account for panel borders (top + bottom = 2 lines)
	usedHeight += 2

	mysisListHeight := height - usedHeight
	// Each mysis takes 2 lines (info + message), minimum 4 lines for 2 rows
	if mysisListHeight < 4 {
		mysisListHeight = 4
	}

	// Mysis list - DoubleBorder adds 2 chars each side, so content width is width-4
	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	if len(myses) == 0 {
		emptyMsg := dimmedStyle.Render("No myses. Press 'n' to create one.")
		mysisList := mysisListStyle.Width(width - 2).Height(mysisListHeight).Render(emptyMsg)
		sections = append(sections, mysisList)
	} else {
		var mysisLines []string
		for i, m := range myses {
			isLoading := loadingSet[m.ID]
			lines := renderMysisLine(m, i == selectedIdx, isLoading, spinnerView, contentWidth, currentTick)
			// lines[0] = info row, lines[1] = message row
			mysisLines = append(mysisLines, lines[0])
			mysisLines = append(mysisLines, lines[1])
		}
		content := strings.Join(mysisLines, "\n")
		mysisList := mysisListStyle.Width(width - 2).Height(mysisListHeight).Render(content)
		sections = append(sections, mysisList)
	}

	// Footer with hint
	hint := dimmedStyle.Render("[ ? ] HELP  ·  [ n ] NEW MYSIS  ·  [ b ] BROADCAST")
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderMysisLine(m MysisInfo, selected, isLoading bool, spinnerView string, width int, currentTick int64) []string {
	// Build info row (first line)
	infoRow := buildInfoRow(m, selected, isLoading, spinnerView, width)

	// Build message row (second line)
	messageRow := buildMessageRow(m, width, currentTick)

	return []string{infoRow, messageRow}
}

func buildInfoRow(m MysisInfo, selected, isLoading bool, spinnerView string, width int) string {
	// State indicator: activity-specific for running myses, static for others
	var stateIndicator string
	if isLoading {
		stateIndicator = spinnerView
	} else {
		switch m.State {
		case "running":
			// Show activity-specific indicator for running myses
			switch m.Activity {
			case "llm_call":
				// Purple ellipsis for LLM thinking
				stateIndicator = lipgloss.NewStyle().Foreground(colorBrand).Render("⋯")
			case "mcp_call":
				// Teal gear for MCP tool execution
				stateIndicator = lipgloss.NewStyle().Foreground(colorTeal).Render("⚙")
			case "traveling":
				// Teal arrow for traveling
				stateIndicator = lipgloss.NewStyle().Foreground(colorTeal).Render("→")
			case "mining":
				// Yellow pickaxe for mining
				stateIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Render("⛏")
			case "in_combat":
				// Red crossed swords for combat
				stateIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("⚔")
			case "cooldown":
				// Dimmed hourglass for cooldown
				stateIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("⏳")
			default:
				// Default spinner for idle or unknown activity
				stateIndicator = spinnerView
			}
		case "idle":
			stateIndicator = stateIdleStyle.Render("◦")
		case "stopped":
			stateIndicator = stateStoppedStyle.Render("◌")
		case "errored":
			stateIndicator = stateErroredStyle.Render("✖")
		default:
			stateIndicator = "?"
		}
	}

	// Build line - use display width for truncation
	name := m.Name
	if lipgloss.Width(name) > 8 {
		name = truncateToWidth(name, 5) + "..."
	}

	// Format provider without brackets and pad to 12 chars
	providerFormatted := m.Provider
	if lipgloss.Width(providerFormatted) > 12 {
		providerFormatted = truncateToWidth(providerFormatted, 9) + "..."
	}
	providerFormatted = fmt.Sprintf("%-12s", providerFormatted)
	provider := dimmedStyle.Render(providerFormatted)

	stateText := StateStyle(m.State).Render(fmt.Sprintf("%-8s", m.State))

	// Account username display - fixed width 12 chars
	var accountFormatted string
	if m.AccountUsername != "" {
		accountFormatted = fmt.Sprintf("@%s", m.AccountUsername)
	} else {
		accountFormatted = "logged out"
	}
	// Truncate if longer than 12 chars
	if lipgloss.Width(accountFormatted) > 12 {
		accountFormatted = truncateToWidth(accountFormatted, 9) + "..."
	}
	// Pad to 12 chars
	accountFormatted = fmt.Sprintf("%-12s", accountFormatted)
	accountText := dimmedStyle.Render(accountFormatted)

	// Content part: name + provider + state + account (NO message content)
	// Order: name (8) + space + provider (12) + space + state (8) + space + account (12)
	contentPart := fmt.Sprintf("%-8s %s %s %s", name, provider, stateText, accountText)

	// Calculate prefix width
	// Format: "[→ ] ⠋  " or "[  ] ⠋  " = 4 (bracket) + 1 (space) + 1 (indicator) + 2 (spaces) = 8 chars total
	prefixWidth := cursorDisplayWidth + 1 + stateIndicatorDisplayWidth + 2

	// Content width: width - prefix
	contentStyleWidth := width - prefixWidth

	if selected {
		// Dim purple background, bright purple foreground on [→ ] (4 chars), then normal space
		cursorStyle := lipgloss.NewStyle().Background(colorBrandDim).Foreground(colorBrand)
		cursorBracketOpen := cursorStyle.Render("[")
		cursorArrow := cursorStyle.Render("→")
		cursorSpace := cursorStyle.Render(" ")
		cursorBracketClose := cursorStyle.Render("]")
		cursor := cursorBracketOpen + cursorArrow + cursorSpace + cursorBracketClose
		return cursor + " " + stateIndicator + "  " + mysisItemSelectedStyle.PaddingLeft(0).PaddingRight(1).Width(contentStyleWidth).Render(contentPart)
	}
	// Unselected: [  ] with dim purple brackets, no background
	unselectedBracketStyle := lipgloss.NewStyle().Foreground(colorBrandDim)
	unselectedCursor := unselectedBracketStyle.Render("[") + "  " + unselectedBracketStyle.Render("]")
	return unselectedCursor + " " + stateIndicator + "  " + mysisItemStyle.PaddingLeft(0).PaddingRight(1).Width(contentStyleWidth).Render(contentPart)
}

func buildMessageRow(m MysisInfo, width int, currentTick int64) string {
	prefix := "  └─ " // 2 spaces + corner + dash + space = 5 chars
	prefixWidth := 5

	// Calculate available width for message content
	// Subtract prefix (5) and borders (2)
	availableWidth := width - prefixWidth - 2
	if availableWidth < 10 {
		availableWidth = 10
	}

	// Get message content using existing formatMessageRow
	messageContent := formatMessageRow(m, currentTick, availableWidth)

	if messageContent == "" {
		messageContent = dimmedStyle.Render("(no recent activity)")
	}

	return dimmedStyle.Render(prefix) + messageContent
}

// formatMessageRow formats the message row based on message type and priority.
// Priority order: 1) Errors, 2) AI replies, 3) Tool calls, 4) User messages/broadcasts
func formatMessageRow(m MysisInfo, currentTick int64, maxWidth int) string {
	// Priority 1: Errors (if state is errored and LastError is set)
	if m.State == "errored" && m.LastError != "" {
		return formatErrorMessage(m.LastError, currentTick, m.LastMessageAt, maxWidth)
	}

	// If no recent memories but we have LastMessage (backward compatibility for tests),
	// show it as a simple message
	if len(m.RecentMemories) == 0 {
		if m.LastMessage != "" {
			return formatLegacyMessage(m.LastMessage, currentTick, m.LastMessageAt, maxWidth)
		}
		return ""
	}

	// Priority 2: AI replies (not tool calls)
	for _, mem := range m.RecentMemories {
		if mem.Role == store.MemoryRoleAssistant && !strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			return formatAIReply(mem, currentTick, maxWidth)
		}
	}

	// Priority 3: Tool calls
	for _, mem := range m.RecentMemories {
		if mem.Role == store.MemoryRoleAssistant && strings.HasPrefix(mem.Content, constants.ToolCallStoragePrefix) {
			return formatToolCall(mem, currentTick, maxWidth)
		}
	}

	// Priority 4: User messages/broadcasts
	for _, mem := range m.RecentMemories {
		if mem.Role == store.MemoryRoleUser {
			return formatUserMessage(mem, currentTick, maxWidth)
		}
	}

	return "" // No message to display
}

// formatLegacyMessage formats a simple message (backward compatibility for tests).
func formatLegacyMessage(msg string, currentTick int64, timestamp time.Time, maxWidth int) string {
	prefix := ""
	if !timestamp.IsZero() {
		prefix = formatTickTimestamp(currentTick, timestamp) + " "
	}
	prefixWidth := lipgloss.Width(prefix)
	availableWidth := maxWidth - prefixWidth

	content := strings.ReplaceAll(msg, "\n", " ")
	if lipgloss.Width(content) > availableWidth {
		content = truncateToWidth(content, availableWidth-3) + "..."
	}

	return dimmedStyle.Render(prefix + content)
}

// formatErrorMessage formats error messages.
func formatErrorMessage(errMsg string, currentTick int64, timestamp time.Time, maxWidth int) string {
	prefix := "Error: "
	if !timestamp.IsZero() {
		prefix = formatTickTimestamp(currentTick, timestamp) + " " + prefix
	}
	prefixWidth := lipgloss.Width(prefix)
	availableWidth := maxWidth - prefixWidth

	msg := strings.ReplaceAll(errMsg, "\n", " ")
	if lipgloss.Width(msg) > availableWidth {
		msg = truncateToWidth(msg, availableWidth-3) + "..."
	}

	return dimmedStyle.Render(prefix + msg)
}

// formatAIReply formats AI assistant replies.
func formatAIReply(mem *store.Memory, currentTick int64, maxWidth int) string {
	prefix := formatTickTimestamp(currentTick, mem.CreatedAt) + " [AI] "
	prefixWidth := lipgloss.Width(prefix)
	availableWidth := maxWidth - prefixWidth

	msg := strings.ReplaceAll(mem.Content, "\n", " ")
	if lipgloss.Width(msg) > availableWidth {
		msg = truncateToWidth(msg, availableWidth-3) + "..."
	}

	return dimmedStyle.Render(prefix + msg)
}

// formatToolCall formats tool call messages.
func formatToolCall(mem *store.Memory, currentTick int64, maxWidth int) string {
	// Parse tool call from storage format
	stored := strings.TrimPrefix(mem.Content, constants.ToolCallStoragePrefix)
	if stored == "" {
		return ""
	}

	// Parse first tool call record
	parts := strings.Split(stored, constants.ToolCallStorageRecordDelimiter)
	if len(parts) == 0 {
		return ""
	}

	fields := strings.SplitN(parts[0], constants.ToolCallStorageFieldDelimiter, constants.ToolCallStorageFieldCount)
	if len(fields) < constants.ToolCallStorageFieldCount {
		return ""
	}

	toolName := fields[1]
	argsJSON := fields[2]

	// Format args (remove outer braces, simplify nested structures)
	argsFormatted := formatToolArgs(argsJSON)

	// Build message: timestamp + label + function_name(args)
	timestamp := formatTickTimestamp(currentTick, mem.CreatedAt)
	label := lipgloss.NewStyle().Foreground(colorTool).Render("→ call")
	funcName := lipgloss.NewStyle().Foreground(colorTool).Bold(true).Render(toolName)
	argsStyled := lipgloss.NewStyle().Foreground(colorTool).Render("(" + argsFormatted + ")")

	prefix := timestamp + " " + label + " "
	prefixWidth := lipgloss.Width(prefix)
	funcWidth := lipgloss.Width(toolName) + lipgloss.Width(argsFormatted) + 2 // +2 for parentheses

	availableWidth := maxWidth - prefixWidth
	if funcWidth > availableWidth {
		// Truncate args
		argsAvailable := availableWidth - lipgloss.Width(toolName) - 5 // -5 for "(...)"
		if argsAvailable < 3 {
			argsFormatted = "..."
		} else {
			argsFormatted = truncateToWidth(argsFormatted, argsAvailable-3) + "..."
		}
		argsStyled = lipgloss.NewStyle().Foreground(colorTool).Render("(" + argsFormatted + ")")
	}

	return prefix + funcName + argsStyled
}

// formatToolArgs formats tool arguments for display.
func formatToolArgs(argsJSON string) string {
	if argsJSON == "{}" {
		return ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "..."
	}

	if len(args) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	var keys []string
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := args[k]
		valStr := formatArgValue(v)
		parts = append(parts, k+": "+valStr)
	}

	return strings.Join(parts, ", ")
}

// formatArgValue formats a single argument value.
func formatArgValue(v interface{}) string {
	switch v := v.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case float64, int, bool:
		return fmt.Sprintf("%v", v)
	case map[string]interface{}:
		return "{...}"
	case []interface{}:
		return "[...]"
	default:
		return "{...}"
	}
}

// formatUserMessage formats user messages and broadcasts.
func formatUserMessage(mem *store.Memory, currentTick int64, maxWidth int) string {
	var sourceLabel string
	if mem.Source == store.MemorySourceBroadcast {
		sourceLabel = "[SWARM]"
	} else {
		sourceLabel = "[YOU]"
	}

	prefix := formatTickTimestamp(currentTick, mem.CreatedAt) + " " + sourceLabel + " "
	prefixWidth := lipgloss.Width(prefix)
	availableWidth := maxWidth - prefixWidth

	msg := strings.ReplaceAll(mem.Content, "\n", " ")
	if lipgloss.Width(msg) > availableWidth {
		msg = truncateToWidth(msg, availableWidth-3) + "..."
	}

	return dimmedStyle.Render(prefix + msg)
}

// MysisInfoFromCore converts a core.Mysis to MysisInfo.
func MysisInfoFromCore(m *core.Mysis) MysisInfo {
	return MysisInfo{
		ID:              m.ID(),
		Name:            m.Name(),
		State:           string(m.State()),
		Activity:        string(m.ActivityState()), // NEW: copy activity state
		Provider:        m.ProviderName(),
		AccountUsername: m.CurrentAccountUsername(), // NEW: copy account username
		CreatedAt:       m.CreatedAt(),
		LastError:       formatCoreError(m.LastError()),
	}
}

func formatCoreError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
