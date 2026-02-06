package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Colors - Retro-futuristic aesthetic based on Zoea Nova brand
// Brand colors from logo: #9D00FF (electric purple), #00FFCC (bright teal)
var (
	// Brand colors
	colorBrand    = lipgloss.Color("#9D00FF") // Electric purple (from logo)
	colorTeal     = lipgloss.Color("#00FFCC") // Bright teal (from logo)
	colorBrandDim = lipgloss.Color("#6B00B3") // Dimmed purple for subtle accents
	colorTealDim  = lipgloss.Color("#00AA99") // Dimmed teal

	// Role colors (per design doc: user=green, assistant=magenta, system=cyan, tool=yellow)
	colorUser      = lipgloss.Color("#00FF66") // Bright green for user messages
	colorAssistant = lipgloss.Color("#FF00CC") // Magenta/pink for assistant (complements brand purple)
	colorSystem    = lipgloss.Color("#00CCFF") // Cyan for system messages
	colorTool      = lipgloss.Color("#FFCC00") // Yellow/gold for tool calls

	// Semantic colors
	colorWarning = lipgloss.Color("#FF6600") // Warning orange
	colorError   = lipgloss.Color("#FF3366") // Error red-pink
	colorSuccess = lipgloss.Color("#00FF66") // Success green
	colorMuted   = lipgloss.Color("#5555AA") // Muted purple-gray

	// Backgrounds - deep space with purple undertones
	colorBg       = lipgloss.Color("#08080F") // Deep space black
	colorBgAlt    = lipgloss.Color("#101018") // Slightly lighter
	colorBgPanel  = lipgloss.Color("#14141F") // Panel background
	colorBorder   = lipgloss.Color("#2A2A55") // Purple-tinted border
	colorBorderHi = lipgloss.Color("#4040AA") // Highlighted border

	// Legacy aliases for compatibility
	colorPrimary   = colorBrand
	colorSecondary = colorTeal
	colorAccent    = colorTeal
)

// Styles - Retro-futuristic command center aesthetic
var (
	// Base styles
	baseStyle = lipgloss.NewStyle().
			Background(colorBg)

	// Header - Bold brand presence, no border (decorative lines in content)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand).
			Background(colorBgAlt).
			MarginBottom(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBrand)

	// Mysis list - double border for that 80s terminal aesthetic
	mysisListStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorBrandDim) // Was: colorBorder (#2A2A55, contrast 1.48:1)
		// Now: colorBrandDim (#6B00B3, contrast ~3.0:1)

	mysisItemStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Padding(0, 1)

	mysisItemSelectedStyle = lipgloss.NewStyle().
				Foreground(colorBg).
				Background(colorBrand).
				Bold(true).
				Padding(0, 1)

	// Mysis states - distinct colors for each state
	stateRunningStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	stateIdleStyle = lipgloss.NewStyle().
			Foreground(colorTeal)

	stateStoppedStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	stateErroredStyle = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)

	// Logs/Messages - conversation styling per design doc
	// No padding here - padding is added to content lines directly
	logStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBrandDim) // Was: colorBorder (#2A2A55, contrast 1.48:1)
		// Now: colorBrandDim (#6B00B3, contrast ~3.0:1)

	// Role colors per design doc: user=green, assistant=magenta, system=cyan, tool=yellow
	logUserStyle = lipgloss.NewStyle().
			Foreground(colorUser).
			Bold(true)

	logAssistantStyle = lipgloss.NewStyle().
				Foreground(colorAssistant)

	logSystemStyle = lipgloss.NewStyle().
			Foreground(colorSystem).
			Italic(true)

	logToolStyle = lipgloss.NewStyle().
			Foreground(colorTool).
			Italic(true)

	// Input - teal border, brand prompt
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorTeal).
			Padding(0, 1)

	inputPromptStyle = lipgloss.NewStyle().
				Foreground(colorBrand).
				Bold(true)

	// Help - double border for prominence, centered brand feel
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorBrand).
			Background(colorBgPanel).
			Padding(1, 2).
			Margin(1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Panels - consistent with mysis list
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorBgPanel).
			Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	// Info/Stats - brand colors for values
	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorBrand).
			Bold(true)

	// Dimmed/Disabled
	dimmedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Highlight style for important info
	highlightStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)
)

// StateStyle returns the appropriate style for a mysis state.
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
	case "tool":
		return logToolStyle
	default:
		return logSystemStyle
	}
}

// RoleColor returns the foreground color for a message role.
func RoleColor(role string) lipgloss.Color {
	switch role {
	case "user":
		return colorUser
	case "assistant":
		return colorAssistant
	case "system":
		return colorSystem
	case "tool":
		return colorTool
	default:
		return colorSystem
	}
}

// renderSectionTitle renders a section title that spans the full width.
func renderSectionTitle(title string, width int) string {
	return renderSectionTitleWithSuffix(title, "", width)
}

// renderSectionTitleWithSuffix renders a section title with an optional suffix (like scroll indicator).
func renderSectionTitleWithSuffix(title, suffix string, width int) string {
	// Format: ⬧── TITLE ──⬧ [suffix] with dashes filling the remaining space
	// Account for: 2 (⬧ markers) + 2 (─ adjacent to markers) + title + 2 (spaces around title) = 4
	titleWithSpaces := " " + title + " "
	titleDisplayWidth := lipgloss.Width(titleWithSpaces)
	suffixDisplayWidth := lipgloss.Width(suffix)
	// Total fixed chars: ⬧─ (2) on each side = 4
	availableWidth := width - titleDisplayWidth - 4 - suffixDisplayWidth
	if availableWidth < 2 {
		availableWidth = 2
	}
	leftDashes := availableWidth / 2
	rightDashes := availableWidth - leftDashes

	line := "⬧─" + strings.Repeat("─", leftDashes) + titleWithSpaces + strings.Repeat("─", rightDashes) + "─⬧"
	if suffix != "" {
		line += suffix
	}
	return panelTitleStyle.Width(width).Render(line)
}

// truncateToWidth truncates a string to fit within maxWidth display columns.
// Uses rune-aware iteration to avoid cutting multi-byte characters.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	currentWidth := 0
	for i, r := range s {
		charWidth := lipgloss.Width(string(r))
		if currentWidth+charWidth > maxWidth {
			return s[:i]
		}
		currentWidth += charWidth
	}
	return s
}

func formatSenderLabel(senderID, senderName string) string {
	const maxLabelWidth = 12
	if senderName != "" {
		return truncateWithEllipsis(senderName, maxLabelWidth)
	}
	if senderID == "" {
		return ""
	}
	prefix := "id:"
	maxIDWidth := maxLabelWidth - lipgloss.Width(prefix)
	if maxIDWidth <= 0 {
		return prefix
	}
	return prefix + truncateWithEllipsis(senderID, maxIDWidth)
}

func truncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return truncateToWidth(s, maxWidth)
	}
	return truncateToWidth(s, maxWidth-3) + "..."
}

// formatTickTimestamp formats a tick number and timestamp as "T#### ⬡ [HH:MM]" with colors.
// The timestamp is converted to local time.
// Colors: tick (teal), hexagon (brand purple), brackets+time (dimmed)
func formatTickTimestamp(tick int64, ts time.Time) string {
	timeStr := ts.Local().Format("15:04")

	// Style components
	tickStyle := lipgloss.NewStyle().Foreground(colorSecondary) // Teal
	hexStyle := lipgloss.NewStyle().Foreground(colorBrand)      // Brand purple
	timeStyle := lipgloss.NewStyle().Foreground(colorMuted)     // Dimmed

	tickPart := tickStyle.Render(fmt.Sprintf("T%d", tick))
	hexPart := hexStyle.Render("⬡")
	timePart := timeStyle.Render(fmt.Sprintf("[%s]", timeStr))

	return tickPart + " " + hexPart + " " + timePart
}
