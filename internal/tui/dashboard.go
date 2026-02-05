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
	ID              string
	Name            string
	State           string
	Provider        string
	AccountUsername string    // NEW: game account username
	LastMessage     string    // Most recent message (user or assistant)
	CreatedAt       time.Time // When mysis was created
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
	if lipgloss.Width(name) > 16 {
		name = truncateToWidth(name, 13) + "..."
	}

	stateText := StateStyle(m.State).Render(fmt.Sprintf("%-8s", m.State))
	provider := dimmedStyle.Render(fmt.Sprintf("[%s]", m.Provider))

	// Account username display
	var accountText string
	if m.AccountUsername != "" {
		accountText = dimmedStyle.Render(fmt.Sprintf("@%s", m.AccountUsername))
	} else {
		accountText = dimmedStyle.Render("(no account)")
	}

	// Content part: name + state + provider + account (NO indicator - it goes outside)
	contentPart := fmt.Sprintf("%-16s %s %s %s", name, stateText, provider, accountText)

	// Calculate remaining width for last message
	// Account for the prefix "│ " for the message
	// Use lipgloss.Width() for proper Unicode width calculation
	// Format: name(16) + space(1) + state(8) + space(1) + provider + space(1) + account
	providerWidth := lipgloss.Width(m.Provider)
	accountTextWidth := lipgloss.Width(accountText)
	usedWidth := 16 + 1 + 8 + 1 + providerWidth + 2 + 1 + accountTextWidth + 4
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

	line := contentPart + msgPart

	// Apply style with full width to ensure background fills the line
	// Render indicator and space OUTSIDE the styled content so they don't get background color
	// Format: space + indicator + space + [styled content]
	// Remove left padding from style (PaddingLeft(0)) and add right padding only
	if selected {
		return " " + stateIndicator + " " + mysisItemSelectedStyle.PaddingLeft(0).PaddingRight(1).Width(width-3).Render(line)
	}
	return " " + stateIndicator + " " + mysisItemStyle.PaddingLeft(0).PaddingRight(1).Width(width-3).Render(line)
}

// MysisInfoFromCore converts a core.Mysis to MysisInfo.
func MysisInfoFromCore(m *core.Mysis) MysisInfo {
	return MysisInfo{
		ID:              m.ID(),
		Name:            m.Name(),
		State:           string(m.State()),
		Provider:        m.ProviderName(),
		AccountUsername: m.CurrentAccountUsername(), // NEW: copy account username
		CreatedAt:       m.CreatedAt(),
	}
}
