package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/core"
)

// AgentInfo holds display info for an agent.
type AgentInfo struct {
	ID       string
	Name     string
	State    string
	Provider string
}

// RenderDashboard renders the main dashboard view.
func RenderDashboard(agents []AgentInfo, selectedIdx int, width, height int) string {
	var sections []string

	// Header
	header := headerStyle.Width(width).Render("╔═══ ZOEA NOVA COMMAND CENTER ═══╗")
	sections = append(sections, header)

	// Stats bar
	var running, stopped, errored int
	for _, a := range agents {
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
	statsBar := statusBarStyle.Width(width).Render(stats)
	sections = append(sections, statsBar)

	// Agent list
	if len(agents) == 0 {
		emptyMsg := dimmedStyle.Render("No agents. Press 'n' to create one.")
		agentList := agentListStyle.Width(width - 4).Render(emptyMsg)
		sections = append(sections, agentList)
	} else {
		var agentLines []string
		for i, a := range agents {
			line := renderAgentLine(a, i == selectedIdx)
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

func renderAgentLine(a AgentInfo, selected bool) string {
	// State indicator
	var stateIndicator string
	switch a.State {
	case "running":
		stateIndicator = stateRunningStyle.Render("●")
	case "idle":
		stateIndicator = stateIdleStyle.Render("○")
	case "stopped":
		stateIndicator = stateStoppedStyle.Render("◌")
	case "errored":
		stateIndicator = stateErroredStyle.Render("✖")
	default:
		stateIndicator = "?"
	}

	// Build line
	name := a.Name
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	stateText := StateStyle(a.State).Render(fmt.Sprintf("%-8s", a.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", a.Provider))

	line := fmt.Sprintf("%s %-20s %s %s", stateIndicator, name, stateText, provider)

	if selected {
		return agentItemSelectedStyle.Render(line)
	}
	return agentItemStyle.Render(line)
}

// AgentInfoFromCore converts a core.Agent to AgentInfo.
func AgentInfoFromCore(a *core.Agent) AgentInfo {
	return AgentInfo{
		ID:       a.ID(),
		Name:     a.Name(),
		State:    string(a.State()),
		Provider: a.ProviderName(),
	}
}
