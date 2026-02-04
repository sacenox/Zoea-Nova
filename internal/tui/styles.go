package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors - Retro 80s/90s CRT aesthetic
var (
	// Primary colors
	colorPrimary   = lipgloss.Color("#00FF00") // Classic green terminal
	colorSecondary = lipgloss.Color("#00AAAA") // Cyan
	colorAccent    = lipgloss.Color("#FFAA00") // Amber/Orange
	colorWarning   = lipgloss.Color("#FF5500") // Warning orange
	colorError     = lipgloss.Color("#FF0000") // Error red
	colorMuted     = lipgloss.Color("#666666") // Muted gray

	// Background
	colorBg     = lipgloss.Color("#0A0A0A") // Near black
	colorBgAlt  = lipgloss.Color("#1A1A1A") // Slightly lighter
	colorBorder = lipgloss.Color("#333333") // Border gray
)

// Styles
var (
	// Base styles
	baseStyle = lipgloss.NewStyle().
			Background(colorBg)

	// Header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Background(colorBgAlt).
			Padding(0, 1).
			MarginBottom(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Background(colorBgAlt).
			Padding(0, 1)

	// Agent list
	agentListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	agentItemStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Padding(0, 1)

	agentItemSelectedStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorPrimary).
				Bold(true).
				Padding(0, 1)

	// Agent states
	stateRunningStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	stateIdleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary)

	stateStoppedStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	stateErroredStyle = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)

	// Logs/Messages
	logStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	logUserStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	logAssistantStyle = lipgloss.NewStyle().
				Foreground(colorPrimary)

	logSystemStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// Input
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSecondary).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	// Help
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorSecondary).
			Padding(1, 2).
			Margin(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Panels
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true).
			Padding(0, 1)

	// Info/Stats
	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Dimmed/Disabled
	dimmedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

// StateStyle returns the appropriate style for an agent state.
func StateStyle(state string) lipgloss.Style {
	switch state {
	case "running":
		return stateRunningStyle
	case "idle":
		return stateIdleStyle
	case "stopped":
		return stateStoppedStyle
	case "errored":
		return stateErroredStyle
	default:
		return stateIdleStyle
	}
}

// RoleStyle returns the appropriate style for a message role.
func RoleStyle(role string) lipgloss.Style {
	switch role {
	case "user":
		return logUserStyle
	case "assistant":
		return logAssistantStyle
	case "system":
		return logSystemStyle
	default:
		return logSystemStyle
	}
}
