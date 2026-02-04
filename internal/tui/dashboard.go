package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/core"
)

// AgentInfo holds display info for an agent.
type AgentInfo struct {
	ID          string
	Name        string
	State       string
	Provider    string
	LastMessage string    // Most recent message (user or assistant)
	CreatedAt   time.Time // When agent was created
}

// SwarmMessageInfo holds display info for a broadcast message.
type SwarmMessageInfo struct {
	Content   string
	CreatedAt time.Time
}

// RenderDashboard renders the main dashboard view.
func RenderDashboard(agents []AgentInfo, swarmMessages []SwarmMessageInfo, selectedIdx int, width, height int, loadingSet map[string]bool, spinnerView string) string {
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
	for _, a := range agents {
		if loadingSet[a.ID] {
			loading++
		}
		switch a.State {
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
		len(agents)-running-stopped-errored,
		stateStoppedStyle.Render("◌"),
		stopped,
		stateErroredStyle.Render("✖"),
		errored,
	)
	// Add loading indicator if any agents are loading
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

	// Agent list header
	agentHeader := renderSectionTitle("AGENT SWARM", width)
	sections = append(sections, agentHeader)

	// Calculate height used by other elements to fill remaining space
	// Header: 3 lines + margin, Stats: 1 line, Swarm: header + messages, Agent header: 1 line, Footer: 1 line
	usedHeight := 6 // header (3 + margin) + stats (1) + agent header (1) + footer (1)
	if len(swarmMessages) > 0 {
		usedHeight += 1 + len(swarmMessages) // swarm header + messages
	}
	// Account for panel borders (top + bottom = 2 lines)
	usedHeight += 2

	agentListHeight := height - usedHeight
	if agentListHeight < 3 {
		agentListHeight = 3
	}

	// Agent list - DoubleBorder adds 2 chars each side, so content width is width-4
	contentWidth := width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	if len(agents) == 0 {
		emptyMsg := dimmedStyle.Render("No agents. Press 'n' to create one.")
		agentList := agentListStyle.Width(width - 2).Height(agentListHeight).Render(emptyMsg)
		sections = append(sections, agentList)
	} else {
		var agentLines []string
		for i, a := range agents {
			isLoading := loadingSet[a.ID]
			line := renderAgentLine(a, i == selectedIdx, isLoading, spinnerView, contentWidth)
			agentLines = append(agentLines, line)
		}
		content := strings.Join(agentLines, "\n")
		agentList := agentListStyle.Width(width - 2).Height(agentListHeight).Render(content)
		sections = append(sections, agentList)
	}

	// Footer with hint
	hint := dimmedStyle.Render("[ ? ] HELP  ·  [ n ] NEW AGENT  ·  [ b ] BROADCAST")
	sections = append(sections, hint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderAgentLine(a AgentInfo, selected, isLoading bool, spinnerView string, width int) string {
	// State indicator: animated for running/loading, static for others
	var stateIndicator string
	if isLoading {
		stateIndicator = spinnerView
	} else {
		switch a.State {
		case "running":
			// Animated indicator for running agents
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
	name := a.Name
	if lipgloss.Width(name) > 16 {
		name = truncateToWidth(name, 13) + "..."
	}

	stateText := StateStyle(a.State).Render(fmt.Sprintf("%-8s", a.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", a.Provider))

	// First part: indicator + name + state + provider
	firstPart := fmt.Sprintf("%s %-16s %s %s", stateIndicator, name, stateText, provider)

	// Calculate remaining width for last message
	// Account for the prefix "│ " for the message
	// Use lipgloss.Width() for proper Unicode width calculation
	providerWidth := lipgloss.Width(a.Provider)
	usedWidth := 2 + 16 + 1 + 8 + 1 + providerWidth + 2 + 4 // rough estimate
	msgWidth := width - usedWidth - 8
	if msgWidth < 10 {
		msgWidth = 10
	}

	// Format last message (truncated) - use display width
	var msgPart string
	if a.LastMessage != "" {
		msg := strings.ReplaceAll(a.LastMessage, "\n", " ")
		if lipgloss.Width(msg) > msgWidth {
			msg = truncateToWidth(msg, msgWidth-3) + "..."
		}
		msgPart = dimmedStyle.Render(" │ " + msg)
	}

	line := firstPart + msgPart

	// Apply style with full width to ensure background fills the line
	if selected {
		return agentItemSelectedStyle.Width(width).Render(line)
	}
	return agentItemStyle.Width(width).Render(line)
}

// AgentInfoFromCore converts a core.Agent to AgentInfo.
func AgentInfoFromCore(a *core.Agent) AgentInfo {
	return AgentInfo{
		ID:        a.ID(),
		Name:      a.Name(),
		State:     string(a.State()),
		Provider:  a.ProviderName(),
		CreatedAt: a.CreatedAt(),
	}
}
