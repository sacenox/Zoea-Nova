// Package tui provides the terminal user interface for Zoea Nova.
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/store"
)

// View represents the current view mode.
type View int

const (
	ViewDashboard View = iota
	ViewFocus
)

// InputStage represents stages in multi-step input flows.
type InputStage int

const (
	InputStageName InputStage = iota
	InputStageProvider
)

// Model is the main TUI model.
type Model struct {
	commander *core.Commander
	store     *store.Store
	eventCh   <-chan core.Event
	config    *config.Config

	view        View
	width       int
	height      int
	selectedIdx int
	showHelp    bool
	verboseJSON bool // show full JSON trees without truncation

	input   InputModel
	myses   []MysisInfo
	logs    []LogEntry
	focusID string

	// Multi-stage input for mysis creation
	inputStage           InputStage
	pendingMysisName     string
	pendingMysisProvider string

	pendingProvider string
	startSwarm      bool // auto-start idle myses on launch

	// Swarm broadcast history
	swarmMessages []SwarmMessage

	// Current swarm aggregate tick
	currentTick int64

	// Test-only: override time.Now() for deterministic timestamps in tests
	testTime *time.Time

	spinner     spinner.Model
	loadingSet  map[string]bool // mysisIDs currently loading
	sending     bool
	sendingMode InputMode

	// Conversation viewport
	viewport           viewport.Model
	viewportTotalLines int // total lines in viewport content

	// Network activity indicator
	netIndicator     NetIndicator
	activeNetworkOps int // Count of active network operations (LLM/MCP calls)

	providerErrorTimes []time.Time

	onQuit func() // Callback to run before quitting
	err    error
}

const (
	providerErrorWindow    = 10 * time.Minute
	maxConversationEntries = 200 // Maximum conversation log entries to load for performance
)

// SwarmMessage represents a broadcast message for display.
type SwarmMessage struct {
	SenderID  string
	Content   string
	CreatedAt time.Time
}

// EventMsg wraps a core event for the TUI.
type EventMsg struct {
	Event core.Event
}

// New creates a new TUI model.
func New(commander *core.Commander, s *store.Store, eventCh <-chan core.Event, startSwarm bool, cfg *config.Config) Model {
	// Use provided config (already loaded in main.go)

	// Initialize spinner with hexagonal theme (matching logo)
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "⬦", "⬥", "⬦", "⬥"},
		FPS:    time.Second / 8, // 8 frames per second
	}
	sp.Style = lipgloss.NewStyle().Foreground(colorBrand)

	// Initialize viewport for conversation scrolling
	vp := viewport.New(80, 20)
	vp.Style = logStyle

	return Model{
		commander:    commander,
		store:        s,
		eventCh:      eventCh,
		config:       cfg,
		view:         ViewDashboard,
		input:        NewInputModel(),
		myses:        []MysisInfo{},
		inputStage:   InputStageName,
		spinner:      sp,
		loadingSet:   make(map[string]bool),
		viewport:     vp,
		netIndicator: NewNetIndicator(),
		startSwarm:   startSwarm,
	}
}

// SetOnQuit sets a callback to run before the TUI quits.
func (m *Model) SetOnQuit(fn func()) {
	m.onQuit = fn
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	// Auto-start idle myses if flag enabled
	if m.startSwarm {
		go m.autoStartIdleMyses()
	}

	return tea.Batch(
		m.netIndicator.Init(),
		m.refreshMyses(),
		m.listenForEvents(),
		m.spinner.Tick,
	)
}

// autoStartIdleMyses starts all myses in idle state (no error).
func (m Model) autoStartIdleMyses() {
	myses := m.commander.ListMyses()
	for _, mysis := range myses {
		// Start myses that are not errored (idle state)
		if mysis.LastError() == nil {
			_ = m.commander.StartMysis(mysis.ID())
		}
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	m.pruneProviderErrors(time.Now())

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)

		// Update viewport size (account for header, info panel, title, footer)
		headerHeight := 7 // 2 for header + 3 for info panel + 1 for conversation title + 1 margin
		footerHeight := 2
		vpHeight := msg.Height - headerHeight - footerHeight - 3
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = msg.Width - 4 // -2 for borders, -2 for scrollbar (space + char)
		m.viewport.Height = vpHeight

		// Re-render content if in focus view
		if m.view == ViewFocus {
			m.updateViewportContent()
		}

	case tea.KeyMsg:
		// Handle input mode first
		if m.input.IsActive() {
			return m.handleInputKey(msg)
		}

		// Handle help toggle
		if key.Matches(msg, keys.Help) {
			m.showHelp = !m.showHelp
			return m, nil
		}

		// Close help if shown
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Handle global keys
		switch {
		case key.Matches(msg, keys.Quit):
			// Call cleanup callback before quitting
			if m.onQuit != nil {
				m.onQuit()
			}
			return m, tea.Quit

		case key.Matches(msg, keys.Escape):
			if m.view == ViewFocus {
				m.view = ViewDashboard
				m.focusID = ""
			}
			return m, nil
		}

		// View-specific keys
		if m.view == ViewDashboard {
			return m.handleDashboardKey(msg)
		} else {
			return m.handleFocusKey(msg)
		}

	case EventMsg:
		// Don't re-schedule if channel closed (zero-value event)
		if msg.Event.Type == "" {
			return m, nil
		}
		m.handleEvent(msg.Event)
		return m, m.listenForEvents()

	case refreshMysesMsg:
		m.refreshMysisList()
		m.refreshSwarmMessages()
		m.refreshTick()

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case sendMessageResult:
		// Message finished sending (success or failure)
		delete(m.loadingSet, msg.mysisID)
		if msg.err != nil {
			m.err = msg.err
		}
		// Check if any loading is still active
		if len(m.loadingSet) == 0 {
			m.netIndicator.SetActivity(NetActivityIdle)
			m.sending = false
			m.sendingMode = InputModeNone
			m.input.Reset()
		}
		if m.view == ViewFocus && msg.mysisID == m.focusID {
			m.loadMysisLogs()
		}

	case broadcastResult:
		// Broadcast finished - clear all loading states
		m.loadingSet = make(map[string]bool)
		m.netIndicator.SetActivity(NetActivityIdle)
		m.sending = false
		m.sendingMode = InputModeNone
		m.input.Reset()
		if msg.err != nil {
			m.err = msg.err
		}
		// Refresh swarm messages to show the new broadcast
		m.refreshSwarmMessages()

	case NetIndicatorTickMsg:
		var cmd tea.Cmd
		m.netIndicator, cmd = m.netIndicator.Update(msg)
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Check for minimum terminal dimensions
	const minWidth = 80
	const minHeight = 20
	if m.width < minWidth || m.height < minHeight {
		warning := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true).
			Render(fmt.Sprintf("Terminal too small!\n\nMinimum size: %dx%d\nCurrent size: %dx%d\n\nPlease resize your terminal.",
				minWidth, minHeight, m.width, m.height))
		return warning
	}

	var content string

	// Check if currently focused mysis is loading
	isLoading := m.loadingSet[m.focusID]

	// Reserve space for status bar
	contentHeight := m.height - 1

	if m.showHelp {
		content = RenderHelp(m.width, contentHeight)
	} else if m.view == ViewFocus {
		focusIndex, totalMyses := m.focusPosition(m.focusID)
		content = RenderFocusViewWithViewport(m.mysisByID(m.focusID), m.viewport, m.width, isLoading, m.spinner.View(), m.verboseJSON, m.viewportTotalLines, focusIndex, totalMyses, m.currentTick)
	} else {
		// Convert swarm messages for display (reversed so most recent is first)
		swarmInfos := make([]SwarmMessageInfo, len(m.swarmMessages))
		for i, msg := range m.swarmMessages {
			// Reverse order: most recent first
			swarmInfos[len(m.swarmMessages)-1-i] = SwarmMessageInfo{
				SenderID:   msg.SenderID,
				SenderName: m.mysisNameByID(msg.SenderID),
				Content:    msg.Content,
				CreatedAt:  msg.CreatedAt,
			}
		}
		content = RenderDashboard(m.myses, swarmInfos, m.selectedIdx, m.width, contentHeight-3, m.loadingSet, m.spinner.View(), m.currentTick)
	}

	// Always show message bar
	sendingLabel := ""
	if m.sending {
		switch m.sendingMode {
		case InputModeBroadcast:
			sendingLabel = "Broadcasting..."
		case InputModeMessage:
			sendingLabel = "Sending message..."
		default:
			sendingLabel = "Sending..."
		}
	}
	content += "\n" + m.input.ViewAlways(m.width, m.sending, sendingLabel, m.spinner.View())

	// Add error if present
	if m.err != nil {
		errMsg := stateErroredStyle.Render(fmt.Sprintf("Error: %v", m.err))
		content += "\n" + errMsg
	}

	// Build status bar
	statusBar := m.renderStatusBar()

	return content + "\n" + statusBar
}

// renderStatusBar renders the bottom status bar with activity indicator, tick, and state counts.
// Layout: [activity indicator]  |  T#### ⬡ [HH:MM]  |  [state icons + counts]
func (m Model) renderStatusBar() string {
	// Left segment: Activity indicator (LLM/MCP/IDLE)
	leftSegment := m.netIndicator.View()

	// Middle segment: Tick + timestamp
	middleSegment := m.renderTickTimestamp()

	// Right segment: State counts with animated icons
	rightSegment := m.renderStateCounts()

	// Calculate widths
	leftWidth := lipgloss.Width(leftSegment)
	middleWidth := lipgloss.Width(middleSegment)
	rightWidth := lipgloss.Width(rightSegment)

	// Separator style
	separators := lipgloss.NewStyle().Foreground(colorMuted).Render(" | ")
	sepWidth := lipgloss.Width(separators)

	// Calculate center position for middle segment
	centerPos := m.width / 2
	middleStart := centerPos - (middleWidth / 2)

	// Calculate padding to center the middle segment
	// Left side: leftSegment + separator + padding
	leftSideWidth := leftWidth + sepWidth
	leftPad := middleStart - leftSideWidth
	if leftPad < 1 {
		leftPad = 1
	}

	// Right side: padding + separator + rightSegment
	rightSideStart := middleStart + middleWidth
	rightPad := m.width - rightSideStart - sepWidth - rightWidth
	if rightPad < 1 {
		rightPad = 1
	}

	// Style the status bar
	barStyle := lipgloss.NewStyle().
		Background(colorBorder).
		Foreground(colorBrand).
		Width(m.width)

	bar := leftSegment +
		separators +
		strings.Repeat(" ", leftPad) +
		middleSegment +
		strings.Repeat(" ", rightPad) +
		separators +
		rightSegment

	return barStyle.Render(bar)
}

// renderStateCounts renders the state counts with animated icons.
// Format: ⬡ 3  ◦ 2  ◌ 1  ✖ 0
func (m Model) renderStateCounts() string {
	// Count states
	counts := map[string]int{
		"running": 0,
		"idle":    0,
		"stopped": 0,
		"errored": 0,
	}

	for _, mysis := range m.myses {
		counts[mysis.State]++
	}

	// Get spinner frame for running/loading states
	spinnerFrame := m.spinner.View()

	// Build state count segments
	var parts []string

	// Running: animated spinner
	if counts["running"] > 0 {
		icon := lipgloss.NewStyle().Foreground(colorBrand).Render(spinnerFrame)
		count := fmt.Sprintf("%d", counts["running"])
		parts = append(parts, icon+" "+count)
	}

	// Idle: ◦
	if counts["idle"] > 0 {
		icon := stateIdleStyle.Render("◦")
		count := fmt.Sprintf("%d", counts["idle"])
		parts = append(parts, icon+" "+count)
	}

	// Stopped: ◌
	if counts["stopped"] > 0 {
		icon := stateStoppedStyle.Render("◌")
		count := fmt.Sprintf("%d", counts["stopped"])
		parts = append(parts, icon+" "+count)
	}

	// Errored: ✖
	if counts["errored"] > 0 {
		icon := stateErroredStyle.Render("✖")
		count := fmt.Sprintf("%d", counts["errored"])
		parts = append(parts, icon+" "+count)
	}

	if len(parts) == 0 {
		return dimmedStyle.Render("(no myses)")
	}

	return strings.Join(parts, "  ")
}

// renderTickTimestamp renders the tick + timestamp in format: T#### ⬡ [HH:MM]
func (m Model) renderTickTimestamp() string {
	// Get current time (or test time override)
	var now time.Time
	if m.testTime != nil {
		now = *m.testTime
	} else {
		now = time.Now()
	}

	// Use the shared formatter from styles.go
	// formatTickTimestamp returns pre-styled string with colors
	return formatTickTimestamp(m.currentTick, now)
}

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up), key.Matches(msg, keys.ShiftTab):
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case key.Matches(msg, keys.Down), key.Matches(msg, keys.Tab):
		if m.selectedIdx < len(m.myses)-1 {
			m.selectedIdx++
		}

	case key.Matches(msg, keys.Enter):
		if len(m.myses) > 0 && m.selectedIdx < len(m.myses) {
			m.focusID = m.myses[m.selectedIdx].ID
			m.view = ViewFocus
			m.loadMysisLogs()
			// Start at bottom when entering focus view
			m.viewport.GotoBottom()
		}

	case key.Matches(msg, keys.NewMysis):
		m.input.SetMode(InputModeNewMysis, "")
		m.inputStage = InputStageName // Reset to name stage
		m.pendingMysisName = ""
		m.pendingMysisProvider = ""
		return m, m.input.Focus()

	case key.Matches(msg, keys.Delete):
		if len(m.myses) > 0 && m.selectedIdx < len(m.myses) {
			id := m.myses[m.selectedIdx].ID
			m.err = m.commander.DeleteMysis(id, true)
			m.refreshMysisList()
			if m.selectedIdx >= len(m.myses) && m.selectedIdx > 0 {
				m.selectedIdx--
			}
		}

	case key.Matches(msg, keys.Relaunch):
		if len(m.myses) > 0 && m.selectedIdx < len(m.myses) {
			id := m.myses[m.selectedIdx].ID
			m.err = m.commander.StartMysis(id)
		}

	case key.Matches(msg, keys.Stop):
		if len(m.myses) > 0 && m.selectedIdx < len(m.myses) {
			id := m.myses[m.selectedIdx].ID
			m.err = m.commander.StopMysis(id)
		}

	case key.Matches(msg, keys.Broadcast):
		m.input.SetMode(InputModeBroadcast, "")
		return m, m.input.Focus()

	case key.Matches(msg, keys.Message):
		if len(m.myses) > 0 && m.selectedIdx < len(m.myses) {
			id := m.myses[m.selectedIdx].ID
			m.input.SetMode(InputModeMessage, id)
			return m, m.input.Focus()
		}

	case key.Matches(msg, keys.Configure):
		if len(m.myses) > 0 && m.selectedIdx < len(m.myses) {
			id := m.myses[m.selectedIdx].ID
			m.input.SetMode(InputModeConfigProvider, id)
			return m, m.input.Focus()
		}
	}

	return m, nil
}

func (m Model) handleFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Message):
		m.input.SetMode(InputModeMessage, m.focusID)
		return m, m.input.Focus()

	case key.Matches(msg, keys.Relaunch):
		m.err = m.commander.StartMysis(m.focusID)
		return m, nil

	case key.Matches(msg, keys.Stop):
		m.err = m.commander.StopMysis(m.focusID)
		return m, nil

	case key.Matches(msg, keys.Configure):
		m.input.SetMode(InputModeConfigProvider, m.focusID)
		return m, m.input.Focus()

	case key.Matches(msg, keys.Broadcast):
		m.input.SetMode(InputModeBroadcast, "")
		return m, m.input.Focus()

	case key.Matches(msg, keys.End):
		// Go to bottom
		m.viewport.GotoBottom()
		return m, nil

	case key.Matches(msg, keys.VerboseToggle):
		m.verboseJSON = !m.verboseJSON
		// Re-render viewport content with new verbose setting
		m.updateViewportContent()
		return m, nil
	}

	// Pass other keys to viewport for scrolling
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)

	return m, cmd
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.resetInput()
		m.pendingProvider = ""
		m.err = nil
		return m, nil

	case key.Matches(msg, keys.Enter):
		value := strings.TrimSpace(m.input.Value())

		// Allow empty values for optional fields (provider selection)
		// but block for required fields (handled per-mode)

		var cmd tea.Cmd

		switch m.input.Mode() {
		case InputModeBroadcast:
			if value == "" {
				m.input.Reset()
				return m, nil
			}
			// Add to history before sending
			m.input.AddToHistory(value)
			m.input.Reset()
			// Mark all running myses as loading
			myses := m.commander.ListMyses()
			for _, mysis := range myses {
				if mysis.State() == core.MysisStateRunning {
					m.loadingSet[mysis.ID()] = true
				}
			}
			m.netIndicator.SetActivity(NetActivityLLM)
			m.sending = true
			m.sendingMode = InputModeBroadcast
			// Use Broadcast to properly set source='broadcast'
			cmd = m.broadcastAsync(value)

		case InputModeMessage:
			// Add to history before sending
			m.input.AddToHistory(value)
			targetID := m.input.TargetID() // Save targetID BEFORE reset
			m.input.Reset()                // Clear input immediately after saving targetID
			m.loadingSet[targetID] = true
			m.netIndicator.SetActivity(NetActivityLLM)
			// Note: m.sending NOT set here - SendMessageAsync returns immediately
			// Network indicator shows activity, but input is ready for next message
			cmd = m.sendMessageAsync(targetID, value)
			if m.view == ViewFocus {
				m.loadMysisLogs()
			}

		case InputModeNewMysis:
			// Multi-stage flow: name → provider
			switch m.inputStage {
			case InputStageName:
				// Name is required
				if value == "" {
					m.err = fmt.Errorf("name cannot be empty")
					return m, nil
				}
				m.pendingMysisName = value
				m.inputStage = InputStageProvider
				m.input.textInput.SetValue("") // Clear input but keep mode
				m.input.textInput.Placeholder = fmt.Sprintf("Provider (empty for default: %s)...", m.config.Swarm.DefaultProvider)
				return m, nil

			case InputStageProvider:
				// Provider is optional - use default if empty
				provider := value
				if provider == "" {
					provider = m.config.Swarm.DefaultProvider
				}

				// Validate provider exists
				if _, ok := m.config.Providers[provider]; !ok {
					// Debug: show available providers
					var available []string
					for p := range m.config.Providers {
						available = append(available, p)
					}
					m.err = fmt.Errorf("provider '%s' not found. Available: %v", provider, available)
					m.resetInput()
					m.refreshMysisList()
					return m, nil
				}

				// Create mysis with selected provider
				mysis, err := m.commander.CreateMysis(m.pendingMysisName, provider)
				if err == nil {
					// Auto-start newly created myses
					m.err = m.commander.StartMysis(mysis.ID())
				} else {
					m.err = err
				}
				m.resetInput()
				m.refreshMysisList()
				return m, nil

			default:
				// Safety: reset if in unexpected state
				m.err = fmt.Errorf("unexpected input stage (%d), resetting", m.inputStage)
				m.resetInput()
				return m, nil
			}

		case InputModeConfigProvider:
			// Validate provider exists in config
			if _, ok := m.config.Providers[value]; !ok {
				var available []string
				for p := range m.config.Providers {
					available = append(available, p)
				}
				m.err = fmt.Errorf("provider '%s' not found. Available: %v", value, available)
				m.input.Reset()
				return m, nil
			}
			m.pendingProvider = value
			m.input.SetMode(InputModeConfigModel, m.input.TargetID())
			return m, m.input.Focus()

		case InputModeConfigModel:
			m.err = m.commander.ConfigureMysis(m.input.TargetID(), m.pendingProvider, value)
			m.pendingProvider = ""
		}

		m.input.Reset()
		return m, cmd
	}

	// Pass to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// sendMessageAsync sends a message to a mysis asynchronously.
func (m Model) sendMessageAsync(mysisID, content string) tea.Cmd {
	return func() tea.Msg {
		// Use async version to clear input immediately when message is sent,
		// not when LLM completes processing
		err := m.commander.SendMessageAsync(mysisID, content)
		return sendMessageResult{mysisID: mysisID, err: err}
	}
}

type broadcastResult struct {
	err error
}

func (m Model) broadcastAsync(content string) tea.Cmd {
	return func() tea.Msg {
		err := m.commander.Broadcast(content)
		return broadcastResult{err: err}
	}
}

func (m *Model) handleEvent(event core.Event) {
	switch event.Type {
	case core.EventMysisCreated, core.EventMysisDeleted, core.EventMysisStateChanged, core.EventMysisConfigChanged:
		m.refreshMysisList()

	case core.EventMysisResponse, core.EventMysisMessage:
		// Refresh dashboard to update last message
		m.refreshMysisList()
		// Refresh tick in case tool results updated server tick
		m.refreshTick()
		if m.view == ViewFocus && event.MysisID == m.focusID {
			m.loadMysisLogs()
		}

	case core.EventBroadcast:
		// Refresh swarm message history
		m.refreshSwarmMessages()

	case core.EventNetworkLLM:
		m.activeNetworkOps++
		m.netIndicator.SetActivity(NetActivityLLM)

	case core.EventNetworkMCP:
		m.activeNetworkOps++
		m.netIndicator.SetActivity(NetActivityMCP)

	case core.EventNetworkIdle:
		// Decrement active operations counter
		m.activeNetworkOps--
		if m.activeNetworkOps < 0 {
			m.activeNetworkOps = 0 // Safety: prevent negative counts
		}
		// Only go idle if no active network operations
		if m.activeNetworkOps == 0 {
			m.netIndicator.SetActivity(NetActivityIdle)
		}
		// Refresh tick when network goes idle (tool calls completed)
		m.refreshTick()

	case core.EventMysisError:
		if event.Error != nil {
			if strings.Contains(strings.ToLower(event.Error.Error), "provider chat") {
				m.recordProviderError(event.Timestamp)
			}
		}
	}

	// Clear error on successful events
	m.err = nil
}

func (m *Model) recordProviderError(ts time.Time) {
	if ts.IsZero() {
		ts = time.Now()
	}
	m.pruneProviderErrors(ts)
	m.providerErrorTimes = append(m.providerErrorTimes, ts)
}

func (m *Model) pruneProviderErrors(now time.Time) {
	cutoff := now.Add(-providerErrorWindow)
	filtered := m.providerErrorTimes[:0]
	for _, t := range m.providerErrorTimes {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	m.providerErrorTimes = filtered
}

func (m *Model) refreshMysisList() {
	myses := m.commander.ListMyses()
	m.myses = make([]MysisInfo, len(myses))
	for i, mysis := range myses {
		info := MysisInfoFromCore(mysis)

		// Fetch last message for this mysis
		memories, err := m.store.GetRecentMemories(mysis.ID(), 1)
		if err == nil && len(memories) > 0 {
			info.LastMessage = memories[0].Content
			info.LastMessageAt = memories[0].CreatedAt
		}

		m.myses[i] = info
	}

	// Sort by creation time (oldest first)
	sort.Slice(m.myses, func(i, j int) bool {
		return m.myses[i].CreatedAt.Before(m.myses[j].CreatedAt)
	})
}

func (m *Model) refreshSwarmMessages() {
	broadcasts, err := m.store.GetRecentBroadcasts(10)
	if err != nil {
		m.swarmMessages = nil
		return
	}

	m.swarmMessages = make([]SwarmMessage, len(broadcasts))
	for i, b := range broadcasts {
		m.swarmMessages[i] = SwarmMessage{
			SenderID:  b.SenderID,
			Content:   b.Content,
			CreatedAt: b.CreatedAt,
		}
	}
}

// refreshTick updates the currentTick field from the Commander's aggregate tick.
func (m *Model) refreshTick() {
	m.currentTick = m.commander.AggregateTick()
}

func (m *Model) loadMysisLogs() {
	if m.focusID == "" {
		m.logs = nil
		m.viewport.SetContent("")
		return
	}

	memories, err := m.store.GetRecentMemories(m.focusID, maxConversationEntries)
	if err != nil {
		m.err = err
		return
	}

	// Filter out broadcast messages from conversation log
	var filteredLogs []LogEntry
	for _, mem := range memories {
		// Skip broadcast messages (both sent and received)
		if mem.Source == store.MemorySourceBroadcast {
			continue
		}
		senderName := m.mysisNameByID(mem.SenderID)
		filteredLogs = append(filteredLogs, LogEntryFromMemory(mem, m.focusID, senderName))
	}
	m.logs = filteredLogs

	m.updateViewportContent()
}

// updateViewportContent renders log entries and sets viewport content.
// Smart auto-scroll: if user is at bottom when content updates, stay at bottom.
func (m *Model) updateViewportContent() {
	if len(m.logs) == 0 {
		m.viewport.SetContent(dimmedStyle.Render("No conversation history."))
		return
	}

	// Remember if user was at bottom before updating content
	wasAtBottom := m.viewport.AtBottom()

	// Log entries must fill the panel content area exactly.
	// Panel is rendered with logStyle.Width(m.width - 2) without padding (commit d023227)
	// Content width = m.width - 2 (borders), minus 2 (scrollbar) = m.width - 4
	panelContentWidth := m.width - 4 // -2 for borders, -2 for scrollbar

	var lines []string
	for _, entry := range m.logs {
		entryLines := renderLogEntryImpl(entry, panelContentWidth, m.verboseJSON, m.currentTick)
		lines = append(lines, entryLines...)
	}

	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	m.viewportTotalLines = len(lines) // Track total lines

	// Smart auto-scroll: if user was at bottom, keep them at bottom after content update
	// This prevents jarring jumps when user has manually scrolled up to read history
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m Model) mysisByID(id string) MysisInfo {
	for _, mysis := range m.myses {
		if mysis.ID == id {
			return mysis
		}
	}
	return MysisInfo{ID: id, Name: "Unknown", State: "unknown"}
}

func (m Model) mysisNameByID(id string) string {
	if id == "" {
		return ""
	}
	for _, mysis := range m.myses {
		if mysis.ID == id {
			return mysis.Name
		}
	}
	return ""
}

func (m Model) focusPosition(focusID string) (int, int) {
	total := len(m.myses)
	if focusID == "" || total == 0 {
		return 0, total
	}
	for i, mysis := range m.myses {
		if mysis.ID == focusID {
			return i + 1, total
		}
	}
	return 0, total
}

// resetInput resets input state and clears multi-stage flow.
func (m *Model) resetInput() {
	m.input.Reset()
	m.inputStage = InputStageName
	m.pendingMysisName = ""
	m.pendingMysisProvider = ""
}

type refreshMysesMsg struct{}

// sendMessageResult is returned when an async message send completes.
type sendMessageResult struct {
	mysisID string
	err     error
}

func (m Model) refreshMyses() tea.Cmd {
	return func() tea.Msg {
		return refreshMysesMsg{}
	}
}

func (m Model) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.eventCh
		if !ok {
			return nil
		}
		return EventMsg{Event: event}
	}
}

// Key bindings
var keys = struct {
	Quit          key.Binding
	Help          key.Binding
	Escape        key.Binding
	Enter         key.Binding
	Tab           key.Binding
	ShiftTab      key.Binding
	Up            key.Binding
	Down          key.Binding
	NewMysis      key.Binding
	Delete        key.Binding
	Relaunch      key.Binding
	Stop          key.Binding
	Broadcast     key.Binding
	Message       key.Binding
	Configure     key.Binding
	End           key.Binding
	VerboseToggle key.Binding
}{
	Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c")),
	Help:          key.NewBinding(key.WithKeys("?")),
	Escape:        key.NewBinding(key.WithKeys("esc")),
	Enter:         key.NewBinding(key.WithKeys("enter")),
	Tab:           key.NewBinding(key.WithKeys("tab")),
	ShiftTab:      key.NewBinding(key.WithKeys("shift+tab")),
	Up:            key.NewBinding(key.WithKeys("up", "k")),
	Down:          key.NewBinding(key.WithKeys("down", "j")),
	NewMysis:      key.NewBinding(key.WithKeys("n")),
	Delete:        key.NewBinding(key.WithKeys("d")),
	Relaunch:      key.NewBinding(key.WithKeys("r")),
	Stop:          key.NewBinding(key.WithKeys("s")),
	Broadcast:     key.NewBinding(key.WithKeys("b")),
	Message:       key.NewBinding(key.WithKeys("m")),
	Configure:     key.NewBinding(key.WithKeys("c")),
	End:           key.NewBinding(key.WithKeys("end", "G")),
	VerboseToggle: key.NewBinding(key.WithKeys("v")),
}
