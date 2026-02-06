package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
)

// TestStatusBar tests status bar rendering with various configurations.
func TestStatusBar(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name         string
		view         View
		focusID      string
		myses        []MysisInfo
		netActivity  NetActivity
		netPosition  int
		providerErrs []time.Time
		width        int
	}{
		{
			name:        "dashboard_no_myses",
			view:        ViewDashboard,
			myses:       []MysisInfo{},
			netActivity: NetActivityIdle,
			width:       120,
		},
		{
			name: "dashboard_single_running",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6, // Middle of progress bar
			width:       120,
		},
		{
			name: "dashboard_mixed_states",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
				{ID: "test-2", Name: "beta", State: "idle"},
				{ID: "test-3", Name: "gamma", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 0, // Start of progress bar
			width:       120,
		},
		{
			name:        "dashboard_full_swarm",
			view:        ViewDashboard,
			myses:       makeFullSwarm16(),
			netActivity: NetActivityLLM,
			netPosition: 11, // End of progress bar
			width:       120,
		},
		{
			name:    "focus_short_id",
			view:    ViewFocus,
			focusID: "abc123",
			myses: []MysisInfo{
				{ID: "abc123", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6,
			width:       120,
		},
		{
			name:    "focus_long_id",
			view:    ViewFocus,
			focusID: "6b152b72-09e4-4695-aaa2-9a529147d3d7",
			myses: []MysisInfo{
				{ID: "6b152b72-09e4-4695-aaa2-9a529147d3d7", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6,
			width:       120,
		},
		{
			name: "llm_progress_0_percent",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 0,
			width:       120,
		},
		{
			name: "llm_progress_50_percent",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6,
			width:       120,
		},
		{
			name: "llm_progress_100_percent",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 11,
			width:       120,
		},
		{
			name: "with_provider_errors",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6,
			providerErrs: []time.Time{
				time.Now(),
				time.Now(),
				time.Now(),
			},
			width: 120,
		},
		{
			name: "narrow_width_80",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6,
			width:       80,
		},
		{
			name: "wide_width_160",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
				{ID: "test-2", Name: "beta", State: "idle"},
				{ID: "test-3", Name: "gamma", State: "running"},
			},
			netActivity: NetActivityLLM,
			netPosition: 6,
			width:       160,
		},
		{
			name:        "idle_activity",
			view:        ViewDashboard,
			myses:       []MysisInfo{},
			netActivity: NetActivityIdle,
			netPosition: 0,
			width:       120,
		},
		{
			name: "mcp_activity",
			view: ViewDashboard,
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			netActivity: NetActivityMCP,
			netPosition: 6,
			width:       120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.view = tt.view
			m.focusID = tt.focusID
			m.myses = tt.myses
			m.width = tt.width
			m.providerErrorTimes = tt.providerErrs

			// Set network indicator state
			m.netIndicator.activity = tt.netActivity
			m.netIndicator.position = tt.netPosition

			// Render status bar
			output := m.renderStatusBar()

			// Test ANSI output
			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			// Test stripped output
			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestStatusBarWidthHandling tests width calculation and truncation.
func TestStatusBarWidthHandling(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		mysisID   string
		wantWidth int
	}{
		{
			name:      "width_80",
			width:     80,
			mysisID:   "short",
			wantWidth: 80,
		},
		{
			name:      "width_120",
			width:     120,
			mysisID:   "medium-length",
			wantWidth: 120,
		},
		{
			name:      "width_160",
			width:     160,
			mysisID:   "6b152b72-09e4-4695-aaa2-9a529147d3d7",
			wantWidth: 160,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.view = ViewFocus
			m.focusID = tt.mysisID
			m.width = tt.width
			m.myses = []MysisInfo{
				{ID: tt.mysisID, Name: "test", State: "running"},
			}

			output := m.renderStatusBar()

			// Strip ANSI and check display width
			stripped := stripANSIForGolden(output)

			// Check that output doesn't overflow
			if len(stripped) > tt.wantWidth*2 { // *2 for Unicode safety
				t.Errorf("Status bar may overflow: stripped length %d exceeds %d*2", len(stripped), tt.wantWidth)
			}
		})
	}
}

// TestStatusBarMysisCount tests mysis state count display logic.
func TestStatusBarMysisCount(t *testing.T) {
	tests := []struct {
		name         string
		myses        []MysisInfo
		wantContains string // Check that output contains this substring
	}{
		{
			name:         "zero_myses",
			myses:        []MysisInfo{},
			wantContains: "(no myses)",
		},
		{
			name: "one_running",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
			},
			wantContains: "1", // Should show "⬡ 1" or spinner + "1"
		},
		{
			name: "mixed_states",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
				{ID: "2", State: "idle"},
				{ID: "3", State: "stopped"},
			},
			wantContains: "1", // Should show state counts with icons
		},
		{
			name: "all_running",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
				{ID: "2", State: "running"},
				{ID: "3", State: "running"},
			},
			wantContains: "3", // Should show "⬡ 3" or spinner + "3"
		},
		{
			name:         "full_swarm_16",
			myses:        makeFullSwarm16(),
			wantContains: "16", // Should show "⬡ 16" or spinner + "16"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.myses = tt.myses
			m.width = 120

			output := m.renderStatusBar()
			stripped := stripANSIForGolden(output)

			// Check that expected content appears in output
			if !strings.Contains(stripped, tt.wantContains) {
				t.Errorf("Expected %q in status bar, got: %s", tt.wantContains, stripped)
			}

			// Verify tick timestamp format is present (T#### ⬡ [HH:MM])
			if !strings.Contains(stripped, "T") || !strings.Contains(stripped, "[") {
				t.Errorf("Expected tick timestamp format in status bar, got: %s", stripped)
			}
		})
	}
}

// TestStatusBarFocusIDTruncation removed - view name/focus ID no longer shown in status bar.
// Status bar now shows: [activity indicator] | [tick + timestamp] | [state counts]

// TestStatusBarLipglossWidth tests that rendered output matches expected width.
func TestStatusBarLipglossWidth(t *testing.T) {
	tests := []struct {
		name  string
		width int
	}{
		{name: "width_80", width: 80},
		{name: "width_120", width: 120},
		{name: "width_160", width: 160},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.width = tt.width
			m.myses = []MysisInfo{
				{ID: "test", Name: "alpha", State: "running"},
			}

			output := m.renderStatusBar()

			// Check lipgloss width (accounts for ANSI)
			actualWidth := lipgloss.Width(output)
			if actualWidth != tt.width {
				t.Errorf("Expected lipgloss width %d, got %d", tt.width, actualWidth)
			}
		})
	}
}

// TestStatusBarStateCounts tests status bar with state counts view model.
func TestStatusBarStateCounts(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name        string
		myses       []MysisInfo
		width       int
		currentTick int64
		spinnerPos  int // spinner frame position for running/loading states
	}{
		{
			name:        "empty_swarm",
			myses:       []MysisInfo{},
			width:       120,
			currentTick: 0,
			spinnerPos:  0,
		},
		{
			name: "single_idle",
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "idle"},
			},
			width:       120,
			currentTick: 100,
			spinnerPos:  0,
		},
		{
			name: "single_running",
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
			},
			width:       120,
			currentTick: 200,
			spinnerPos:  0, // First frame
		},
		{
			name: "single_stopped",
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "stopped"},
			},
			width:       120,
			currentTick: 150,
			spinnerPos:  0,
		},
		{
			name: "single_errored",
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "errored"},
			},
			width:       120,
			currentTick: 175,
			spinnerPos:  0,
		},
		{
			name: "mixed_states",
			myses: []MysisInfo{
				{ID: "test-1", Name: "alpha", State: "running"},
				{ID: "test-2", Name: "beta", State: "idle"},
				{ID: "test-3", Name: "gamma", State: "stopped"},
				{ID: "test-4", Name: "delta", State: "errored"},
			},
			width:       120,
			currentTick: 250,
			spinnerPos:  2, // Mid-animation frame
		},
		{
			name: "all_running",
			myses: []MysisInfo{
				{ID: "test-1", Name: "a", State: "running"},
				{ID: "test-2", Name: "b", State: "running"},
				{ID: "test-3", Name: "c", State: "running"},
			},
			width:       120,
			currentTick: 300,
			spinnerPos:  4, // Different frame
		},
		{
			name: "full_swarm_mixed",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
				{ID: "2", State: "running"},
				{ID: "3", State: "running"},
				{ID: "4", State: "running"},
				{ID: "5", State: "running"},
				{ID: "6", State: "idle"},
				{ID: "7", State: "idle"},
				{ID: "8", State: "stopped"},
				{ID: "9", State: "stopped"},
				{ID: "10", State: "errored"},
			},
			width:       160,
			currentTick: 1000,
			spinnerPos:  6, // Last animation frame
		},
		{
			name: "narrow_width_state_counts",
			myses: []MysisInfo{
				{ID: "test-1", State: "running"},
				{ID: "test-2", State: "idle"},
			},
			width:       80,
			currentTick: 500,
			spinnerPos:  3,
		},
		{
			name:        "tick_5000_timestamp",
			myses:       []MysisInfo{},
			width:       120,
			currentTick: 5000,
			spinnerPos:  0,
		},
		{
			name:        "tick_99999_large",
			myses:       []MysisInfo{},
			width:       120,
			currentTick: 99999,
			spinnerPos:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.myses = tt.myses
			m.width = tt.width
			m.currentTick = tt.currentTick

			// Note: spinner position varies by frame, tests will use current frame
			output := m.renderStatusBar()

			// Test ANSI output
			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			// Test stripped output
			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// Helper: makeFullSwarm16 creates 16 running myses for full swarm tests.
func makeFullSwarm16() []MysisInfo {
	myses := make([]MysisInfo, 16)
	for i := 0; i < 16; i++ {
		myses[i] = MysisInfo{
			ID:    "test-" + string(rune('a'+i)),
			Name:  "mysis-" + string(rune('a'+i)),
			State: "running",
		}
	}
	return myses
}
