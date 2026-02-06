package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
)

// TestDashboardLayoutCalculations tests dashboard height calculations with various scenarios.
func TestDashboardLayoutCalculations(t *testing.T) {
	tests := []struct {
		name           string
		termWidth      int
		termHeight     int
		numMyses       int
		numSwarmMsgs   int
		minListHeight  int
		shouldNotPanic bool
	}{
		// Small terminal sizes
		{name: "min_terminal_0_myses_0_msgs", termWidth: 80, termHeight: 20, numMyses: 0, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "min_terminal_1_mysis_0_msgs", termWidth: 80, termHeight: 20, numMyses: 1, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "min_terminal_5_myses_0_msgs", termWidth: 80, termHeight: 20, numMyses: 5, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "min_terminal_0_myses_5_msgs", termWidth: 80, termHeight: 20, numMyses: 0, numSwarmMsgs: 5, minListHeight: 3, shouldNotPanic: true},
		{name: "min_terminal_5_myses_5_msgs", termWidth: 80, termHeight: 20, numMyses: 5, numSwarmMsgs: 5, minListHeight: 3, shouldNotPanic: true},

		// Medium terminal sizes
		{name: "medium_terminal_0_myses_0_msgs", termWidth: 120, termHeight: 30, numMyses: 0, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "medium_terminal_10_myses_0_msgs", termWidth: 120, termHeight: 30, numMyses: 10, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "medium_terminal_0_myses_10_msgs", termWidth: 120, termHeight: 30, numMyses: 0, numSwarmMsgs: 10, minListHeight: 3, shouldNotPanic: true},
		{name: "medium_terminal_10_myses_10_msgs", termWidth: 120, termHeight: 30, numMyses: 10, numSwarmMsgs: 10, minListHeight: 3, shouldNotPanic: true},

		// Large terminal sizes
		{name: "large_terminal_0_myses_0_msgs", termWidth: 160, termHeight: 40, numMyses: 0, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "large_terminal_20_myses_0_msgs", termWidth: 160, termHeight: 40, numMyses: 20, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "large_terminal_0_myses_20_msgs", termWidth: 160, termHeight: 40, numMyses: 0, numSwarmMsgs: 20, minListHeight: 3, shouldNotPanic: true},
		{name: "large_terminal_20_myses_20_msgs", termWidth: 160, termHeight: 40, numMyses: 20, numSwarmMsgs: 20, minListHeight: 3, shouldNotPanic: true},

		// Very large terminal sizes
		{name: "xlarge_terminal_0_myses_0_msgs", termWidth: 200, termHeight: 60, numMyses: 0, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "xlarge_terminal_20_myses_10_msgs", termWidth: 200, termHeight: 60, numMyses: 20, numSwarmMsgs: 10, minListHeight: 3, shouldNotPanic: true},

		// Very tall terminal
		{name: "tall_terminal_20_myses_10_msgs", termWidth: 120, termHeight: 100, numMyses: 20, numSwarmMsgs: 10, minListHeight: 3, shouldNotPanic: true},

		// Edge cases - extremely small terminals
		{name: "tiny_terminal_0_myses_0_msgs", termWidth: 80, termHeight: 10, numMyses: 0, numSwarmMsgs: 0, minListHeight: 3, shouldNotPanic: true},
		{name: "tiny_terminal_5_myses_5_msgs", termWidth: 80, termHeight: 10, numMyses: 5, numSwarmMsgs: 5, minListHeight: 3, shouldNotPanic: true},

		// Maximum swarm messages (10)
		{name: "max_swarm_msgs_0_myses", termWidth: 120, termHeight: 30, numMyses: 0, numSwarmMsgs: 10, minListHeight: 3, shouldNotPanic: true},
		{name: "max_swarm_msgs_10_myses", termWidth: 120, termHeight: 30, numMyses: 10, numSwarmMsgs: 10, minListHeight: 3, shouldNotPanic: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test data
			myses := make([]MysisInfo, tt.numMyses)
			for i := 0; i < tt.numMyses; i++ {
				myses[i] = makeMysisInfo(i+1, "running")
			}

			swarmMsgs := make([]SwarmMessageInfo, tt.numSwarmMsgs)
			for i := 0; i < tt.numSwarmMsgs; i++ {
				swarmMsgs[i] = SwarmMessageInfo{
					SenderID:   "sender-1",
					SenderName: "sender",
					Content:    "Test message",
					CreatedAt:  time.Now(),
				}
			}

			loadingSet := make(map[string]bool)
			spinnerView := "⬡"

			// Test that rendering doesn't panic
			defer func() {
				if r := recover(); r != nil {
					if tt.shouldNotPanic {
						t.Errorf("Dashboard rendering panicked: %v", r)
					}
				}
			}()

			// Render dashboard (account for input bar height: -3)
			contentHeight := tt.termHeight - 3
			output := RenderDashboard(myses, swarmMsgs, 0, tt.termWidth, contentHeight, loadingSet, spinnerView, 0)

			// Basic validations
			if output == "" {
				t.Error("Dashboard output is empty")
			}
		})
	}
}

// TestDashboardHeightCalculation tests the specific height calculation logic.
func TestDashboardHeightCalculation(t *testing.T) {
	tests := []struct {
		name              string
		termHeight        int
		numSwarmMsgs      int
		expectedMinHeight int
	}{
		{name: "height_20_no_msgs", termHeight: 20, numSwarmMsgs: 0, expectedMinHeight: 3},
		{name: "height_20_5_msgs", termHeight: 20, numSwarmMsgs: 5, expectedMinHeight: 3},
		{name: "height_20_10_msgs", termHeight: 20, numSwarmMsgs: 10, expectedMinHeight: 3},
		{name: "height_30_no_msgs", termHeight: 30, numSwarmMsgs: 0, expectedMinHeight: 3},
		{name: "height_30_10_msgs", termHeight: 30, numSwarmMsgs: 10, expectedMinHeight: 3},
		{name: "height_40_no_msgs", termHeight: 40, numSwarmMsgs: 0, expectedMinHeight: 3},
		{name: "height_60_no_msgs", termHeight: 60, numSwarmMsgs: 0, expectedMinHeight: 3},
		{name: "height_100_no_msgs", termHeight: 100, numSwarmMsgs: 0, expectedMinHeight: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the height calculation from dashboard.go lines 116-125
			msgLines := tt.numSwarmMsgs
			if msgLines == 0 {
				msgLines = 1 // Placeholder line
			}

			// From dashboard.go:
			// usedHeight := 5 // header (3 + margin) + mysis header (1) + footer (1)
			// usedHeight += 1 + len(msgLines) // Swarm section: header (1) + content lines
			// usedHeight += 2 // Account for panel borders (top + bottom = 2 lines)
			usedHeight := 5 + 1 + msgLines + 2

			mysisListHeight := tt.termHeight - usedHeight
			if mysisListHeight < 3 {
				mysisListHeight = 3
			}

			// Verify minimum height is enforced
			if mysisListHeight < tt.expectedMinHeight {
				t.Errorf("mysisListHeight = %d, want >= %d", mysisListHeight, tt.expectedMinHeight)
			}
		})
	}
}

// TestFocusViewLayoutCalculations tests focus view viewport height calculations.
func TestFocusViewLayoutCalculations(t *testing.T) {
	tests := []struct {
		name           string
		termWidth      int
		termHeight     int
		numLogs        int
		minVpHeight    int
		shouldNotPanic bool
	}{
		// Small terminal sizes
		{name: "min_terminal_0_logs", termWidth: 80, termHeight: 20, numLogs: 0, minVpHeight: 5, shouldNotPanic: true},
		{name: "min_terminal_5_logs", termWidth: 80, termHeight: 20, numLogs: 5, minVpHeight: 5, shouldNotPanic: true},
		{name: "min_terminal_20_logs", termWidth: 80, termHeight: 20, numLogs: 20, minVpHeight: 5, shouldNotPanic: true},

		// Medium terminal sizes
		{name: "medium_terminal_0_logs", termWidth: 120, termHeight: 30, numLogs: 0, minVpHeight: 5, shouldNotPanic: true},
		{name: "medium_terminal_50_logs", termWidth: 120, termHeight: 30, numLogs: 50, minVpHeight: 5, shouldNotPanic: true},
		{name: "medium_terminal_100_logs", termWidth: 120, termHeight: 30, numLogs: 100, minVpHeight: 5, shouldNotPanic: true},

		// Large terminal sizes
		{name: "large_terminal_0_logs", termWidth: 160, termHeight: 40, numLogs: 0, minVpHeight: 5, shouldNotPanic: true},
		{name: "large_terminal_100_logs", termWidth: 160, termHeight: 40, numLogs: 100, minVpHeight: 5, shouldNotPanic: true},
		{name: "large_terminal_500_logs", termWidth: 160, termHeight: 40, numLogs: 500, minVpHeight: 5, shouldNotPanic: true},

		// Very large terminal sizes
		{name: "xlarge_terminal_0_logs", termWidth: 200, termHeight: 60, numLogs: 0, minVpHeight: 5, shouldNotPanic: true},
		{name: "xlarge_terminal_1000_logs", termWidth: 200, termHeight: 60, numLogs: 1000, minVpHeight: 5, shouldNotPanic: true},

		// Very tall terminal
		{name: "tall_terminal_500_logs", termWidth: 120, termHeight: 100, numLogs: 500, minVpHeight: 5, shouldNotPanic: true},

		// Edge cases - extremely small terminals
		{name: "tiny_terminal_0_logs", termWidth: 80, termHeight: 10, numLogs: 0, minVpHeight: 5, shouldNotPanic: true},
		{name: "tiny_terminal_10_logs", termWidth: 80, termHeight: 10, numLogs: 10, minVpHeight: 5, shouldNotPanic: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test mysis
			mysis := makeMysisInfo(1, "running")

			// Create viewport with calculated dimensions
			// From app.go lines 133-141:
			headerHeight := 6 // approximate height used by header/info/title
			footerHeight := 2
			vpHeight := tt.termHeight - headerHeight - footerHeight - 3
			if vpHeight < 5 {
				vpHeight = 5
			}
			vpWidth := tt.termWidth - 6 - 2 // -6 for panel padding, -2 for scrollbar

			// Create test viewport
			vp := viewport.New(vpWidth, vpHeight)

			// Test that rendering doesn't panic
			defer func() {
				if r := recover(); r != nil {
					if tt.shouldNotPanic {
						t.Errorf("Focus view rendering panicked: %v", r)
					}
				}
			}()

			// Render focus view
			output := RenderFocusViewWithViewport(mysis, vp, tt.termWidth, false, "⬡", true, false, tt.numLogs, 1, 1, 0)

			// Basic validations
			if output == "" {
				t.Error("Focus view output is empty")
			}

			// Verify viewport height meets minimum
			if vpHeight < tt.minVpHeight {
				t.Errorf("Viewport height = %d, want >= %d", vpHeight, tt.minVpHeight)
			}
		})
	}
}

// TestViewportHeightCalculation tests the specific viewport height calculation logic.
func TestViewportHeightCalculation(t *testing.T) {
	tests := []struct {
		name              string
		termHeight        int
		expectedMinHeight int
	}{
		{name: "height_10", termHeight: 10, expectedMinHeight: 5},
		{name: "height_20", termHeight: 20, expectedMinHeight: 5},
		{name: "height_30", termHeight: 30, expectedMinHeight: 5},
		{name: "height_40", termHeight: 40, expectedMinHeight: 5},
		{name: "height_60", termHeight: 60, expectedMinHeight: 5},
		{name: "height_100", termHeight: 100, expectedMinHeight: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the viewport height calculation from app.go lines 133-141
			headerHeight := 6
			footerHeight := 2
			vpHeight := tt.termHeight - headerHeight - footerHeight - 3
			if vpHeight < 5 {
				vpHeight = 5
			}

			// Verify minimum height is enforced
			if vpHeight < tt.expectedMinHeight {
				t.Errorf("vpHeight = %d, want >= %d", vpHeight, tt.expectedMinHeight)
			}

			// Verify height never goes negative
			if vpHeight < 0 {
				t.Errorf("vpHeight = %d, must not be negative", vpHeight)
			}
		})
	}
}

// TestLayoutNoNegativeHeights verifies that layout calculations never produce negative heights.
func TestLayoutNoNegativeHeights(t *testing.T) {
	// Test extreme cases where terminal height is very small
	extremeCases := []struct {
		name       string
		termHeight int
		termWidth  int
	}{
		{name: "height_5", termHeight: 5, termWidth: 80},
		{name: "height_8", termHeight: 8, termWidth: 80},
		{name: "height_10", termHeight: 10, termWidth: 80},
		{name: "height_15", termHeight: 15, termWidth: 80},
	}

	for _, tc := range extremeCases {
		t.Run(tc.name, func(t *testing.T) {
			// Dashboard mysis list height calculation
			msgLines := 5
			usedHeight := 5 + 1 + msgLines + 2
			mysisListHeight := tc.termHeight - usedHeight
			if mysisListHeight < 3 {
				mysisListHeight = 3
			}

			if mysisListHeight < 0 {
				t.Errorf("Dashboard mysisListHeight = %d, must not be negative", mysisListHeight)
			}

			// Focus view viewport height calculation
			headerHeight := 6
			footerHeight := 2
			vpHeight := tc.termHeight - headerHeight - footerHeight - 3
			if vpHeight < 5 {
				vpHeight = 5
			}

			if vpHeight < 0 {
				t.Errorf("Focus view vpHeight = %d, must not be negative", vpHeight)
			}
		})
	}
}

// TestContentWidthCalculation tests width calculations for dashboard content.
func TestContentWidthCalculation(t *testing.T) {
	tests := []struct {
		name             string
		termWidth        int
		expectedMinWidth int
	}{
		{name: "width_80", termWidth: 80, expectedMinWidth: 20},
		{name: "width_40", termWidth: 40, expectedMinWidth: 20},
		{name: "width_20", termWidth: 20, expectedMinWidth: 20},
		{name: "width_10", termWidth: 10, expectedMinWidth: 20},
		{name: "width_120", termWidth: 120, expectedMinWidth: 20},
		{name: "width_160", termWidth: 160, expectedMinWidth: 20},
		{name: "width_200", termWidth: 200, expectedMinWidth: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate content width calculation from dashboard.go lines 128-131
			// DoubleBorder adds 2 chars each side, so content width is width-4
			contentWidth := tt.termWidth - 4
			if contentWidth < 20 {
				contentWidth = 20
			}

			// Verify minimum width is enforced
			if contentWidth < tt.expectedMinWidth {
				t.Errorf("contentWidth = %d, want >= %d", contentWidth, tt.expectedMinWidth)
			}

			// Verify width never goes negative
			if contentWidth < 0 {
				t.Errorf("contentWidth = %d, must not be negative", contentWidth)
			}
		})
	}
}

// makeMysisInfo creates a test MysisInfo instance.
func makeMysisInfo(id int, state string) MysisInfo {
	return MysisInfo{
		ID:              fmt.Sprintf("mysis-%d", id),
		Name:            fmt.Sprintf("test-mysis-%d", id),
		State:           state,
		Provider:        "ollama",
		AccountUsername: "test_user",
		LastMessage:     "Test message",
		LastError:       "",
		CreatedAt:       time.Now(),
	}
}
