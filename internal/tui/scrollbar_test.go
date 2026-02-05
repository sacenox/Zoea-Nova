package tui

import (
	"strings"
	"testing"
)

func TestRenderScrollbar_AtTop(t *testing.T) {
	height := 10
	totalLines := 100
	scrollOffset := 0

	bar := renderScrollbar(height, totalLines, scrollOffset)

	// Should be height lines
	lines := strings.Split(bar, "\n")
	if len(lines) != height {
		t.Errorf("Expected %d lines, got %d", height, len(lines))
	}

	// First line should have thumb indicator
	if !strings.Contains(lines[0], "█") && !strings.Contains(lines[0], "▓") {
		t.Error("Expected thumb indicator in first line when at top")
	}
}

func TestRenderScrollbar_AtBottom(t *testing.T) {
	height := 10
	totalLines := 100
	scrollOffset := 90 // At bottom (90 + 10 = 100)

	bar := renderScrollbar(height, totalLines, scrollOffset)

	lines := strings.Split(bar, "\n")
	if len(lines) != height {
		t.Errorf("Expected %d lines, got %d", height, len(lines))
	}

	// Last line should have thumb indicator
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "█") && !strings.Contains(lastLine, "▓") {
		t.Error("Expected thumb indicator in last line when at bottom")
	}
}

func TestRenderScrollbar_Middle(t *testing.T) {
	height := 10
	totalLines := 100
	scrollOffset := 45 // Roughly middle

	bar := renderScrollbar(height, totalLines, scrollOffset)

	lines := strings.Split(bar, "\n")

	// Thumb should be somewhere in the middle lines (not first or last)
	hasThumbInMiddle := false
	for i := 2; i < len(lines)-2; i++ {
		if strings.Contains(lines[i], "█") || strings.Contains(lines[i], "▓") {
			hasThumbInMiddle = true
			break
		}
	}

	if !hasThumbInMiddle {
		t.Error("Expected thumb indicator in middle lines when scrolled to middle")
	}
}

func TestRenderScrollbar_NoScroll(t *testing.T) {
	height := 10
	totalLines := 5 // Content fits in viewport
	scrollOffset := 0

	bar := renderScrollbar(height, totalLines, scrollOffset)

	lines := strings.Split(bar, "\n")

	// All lines should show track (no thumb needed when everything fits)
	for i, line := range lines {
		if !strings.Contains(line, "│") && !strings.Contains(line, "║") {
			t.Errorf("Expected track character in line %d when no scroll needed", i)
		}
	}
}
