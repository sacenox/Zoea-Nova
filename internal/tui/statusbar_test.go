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

// TestStatusBarMysisCount tests mysis count display logic.
func TestStatusBarMysisCount(t *testing.T) {
	tests := []struct {
		name      string
		myses     []MysisInfo
		wantCount string
	}{
		{
			name:      "zero_myses",
			myses:     []MysisInfo{},
			wantCount: "0/0",
		},
		{
			name: "one_running",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
			},
			wantCount: "1/1",
		},
		{
			name: "mixed_states",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
				{ID: "2", State: "idle"},
				{ID: "3", State: "stopped"},
			},
			wantCount: "1/3",
		},
		{
			name: "all_running",
			myses: []MysisInfo{
				{ID: "1", State: "running"},
				{ID: "2", State: "running"},
				{ID: "3", State: "running"},
			},
			wantCount: "3/3",
		},
		{
			name:      "full_swarm_16",
			myses:     makeFullSwarm16(),
			wantCount: "16/16",
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

			// Check that count appears in output
			if !strings.Contains(stripped, tt.wantCount) {
				t.Errorf("Expected count %q in status bar, got: %s", tt.wantCount, stripped)
			}
		})
	}
}

// TestStatusBarFocusIDTruncation tests focus ID display truncation.
func TestStatusBarFocusIDTruncation(t *testing.T) {
	tests := []struct {
		name         string
		focusID      string
		wantInOutput string
	}{
		{
			name:         "short_id",
			focusID:      "abc",
			wantInOutput: "abc",
		},
		{
			name:         "exact_8_chars",
			focusID:      "12345678",
			wantInOutput: "12345678",
		},
		{
			name:         "long_id_truncated",
			focusID:      "6b152b72-09e4-4695-aaa2-9a529147d3d7",
			wantInOutput: "6b152b72", // First 8 chars
		},
		{
			name:         "uuid_truncated",
			focusID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			wantInOutput: "aaaaaaaa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cleanup := setupTestModel(t)
			defer cleanup()

			m.view = ViewFocus
			m.focusID = tt.focusID
			m.width = 120

			output := m.renderStatusBar()
			stripped := stripANSIForGolden(output)

			// Check that expected ID substring appears
			if !strings.Contains(stripped, tt.wantInOutput) {
				t.Errorf("Expected %q in status bar, got: %s", tt.wantInOutput, stripped)
			}

			// Check that full ID doesn't appear if it should be truncated
			if len(tt.focusID) > 8 && strings.Contains(stripped, tt.focusID) {
				t.Errorf("Expected focus ID to be truncated, but found full ID in: %s", stripped)
			}
		})
	}
}

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
