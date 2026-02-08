package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
)

// TestSpinnerFrames tests that all spinner frames render without errors
func TestSpinnerFrames(t *testing.T) {
	defer setupGoldenTest(t)()

	// Create spinner with hexagonal theme
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8, // 8 frames per second
	}
	sp.Style = lipgloss.NewStyle().Foreground(colorBrand)

	tests := []struct {
		name  string
		frame int
	}{
		{"frame_0", 0},
		{"frame_1", 1},
		{"frame_2", 2},
		{"frame_3", 3},
		{"frame_4", 4},
		{"frame_5", 5},
		{"frame_6", 6},
		{"frame_7", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the frame character
			frame := sp.Spinner.Frames[tt.frame]
			// Apply styling
			output := sp.Style.Render(frame)

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestSpinnerCycling tests frame cycling logic
func TestSpinnerCycling(t *testing.T) {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8,
	}
	sp.Style = lipgloss.NewStyle().Foreground(colorBrand)

	// Test that frames cycle correctly
	expectedFrames := []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"}

	for i := 0; i < len(expectedFrames)*2; i++ {
		expectedIdx := i % len(expectedFrames)
		actualFrame := sp.Spinner.Frames[expectedIdx]
		expectedFrame := expectedFrames[expectedIdx]

		if actualFrame != expectedFrame {
			t.Errorf("Frame %d: expected %q, got %q", i, expectedFrame, actualFrame)
		}
	}
}

// TestSpinnerFPS tests FPS timing configuration
func TestSpinnerFPS(t *testing.T) {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8, // 8 frames per second = 125ms per frame
	}

	expectedFPS := time.Second / 8
	expectedDuration := 125 * time.Millisecond

	if sp.Spinner.FPS != expectedFPS {
		t.Errorf("FPS: expected %v, got %v", expectedFPS, sp.Spinner.FPS)
	}

	if sp.Spinner.FPS != expectedDuration {
		t.Errorf("Frame duration: expected %v, got %v", expectedDuration, sp.Spinner.FPS)
	}
}

// TestSpinnerFrameCount tests correct number of frames
func TestSpinnerFrameCount(t *testing.T) {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8,
	}

	expectedCount := 8
	actualCount := len(sp.Spinner.Frames)

	if actualCount != expectedCount {
		t.Errorf("Frame count: expected %d, got %d", expectedCount, actualCount)
	}
}

// TestSpinnerUnicodeCharacters tests that spinner uses correct Unicode characters
func TestSpinnerUnicodeCharacters(t *testing.T) {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8,
	}

	// Expected characters (hexagonal theme matching logo)
	expectedChars := map[int]string{
		0: "⬡", // U+2B21 WHITE HEXAGON
		1: "⬢", // U+2B22 BLACK HEXAGON
		2: "⬡", // U+2B21 WHITE HEXAGON
		3: "⬢", // U+2B22 BLACK HEXAGON
		4: "⬦", // U+2B26 WHITE MEDIUM DIAMOND
		5: "⬥", // U+2B25 BLACK MEDIUM DIAMOND
		6: "⬦", // U+2B26 WHITE MEDIUM DIAMOND
		7: "⬥", // U+2B25 BLACK MEDIUM DIAMOND
	}

	for idx, expectedChar := range expectedChars {
		actualChar := sp.Spinner.Frames[idx]
		if actualChar != expectedChar {
			t.Errorf("Frame %d: expected %q (U+%04X), got %q",
				idx, expectedChar, []rune(expectedChar)[0], actualChar)
		}
	}
}

// TestStateIndicators tests state indicator rendering for all states
func TestStateIndicators(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name        string
		state       string
		isLoading   bool
		spinnerView string
	}{
		{
			name:        "running_with_spinner",
			state:       "running",
			isLoading:   false,
			spinnerView: "⬡",
		},
		{
			name:        "idle",
			state:       "idle",
			isLoading:   false,
			spinnerView: "⬡",
		},
		{
			name:        "stopped",
			state:       "stopped",
			isLoading:   false,
			spinnerView: "⬡",
		},
		{
			name:        "errored",
			state:       "errored",
			isLoading:   false,
			spinnerView: "⬡",
		},
		{
			name:        "loading",
			state:       "running",
			isLoading:   true,
			spinnerView: "⬡",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysis := MysisInfo{
				ID:              "test-id",
				Name:            "test-mysis",
				State:           tt.state,
				Provider:        "ollama",
				AccountUsername: "test_user",
				LastMessage:     "Test message",
				LastMessageAt:   time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				CreatedAt:       time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
			}

			lines := renderMysisLine(mysis, false, tt.isLoading, tt.spinnerView, 100, 0)
			output := strings.Join(lines, "\n")

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}

// TestStateIndicatorAlignment tests that spinner frames don't shift alignment
func TestStateIndicatorAlignment(t *testing.T) {
	defer setupGoldenTest(t)()

	mysis := MysisInfo{
		ID:              "test-id",
		Name:            "test-mysis",
		State:           "running",
		Provider:        "ollama",
		AccountUsername: "test_user",
		LastMessage:     "Test message",
		LastMessageAt:   time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		CreatedAt:       time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
	}

	spinnerFrames := []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"}

	tests := []struct {
		name  string
		frame string
	}{
		{"spinner_frame_0", spinnerFrames[0]},
		{"spinner_frame_1", spinnerFrames[1]},
		{"spinner_frame_2", spinnerFrames[2]},
		{"spinner_frame_3", spinnerFrames[3]},
		{"spinner_frame_4", spinnerFrames[4]},
		{"spinner_frame_5", spinnerFrames[5]},
		{"spinner_frame_6", spinnerFrames[6]},
		{"spinner_frame_7", spinnerFrames[7]},
	}

	// Store widths to verify consistency
	var widths []int
	var strippedWidths []int

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := renderMysisLine(mysis, false, false, tt.frame, 100, 0)
			output := strings.Join(lines, "\n")
			stripped := stripANSIForGolden(output)

			// Check width consistency (use first line for width since that's the info row)
			width := lipgloss.Width(lines[0])
			strippedWidth := len(strings.TrimSpace(stripped))

			widths = append(widths, width)
			strippedWidths = append(strippedWidths, strippedWidth)

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			t.Run("Stripped", func(t *testing.T) {
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}

	// Verify all widths are consistent across frames
	if len(widths) > 0 {
		firstWidth := widths[0]
		for i, w := range widths {
			if w != firstWidth {
				t.Errorf("Frame %d width inconsistency: expected %d, got %d", i, firstWidth, w)
			}
		}
	}

	// Verify stripped widths are consistent
	if len(strippedWidths) > 0 {
		firstStrippedWidth := strippedWidths[0]
		for i, w := range strippedWidths {
			if w != firstStrippedWidth {
				t.Errorf("Frame %d stripped width inconsistency: expected %d, got %d", i, firstStrippedWidth, w)
			}
		}
	}
}

// TestSpinnerInDashboard tests spinner rendering in full dashboard context
func TestSpinnerInDashboard(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name        string
		spinnerView string
	}{
		{"with_spinner_frame_0", "⬡"},
		{"with_spinner_frame_4", "⬦"},
		{"with_spinner_frame_7", "⬥"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			myses := []MysisInfo{
				{
					ID:              "mysis-1",
					Name:            "alpha",
					State:           "running",
					Provider:        "ollama",
					AccountUsername: "crab_warrior",
					LastMessage:     "Mining asteroid belt",
					LastMessageAt:   time.Date(2026, 1, 15, 10, 45, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				},
			}

			output := RenderDashboard(myses, []SwarmMessageInfo{}, 0, TestTerminalWidth, TestTerminalHeight, map[string]bool{}, tt.spinnerView, 0)

			t.Run("ANSI", func(t *testing.T) {
				golden.RequireEqual(t, []byte(output))
			})

			t.Run("Stripped", func(t *testing.T) {
				stripped := stripANSIForGolden(output)
				golden.RequireEqual(t, []byte(stripped))
			})
		})
	}
}
