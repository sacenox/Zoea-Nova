// Package tui provides the terminal user interface for Zoea Nova.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

	input   InputModel
	agents  []AgentInfo
	logs    []LogEntry
	focusID string

	err error
}

// EventMsg wraps a core event for the TUI.
type EventMsg struct {
	Event core.Event
}

// New creates a new TUI model.
func New(commander *core.Commander, s *store.Store, eventCh <-chan core.Event) Model {
	return Model{
		commander: commander,
		store:     s,
		eventCh:   eventCh,
		view:      ViewDashboard,
		input:     NewInputModel(),
		agents:    []AgentInfo{},
	}
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.refreshAgents(),
		m.listenForEvents(),
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

	case refreshAgentsMsg:
		m.refreshAgentList()
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content string

	if m.showHelp {
		content = RenderHelp(m.width, m.height)
	} else if m.view == ViewFocus {
		content = RenderFocusView(m.agentByID(m.focusID), m.logs, m.width, m.height-3)
	} else {
		content = RenderDashboard(m.agents, m.selectedIdx, m.width, m.height-3)
	}

	// Add input if active
	if m.input.IsActive() {
		content += "\n" + m.input.View()
	}

	// Add error if present
	if m.err != nil {
		errMsg := stateErroredStyle.Render(fmt.Sprintf("Error: %v", m.err))
		content += "\n" + errMsg
	}

	return content
}

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up), key.Matches(msg, keys.ShiftTab):
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case key.Matches(msg, keys.Down), key.Matches(msg, keys.Tab):
		if m.selectedIdx < len(m.agents)-1 {
			m.selectedIdx++
		}

	case key.Matches(msg, keys.Enter):
		if len(m.agents) > 0 && m.selectedIdx < len(m.agents) {
			m.focusID = m.agents[m.selectedIdx].ID
			m.view = ViewFocus
			m.loadAgentLogs()
		}

	case key.Matches(msg, keys.NewAgent):
		m.input.SetMode(InputModeNewAgent, "")

	case key.Matches(msg, keys.Delete):
		if len(m.agents) > 0 && m.selectedIdx < len(m.agents) {
			id := m.agents[m.selectedIdx].ID
			m.err = m.commander.DeleteAgent(id, true)
			m.refreshAgentList()
			if m.selectedIdx >= len(m.agents) && m.selectedIdx > 0 {
				m.selectedIdx--
			}
		}

	case key.Matches(msg, keys.Relaunch):
		if len(m.agents) > 0 && m.selectedIdx < len(m.agents) {
			id := m.agents[m.selectedIdx].ID
			m.err = m.commander.StartAgent(id)
		}

	case key.Matches(msg, keys.Stop):
		if len(m.agents) > 0 && m.selectedIdx < len(m.agents) {
			id := m.agents[m.selectedIdx].ID
			m.err = m.commander.StopAgent(id)
		}

	case key.Matches(msg, keys.Broadcast):
		m.input.SetMode(InputModeBroadcast, "")

	case key.Matches(msg, keys.Message):
		if len(m.agents) > 0 && m.selectedIdx < len(m.agents) {
			id := m.agents[m.selectedIdx].ID
			m.input.SetMode(InputModeMessage, id)
		}

	case key.Matches(msg, keys.Configure):
		if len(m.agents) > 0 && m.selectedIdx < len(m.agents) {
			id := m.agents[m.selectedIdx].ID
			m.input.SetMode(InputModeConfigProvider, id)
		}
	}

	return m, nil
}

func (m Model) handleFocusKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Message):
		m.input.SetMode(InputModeMessage, m.focusID)

	case key.Matches(msg, keys.Relaunch):
		m.err = m.commander.StartAgent(m.focusID)

	case key.Matches(msg, keys.Stop):
		m.err = m.commander.StopAgent(m.focusID)

	case key.Matches(msg, keys.Configure):
		m.input.SetMode(InputModeConfigProvider, m.focusID)
	}

	return m, nil
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.input.Reset()
		return m, nil

	case key.Matches(msg, keys.Enter):
		value := strings.TrimSpace(m.input.Value())
		if value == "" {
			m.input.Reset()
			return m, nil
		}

		switch m.input.Mode() {
		case InputModeBroadcast:
			m.err = m.commander.Broadcast(value)

		case InputModeMessage:
			m.err = m.commander.SendMessage(m.input.TargetID(), value)
			if m.view == ViewFocus {
				m.loadAgentLogs()
			}

		case InputModeNewAgent:
			_, m.err = m.commander.CreateAgent(value, "ollama")
			m.refreshAgentList()

		case InputModeConfigProvider:
			m.err = m.commander.ConfigureAgent(m.input.TargetID(), value)
		}

		m.input.Reset()
		return m, nil
	}

	// Pass to text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) handleEvent(event core.Event) {
	switch event.Type {
	case core.EventAgentCreated, core.EventAgentDeleted, core.EventAgentStateChanged, core.EventAgentConfigChanged:
		m.refreshAgentList()

	case core.EventAgentResponse:
		if m.view == ViewFocus && event.AgentID == m.focusID {
			m.loadAgentLogs()
		}
	}

	// Clear error on successful events
	m.err = nil
}

func (m *Model) refreshAgentList() {
	agents := m.commander.ListAgents()
	m.agents = make([]AgentInfo, len(agents))
	for i, a := range agents {
		m.agents[i] = AgentInfoFromCore(a)
	}
}

func (m *Model) loadAgentLogs() {
	if m.focusID == "" {
		m.logs = nil
		return
	}

	memories, err := m.store.GetRecentMemories(m.focusID, 50)
	if err != nil {
		m.err = err
		return
	}

	m.logs = make([]LogEntry, len(memories))
	for i, mem := range memories {
		m.logs[i] = LogEntryFromMemory(mem)
	}
}

func (m Model) agentByID(id string) AgentInfo {
	for _, a := range m.agents {
		if a.ID == id {
			return a
		}
	}
	return AgentInfo{ID: id, Name: "Unknown", State: "unknown"}
}

type refreshAgentsMsg struct{}

func (m Model) refreshAgents() tea.Cmd {
	return func() tea.Msg {
		return refreshAgentsMsg{}
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
	Quit      key.Binding
	Help      key.Binding
	Escape    key.Binding
	Enter     key.Binding
	Tab       key.Binding
	ShiftTab  key.Binding
	Up        key.Binding
	Down      key.Binding
	NewAgent  key.Binding
	Delete    key.Binding
	Relaunch  key.Binding
	Stop      key.Binding
	Broadcast key.Binding
	Message   key.Binding
	Configure key.Binding
}{
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c")),
	Help:      key.NewBinding(key.WithKeys("?")),
	Escape:    key.NewBinding(key.WithKeys("esc")),
	Enter:     key.NewBinding(key.WithKeys("enter")),
	Tab:       key.NewBinding(key.WithKeys("tab")),
	ShiftTab:  key.NewBinding(key.WithKeys("shift+tab")),
	Up:        key.NewBinding(key.WithKeys("up", "k")),
	Down:      key.NewBinding(key.WithKeys("down", "j")),
	NewAgent:  key.NewBinding(key.WithKeys("n")),
	Delete:    key.NewBinding(key.WithKeys("d")),
	Relaunch:  key.NewBinding(key.WithKeys("r")),
	Stop:      key.NewBinding(key.WithKeys("s")),
	Broadcast: key.NewBinding(key.WithKeys("b")),
	Message:   key.NewBinding(key.WithKeys("m")),
	Configure: key.NewBinding(key.WithKeys("c")),
}
