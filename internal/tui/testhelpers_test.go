package tui

import (
	"regexp"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Test constants for consistent terminal dimensions
const (
	TestTerminalWidth  = 120
	TestTerminalHeight = 40
)

// setupGoldenTest forces TrueColor output for consistent golden file generation.
// Returns a cleanup function that should be deferred.
func setupGoldenTest(t *testing.T) func() {
	t.Helper()
	lipgloss.SetColorProfile(termenv.TrueColor)
	return func() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// ansiRegex matches ANSI escape codes (duplicated from tui_test.go for golden tests)
var ansiStripRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSIForGolden removes all ANSI escape codes from a string for stripped golden files.
func stripANSIForGolden(s string) string {
	return ansiStripRegex.ReplaceAllString(s, "")
}
