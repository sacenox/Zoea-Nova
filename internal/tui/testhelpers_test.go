package tui

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
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

// newTestViewport creates a viewport for testing.
func newTestViewport(width, height int) viewport.Model {
	vp := viewport.New(width, height)
	vp.Style = logStyle
	return vp
}

// testMysisID returns a consistent test mysis ID.
func testMysisID(id int) string {
	return fmt.Sprintf("mysis-test-%d", id)
}

// testMysisName returns a consistent test mysis name.
func testMysisName(id int) string {
	return fmt.Sprintf("test-mysis-%d", id)
}

// testTime returns a consistent test timestamp for today at a fixed time.
// This prevents golden file mismatches from timestamps changing.
// Returns time for today at 12:00:00 in local timezone.
func testTime() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
}
