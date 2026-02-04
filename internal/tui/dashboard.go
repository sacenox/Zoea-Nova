package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/core"
)

// MysisInfo holds display info for a mysis.
type MysisInfo struct {
	ID          string
	Name        string
	State       string
	Provider    string
	LastMessage string    // Most recent message (user or assistant)
	CreatedAt   time.Time // When mysis was created
}

// SwarmMessageInfo holds display info for a broadcast message.
type SwarmMessageInfo struct {
	Content   string
	CreatedAt time.Time
}

// RenderDashboard renders the main dashboard view.
func RenderDashboard(myses []MysisInfo, swarmMessages []SwarmMessageInfo, selectedIdx int, width, height int, loadingSet map[string]bool, spinnerView string) string {
	var sections []string

	// Header - retro-futuristic command center banner with hexagonal motif (matching logo)
	// Build width-spanning lines (exactly `width` characters)
	if width < 20 {
		width = 20
	}
	topLine := "◆" + strings.Repeat("═", width-2) + "◆"
	titleText := " ⬡ Z O E A   N O V A ⬡   COMMAND CENTER"
	// Center the title and pad to full width - use lipgloss.Width() for Unicode
	titleDisplayWidth := lipgloss.Width(titleText)
	titlePadding := (width - titleDisplayWidth) / 2
	if titlePadding < 0 {
		titlePadding = 0
	}
	titleLine := strings.Repeat(" ", titlePadding) + titleText
	// Pad right side to fill width
	titleLineWidth := lipgloss.Width(titleLine)
	if titleLineWidth < width {
		titleLine += strings.Repeat(" ", width-titleLineWidth)
	}
	bottomLine := "◆" + strings.Repeat("═", width-2) + "◆"

	headerText := topLine + "\n" + titleLine + "\n" + bottomLine
	header := headerStyle.Width(width).Render(headerText)
	sections = append(sections, header)

	// Stats bar
	var running, stopped, errored, loading int
	for _, m := range myses {
		if loadingSet[m.ID] {
			loading++
		}
		switch m.State {
		case "running":
			running++
		case "stopped":
			stopped++
		case "errored":
			errored++
		}
	}
	stats := fmt.Sprintf(
		"%s %d  %s %d  %s %d  %s %d",
		stateRunningStyle.Render("●"),
		running,
		stateIdleStyle.Render("○"),
		len(myses)-running-stopped-errored,
		stateStoppedStyle.Render("◌"),
		stopped,
		stateErroredStyle.Render("✖"),
		errored,
	)
	// Add loading indicator if any myses are loading
	if loading > 0 {
		stats += fmt.Sprintf("  %s %d", spinnerView, loading)
	}
	statsBar := statusBarStyle.Width(width).Render(stats)
	sections = append(sections, statsBar)

	// Swarm message history
	if len(swarmMessages) > 0 {
		swarmHeader := renderSectionTitle("SWARM BROADCAST", width)
		sections = append(sections, swarmHeader)

		var msgLines []string
		for _, msg := range swarmMessages {
			timeStr := msg.CreatedAt.Local().Format("15:04:05")
			content := strings.ReplaceAll(msg.Content, "\n", " ")
			maxLen := width - 15
			if maxLen < 20 {
				maxLen = 20
			}
			if lipgloss.Width(content) > maxLen {
				content = truncateToWidth(content, maxLen-3) + "..."
			}
			line := fmt.Sprintf("%s %s", dimmedStyle.Render(timeStr), content)
			msgLines = append(msgLines, line)
		}
		swarmContent := strings.Join(msgLines, "\n")
		sections = append(sections, swarmContent)
	}

	// Mysis list header
	mysisHeader := renderSectionTitle("MYSIS SWARM", width)
	sections = append(sections, mysisHeader)

	// Calculate height used by other elements to fill remaining space
	// Header: 3 lines + margin, Stats: 1 line, Swarm: header + messages, Mysis header: 1 line, Footer: 1 line
	usedHeight := 6 // header (3 + margin) + stats (1) + mysis header (1) + footer (1)
	if len(swarmMessages) > 0 {
		usedHeight += 1 + len(swarmMessages) // swarm header + messages
	}
	// Account for panel borders (top + bottom = 2 lines)
	usedHeight += 2

	mysisListHeight := height - usedHeight
	if mysisListHeight < 3 {
		mysisListHeight = 3
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
			line := renderMysisLine(m, i == selectedIdx, isLoading, spinnerView, contentWidth)
			mysisLines = append(mysisLines, line)
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

func renderMysisLine(m MysisInfo, selected, isLoading bool, spinnerView string, width int) string {
	// State indicator: animated for running/loading, static for others
	var stateIndicator string
	if isLoading {
		stateIndicator = spinnerView
	} else {
		switch m.State {
		case "running":
			// Animated indicator for running myses
			stateIndicator = spinnerView
		case "idle":
			stateIndicator = stateIdleStyle.Render("○")
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
	if lipgloss.Width(name) > 16 {
		name = truncateToWidth(name, 13) + "..."
	}

	stateText := StateStyle(m.State).Render(fmt.Sprintf("%-8s", m.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", m.Provider))

	// First part: indicator + name + state + provider
	firstPart := fmt.Sprintf("%s %-16s %s %s", stateIndicator, name, stateText, provider)

	// Calculate remaining width for last message
	// Account for the prefix "│ " for the message
	// Use lipgloss.Width() for proper Unicode width calculation
	providerWidth := lipgloss.Width(m.Provider)
	usedWidth := 2 + 16 + 1 + 8 + 1 + providerWidth + 2 + 4 // rough estimate
	msgWidth := width - usedWidth - 8
	if msgWidth < 10 {
		msgWidth = 10
	}

	// Format last message (truncated) - use display width
	var msgPart string
	if m.LastMessage != "" {
		msg := strings.ReplaceAll(m.LastMessage, "\n", " ")
		if lipgloss.Width(msg) > msgWidth {
			msg = truncateToWidth(msg, msgWidth-3) + "..."
		}
		msgPart = dimmedStyle.Render(" │ " + msg)
	}

	line := firstPart + msgPart

	// Apply style with full width to ensure background fills the line
	if selected {
		return mysisItemSelectedStyle.Width(width).Render(line)
	}
	return mysisItemStyle.Width(width).Render(line)
}

// MysisInfoFromCore converts a core.Mysis to MysisInfo.
func MysisInfoFromCore(m *core.Mysis) MysisInfo {
	return MysisInfo{
		ID:        m.ID(),
		Name:      m.Name(),
		State:     string(m.State()),
		Provider:  m.ProviderName(),
		CreatedAt: m.CreatedAt(),
	}
}
