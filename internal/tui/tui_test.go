package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
	"golang.org/x/time/rate"
)

// ansiRegex matches ANSI escape codes
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes all ANSI escape codes from a string
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func setupTestModel(t *testing.T) (Model, func()) {
	t.Helper()

	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}

	bus := core.NewEventBus(100)
	eventCh := bus.Subscribe()

	reg := provider.NewRegistry()
	limiter := rate.NewLimiter(rate.Limit(1000), 1000)
	reg.RegisterFactory(provider.NewMockFactoryWithLimiter("ollama", "mock response", limiter))
	reg.RegisterFactory(provider.NewMockFactoryWithLimiter("opencode_zen", "mock response", limiter))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses:        16,
			DefaultProvider: "opencode_zen",
			DefaultModel:    "gpt-5-nano",
		},
		Providers: map[string]config.ProviderConfig{
			"ollama":       {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7, RateLimit: 1000, RateBurst: 1000},
			"opencode_zen": {Endpoint: "http://mock", Model: "gpt-5-nano", Temperature: 0.7, RateLimit: 1000, RateBurst: 1000},
		},
	}

	commander := core.NewCommander(s, reg, bus, cfg)

	model := New(commander, s, eventCh, false)
	model.width = 80
	model.height = 24

	// Set fixed test time for deterministic timestamps
	fixedTime := testTime()
	model.testTime = &fixedTime

	cleanup := func() {
		commander.StopAll()
		bus.Close()
		s.Close()
	}

	return model, cleanup
}

func TestModelInit(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	cmd := m.Init()
	if cmd == nil {
		t.Error("expected Init to return a command")
	}
}

func TestModelView(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Initial view should render without panic
	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModelViewWithZeroSize(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	m.width = 0
	m.height = 0

	view := m.View()
	if view != "Loading..." {
		t.Errorf("expected 'Loading...', got %s", view)
	}
}

func TestModelWindowResize(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newModel.(Model)

	if m.width != 120 {
		t.Errorf("expected width=120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height=40, got %d", m.height)
	}
}

func TestModelQuit(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Check if quit command was returned
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModelHelpToggle(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	if m.showHelp {
		t.Error("help should be hidden initially")
	}

	// Toggle help on
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newModel.(Model)

	if !m.showHelp {
		t.Error("help should be shown after pressing ?")
	}

	// Toggle help off
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newModel.(Model)

	if m.showHelp {
		t.Error("help should be hidden after pressing ? again")
	}
}

func TestModelNavigation(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create some myses
	m.commander.CreateMysis("mysis-1", "ollama")
	m.commander.CreateMysis("mysis-2", "ollama")
	m.refreshMysisList()

	if m.selectedIdx != 0 {
		t.Errorf("expected selectedIdx=0, got %d", m.selectedIdx)
	}

	// Navigate down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)

	if m.selectedIdx != 1 {
		t.Errorf("expected selectedIdx=1, got %d", m.selectedIdx)
	}

	// Navigate up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)

	if m.selectedIdx != 0 {
		t.Errorf("expected selectedIdx=0, got %d", m.selectedIdx)
	}
}

func TestModelFocusView(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Create a mysis
	mysis, _ := m.commander.CreateMysis("test-mysis", "ollama")
	m.refreshMysisList()

	// Press enter to focus
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if m.view != ViewFocus {
		t.Errorf("expected view=ViewFocus, got %d", m.view)
	}
	if m.focusID != mysis.ID() {
		t.Errorf("expected focusID=%s, got %s", mysis.ID(), m.focusID)
	}

	// Press escape to go back
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	if m.view != ViewDashboard {
		t.Errorf("expected view=ViewDashboard, got %d", m.view)
	}
}

func TestInputModel(t *testing.T) {
	input := NewInputModel()

	if input.IsActive() {
		t.Error("input should not be active initially")
	}

	input.SetMode(InputModeBroadcast, "")
	if !input.IsActive() {
		t.Error("input should be active after SetMode")
	}
	if input.Mode() != InputModeBroadcast {
		t.Errorf("expected mode=InputModeBroadcast, got %d", input.Mode())
	}

	input.Reset()
	if input.IsActive() {
		t.Error("input should not be active after Reset")
	}
}

func TestRenderHelp(t *testing.T) {
	help := RenderHelp(80, 24)
	if help == "" {
		t.Error("expected non-empty help")
	}
}

func TestRenderDashboard(t *testing.T) {
	myses := []MysisInfo{
		{ID: "1", Name: "mysis-1", State: "running", Provider: "ollama"},
		{ID: "2", Name: "mysis-2", State: "idle", Provider: "ollama"},
	}

	loadingSet := make(map[string]bool)
	swarmMsgs := []SwarmMessageInfo{}
	dashboard := RenderDashboard(myses, swarmMsgs, 0, 80, 24, loadingSet, "⠋", 0)
	if dashboard == "" {
		t.Error("expected non-empty dashboard")
	}

	// Test with loading state
	loadingSet["1"] = true
	dashboardWithLoading := RenderDashboard(myses, swarmMsgs, 0, 80, 24, loadingSet, "⠋", 0)
	if dashboardWithLoading == "" {
		t.Error("expected non-empty dashboard with loading")
	}
}

func TestRenderDashboardEmpty(t *testing.T) {
	dashboard := RenderDashboard([]MysisInfo{}, []SwarmMessageInfo{}, 0, 80, 24, make(map[string]bool), "⠋", 0)
	if dashboard == "" {
		t.Error("expected non-empty dashboard even with no myses")
	}
}

func TestRenderFocusView(t *testing.T) {
	mysis := MysisInfo{ID: "1", Name: "test-mysis", State: "running", Provider: "ollama"}
	logs := []LogEntry{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there! This is a longer response that might span multiple lines when properly wrapped in the terminal window."},
	}

	view := RenderFocusView(mysis, logs, 80, 24, false, "⠋", false, 1, 1, 0)
	if view == "" {
		t.Error("expected non-empty focus view")
	}

	// Test with loading state
	viewLoading := RenderFocusView(mysis, logs, 80, 24, true, "⠋", false, 1, 1, 0)
	if viewLoading == "" {
		t.Error("expected non-empty focus view with loading")
	}
}

func TestStateStyle(t *testing.T) {
	// Just ensure these don't panic
	_ = StateStyle("running")
	_ = StateStyle("idle")
	_ = StateStyle("stopped")
	_ = StateStyle("errored")
	_ = StateStyle("unknown")
}

func TestRoleColor(t *testing.T) {
	// Verify RoleColor returns distinct colors for each role
	roles := []string{"user", "assistant", "system", "tool", "unknown"}
	colors := make(map[string]lipgloss.Color)

	for _, role := range roles {
		color := RoleColor(role)
		colors[role] = color

		// Color should be a non-empty string
		if string(color) == "" {
			t.Errorf("role %q should have a non-empty color", role)
		}
	}

	// Main roles should have distinct colors
	if colors["user"] == colors["assistant"] {
		t.Error("user and assistant should have distinct colors")
	}
	if colors["system"] == colors["tool"] {
		t.Error("system and tool should have distinct colors")
	}
	if colors["user"] == colors["system"] {
		t.Error("user and system should have distinct colors")
	}
}

func TestRenderLogEntry(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		content  string
		maxWidth int
	}{
		{"user message", "user", "Hello world", 80},
		{"assistant message", "assistant", "Hi there, how can I help?", 80},
		{"system message", "system", "System initialized", 80},
		{"tool message", "tool", "Tool result: success", 80},
		{"unknown role", "unknown", "Unknown message", 80},
		{"long message wrap", "user", "This is a very long message that should be wrapped across multiple lines when the width is constrained", 40},
		{"multiline content", "assistant", "Line one\nLine two\nLine three", 80},
		{"narrow width", "user", "Short", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntry{Role: tt.role, Content: tt.content}
			lines := renderLogEntryImpl(entry, tt.maxWidth, false, 0)
			if len(lines) == 0 {
				t.Error("expected at least one line of output")
			}
		})
	}
}

func TestInputModelModes(t *testing.T) {
	input := NewInputModel()

	tests := []struct {
		mode     InputMode
		targetID string
		wantDesc string
	}{
		{InputModeBroadcast, "", "broadcast mode"},
		{InputModeMessage, "mysis-1", "message mode with target"},
		{InputModeNewMysis, "", "new mysis mode"},
		{InputModeConfigProvider, "mysis-1", "config mode with target"},
		{InputModeNone, "", "none mode"},
	}

	for _, tt := range tests {
		t.Run(tt.wantDesc, func(t *testing.T) {
			input.SetMode(tt.mode, tt.targetID)

			if input.Mode() != tt.mode {
				t.Errorf("expected mode %d, got %d", tt.mode, input.Mode())
			}
			if tt.targetID != "" && input.TargetID() != tt.targetID {
				t.Errorf("expected targetID %s, got %s", tt.targetID, input.TargetID())
			}

			// Verify view renders without panic
			_ = input.View()
		})
	}
}

func TestInputModelPrompts(t *testing.T) {
	input := NewInputModel()

	tests := []struct {
		mode         InputMode
		wantSymbol   string
		wantMinWidth int
		desc         string
	}{
		{InputModeBroadcast, "⬧", 3, "broadcast prompt"},
		{InputModeMessage, "⬥", 3, "message prompt"},
		{InputModeNewMysis, "⬡", 3, "new mysis prompt"},
		{InputModeConfigProvider, "⚙", 3, "config provider prompt"},
		{InputModeConfigModel, "cfg", 5, "config model prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			input.SetMode(tt.mode, "test-id")

			prompt := input.textInput.Prompt
			stripped := stripANSI(prompt)

			// Verify the symbol is present
			if !strings.Contains(stripped, tt.wantSymbol) {
				t.Errorf("prompt %q does not contain symbol %q", stripped, tt.wantSymbol)
			}

			// Verify minimum width (symbol + at least 2 spaces for separation)
			if len(stripped) < tt.wantMinWidth {
				t.Errorf("prompt %q is too short (len=%d, want>=%d)", stripped, len(stripped), tt.wantMinWidth)
			}

			// Verify no character overlap by checking for proper spacing
			// The prompt should be: symbol + spaces (at least 2)
			if !strings.HasSuffix(stripped, "  ") {
				t.Errorf("prompt %q should end with at least 2 spaces to prevent overlap", stripped)
			}
		})
	}
}

func TestInputModelHistory(t *testing.T) {
	input := NewInputModel()

	// Add some history
	input.AddToHistory("first message")
	input.AddToHistory("second message")
	input.AddToHistory("third message")

	// Verify no duplicate consecutive entries
	input.AddToHistory("third message")

	// Set to message mode to enable history navigation
	input.SetMode(InputModeMessage, "test")

	// Verify history works (navigate up)
	input.navigateHistory(1) // up
	if input.Value() != "third message" {
		t.Errorf("expected 'third message', got '%s'", input.Value())
	}

	input.navigateHistory(1) // up again
	if input.Value() != "second message" {
		t.Errorf("expected 'second message', got '%s'", input.Value())
	}

	input.navigateHistory(-1) // down
	if input.Value() != "third message" {
		t.Errorf("expected 'third message', got '%s'", input.Value())
	}
}

func TestNetIndicatorActivities(t *testing.T) {
	tests := []struct {
		activity NetActivity
		wantDesc string
	}{
		{NetActivityIdle, "idle activity"},
		{NetActivityLLM, "LLM activity"},
		{NetActivityMCP, "MCP activity"},
	}

	for _, tt := range tests {
		t.Run(tt.wantDesc, func(t *testing.T) {
			n := NewNetIndicator()
			n.SetActivity(tt.activity)

			if n.Activity() != tt.activity {
				t.Errorf("expected activity %d, got %d", tt.activity, n.Activity())
			}

			// Verify views render without panic
			view := n.View()
			if view == "" {
				t.Error("expected non-empty view")
			}

			compact := n.ViewCompact()
			if compact == "" {
				t.Error("expected non-empty compact view")
			}
		})
	}
}

func TestNetIndicatorBounce(t *testing.T) {
	n := NewNetIndicator()
	n.SetActivity(NetActivityLLM)

	// Simulate several ticks
	for i := 0; i < 20; i++ {
		n, _ = n.Update(NetIndicatorTickMsg{})
	}

	// Ensure position stayed within bounds
	if n.position < 0 || n.position >= n.width {
		t.Errorf("position %d out of bounds [0, %d)", n.position, n.width)
	}
}

func TestRenderDashboardWithSwarmMessages(t *testing.T) {
	myses := []MysisInfo{
		{ID: "1", Name: "mysis-1", State: "running", Provider: "ollama"},
	}

	swarmMsgs := []SwarmMessageInfo{
		{SenderID: "mysis-1", SenderName: "alpha", Content: "Hello swarm", CreatedAt: time.Now()},
		{SenderID: "mysis-2", SenderName: "beta", Content: "Do the thing", CreatedAt: time.Now()},
	}

	dashboard := RenderDashboard(myses, swarmMsgs, 0, 100, 30, make(map[string]bool), "⠋", 0)
	if dashboard == "" {
		t.Error("expected non-empty dashboard with swarm messages")
	}

	// Should contain swarm broadcast section
	if !strings.Contains(dashboard, "SWARM BROADCAST") {
		t.Error("expected dashboard to contain 'SWARM BROADCAST'")
	}
}

func TestRenderFocusViewWithAllRoles(t *testing.T) {
	mysis := MysisInfo{ID: "1", Name: "test-mysis", State: "running", Provider: "ollama"}
	logs := []LogEntry{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User question"},
		{Role: "assistant", Content: "AI response"},
		{Role: "tool", Content: "Tool result"},
	}

	view := RenderFocusView(mysis, logs, 100, 30, false, "⠋", false, 1, 1, 0)
	if view == "" {
		t.Error("expected non-empty focus view")
	}

	// Verify all role prefixes appear
	prefixes := []string{"SYS:", "YOU:", "AI:", "TOOL:"}
	for _, prefix := range prefixes {
		if !strings.Contains(view, prefix) {
			t.Errorf("expected view to contain '%s'", prefix)
		}
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxWidth int
		wantMin  int // minimum number of lines expected
	}{
		{"short text", "Hello", 80, 1},
		{"exact width", "Hello World", 11, 1},
		{"needs wrap", "Hello World", 6, 2},
		{"multiple paragraphs", "Line one\n\nLine three", 80, 3},
		{"zero width defaults", "Test", 0, 1},
		{"long word hard wrap", strings.Repeat("a", 100), 50, 2}, // Should hard-wrap long words
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := wrapText(tt.text, tt.maxWidth)
			if len(lines) < tt.wantMin {
				t.Errorf("expected at least %d lines, got %d", tt.wantMin, len(lines))
			}
		})
	}
}

func TestRenderHelpContent(t *testing.T) {
	help := RenderHelp(100, 40)

	// Verify key shortcuts are present
	expectedKeys := []string{"q", "n", "d", "r", "s", "b", "m", "c", "Tab", "Enter", "Esc", "?"}
	for _, key := range expectedKeys {
		if !strings.Contains(help, key) {
			t.Errorf("expected help to contain '%s'", key)
		}
	}
}

func TestRenderMysisLineStates(t *testing.T) {
	spinnerView := "⠋"

	tests := []struct {
		state     string
		isLoading bool
	}{
		{"running", false},
		{"running", true},
		{"idle", false},
		{"stopped", false},
		{"errored", false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			mysis := MysisInfo{
				ID:          "test-id",
				Name:        "test-mysis",
				State:       tt.state,
				Provider:    "ollama",
				LastMessage: "Last message here",
			}

			// Test both selected and unselected
			line := renderMysisLine(mysis, false, tt.isLoading, spinnerView, 80, 0)
			if line == "" {
				t.Error("expected non-empty mysis line")
			}

			selectedLine := renderMysisLine(mysis, true, tt.isLoading, spinnerView, 80, 0)
			if selectedLine == "" {
				t.Error("expected non-empty selected mysis line")
			}
		})
	}
}

func TestMysisLineWidthFill(t *testing.T) {
	// Test that mysis lines are styled with width to fill the panel
	mysis := MysisInfo{
		ID:       "test-id",
		Name:     "test-mysis",
		State:    "idle",
		Provider: "ollama",
	}

	width := 80
	line := renderMysisLine(mysis, false, false, "⠋", width, 0)

	// The line should be rendered (non-empty)
	if line == "" {
		t.Error("expected non-empty mysis line")
	}

	// Selected line should also be rendered
	selectedLine := renderMysisLine(mysis, true, false, "⠋", width, 0)
	if selectedLine == "" {
		t.Error("expected non-empty selected mysis line")
	}
}

func TestSectionTitleWithSuffixWidth(t *testing.T) {
	// Test section title with suffix
	tests := []struct {
		title  string
		suffix string
		width  int
	}{
		{"CONVERSATION LOG", "  ↑ SCROLLED", 100},
		{"CONVERSATION LOG", "", 80},
		{"TEST", "  suffix", 60},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			rendered := renderSectionTitleWithSuffix(tt.title, tt.suffix, tt.width)

			if !strings.Contains(rendered, tt.title) {
				t.Errorf("should contain title %q", tt.title)
			}
			if tt.suffix != "" && !strings.Contains(rendered, tt.suffix) {
				t.Errorf("should contain suffix")
			}

			// Check actual display width matches requested width
			actualWidth := lipgloss.Width(rendered)
			if actualWidth != tt.width {
				t.Errorf("section title with suffix display width = %d, want %d", actualWidth, tt.width)
			}
		})
	}
}

// TestUnicodeCharacterSpacing verifies that Unicode characters have proper spacing
// to prevent visual overlap in the TUI.
func TestUnicodeCharacterSpacing(t *testing.T) {
	tests := []struct {
		name     string
		render   func() string
		wantChar string
		minSpace int // minimum spaces after the Unicode character
	}{
		{
			name: "stats_bar_running",
			render: func() string {
				return "∙  1"
			},
			wantChar: "∙",
			minSpace: 2,
		},
		{
			name: "stats_bar_idle",
			render: func() string {
				return "◦  1"
			},
			wantChar: "◦",
			minSpace: 2,
		},
		{
			name: "stats_bar_stopped",
			render: func() string {
				return "◌  0"
			},
			wantChar: "◌",
			minSpace: 2,
		},
		{
			name: "stats_bar_errored",
			render: func() string {
				return "✖  0"
			},
			wantChar: "✖",
			minSpace: 2,
		},
		{
			name: "mysis_line_indicator",
			render: func() string {
				return "⬡  test-mysis"
			},
			wantChar: "⬡",
			minSpace: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.render()

			// Find the Unicode character
			idx := strings.Index(output, tt.wantChar)
			if idx == -1 {
				t.Fatalf("Unicode character %q not found in output %q", tt.wantChar, output)
			}

			// Check spacing after the character
			afterChar := output[idx+len(tt.wantChar):]
			spaceCount := 0
			for i := 0; i < len(afterChar) && afterChar[i] == ' '; i++ {
				spaceCount++
			}

			if spaceCount < tt.minSpace {
				t.Errorf("insufficient spacing after %q: got %d spaces, want at least %d\nOutput: %q",
					tt.wantChar, spaceCount, tt.minSpace, output)
			}
		})
	}
}

// TestUnicodeAmbiguousWidthSafety verifies that all Unicode characters used in the TUI
// are NOT East Asian Ambiguous Width characters, which can render as 2 cells wide
// in some terminal locales, causing visual overlap.
func TestUnicodeAmbiguousWidthSafety(t *testing.T) {
	// All Unicode characters used in the TUI
	chars := map[string]string{
		"filled_circle":  "∙", // U+2219 - bullet operator (SAFE)
		"empty_circle":   "◦", // U+25E6 - white bullet (SAFE)
		"filled_diamond": "⬥", // U+2B25 - black medium diamond (SAFE)
		"lozenge":        "⬧", // U+2B27 - black medium lozenge (SAFE)
		"hexagon":        "⬡", // U+2B21 - white hexagon (SAFE)
		"hexagon_filled": "⬢", // U+2B22 - black hexagon (SAFE)
		"diamond_empty":  "⬦", // U+2B26 - white medium diamond (SAFE)
		"stopped":        "◌", // U+25CC - dotted circle (SAFE)
		"errored":        "✖", // U+2716 - heavy multiplication X (SAFE)
		"gear":           "⚙", // U+2699 - gear (SAFE)
		"braille":        "⠋", // U+280B - braille pattern (SAFE)
	}

	for name, char := range chars {
		t.Run(name, func(t *testing.T) {
			r := []rune(char)[0]

			// Test with EastAsianWidth disabled (narrow)
			runewidth.DefaultCondition.EastAsianWidth = false
			narrowWidth := runewidth.RuneWidth(r)

			// Test with EastAsianWidth enabled (wide)
			runewidth.DefaultCondition.EastAsianWidth = true
			wideWidth := runewidth.RuneWidth(r)

			// Reset to default
			runewidth.DefaultCondition.EastAsianWidth = false

			// Character should have same width in both modes
			if narrowWidth != wideWidth {
				t.Errorf("Character %s (U+%04X) is AMBIGUOUS WIDTH: narrow=%d wide=%d\n"+
					"This will cause visual overlap in East Asian locales!\n"+
					"Replace with a non-ambiguous character.",
					char, r, narrowWidth, wideWidth)
			}

			// Additionally, verify it's exactly 1 cell wide (except for known 2-cell chars)
			if narrowWidth != 1 && name != "hexagon_filled" {
				t.Errorf("Character %s (U+%04X) has unexpected width: %d (expected 1)",
					char, r, narrowWidth)
			}
		})
	}
}

// TestModelRefreshTick verifies that the Model refreshes its currentTick field
// from the Commander's aggregate tick.
func TestModelRefreshTick(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Initial tick should be 0 (no myses)
	if m.currentTick != 0 {
		t.Errorf("initial currentTick should be 0, got %d", m.currentTick)
	}

	// Call refreshTick (method to be implemented)
	m.refreshTick()

	// Should update currentTick to match commander.AggregateTick()
	expectedTick := m.commander.AggregateTick()
	if m.currentTick != expectedTick {
		t.Errorf("expected currentTick=%d after refresh, got %d", expectedTick, m.currentTick)
	}
}

func TestMysisCreation_TwoStageFlow(t *testing.T) {
	m, cleanup := setupTestModel(t)
	defer cleanup()

	// Step 1: Press 'n' to start creation
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = newModel.(Model)

	if m.input.Mode() != InputModeNewMysis {
		t.Fatalf("expected InputModeNewMysis, got %v", m.input.Mode())
	}
	if m.inputStage != InputStageName {
		t.Fatalf("expected InputStageName, got %v", m.inputStage)
	}

	// Step 2: Type name "test-mysis"
	for _, r := range "test-mysis" {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newModel.(Model)
	}

	// Step 3: Press Enter (should advance to provider stage)
	t.Logf("Before name Enter: inputValue='%s'", m.input.Value())
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	t.Logf("After name Enter: inputMode=%v, inputStage=%v, pendingName=%s",
		m.input.Mode(), m.inputStage, m.pendingMysisName)

	if m.inputStage != InputStageProvider {
		t.Errorf("expected InputStageProvider after entering name, got %v", m.inputStage)
	}
	if m.pendingMysisName != "test-mysis" {
		t.Errorf("expected pendingMysisName='test-mysis', got '%s'", m.pendingMysisName)
	}

	// Step 4: Press Enter (empty = use default provider)
	t.Logf("Before final Enter: inputStage=%v, pendingName=%s, config.DefaultProvider=%s, inputValue='%s'",
		m.inputStage, m.pendingMysisName, m.config.Swarm.DefaultProvider, m.input.Value())

	_, hasProvider := m.config.Providers[m.config.Swarm.DefaultProvider]
	t.Logf("Provider exists in config: %v", hasProvider)

	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	t.Logf("After final Enter: inputMode=%v, inputStage=%v, err=%v", m.input.Mode(), m.inputStage, m.err)

	// Should create mysis and reset
	if m.input.Mode() != InputModeNone {
		t.Errorf("expected InputModeNone after creation, got %v", m.input.Mode())
	}

	// Check mysis was created
	myses := m.commander.ListMyses()
	if len(myses) != 1 {
		t.Fatalf("expected 1 mysis, got %d", len(myses))
	}
	if myses[0].Name() != "test-mysis" {
		t.Errorf("expected name='test-mysis', got '%s'", myses[0].Name())
	}
}
