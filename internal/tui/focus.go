package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/store"
)

// LogEntry represents a log entry for display.
type LogEntry struct {
	Role    string
	Content string
}

// RenderFocusView renders the detailed agent view.
func RenderFocusView(agent AgentInfo, logs []LogEntry, width, height int) string {
	var sections []string

	// Header with agent name
	headerText := fmt.Sprintf("═══ AGENT: %s ═══", agent.Name)
	header := headerStyle.Width(width).Render(headerText)
	sections = append(sections, header)

	// Agent info panel
	infoLines := []string{
		fmt.Sprintf("%s %s", labelStyle.Render("ID:"), valueStyle.Render(agent.ID)),
		fmt.Sprintf("%s %s", labelStyle.Render("State:"), StateStyle(agent.State).Render(agent.State)),
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
		// Show most recent logs that fit
		startIdx := 0
		if len(logs) > logHeight {
			startIdx = len(logs) - logHeight
		}

		for _, entry := range logs[startIdx:] {
			line := renderLogEntry(entry, width-6)
			logLines = append(logLines, line)
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

func renderLogEntry(entry LogEntry, maxWidth int) string {
	roleStyle := RoleStyle(entry.Role)

	var prefix string
	switch entry.Role {
	case "user":
		prefix = "YOU: "
	case "assistant":
		prefix = "AI:  "
	case "system":
		prefix = "SYS: "
	default:
		prefix = "???: "
	}

	// Truncate content if too long
	content := entry.Content
	maxContent := maxWidth - len(prefix) - 2
	if maxContent < 10 {
		maxContent = 10
	}
	if len(content) > maxContent {
		content = content[:maxContent-3] + "..."
	}

	// Replace newlines with spaces for single-line display
	content = strings.ReplaceAll(content, "\n", " ")

	return roleStyle.Render(prefix) + content
}

// LogEntryFromMemory converts a store.Memory to LogEntry.
func LogEntryFromMemory(m *store.Memory) LogEntry {
	return LogEntry{
		Role:    string(m.Role),
		Content: m.Content,
	}
}
