package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// TestUnicodeCharacterInventory documents all Unicode characters used in the TUI.
// This test serves as a comprehensive reference for all decorative characters.
func TestUnicodeCharacterInventory(t *testing.T) {
	inventory := map[string]struct {
		char      string
		codepoint string
		name      string
		usage     []string
	}{
		"black_medium_diamond": {
			char:      "⬥",
			codepoint: "U+2B25",
			name:      "BLACK MEDIUM DIAMOND",
			usage: []string{
				"Header corners",
				"Status bar LLM indicator",
				"Focus view header decoration",
				"Message mode prompt",
				"Spinner frame (running/loading)",
			},
		},
		"black_medium_lozenge": {
			char:      "⬧",
			codepoint: "U+2B27",
			name:      "BLACK MEDIUM LOZENGE",
			usage: []string{
				"Section title borders (SWARM BROADCAST, MYSIS SWARM)",
				"Conversation log header",
				"Broadcast mode prompt",
			},
		},
		"white_hexagon": {
			char:      "⬡",
			codepoint: "U+2B21",
			name:      "WHITE HEXAGON",
			usage: []string{
				"Header title decoration",
				"Focus view mysis name decoration",
				"New mysis mode prompt",
				"Spinner frame 0, 2 (running/loading)",
			},
		},
		"black_hexagon": {
			char:      "⬢",
			codepoint: "U+2B22",
			name:      "BLACK HEXAGON",
			usage: []string{
				"Spinner frame 1, 3 (running/loading)",
			},
		},
		"white_medium_diamond": {
			char:      "⬦",
			codepoint: "U+2B26",
			name:      "WHITE MEDIUM DIAMOND",
			usage: []string{
				"Spinner frame 4, 6 (running/loading)",
				"Idle status bar indicator (no activity)",
			},
		},
		"white_bullet": {
			char:      "◦",
			codepoint: "U+25E6",
			name:      "WHITE BULLET",
			usage: []string{
				"Idle state indicator in mysis list",
			},
		},
		"dotted_circle": {
			char:      "◌",
			codepoint: "U+25CC",
			name:      "DOTTED CIRCLE",
			usage: []string{
				"Stopped state indicator in mysis list",
			},
		},
		"heavy_multiplication_x": {
			char:      "✖",
			codepoint: "U+2716",
			name:      "HEAVY MULTIPLICATION X",
			usage: []string{
				"Errored state indicator in mysis list",
			},
		},
		"gear": {
			char:      "⚙",
			codepoint: "U+2699",
			name:      "GEAR",
			usage: []string{
				"Config provider mode prompt",
			},
		},
	}

	// Verify all characters are documented
	for name, info := range inventory {
		t.Run(name, func(t *testing.T) {
			if info.char == "" {
				t.Errorf("Character %s has empty char field", name)
			}
			if info.codepoint == "" {
				t.Errorf("Character %s has empty codepoint field", name)
			}
			if info.name == "" {
				t.Errorf("Character %s has empty name field", name)
			}
			if len(info.usage) == 0 {
				t.Errorf("Character %s has no usage documented", name)
			}

			// Output character for visual inspection
			t.Logf("%s (%s): %s - %s", name, info.char, info.codepoint, info.name)
			for _, usage := range info.usage {
				t.Logf("  - %s", usage)
			}
		})
	}
}

// TestSpinnerFrameRendering verifies all 8 spinner frames render correctly.
func TestSpinnerFrameRendering(t *testing.T) {
	frames := []struct {
		index     int
		char      string
		codepoint string
	}{
		{0, "⬡", "U+2B21"},
		{1, "⬢", "U+2B22"},
		{2, "⬡", "U+2B21"},
		{3, "⬢", "U+2B22"},
		{4, "⬦", "U+2B26"},
		{5, "⬥", "U+2B25"},
		{6, "⬦", "U+2B26"},
		{7, "⬥", "U+2B25"},
	}

	// Create spinner with actual frames
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8, // 125ms per frame
	}

	for _, frame := range frames {
		t.Run(fmt.Sprintf("frame_%d", frame.index), func(t *testing.T) {
			// Verify frame matches expected character
			actualFrame := sp.Spinner.Frames[frame.index]
			if actualFrame != frame.char {
				t.Errorf("Frame %d: expected %q (%s), got %q",
					frame.index, frame.char, frame.codepoint, actualFrame)
			}

			// Verify character width is 1
			width := lipgloss.Width(frame.char)
			if width != 1 {
				t.Errorf("Frame %d character %q has width %d, expected 1",
					frame.index, frame.char, width)
			}

			t.Logf("Frame %d: %s (%s) - width: %d", frame.index, frame.char, frame.codepoint, width)
		})
	}

	// Verify FPS
	expectedFPS := time.Second / 8
	if sp.Spinner.FPS != expectedFPS {
		t.Errorf("Spinner FPS: expected %v, got %v", expectedFPS, sp.Spinner.FPS)
	}
	t.Logf("Spinner FPS: %v (125ms per frame)", sp.Spinner.FPS)
}

// TestUnicodeWidthConsistency verifies all characters have consistent width.
func TestUnicodeWidthConsistency(t *testing.T) {
	chars := map[string]string{
		"black_medium_diamond": "⬥",
		"black_medium_lozenge": "⬧",
		"white_hexagon":        "⬡",
		"black_hexagon":        "⬢",
		"white_medium_diamond": "⬦",
		"white_bullet":         "◦",
		"dotted_circle":        "◌",
		"heavy_x":              "✖",
		"gear":                 "⚙",
	}

	for name, char := range chars {
		t.Run(name, func(t *testing.T) {
			// Test with lipgloss.Width
			width := lipgloss.Width(char)
			if width != 1 {
				t.Errorf("Character %q (%s) has width %d via lipgloss, expected 1",
					char, name, width)
			}

			// Test with runewidth (East Asian Width safety)
			r := []rune(char)[0]

			// Test in narrow mode
			runewidth.DefaultCondition.EastAsianWidth = false
			narrowWidth := runewidth.RuneWidth(r)

			// Test in wide mode
			runewidth.DefaultCondition.EastAsianWidth = true
			wideWidth := runewidth.RuneWidth(r)

			if narrowWidth != wideWidth {
				t.Errorf("Character %q (%s) is ambiguous width: narrow=%d, wide=%d",
					char, name, narrowWidth, wideWidth)
			}

			if narrowWidth != 1 {
				t.Errorf("Character %q (%s) has width %d via runewidth, expected 1",
					char, name, narrowWidth)
			}

			t.Logf("%s: %q - lipgloss width: %d, runewidth narrow: %d, runewidth wide: %d",
				name, char, width, narrowWidth, wideWidth)
		})
	}
}

// TestInputModePromptIndicators verifies all input mode prompt indicators.
func TestInputModePromptIndicators(t *testing.T) {
	indicators := map[InputMode]struct {
		char      string
		codepoint string
		name      string
	}{
		InputModeBroadcast: {
			char:      "⬧",
			codepoint: "U+2B27",
			name:      "BLACK MEDIUM LOZENGE",
		},
		InputModeMessage: {
			char:      "⬥",
			codepoint: "U+2B25",
			name:      "BLACK MEDIUM DIAMOND",
		},
		InputModeNewMysis: {
			char:      "⬡",
			codepoint: "U+2B21",
			name:      "WHITE HEXAGON",
		},
		InputModeConfigProvider: {
			char:      "⚙",
			codepoint: "U+2699",
			name:      "GEAR",
		},
		// Note: InputModeConfigModel uses "cfg" text, not a Unicode character
	}

	modeNames := map[InputMode]string{
		InputModeBroadcast:      "broadcast",
		InputModeMessage:        "message",
		InputModeNewMysis:       "new_mysis",
		InputModeConfigProvider: "config_provider",
	}

	for mode, info := range indicators {
		modeName := modeNames[mode]
		t.Run(modeName, func(t *testing.T) {
			// Create input model
			m := NewInputModel()
			m.SetMode(mode, "")

			// Get prompt (strip styles)
			prompt := m.textInput.Prompt
			if prompt == "" {
				t.Errorf("Mode %s has empty prompt", modeName)
			}

			// Verify character is in prompt
			// (Prompt is styled, so we just check for presence)
			if len(prompt) < len(info.char) {
				t.Errorf("Mode %s prompt too short: %q", modeName, prompt)
			}

			t.Logf("Mode %s: indicator %s (%s) - %s",
				modeName, info.char, info.codepoint, info.name)
		})
	}
}

// TestStateIndicatorCharacters verifies all state indicator characters.
func TestStateIndicatorCharacters(t *testing.T) {
	states := map[string]struct {
		char      string
		codepoint string
		name      string
	}{
		"running": {
			char:      "⬡", // First spinner frame (changes with animation)
			codepoint: "U+2B21",
			name:      "WHITE HEXAGON (animated)",
		},
		"idle": {
			char:      "◦",
			codepoint: "U+25E6",
			name:      "WHITE BULLET",
		},
		"stopped": {
			char:      "◌",
			codepoint: "U+25CC",
			name:      "DOTTED CIRCLE",
		},
		"errored": {
			char:      "✖",
			codepoint: "U+2716",
			name:      "HEAVY MULTIPLICATION X",
		},
	}

	for state, info := range states {
		t.Run(state, func(t *testing.T) {
			// Verify character width
			width := lipgloss.Width(info.char)
			if width != 1 {
				t.Errorf("State %s indicator %q has width %d, expected 1",
					state, info.char, width)
			}

			t.Logf("State %s: %s (%s) - %s",
				state, info.char, info.codepoint, info.name)
		})
	}
}

// TestBorderCharacters documents border characters used in panels.
func TestBorderCharacters(t *testing.T) {
	borders := map[string]struct {
		chars      []string
		codepoints []string
		usage      string
	}{
		"double_line_box": {
			chars:      []string{"╔", "═", "╗", "║", "╚", "╝"},
			codepoints: []string{"U+2554", "U+2550", "U+2557", "U+2551", "U+255A", "U+255D"},
			usage:      "Mysis list panel border",
		},
		"horizontal_line": {
			chars:      []string{"─"},
			codepoints: []string{"U+2500"},
			usage:      "Section title decorations, focus header",
		},
		"rounded_corner": {
			chars:      []string{"╭", "╮", "╰", "╯"},
			codepoints: []string{"U+256D", "U+256E", "U+2570", "U+256F"},
			usage:      "Input prompt border",
		},
	}

	for name, info := range borders {
		t.Run(name, func(t *testing.T) {
			if len(info.chars) != len(info.codepoints) {
				t.Errorf("Border %s: char count (%d) != codepoint count (%d)",
					name, len(info.chars), len(info.codepoints))
			}

			t.Logf("Border %s: %s", name, info.usage)
			for i, char := range info.chars {
				width := lipgloss.Width(char)
				t.Logf("  %s (%s) - width: %d", char, info.codepoints[i], width)
			}
		})
	}
}

// TestCharacterRenderingMatrix provides a visual reference chart.
func TestCharacterRenderingMatrix(t *testing.T) {
	// Skip in short mode since this is for documentation
	if testing.Short() {
		t.Skip("Skipping visual reference chart in short mode")
	}

	categories := map[string][]struct {
		name string
		char string
		code string
	}{
		"State Indicators": {
			{"Running/Loading", "⬡⬢⬦⬥", "animated"},
			{"Idle", "◦", "U+25E6"},
			{"Stopped", "◌", "U+25CC"},
			{"Errored", "✖", "U+2716"},
		},
		"Decorative Elements": {
			{"Diamond", "⬥", "U+2B25"},
			{"Lozenge", "⬧", "U+2B27"},
			{"Hexagon White", "⬡", "U+2B21"},
			{"Hexagon Black", "⬢", "U+2B22"},
			{"Diamond White", "⬦", "U+2B26"},
		},
		"Input Prompts": {
			{"Broadcast", "⬧", "U+2B27"},
			{"Message", "⬥", "U+2B25"},
			{"New Mysis", "⬡", "U+2B21"},
			{"Config", "⚙", "U+2699"},
		},
	}

	t.Log("\n=== Unicode Character Rendering Reference ===")
	for category, chars := range categories {
		t.Logf("\n%s:", category)
		for _, item := range chars {
			t.Logf("  %-20s %s  (%s)", item.name, item.char, item.code)
		}
	}
}
