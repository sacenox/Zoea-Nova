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
	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/store"
)

// View represents the current view mode.
type View int

const (
	ViewDashboard View = iota
	ViewFocus
)

// Model is the main TUI model.
type Model struct {
	commander *core.Commander
	store     *store.Store
	eventCh   <-chan core.Event

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

	pendingProvider string

	// Swarm broadcast history
	swarmMessages []SwarmMessage

	spinner    spinner.Model
	loadingSet map[string]bool // mysisIDs currently loading

	// Conversation viewport
	viewport           viewport.Model
	autoScroll         bool // true if viewport should auto-scroll to bottom
	viewportTotalLines int  // total lines in viewport content

	// Network activity indicator
	netIndicator NetIndicator

	err error
}

// SwarmMessage represents a broadcast message for display.
type SwarmMessage struct {
	Content   string
	CreatedAt time.Time
}

// EventMsg wraps a core event for the TUI.
type EventMsg struct {
	Event core.Event
}

// New creates a new TUI model.
func New(commander *core.Commander, s *store.Store, eventCh <-chan core.Event) Model {
	// Initialize spinner with hexagonal theme (matching logo)
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"⬡", "⬢", "⬡", "⬢", "◇", "◆", "◇", "◆"},
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
		view:         ViewDashboard,
		input:        NewInputModel(),
		myses:        []MysisInfo{},
		spinner:      sp,
		loadingSet:   make(map[string]bool),
		viewport:     vp,
		autoScroll:   true,
		netIndicator: NewNetIndicator(),
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.netIndicator.Init(),
		m.refreshMyses(),
		m.listenForEvents(),
		m.spinner.Tick,
	)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.SetWidth(msg.Width - 4)

		// Update viewport size (account for header, info panel, title, footer)
		headerHeight := 6 // approximate height used by header/info/title
		footerHeight := 2
		vpHeight := msg.Height - headerHeight - footerHeight - 3
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Width = msg.Width - 6 - 1 // -6 for existing padding, -1 for scrollbar
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
		m.handleEvent(msg.Event)
		return m, m.listenForEvents()

	case refreshMysesMsg:
		m.refreshMysisList()
		m.refreshSwarmMessages()

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
		}
		if m.view == ViewFocus && msg.mysisID == m.focusID {
			m.loadMysisLogs()
		}

	case broadcastResult:
		// Broadcast finished - clear all loading states
		m.loadingSet = make(map[string]bool)
		m.netIndicator.SetActivity(NetActivityIdle)
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

	var content string

	// Check if currently focused mysis is loading
	isLoading := m.loadingSet[m.focusID]

	// Reserve space for status bar
	contentHeight := m.height - 1

	if m.showHelp {
		content = RenderHelp(m.width, contentHeight)
	} else if m.view == ViewFocus {
		content = RenderFocusViewWithViewport(m.mysisByID(m.focusID), m.viewport, m.width, isLoading, m.spinner.View(), m.autoScroll, m.verboseJSON, m.viewportTotalLines)
	} else {
		// Convert swarm messages for display (reversed so most recent is first)
		swarmInfos := make([]SwarmMessageInfo, len(m.swarmMessages))
		for i, msg := range m.swarmMessages {
			// Reverse order: most recent first
			swarmInfos[len(m.swarmMessages)-1-i] = SwarmMessageInfo{
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
			}
		}
		content = RenderDashboard(m.myses, swarmInfos, m.selectedIdx, m.width, contentHeight-3, m.loadingSet, m.spinner.View())
	}

	// Always show message bar
	content += "\n" + m.input.ViewAlways(m.width)

	// Add error if present
	if m.err != nil {
		errMsg := stateErroredStyle.Render(fmt.Sprintf("Error: %v", m.err))
		content += "\n" + errMsg
	}

	// Build status bar
	statusBar := m.renderStatusBar()

	return content + "\n" + statusBar
}

// renderStatusBar renders the bottom status bar with network indicator.
func (m Model) renderStatusBar() string {
	// Network indicator on the left
	netStatus := m.netIndicator.View()

	// View indicator in the middle
	var viewName string
	switch m.view {
	case ViewDashboard:
		viewName = "DASHBOARD"
	case ViewFocus:
		idPreview := m.focusID
		if len(idPreview) > 8 {
			idPreview = idPreview[:8]
		}
		viewName = "FOCUS: " + idPreview
	}

	// Mysis count on the right
	running := 0
	for _, mysis := range m.myses {
		if mysis.State == "running" {
			running++
		}
	}
	mysisStatus := fmt.Sprintf("Myses: %d/%d running", running, len(m.myses))

	// Calculate spacing
	leftWidth := lipgloss.Width(netStatus)
	rightWidth := lipgloss.Width(mysisStatus)
	middleWidth := lipgloss.Width(viewName)
	totalUsed := leftWidth + rightWidth + middleWidth
	spacing := m.width - totalUsed - 4 // -4 for some padding

	if spacing < 2 {
		spacing = 2
	}

	leftPad := spacing / 2
	rightPad := spacing - leftPad

	// Style the status bar
	barStyle := lipgloss.NewStyle().
		Background(colorBorder).
		Foreground(colorPrimary).
		Width(m.width)

	viewStyle := lipgloss.NewStyle().Foreground(colorMuted)
	mysisStyle := lipgloss.NewStyle().Foreground(colorSecondary)

	bar := netStatus +
		strings.Repeat(" ", leftPad) +
		viewStyle.Render(viewName) +
		strings.Repeat(" ", rightPad) +
		mysisStyle.Render(mysisStatus)

	return barStyle.Render(bar)
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
			m.autoScroll = true // Start at bottom when entering focus view
			m.loadMysisLogs()
		}

	case key.Matches(msg, keys.NewMysis):
		m.input.SetMode(InputModeNewMysis, "")
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

	case key.Matches(msg, keys.End):
		// Go to bottom and enable auto-scroll
		m.viewport.GotoBottom()
		m.autoScroll = true
		return m, nil

	case key.Matches(msg, keys.VerboseToggle):
		m.verboseJSON = !m.verboseJSON
		// Re-render viewport content with new verbose setting
		m.updateViewportContent()
		return m, nil
	}

	// Pass other keys to viewport for scrolling
	var cmd tea.Cmd
	wasAtBottom := m.viewport.AtBottom()
	m.viewport, cmd = m.viewport.Update(msg)

	// If user scrolled up, disable auto-scroll
	if wasAtBottom && !m.viewport.AtBottom() {
		m.autoScroll = false
	}
	// If user scrolled to bottom, enable auto-scroll
	if m.viewport.AtBottom() {
		m.autoScroll = true
	}

	return m, cmd
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.input.Reset()
		m.pendingProvider = ""
		m.err = nil
		return m, nil

	case key.Matches(msg, keys.Enter):
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.input.Reset()
			return m, nil
		}

		var cmd tea.Cmd

		switch m.input.Mode() {
		case InputModeBroadcast:
			// Add to history before sending
			m.input.AddToHistory(value)
			// Mark all running myses as loading
			myses := m.commander.ListMyses()
			for _, mysis := range myses {
				if mysis.State() == core.MysisStateRunning {
					m.loadingSet[mysis.ID()] = true
				}
			}
			m.netIndicator.SetActivity(NetActivityLLM)
			// Use Broadcast to properly set source='broadcast'
			cmd = m.broadcastAsync(value)

		case InputModeMessage:
			// Add to history before sending
			m.input.AddToHistory(value)
			targetID := m.input.TargetID()
			m.loadingSet[targetID] = true
			m.netIndicator.SetActivity(NetActivityLLM)
			cmd = m.sendMessageAsync(targetID, value)
			if m.view == ViewFocus {
				m.loadMysisLogs()
			}

		case InputModeNewMysis:
			mysis, err := m.commander.CreateMysis(value, "ollama")
			if err == nil {
				// Auto-start newly created myses
				m.err = m.commander.StartMysis(mysis.ID())
			} else {
				m.err = err
			}
			m.refreshMysisList()

		case InputModeConfigProvider:
			switch value {
			case "ollama", "opencode_zen":
				m.pendingProvider = value
				m.input.SetMode(InputModeConfigModel, m.input.TargetID())
				return m, m.input.Focus()
			default:
				m.err = fmt.Errorf("unknown provider: %s", value)
				m.input.Reset()
				return m, nil
			}

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
		err := m.commander.SendMessage(mysisID, content)
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
		if m.view == ViewFocus && event.MysisID == m.focusID {
			m.loadMysisLogs()
		}

	case core.EventBroadcast:
		// Refresh swarm message history
		m.refreshSwarmMessages()

	case core.EventNetworkLLM:
		m.netIndicator.SetActivity(NetActivityLLM)

	case core.EventNetworkMCP:
		m.netIndicator.SetActivity(NetActivityMCP)

	case core.EventNetworkIdle:
		// Only go idle if no myses are loading
		if len(m.loadingSet) == 0 {
			m.netIndicator.SetActivity(NetActivityIdle)
		}
	}

	// Clear error on successful events
	m.err = nil
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
			Content:   b.Content,
			CreatedAt: b.CreatedAt,
		}
	}
}

func (m *Model) loadMysisLogs() {
	if m.focusID == "" {
		m.logs = nil
		m.viewport.SetContent("")
		return
	}

	memories, err := m.store.GetRecentMemories(m.focusID, 50)
	if err != nil {
		m.err = err
		return
	}

	m.logs = make([]LogEntry, len(memories))
	for i, mem := range memories {
		m.logs[i] = LogEntryFromMemory(mem, m.focusID)
	}

	m.updateViewportContent()
}

// updateViewportContent renders log entries and sets viewport content.
func (m *Model) updateViewportContent() {
	if len(m.logs) == 0 {
		m.viewport.SetContent(dimmedStyle.Render("No conversation history."))
		return
	}

	// Log entries must fill the panel content area exactly.
	// Panel is rendered with logStyle.Width(m.width - 2).Padding(0, 2)
	// Content width = m.width - 2, minus 4 for padding (2 each side) = m.width - 6
	// Scrollbar adds 2 chars (space + scrollbar char), so subtract that too
	panelContentWidth := m.width - 6 - 2 // -2 for scrollbar

	var lines []string
	for _, entry := range m.logs {
		entryLines := renderLogEntryImpl(entry, panelContentWidth, m.verboseJSON)
		lines = append(lines, entryLines...)
	}

	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
	m.viewportTotalLines = len(lines) // Track total lines

	// Auto-scroll to bottom if enabled
	if m.autoScroll {
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
