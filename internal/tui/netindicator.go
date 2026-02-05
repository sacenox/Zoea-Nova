// Package tui provides the terminal user interface for Zoea Nova.
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NetActivity represents the type of network activity.
type NetActivity int

const (
	NetActivityIdle NetActivity = iota
	NetActivityLLM              // Talking to LLM provider
	NetActivityMCP              // Talking to MCP/game server
)

// NetIndicator is a bouncing progress indicator for network activity.
type NetIndicator struct {
	activity  NetActivity
	position  int       // Current position of the "ball" (0-width)
	direction int       // 1 = right, -1 = left
	width     int       // Width of the indicator bar
	lastTick  time.Time // For animation timing
}

// NetIndicatorTickMsg is sent to animate the indicator.
type NetIndicatorTickMsg time.Time

// NewNetIndicator creates a new network activity indicator.
func NewNetIndicator() NetIndicator {
	return NetIndicator{
		activity:  NetActivityIdle,
		position:  0,
		direction: 1,
		width:     12, // Width of the bouncing area
	}
}

// SetActivity sets the current network activity type.
func (n *NetIndicator) SetActivity(activity NetActivity) {
	n.activity = activity
}

// Activity returns the current activity type.
func (n NetIndicator) Activity() NetActivity {
	return n.activity
}

// Update handles tick messages for animation.
func (n NetIndicator) Update(msg tea.Msg) (NetIndicator, tea.Cmd) {
	switch msg.(type) {
	case NetIndicatorTickMsg:
		if n.activity != NetActivityIdle {
			// Move the position
			n.position += n.direction

			// Bounce at edges
			if n.position >= n.width-1 {
				n.position = n.width - 1
				n.direction = -1
			} else if n.position <= 0 {
				n.position = 0
				n.direction = 1
			}
		}
		return n, n.tick()
	}
	return n, nil
}

// tick returns a command that sends a tick after a delay.
func (n NetIndicator) tick() tea.Cmd {
	return tea.Tick(time.Millisecond*80, func(t time.Time) tea.Msg {
		return NetIndicatorTickMsg(t)
	})
}

// Init starts the indicator animation.
func (n NetIndicator) Init() tea.Cmd {
	return n.tick()
}

// View renders the network indicator.
func (n NetIndicator) View() string {
	// Define the bar characters - use block elements for CRT feel
	const (
		barEmpty  = "░"
		barFilled = "█"
		barLeft   = "▐"
		barRight  = "▌"
	)

	// Get style based on activity
	var style lipgloss.Style
	var label string

	switch n.activity {
	case NetActivityIdle:
		style = lipgloss.NewStyle().Foreground(colorMuted)
		label = "⬦ IDLE"
		// For idle, show a static dim bar
		bar := barLeft
		for i := 0; i < n.width; i++ {
			bar += barEmpty
		}
		bar += barRight
		return style.Render(label + " " + bar)

	case NetActivityLLM:
		style = lipgloss.NewStyle().Foreground(colorAssistant).Bold(true)
		label = " ⬥ LLM "
	case NetActivityMCP:
		// MCP activity - diamond with MCP label
		label = " ⬥ MCP "
	}

	// Build the bouncing bar
	bar := barLeft
	for i := 0; i < n.width; i++ {
		// Create a 3-char wide "ball" that bounces
		if i >= n.position-1 && i <= n.position+1 {
			bar += barFilled
		} else {
			bar += barEmpty
		}
	}
	bar += barRight

	return style.Render(label + " " + bar)
}

// ViewCompact renders a compact version for small screens.
func (n NetIndicator) ViewCompact() string {
	// Simpler animation frames for compact view - hexagonal theme
	frames := []string{"⬡", "⬢", "⬡", "⬢"}

	var style lipgloss.Style
	var label string

	switch n.activity {
	case NetActivityIdle:
		style = lipgloss.NewStyle().Foreground(colorMuted)
		return style.Render("⬦ IDLE")

	case NetActivityLLM:
		style = lipgloss.NewStyle().Foreground(colorAssistant).Bold(true)
		label = "LLM"

	case NetActivityMCP:
		style = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
		label = "MCP"
	}

	// Use position to select frame
	frameIdx := n.position % len(frames)
	return style.Render(frames[frameIdx] + " " + label)
}
