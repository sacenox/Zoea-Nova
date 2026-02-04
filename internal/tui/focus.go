package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/store"
)

// LogEntry represents a log entry for display.
type LogEntry struct {
	Role    string
	Content string
}

// wrapText wraps text to fit within maxWidth, preserving words.
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

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) <= maxWidth {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		lines = append(lines, currentLine)
	}

	return lines
}

// RenderFocusView renders the detailed agent view (legacy, without viewport).
func RenderFocusView(agent AgentInfo, logs []LogEntry, width, height int, isLoading bool, spinnerView string) string {
	var sections []string

	// Header with agent name
	headerText := fmt.Sprintf("═══ AGENT: %s ═══", agent.Name)
	header := headerStyle.Width(width).Render(headerText)
	sections = append(sections, header)

	// Agent info panel
	stateDisplay := StateStyle(agent.State).Render(agent.State)
	if isLoading {
		stateDisplay += " " + spinnerView + " thinking..."
	}

	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("ID:"), valueStyle.Render(agent.ID)),
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), stateDisplay),
		fmt.Sprintf("%s %s", labelStyle.Render("Provider:"), valueStyle.Render(agent.Provider)),
	}
	infoContent := strings.Join(infoLines, "  ")
	infoPanel := panelStyle.Width(width - 4).Render(infoContent)
	sections = append(sections, infoPanel)

	// Logs panel
	logTitle := panelTitleStyle.Render("── Conversation ──")
	sections = append(sections, logTitle)

	// Calculate available height for logs
	usedHeight := lipgloss.Height(strings.Join(sections, "\n")) + 4 // +4 for footer
	logHeight := height - usedHeight - 2
	if logHeight < 5 {
		logHeight = 5
	}

	var logLines []string
	if len(logs) == 0 {
		logLines = append(logLines, dimmedStyle.Render("No conversation history."))
	} else {
		// Render all log entries
		for _, entry := range logs {
			entryLines := renderLogEntry(entry, width-6)
			logLines = append(logLines, entryLines...)
		}

		// Show most recent lines that fit
		if len(logLines) > logHeight {
			logLines = logLines[len(logLines)-logHeight:]
		}
	}

	logContent := strings.Join(logLines, "\n")
	logPanel := logStyle.Width(width - 4).Height(logHeight).Render(logContent)
	sections = append(sections, logPanel)

	// Footer
	hint := dimmedStyle.Render("Esc: back | m: message | r: relaunch | s: stop | c: configure")
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// RenderFocusViewWithViewport renders the detailed agent view using a scrollable viewport.
func RenderFocusViewWithViewport(agent AgentInfo, vp viewport.Model, width int, isLoading bool, spinnerView string, autoScroll bool) string {
	var sections []string

	// Header with agent name
	headerText := fmt.Sprintf("═══ AGENT: %s ═══", agent.Name)
	header := headerStyle.Width(width).Render(headerText)
	sections = append(sections, header)

	// Agent info panel
	stateDisplay := StateStyle(agent.State).Render(agent.State)
	if isLoading {
		stateDisplay += " " + spinnerView + " thinking..."
	}

	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("ID:"), valueStyle.Render(agent.ID)),
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), stateDisplay),
		fmt.Sprintf("%s %s", labelStyle.Render("Provider:"), valueStyle.Render(agent.Provider)),
	}
	infoContent := strings.Join(infoLines, "  ")
	infoPanel := panelStyle.Width(width - 4).Render(infoContent)
	sections = append(sections, infoPanel)

	// Conversation title with scroll indicator
	scrollInfo := ""
	if !autoScroll {
		scrollInfo = dimmedStyle.Render(" (scrolled)")
	}
	logTitle := panelTitleStyle.Render("── Conversation ──") + scrollInfo
	sections = append(sections, logTitle)

	// Viewport content (scrollable)
	vpView := logStyle.Width(width - 4).Render(vp.View())
	sections = append(sections, vpView)

	// Footer with scroll hints
	hint := dimmedStyle.Render("Esc: back | m: message | ↑↓/PgUp/PgDn: scroll | G/End: bottom")
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderLogEntry(entry LogEntry, maxWidth int) []string {
	roleStyle := RoleStyle(entry.Role)

	var prefix string
	switch entry.Role {
	case "user":
		prefix = "YOU:  "
	case "assistant":
		prefix = "AI:   "
	case "system":
		prefix = "SYS:  "
	case "tool":
		prefix = "TOOL: "
	default:
		prefix = "???:  "
	}

	// Calculate content width (accounting for prefix on first line, indent on rest)
	contentWidth := maxWidth - len(prefix) - 2
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Wrap the content
	wrappedLines := wrapText(entry.Content, contentWidth)

	var result []string
	indent := strings.Repeat(" ", len(prefix))

	for i, line := range wrappedLines {
		if i == 0 {
			result = append(result, roleStyle.Render(prefix)+line)
		} else {
			result = append(result, roleStyle.Render(indent)+line)
		}
	}

	return result
}

// LogEntryFromMemory converts a store.Memory to LogEntry.
func LogEntryFromMemory(m *store.Memory) LogEntry {
	return LogEntry{
		Role:    string(m.Role),
		Content: m.Content,
	}
}
