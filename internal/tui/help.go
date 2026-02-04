package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpItem struct {
	key  string
	desc string
}

var helpItems = []helpItem{
	{"q / Ctrl+C", "Quit"},
	{"n", "New agent"},
	{"d", "Delete selected agent"},
	{"r", "Relaunch selected agent"},
	{"s", "Stop selected agent"},
	{"b", "Broadcast message to all"},
	{"m", "Message selected agent"},
	{"c", "Configure selected agent"},
	{"Tab / Shift+Tab", "Navigate agents"},
	{"Enter", "Focus selected agent"},
	{"Esc", "Back / Cancel"},
	{"↑ / ↓", "Scroll / Browse history"},
	{"PgUp / PgDn", "Scroll page"},
	{"G / End", "Go to bottom (auto-scroll)"},
	{"?", "Toggle help"},
}

// RenderHelp renders the help overlay.
func RenderHelp(width, height int) string {
	var lines []string
	lines = append(lines, titleStyle.Render("⌨ Keyboard Shortcuts"))
	lines = append(lines, "")

	maxKeyLen := 0
	for _, item := range helpItems {
		if len(item.key) > maxKeyLen {
			maxKeyLen = len(item.key)
		}
	}

	for _, item := range helpItems {
		key := helpKeyStyle.Render(padRight(item.key, maxKeyLen))
		desc := helpDescStyle.Render(item.desc)
		lines = append(lines, key+"  "+desc)
	}

	content := strings.Join(lines, "\n")

	// Center the help box
	box := helpStyle.Render(content)

	// Calculate position to center
	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)

	padLeft := (width - boxWidth) / 2
	padTop := (height - boxHeight) / 2

	if padLeft < 0 {
		padLeft = 0
	}
	if padTop < 0 {
		padTop = 0
	}

	// Create padding
	leftPad := strings.Repeat(" ", padLeft)
	topPad := strings.Repeat("\n", padTop)

	// Add left padding to each line
	boxLines := strings.Split(box, "\n")
	for i, line := range boxLines {
		boxLines[i] = leftPad + line
	}

	return topPad + strings.Join(boxLines, "\n")
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}
