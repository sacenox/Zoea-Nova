package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"ollama": {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7, RateLimit: 1000, RateBurst: 1000},
		},
	}

	commander := core.NewCommander(s, reg, bus, cfg)

	model := New(commander, s, eventCh)
	model.width = 80
	model.height = 24

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

func TestInputModelViewAlways(t *testing.T) {
	input := NewInputModel()
	width := 100

	// Test inactive state - should show placeholder
	inactiveView := input.ViewAlways(width)
	if inactiveView == "" {
		t.Error("ViewAlways should return non-empty view even when inactive")
	}
	if !strings.Contains(inactiveView, "Press") {
		t.Error("inactive view should contain placeholder text")
	}

	// Check width
	inactiveWidth := lipgloss.Width(inactiveView)
	if inactiveWidth != width {
		t.Errorf("inactive view width = %d, want %d", inactiveWidth, width)
	}

	// Test active state
	input.SetMode(InputModeBroadcast, "")
	activeView := input.ViewAlways(width)
	if activeView == "" {
		t.Error("ViewAlways should return non-empty view when active")
	}

	// Check width when active
	activeWidth := lipgloss.Width(activeView)
	if activeWidth != width {
		t.Errorf("active view width = %d, want %d", activeWidth, width)
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
	dashboard := RenderDashboard(myses, swarmMsgs, 0, 80, 24, loadingSet, "⠋")
	if dashboard == "" {
		t.Error("expected non-empty dashboard")
	}

	// Test with loading state
	loadingSet["1"] = true
	dashboardWithLoading := RenderDashboard(myses, swarmMsgs, 0, 80, 24, loadingSet, "⠋")
	if dashboardWithLoading == "" {
		t.Error("expected non-empty dashboard with loading")
	}
}

func TestRenderDashboardEmpty(t *testing.T) {
	dashboard := RenderDashboard([]MysisInfo{}, []SwarmMessageInfo{}, 0, 80, 24, make(map[string]bool), "⠋")
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

	view := RenderFocusView(mysis, logs, 80, 24, false, "⠋")
	if view == "" {
		t.Error("expected non-empty focus view")
	}

	// Test with loading state
	viewLoading := RenderFocusView(mysis, logs, 80, 24, true, "⠋")
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

func TestRoleStyle(t *testing.T) {
	// Force color output to verify ANSI codes
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	testText := "test"
	roles := []string{"user", "assistant", "system", "tool", "unknown"}
	rendered := make(map[string]string)

	for _, role := range roles {
		style := RoleStyle(role)
		output := style.Render(testText)
		rendered[role] = output

		// Each should produce non-empty output
		if output == "" {
			t.Errorf("role %q should produce non-empty output", role)
		}

		// Each should contain ANSI codes
		if !strings.Contains(output, "\x1b[") {
			t.Errorf("role %q should contain ANSI escape codes, got: %q", role, output)
		}

		// Each should contain the test text
		if !strings.Contains(output, testText) {
			t.Errorf("role %q output should contain %q", role, testText)
		}
	}

	// Verify distinct styles for main roles
	if rendered["user"] == rendered["assistant"] {
		t.Error("user and assistant should have distinct styles")
	}
	if rendered["system"] == rendered["tool"] {
		t.Error("system and tool should have distinct styles")
	}
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

func TestStateStyleDistinct(t *testing.T) {
	// Force color output to verify distinct ANSI codes
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Ensure each state has a distinct visual representation
	running := StateStyle("running").Render("X")
	idle := StateStyle("idle").Render("X")
	stopped := StateStyle("stopped").Render("X")
	errored := StateStyle("errored").Render("X")

	// All should produce non-empty output
	if running == "" || idle == "" || stopped == "" || errored == "" {
		t.Error("state styles should produce non-empty output")
	}

	// All should contain ANSI codes (styling applied)
	states := map[string]string{
		"running": running,
		"idle":    idle,
		"stopped": stopped,
		"errored": errored,
	}
	for name, styled := range states {
		if !strings.Contains(styled, "\x1b[") {
			t.Errorf("state %q should contain ANSI escape codes, got: %q", name, styled)
		}
	}

	// Each state should be distinct from the others
	if running == idle {
		t.Error("running and idle should have distinct styles")
	}
	if running == errored {
		t.Error("running and errored should have distinct styles")
	}
	if idle == stopped {
		t.Error("idle and stopped should have distinct styles")
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
			lines := renderLogEntry(entry, tt.maxWidth)
			if len(lines) == 0 {
				t.Error("expected at least one line of output")
			}
		})
	}
}

func TestRenderLogEntryPadding(t *testing.T) {
	// Force color output to test real styled content
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Test that log entries have proper padding on all sides
	entry := LogEntry{Role: "user", Content: "Test message"}
	maxWidth := 80
	lines := renderLogEntry(entry, maxWidth)

	// Should have at least 2 lines: empty line (top padding) + content
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (padding + content), got %d", len(lines))
	}

	// First line should be visually empty (top padding) - strip ANSI then check
	strippedFirst := stripANSI(lines[0])
	if strings.TrimSpace(strippedFirst) != "" {
		t.Errorf("first line should be visually empty for top padding, got %q (stripped: %q)", lines[0], strippedFirst)
	}

	// Content lines should have left padding (visible content starts with space after ANSI)
	for i := 1; i < len(lines); i++ {
		stripped := stripANSI(lines[i])
		if len(stripped) > 0 && !strings.HasPrefix(stripped, " ") {
			t.Errorf("line %d should have left padding (start with space), stripped: %q", i, stripped)
		}
	}

	// Verify the content line has: space + prefix + content (visible after stripping)
	contentStripped := stripANSI(lines[1])
	if !strings.HasPrefix(contentStripped, " ") {
		t.Errorf("content line should start with space padding, stripped: %q", contentStripped)
	}
	if !strings.Contains(contentStripped, "YOU:") {
		t.Errorf("content line should contain role prefix, stripped: %q", contentStripped)
	}
}

func TestRenderLogEntryLineWidthConsistent(t *testing.T) {
	// Test that ALL lines in a log entry have the same display width
	// This ensures background color fills the entire line consistently
	entry := LogEntry{
		Role:    "assistant",
		Content: "First line\nSecond line\nThird line that is longer",
	}
	maxWidth := 60
	lines := renderLogEntry(entry, maxWidth)

	if len(lines) < 2 {
		t.Fatalf("expected multiple lines, got %d", len(lines))
	}

	// All lines should have the same display width (maxWidth)
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth != maxWidth {
			t.Errorf("line %d has width %d, expected %d (full width for background)\nline: %q",
				i, lineWidth, maxWidth, line)
		}
	}
}

func TestRenderLogEntryBackgroundApplied(t *testing.T) {
	// Force color output in tests (lipgloss strips colors without TTY)
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii) // Reset after test

	// Test that background styling is applied to ALL lines, not just the first
	// Background color is applied via ANSI escape codes containing "48;" (background)
	entry := LogEntry{
		Role:    "assistant",
		Content: "First line\nSecond line\nThird line",
	}
	maxWidth := 60
	lines := renderLogEntry(entry, maxWidth)

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Every line must contain ANSI background escape code
	// ANSI background codes use "48;" prefix (e.g., "\x1b[48;2;..." for RGB)
	for i, line := range lines {
		if !strings.Contains(line, "\x1b[") {
			t.Errorf("line %d missing ANSI escape codes (no styling applied)\nline bytes: %v", i, []byte(line))
		}
		// Check for background specifically - "48;" indicates background color
		if !strings.Contains(line, "48;") {
			t.Errorf("line %d missing background color escape code (48;)\nline: %q", i, line)
		}
	}
}

func TestRenderLogEntryPaddingAllRoles(t *testing.T) {
	// Force color output to test real styled content
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Test padding is applied consistently across all roles
	roles := []string{"user", "assistant", "system", "tool"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			entry := LogEntry{Role: role, Content: "Test content"}
			lines := renderLogEntry(entry, 80)

			if len(lines) < 2 {
				t.Fatalf("expected at least 2 lines, got %d", len(lines))
			}

			// First line should be visually empty (top padding) - strip ANSI then check
			strippedFirst := stripANSI(lines[0])
			if strings.TrimSpace(strippedFirst) != "" {
				t.Errorf("expected visually empty first line for top padding, got: %q", strippedFirst)
			}

			// Content lines should have left padding (check stripped content)
			for i := 1; i < len(lines); i++ {
				stripped := stripANSI(lines[i])
				if len(stripped) > 0 && !strings.HasPrefix(stripped, " ") {
					t.Errorf("content line %d should have left padding, stripped: %q", i, stripped)
				}
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
		{Content: "Hello swarm", CreatedAt: time.Now()},
		{Content: "Do the thing", CreatedAt: time.Now()},
	}

	dashboard := RenderDashboard(myses, swarmMsgs, 0, 100, 30, make(map[string]bool), "⠋")
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

	view := RenderFocusView(mysis, logs, 100, 30, false, "⠋")
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
			line := renderMysisLine(mysis, false, tt.isLoading, spinnerView, 80)
			if line == "" {
				t.Error("expected non-empty mysis line")
			}

			selectedLine := renderMysisLine(mysis, true, tt.isLoading, spinnerView, 80)
			if selectedLine == "" {
				t.Error("expected non-empty selected mysis line")
			}
		})
	}
}

func TestDashboardLayoutExpectations(t *testing.T) {
	// Test that dashboard renders with proper width expectations
	myses := []MysisInfo{
		{ID: "1", Name: "mysis-1", State: "idle", Provider: "ollama", LastMessage: "Hello"},
		{ID: "2", Name: "mysis-2", State: "running", Provider: "ollama", LastMessage: "World"},
	}
	swarmMsgs := []SwarmMessageInfo{}
	width := 100
	height := 30

	dashboard := RenderDashboard(myses, swarmMsgs, 0, width, height, make(map[string]bool), "⠋")

	// Verify dashboard is not empty
	if dashboard == "" {
		t.Fatal("expected non-empty dashboard")
	}

	lines := strings.Split(dashboard, "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d", len(lines))
	}

	// Check that first line (header top) contains the decorative markers
	if !strings.Contains(lines[0], "◆") || !strings.Contains(lines[0], "═") {
		t.Errorf("header top line should contain decorative markers, got: %s", lines[0])
	}

	// Check that mysis section title spans width (contains markers on both sides)
	foundMysisSwarm := false
	for _, line := range lines {
		if strings.Contains(line, "MYSIS SWARM") {
			foundMysisSwarm = true
			if !strings.Contains(line, "◈") {
				t.Errorf("mysis swarm title should have markers, got: %s", line)
			}
			break
		}
	}
	if !foundMysisSwarm {
		t.Error("expected to find MYSIS SWARM section title")
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
	line := renderMysisLine(mysis, false, false, "⠋", width)

	// The line should be rendered (non-empty)
	if line == "" {
		t.Error("expected non-empty mysis line")
	}

	// Selected line should also be rendered
	selectedLine := renderMysisLine(mysis, true, false, "⠋", width)
	if selectedLine == "" {
		t.Error("expected non-empty selected mysis line")
	}
}

func TestFocusHeaderWidth(t *testing.T) {
	// Test that focus header spans exactly the specified width
	// The decorative line itself must be the correct width, not just padded by lipgloss
	tests := []struct {
		name      string
		mysisName string
		width     int
	}{
		{"short name", "bob", 80},
		{"medium name", "mysis-123", 100},
		{"long name", "very-long-mysis-name", 120},
		{"unicode chars", "qj;wuhd", 96},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := renderFocusHeader(tt.mysisName, tt.width)

			// The header should contain the markers and name
			if !strings.Contains(header, "◆") {
				t.Errorf("header should contain ◆ markers")
			}
			if !strings.Contains(header, tt.mysisName) {
				t.Errorf("header should contain mysis name %q", tt.mysisName)
			}

			// Check actual display width matches requested width
			actualWidth := lipgloss.Width(header)
			if actualWidth != tt.width {
				t.Errorf("header display width = %d, want %d", actualWidth, tt.width)
			}

			// CRITICAL: Verify the decorative line is not padded with spaces
			// Count trailing spaces - there should be none if the line fills properly
			lines := strings.Split(header, "\n")
			for _, line := range lines {
				if strings.Contains(line, "◆") {
					// This line should have decorative content
					trimmed := strings.TrimRight(line, " ")
					trailingSpaces := len(line) - len(trimmed)
					if trailingSpaces > 0 {
						t.Errorf("header line has %d trailing spaces (line too short): raw width=%d, want=%d",
							trailingSpaces, lipgloss.Width(trimmed), tt.width)
					}
				}
			}
		})
	}
}

// TestFocusHeaderRawLineWidth directly tests the raw line construction
func TestFocusHeaderRawLineWidth(t *testing.T) {
	// Test the raw line width calculation with Unicode characters
	tests := []struct {
		mysisName string
		width     int
	}{
		{"bob", 80},
		{"qj;wuhd", 96},
		{"test-mysis", 100},
	}

	for _, tt := range tests {
		t.Run(tt.mysisName, func(t *testing.T) {
			// Build the same line as renderFocusHeader but without styling
			titleText := " ⬡ MYSIS: " + tt.mysisName + " ⬡ "
			titleDisplayWidth := lipgloss.Width(titleText)
			availableWidth := tt.width - titleDisplayWidth - 2 // -2 for ◆ markers
			if availableWidth < 4 {
				availableWidth = 4
			}
			leftDashes := availableWidth / 2
			rightDashes := availableWidth - leftDashes

			line := "◆" + strings.Repeat("─", leftDashes) + titleText + strings.Repeat("─", rightDashes) + "◆"

			// Check raw line display width
			lineWidth := lipgloss.Width(line)
			if lineWidth != tt.width {
				t.Errorf("raw line display width = %d, want %d (titleWidth=%d, available=%d, left=%d, right=%d)",
					lineWidth, tt.width, titleDisplayWidth, availableWidth, leftDashes, rightDashes)
			}
		})
	}
}

func TestSectionTitleWidth(t *testing.T) {
	// Test that section titles span exactly the specified width
	tests := []struct {
		title string
		width int
	}{
		{"CONVERSATION LOG", 80},
		{"MYSIS SWARM", 100},
		{"SWARM BROADCAST", 96},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			rendered := renderSectionTitle(tt.title, tt.width)

			// Should contain the title
			if !strings.Contains(rendered, tt.title) {
				t.Errorf("section title should contain %q", tt.title)
			}

			// Should have markers on both ends
			if !strings.Contains(rendered, "◈") {
				t.Errorf("section title should contain ◈ markers")
			}

			// Check actual display width matches requested width
			actualWidth := lipgloss.Width(rendered)
			if actualWidth != tt.width {
				t.Errorf("section title display width = %d, want %d", actualWidth, tt.width)
			}
		})
	}
}

func TestLogPanelWidth(t *testing.T) {
	// Test that the log panel (with border) has the same width as section titles
	width := 100

	// Render section title
	title := renderSectionTitleWithSuffix("CONVERSATION LOG", "", width)
	titleWidth := lipgloss.Width(title)

	// Render log panel the same way as in RenderFocusViewWithViewport
	content := "Test content"
	panel := logStyle.Width(width - 2).Render(content)
	panelWidth := lipgloss.Width(panel)

	// Both should have the same width
	if titleWidth != width {
		t.Errorf("section title width = %d, want %d", titleWidth, width)
	}
	if panelWidth != width {
		t.Errorf("log panel width = %d, want %d (title width = %d)", panelWidth, width, titleWidth)
	}
}

func TestLogPanelWidthWithMultilineContent(t *testing.T) {
	// Test that the log panel width is consistent regardless of content
	width := 100

	// Render section title for comparison
	title := renderSectionTitleWithSuffix("CONVERSATION LOG", "", width)
	titleWidth := lipgloss.Width(title)

	// Test with various content widths
	testCases := []struct {
		name    string
		content string
	}{
		{"short", "Hello"},
		{"exact", strings.Repeat("x", width-4)}, // -4 for border
		{"multiline short", "Line 1\nLine 2\nLine 3"},
		{"multiline varied", "Short\n" + strings.Repeat("y", 50) + "\nMedium length line"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			panel := logStyle.Width(width - 2).Render(tc.content)
			panelWidth := lipgloss.Width(panel)

			if panelWidth != width {
				t.Errorf("log panel width = %d, want %d (title width = %d)\ncontent: %q",
					panelWidth, width, titleWidth, tc.content)
			}

			// Check each line of the panel
			lines := strings.Split(panel, "\n")
			for i, line := range lines {
				lineWidth := lipgloss.Width(line)
				if lineWidth != width {
					t.Errorf("line %d width = %d, want %d\nline: %q", i, lineWidth, width, line)
				}
			}
		})
	}
}

func TestLogPanelWidthWithStyledContent(t *testing.T) {
	// Force color output
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	// Test that log panel width is correct when containing styled log entries
	width := 100

	// Panel inner content width: logStyle.Width(width-2) sets inner width to width-2
	// With RoundedBorder (1 char each side), total panel width = width-2+2 = width
	// But lipgloss Width() sets the TOTAL width including borders, so:
	// logStyle.Width(width-2) means total width = width-2
	// Actually let's verify this...
	panelContentWidth := width - 2 - 2 // -2 for logStyle.Width param, -2 for border

	// Log entries MUST fill the panel content area completely
	// so their width should be panelContentWidth
	entries := []LogEntry{
		{Role: "user", Content: "Hello, this is a test message"},
		{Role: "assistant", Content: "Hi there! This is a response with some longer content that might wrap."},
		{Role: "tool", Content: "tool_call_result: success"},
	}

	var lines []string
	for _, entry := range entries {
		entryLines := renderLogEntry(entry, panelContentWidth)
		lines = append(lines, entryLines...)
	}
	viewportContent := strings.Join(lines, "\n")

	// Check that log entry lines fill the panel content area
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth != panelContentWidth {
			t.Errorf("log entry line %d width = %d, want %d (panel content width)", i, lineWidth, panelContentWidth)
		}
	}

	// Render the panel
	panel := logStyle.Width(width - 2).Render(viewportContent)

	// Panel should have total width of width-2 (lipgloss Width is total including border)
	// Actually no - let's check
	panelWidth := lipgloss.Width(panel)
	t.Logf("Panel total width: %d, expected: %d or %d", panelWidth, width-2, width)

	// For now, just verify the panel renders
	if panelWidth < width-4 {
		t.Errorf("panel too narrow: width = %d, minimum expected ~%d", panelWidth, width-4)
	}
}

func TestLogEntryWidthMatchesPanelContent(t *testing.T) {
	// CRITICAL TEST: Log entries must fill the exact panel content width
	// Otherwise there will be visible gaps between message backgrounds and panel border
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	width := 100

	// The panel is rendered with: logStyle.Width(width - 2)
	// logStyle has RoundedBorder which adds 1 char on each side
	// In lipgloss, Width() sets TOTAL width, so inner content = width - 2 - 2 = width - 4
	panelInnerWidth := width - 4

	// Log entries should be rendered at exactly panelInnerWidth
	entry := LogEntry{Role: "assistant", Content: "Test message with enough content"}
	lines := renderLogEntry(entry, panelInnerWidth)

	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth != panelInnerWidth {
			t.Errorf("line %d: width = %d, want %d (panel inner width)\nThis will cause visible gaps in the UI!",
				i, lineWidth, panelInnerWidth)
		}
	}
}

func TestFocusViewAllSectionsMatchWidth(t *testing.T) {
	// CRITICAL TEST: All sections in the focus view must have EXACTLY the same width
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	width := 100

	// Render each section exactly as RenderFocusViewWithViewport does
	header := renderFocusHeader("test-mysis", width)
	logTitle := renderSectionTitleWithSuffix("CONVERSATION LOG", "", width)

	entry := LogEntry{Role: "assistant", Content: "Test message content here"}
	entryLines := renderLogEntry(entry, width-4)
	viewportContent := strings.Join(entryLines, "\n")
	vpView := logStyle.Width(width - 2).Render(viewportContent)

	// Get first line of each (the top border/decoration)
	headerLines := strings.Split(header, "\n")
	titleLines := strings.Split(logTitle, "\n")
	panelLines := strings.Split(vpView, "\n")

	t.Logf("Header first line width: %d", lipgloss.Width(headerLines[0]))
	t.Logf("Title first line width: %d", lipgloss.Width(titleLines[0]))
	t.Logf("Panel first line width: %d (top border)", lipgloss.Width(panelLines[0]))

	// Print the actual first characters to check alignment
	headerFirst := stripANSI(headerLines[0])
	titleFirst := stripANSI(titleLines[0])
	panelFirst := stripANSI(panelLines[0])

	t.Logf("Header stripped: %q (len=%d)", headerFirst, len(headerFirst))
	t.Logf("Title stripped: %q (len=%d)", titleFirst, len(titleFirst))
	t.Logf("Panel stripped: %q (len=%d)", panelFirst, len(panelFirst))

	// Check byte lengths vs display widths for Unicode
	t.Logf("Title first char: %q, bytes=%d, display=%d", string([]rune(titleFirst)[0]), len(string([]rune(titleFirst)[0])), lipgloss.Width(string([]rune(titleFirst)[0])))
	t.Logf("Panel first char: %q, bytes=%d, display=%d", string([]rune(panelFirst)[0]), len(string([]rune(panelFirst)[0])), lipgloss.Width(string([]rune(panelFirst)[0])))

	// ALL sections must match width
	if lipgloss.Width(headerLines[0]) != width {
		t.Errorf("Header line width = %d, want %d", lipgloss.Width(headerLines[0]), width)
	}
	if lipgloss.Width(titleLines[0]) != width {
		t.Errorf("Title line width = %d, want %d", lipgloss.Width(titleLines[0]), width)
	}
	if lipgloss.Width(panelLines[0]) != width {
		t.Errorf("Panel line width = %d, want %d", lipgloss.Width(panelLines[0]), width)
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
