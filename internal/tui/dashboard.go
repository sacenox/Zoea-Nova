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

	// Header
	header := headerStyle.Width(width).Render("╔═══ ZOEA NOVA COMMAND CENTER ═══╗")
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
		swarmHeader := dimmedStyle.Render("─── Swarm Messages ───")
		sections = append(sections, swarmHeader)

		var msgLines []string
		for _, msg := range swarmMessages {
			timeStr := msg.CreatedAt.Local().Format("15:04:05")
			content := strings.ReplaceAll(msg.Content, "\n", " ")
			maxLen := width - 15
			if maxLen < 20 {
				maxLen = 20
			}
			if len(content) > maxLen {
				content = content[:maxLen-3] + "..."
			}
			line := fmt.Sprintf("%s %s", dimmedStyle.Render(timeStr), content)
			msgLines = append(msgLines, line)
		}
		swarmContent := strings.Join(msgLines, "\n")
		sections = append(sections, swarmContent)
	}

	// Agent list header
	agentHeader := dimmedStyle.Render("─── Agents ───")
	sections = append(sections, agentHeader)

	// Agent list
	if len(agents) == 0 {
		emptyMsg := dimmedStyle.Render("No agents. Press 'n' to create one.")
		agentList := agentListStyle.Width(width - 4).Render(emptyMsg)
		sections = append(sections, agentList)
	} else {
		var agentLines []string
		for i, a := range agents {
			isLoading := loadingSet[a.ID]
			line := renderAgentLine(a, i == selectedIdx, isLoading, spinnerView, width-8)
			agentLines = append(agentLines, line)
		}
		content := strings.Join(agentLines, "\n")
		agentList := agentListStyle.Width(width - 4).Render(content)
		sections = append(sections, agentList)
	}

	// Footer with hint
	hint := dimmedStyle.Render("Press ? for help")
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

	// Build line
	name := a.Name
	if len(name) > 16 {
		name = name[:13] + "..."
	}

	stateText := StateStyle(a.State).Render(fmt.Sprintf("%-8s", a.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", a.Provider))

	// First part: indicator + name + state + provider
	firstPart := fmt.Sprintf("%s %-16s %s %s", stateIndicator, name, stateText, provider)

	// Calculate remaining width for last message
	// Account for the prefix "│ " for the message
	usedWidth := 2 + 16 + 1 + 8 + 1 + len(a.Provider) + 2 + 4 // rough estimate
	msgWidth := width - usedWidth - 8
	if msgWidth < 10 {
		msgWidth = 10
	}

	// Format last message (truncated)
	var msgPart string
	if a.LastMessage != "" {
		msg := strings.ReplaceAll(a.LastMessage, "\n", " ")
		if len(msg) > msgWidth {
			msg = msg[:msgWidth-3] + "..."
		}
		msgPart = dimmedStyle.Render(" │ " + msg)
	}

	line := firstPart + msgPart

	if selected {
		return agentItemSelectedStyle.Render(line)
	}
	return agentItemStyle.Render(line)
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
