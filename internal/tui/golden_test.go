package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/xonecas/zoea-nova/internal/store"
)

// TestDashboard tests dashboard rendering with various states
func TestDashboard(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name        string
		myses       []MysisInfo
		swarmMsgs   []SwarmMessageInfo
		selectedIdx int
		width       int
		height      int
	}{
		{
			name:        "empty_swarm",
			myses:       []MysisInfo{},
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
		},
		{
			name: "with_swarm_messages",
			myses: []MysisInfo{
				{
					ID:              "mysis-1",
					Name:            "alpha",
					State:           "running",
					Provider:        "ollama",
					AccountUsername: "crab_warrior",
					LastMessage:     "Mining asteroid belt",
					CreatedAt:       time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				},
				{
					ID:              "mysis-2",
					Name:            "beta",
					State:           "idle",
					Provider:        "opencode_zen",
					AccountUsername: "crab_trader",
					LastMessage:     "Waiting for orders",
					CreatedAt:       time.Date(2026, 1, 15, 10, 35, 0, 0, time.UTC),
				},
			},
			swarmMsgs: []SwarmMessageInfo{
				{Content: "All units: proceed to sector 7", CreatedAt: time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)},
				{Content: "Target rich environment detected", CreatedAt: time.Date(2026, 1, 15, 11, 5, 0, 0, time.UTC)},
			},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadingSet := make(map[string]bool)
			output := RenderDashboard(tt.myses, tt.swarmMsgs, tt.selectedIdx, tt.width, tt.height, loadingSet, "⠋")

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

// TestFocusView tests focus view rendering with various log entries
func TestFocusView(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name   string
		mysis  MysisInfo
		logs   []LogEntry
		width  int
		height int
	}{
		{
			name: "with_all_roles",
			mysis: MysisInfo{
				ID:              "test-mysis",
				Name:            "alpha",
				State:           "running",
				Provider:        "ollama",
				AccountUsername: "crab_explorer",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "system",
					Source:    "system",
					Content:   "You are a space exploration AI controlling a ship in the Crustacean Cosmos.",
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					Role:      "user",
					Source:    "direct",
					Content:   "What is your current status?",
					Timestamp: time.Date(2026, 1, 15, 10, 1, 0, 0, time.UTC),
				},
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "I am currently orbiting planet Crabulous Prime. All systems nominal.",
					Reasoning: "The ship sensors show stable orbit. Fuel at 75%. Cargo hold empty.",
					Timestamp: time.Date(2026, 1, 15, 10, 1, 30, 0, time.UTC),
				},
				{
					Role:      "tool",
					Source:    "tool",
					Content:   `{"name": "alpha", "fuel": 75}`,
					Timestamp: time.Date(2026, 1, 15, 10, 1, 35, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "with_scrollbar",
			mysis: MysisInfo{
				ID:       "test-mysis",
				Name:     "beta",
				State:    "running",
				Provider: "ollama",
			},
			logs: func() []LogEntry {
				// Create enough logs to trigger scrollbar
				logs := make([]LogEntry, 30)
				for i := 0; i < 30; i++ {
					logs[i] = LogEntry{
						Role:      "user",
						Source:    "direct",
						Content:   "Test message line " + string(rune('A'+i)),
						Timestamp: time.Date(2026, 1, 15, 10, i, 0, 0, time.UTC),
					}
				}
				return logs
			}(),
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderFocusView(tt.mysis, tt.logs, tt.width, tt.height, false, "⠋", false)

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

// TestHelp tests help screen rendering
func TestHelp(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{
			name:   "content",
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderHelp(tt.width, tt.height)

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

// TestLogEntry tests individual log entry rendering
func TestLogEntry(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name     string
		entry    LogEntry
		maxWidth int
		verbose  bool
	}{
		{
			name: "with_reasoning",
			entry: LogEntry{
				Role:      "assistant",
				Source:    "llm",
				Content:   "I will mine the asteroid.",
				Reasoning: "The asteroid contains valuable ore. I have enough fuel and cargo space.",
				Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
			maxWidth: 80,
			verbose:  false,
		},
		{
			name: "with_reasoning_truncation",
			entry: LogEntry{
				Role:    "assistant",
				Source:  "llm",
				Content: "I will proceed with the plan.",
				Reasoning: "First line of reasoning that explains the initial thought process. " +
					"Second line continues with more detailed analysis of the situation. " +
					"Third line adds even more context about the decision. " +
					"Fourth line provides additional justification. " +
					"Fifth line continues the explanation with further details. " +
					"Sixth line adds more reasoning depth. " +
					"Seventh line provides even more context. " +
					"Eighth line concludes the reasoning with final thoughts.",
				Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
			maxWidth: 80,
			verbose:  false,
		},
		{
			name: "tool_with_json",
			entry: LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   `{"name": "mysis-1", "state": "running", "fuel": 100}`,
				Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
			maxWidth: 80,
			verbose:  false,
		},
		{
			name: "tool_with_prefixed_json",
			entry: LogEntry{
				Role:      "tool",
				Source:    "tool",
				Content:   `call_59orhh05:{"total": 1, "max": 16, "status": "ok"}`,
				Timestamp: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
			},
			maxWidth: 80,
			verbose:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := renderLogEntryImpl(tt.entry, tt.maxWidth, tt.verbose)
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

// TestFocusViewWithViewport tests focus view with viewport for scrollbar rendering
func TestFocusViewWithViewport(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name       string
		mysis      MysisInfo
		logs       []LogEntry
		width      int
		height     int
		totalLines int
	}{
		{
			name: "with_scrollbar",
			mysis: MysisInfo{
				ID:              "test-id",
				Name:            "test-mysis",
				State:           "running",
				Provider:        "ollama",
				AccountUsername: "crab_pilot",
			},
			logs: func() []LogEntry {
				logs := make([]LogEntry, 50)
				for i := 0; i < 50; i++ {
					logs[i] = LogEntry{
						Role:    "user",
						Source:  "direct",
						Content: "Test line " + string(rune('0'+i%10)),
					}
				}
				return logs
			}(),
			width:      TestTerminalWidth,
			height:     20,
			totalLines: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create viewport
			vp := viewport.New(tt.width-4, 10)
			var contentLines []string
			for _, log := range tt.logs {
				lines := renderLogEntryImpl(log, tt.width-4, false)
				contentLines = append(contentLines, lines...)
			}
			vp.SetContent(strings.Join(contentLines, "\n"))
			vp.GotoTop()

			output := RenderFocusViewWithViewport(tt.mysis, vp, tt.width, false, "⠋", true, false, tt.totalLines)

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

// TestJSONTree tests JSON tree rendering
func TestJSONTree(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name     string
		jsonStr  string
		verbose  bool
		maxWidth int
	}{
		{
			name:     "simple_object",
			jsonStr:  `{"name": "mysis-1", "state": "running", "id": "abc123"}`,
			verbose:  false,
			maxWidth: 80,
		},
		{
			name:     "array_truncation",
			jsonStr:  `[{"id":0},{"id":1},{"id":2},{"id":3},{"id":4},{"id":5},{"id":6},{"id":7},{"id":8},{"id":9}]`,
			verbose:  false,
			maxWidth: 80,
		},
		{
			name:     "verbose_mode",
			jsonStr:  `[{"id":0},{"id":1},{"id":2},{"id":3},{"id":4},{"id":5},{"id":6},{"id":7},{"id":8},{"id":9}]`,
			verbose:  true,
			maxWidth: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := renderJSONTree(tt.jsonStr, tt.verbose, tt.maxWidth)
			if err != nil {
				t.Fatalf("Failed to render JSON tree: %v", err)
			}

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

// TestScrollbar tests scrollbar rendering
func TestScrollbar(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name         string
		height       int
		totalLines   int
		scrollOffset int
	}{
		{
			name:         "at_top",
			height:       10,
			totalLines:   100,
			scrollOffset: 0,
		},
		{
			name:         "at_bottom",
			height:       10,
			totalLines:   100,
			scrollOffset: 90,
		},
		{
			name:         "middle",
			height:       10,
			totalLines:   100,
			scrollOffset: 45,
		},
		{
			name:         "no_scroll",
			height:       10,
			totalLines:   5,
			scrollOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := renderScrollbar(tt.height, tt.totalLines, tt.scrollOffset)

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

// TestMysisLine tests mysis line rendering with account information
func TestMysisLine(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name  string
		mysis MysisInfo
		width int
	}{
		{
			name: "with_account",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test-mysis",
				State:           "running",
				Provider:        "ollama",
				AccountUsername: "crab_miner",
				LastMessage:     "Mining asteroid",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			width: 100,
		},
		{
			name: "without_account",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test-mysis",
				State:           "idle",
				Provider:        "ollama",
				AccountUsername: "",
				LastMessage:     "",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			width: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := renderMysisLine(tt.mysis, false, false, "⬡", tt.width)

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

// TestBroadcastLabels tests broadcast message sender labels
func TestBroadcastLabels(t *testing.T) {
	defer setupGoldenTest(t)()

	currentMysisID := "current-mysis"

	tests := []struct {
		name   string
		memory *store.Memory
	}{
		{
			name: "direct_message",
			memory: &store.Memory{
				Role:    store.MemoryRoleUser,
				Source:  store.MemorySourceDirect,
				Content: "Direct command to this mysis",
			},
		},
		{
			name: "broadcast_from_self",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: currentMysisID,
				Content:  "My broadcast to the swarm",
			},
		},
		{
			name: "broadcast_from_other",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: "other-mysis",
				Content:  "Another mysis's broadcast",
			},
		},
		{
			name: "broadcast_legacy_no_sender",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: "",
				Content:  "Legacy broadcast without sender ID",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntryFromMemory(tt.memory, currentMysisID)
			lines := renderLogEntryImpl(entry, 80, false)
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
