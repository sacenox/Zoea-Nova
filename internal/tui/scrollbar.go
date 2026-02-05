package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	scrollbarThumb = "█" // Solid block for thumb
	scrollbarTrack = "│" // Thin vertical line for track
)

// renderScrollbar creates a vertical scrollbar indicator.
// height: viewport height in lines
// totalLines: total content lines
// scrollOffset: current scroll position (line number at top of viewport)
// Returns a multi-line string with one character per line.
func renderScrollbar(height int, totalLines int, scrollOffset int) string {
	if height <= 0 {
		return ""
	}

	// If content fits in viewport, show empty track
	if totalLines <= height {
		track := make([]string, height)
		trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray
		for i := 0; i < height; i++ {
			track[i] = trackStyle.Render(scrollbarTrack)
		}
		return strings.Join(track, "\n")
	}

	// Calculate thumb position and size
	// Thumb size is proportional to viewport/content ratio, minimum 1 line
	thumbSize := (height * height) / totalLines
	if thumbSize < 1 {
		thumbSize = 1
	}
	if thumbSize > height {
		thumbSize = height
	}

	// Calculate thumb position based on scroll offset
	// Position is proportional to scroll position
	scrollRatio := float64(scrollOffset) / float64(totalLines-height)
	if scrollRatio < 0 {
		scrollRatio = 0
	}
	if scrollRatio > 1 {
		scrollRatio = 1
	}

	maxThumbPos := height - thumbSize
	thumbPos := int(scrollRatio * float64(maxThumbPos))

	// Build scrollbar lines
	lines := make([]string, height)
	trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray
	thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Lighter gray

	for i := 0; i < height; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			lines[i] = thumbStyle.Render(scrollbarThumb)
		} else {
			lines[i] = trackStyle.Render(scrollbarTrack)
		}
	}

	return strings.Join(lines, "\n")
}
