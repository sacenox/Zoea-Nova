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

	// Use fixed timestamps for deterministic golden files.
	tests := []struct {
		name        string
		myses       []MysisInfo
		swarmMsgs   []SwarmMessageInfo
		selectedIdx int
		width       int
		height      int
		loadingSet  map[string]bool
	}{
		{
			name:        "empty_swarm",
			myses:       []MysisInfo{},
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name: "with_swarm_messages",
			myses: []MysisInfo{
				{
					ID:              "mysis-1",
					Name:            "alpha",
					State:           "running",
					Provider:        "ollama-qwen",
					AccountUsername: "crab_warrior",
					LastMessage:     "Mining asteroid belt",
					LastMessageAt:   time.Date(2026, 1, 15, 10, 45, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				},
				{
					ID:              "mysis-2",
					Name:            "beta",
					State:           "idle",
					Provider:        "opencode_zen",
					AccountUsername: "crab_trader",
					LastMessage:     "Waiting for orders",
					LastMessageAt:   time.Date(2026, 1, 15, 10, 46, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 10, 35, 0, 0, time.UTC),
				},
			},
			swarmMsgs: []SwarmMessageInfo{
				{SenderID: "mysis-1", SenderName: "alpha", Content: "All units: proceed to sector 7", CreatedAt: time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC)},
				{SenderID: "mysis-2", SenderName: "beta", Content: "Target rich environment detected", CreatedAt: time.Date(2026, 1, 15, 11, 5, 0, 0, time.UTC)},
			},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name: "with_multiline_broadcast",
			myses: []MysisInfo{
				{
					ID:              "mysis-3",
					Name:            "gamma",
					State:           "idle",
					Provider:        "ollama-qwen",
					AccountUsername: "crab_cartographer",
					LastMessage:     "Standing by",
					LastMessageAt:   time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 11, 55, 0, 0, time.UTC),
				},
			},
			swarmMsgs: []SwarmMessageInfo{
				{SenderID: "mysis-3", SenderName: "gamma", Content: "Line one\nLine two", CreatedAt: time.Date(2026, 1, 15, 12, 1, 0, 0, time.UTC)},
			},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name: "with_loading_mysis",
			myses: []MysisInfo{
				{
					ID:              "mysis-4",
					Name:            "delta",
					State:           "running",
					Provider:        "ollama-qwen",
					AccountUsername: "crab_runner",
					LastMessage:     "Processing",
					LastMessageAt:   time.Date(2026, 1, 15, 12, 5, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 11, 50, 0, 0, time.UTC),
				},
			},
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{"mysis-4": true},
		},
		{
			name: "narrow_terminal",
			myses: []MysisInfo{
				{
					ID:              "mysis-5",
					Name:            "epsilon",
					State:           "idle",
					Provider:        "ollama-qwen",
					AccountUsername: "crab_navigator",
					LastMessage:     "Holding position",
					LastMessageAt:   time.Date(2026, 1, 15, 12, 10, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 12, 5, 0, 0, time.UTC),
				},
			},
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       80,
			height:      24,
			loadingSet:  map[string]bool{},
		},
		{
			name: "wide_terminal",
			myses: []MysisInfo{
				{
					ID:              "mysis-6",
					Name:            "zeta",
					State:           "running",
					Provider:        "opencode_zen",
					AccountUsername: "crab_scout",
					LastMessage:     "Surveying sector",
					LastMessageAt:   time.Date(2026, 1, 15, 12, 20, 0, 0, time.UTC),
					CreatedAt:       time.Date(2026, 1, 15, 12, 15, 0, 0, time.UTC),
				},
			},
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       200,
			height:      60,
			loadingSet:  map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderDashboard(tt.myses, tt.swarmMsgs, tt.selectedIdx, tt.width, tt.height, tt.loadingSet, "⠋", 0, nil)

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
				Provider:        "ollama-qwen",
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
				ID:        "test-mysis",
				Name:      "beta",
				State:     "running",
				Provider:  "ollama-qwen",
				CreatedAt: time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC),
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
		{
			name: "narrow_terminal",
			mysis: MysisInfo{
				ID:              "test-mysis",
				Name:            "gamma",
				State:           "idle",
				Provider:        "opencode_zen",
				AccountUsername: "crab_surveyor",
				CreatedAt:       time.Date(2026, 1, 15, 9, 45, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "Short response for narrow view.",
					Timestamp: time.Date(2026, 1, 15, 9, 46, 0, 0, time.UTC),
				},
			},
			width:  80,
			height: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderFocusView(tt.mysis, tt.logs, tt.width, tt.height, false, "⠋", false, 1, 1, 0, nil)

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
			lines := renderLogEntryImpl(tt.entry, tt.maxWidth, tt.verbose, 0)
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
				Provider:        "ollama-qwen",
				AccountUsername: "crab_pilot",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: func() []LogEntry {
				logs := make([]LogEntry, 50)
				for i := 0; i < 50; i++ {
					logs[i] = LogEntry{
						Role:      "user",
						Source:    "direct",
						Content:   "Test line " + string(rune('0'+i%10)),
						Timestamp: time.Date(2026, 1, 15, 10, i%60, 0, 0, time.UTC),
					}
				}
				return logs
			}(),
			width:      TestTerminalWidth,
			height:     20,
			totalLines: 50,
		},
		{
			name: "with_scroll_indicator",
			mysis: MysisInfo{
				ID:              "test-id",
				Name:            "test-mysis",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "crab_pilot",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: func() []LogEntry {
				logs := make([]LogEntry, 30)
				for i := 0; i < 30; i++ {
					logs[i] = LogEntry{
						Role:      "assistant",
						Source:    "llm",
						Content:   "Focus log line " + string(rune('A'+i)),
						Timestamp: time.Date(2026, 1, 15, 10, i%60, 0, 0, time.UTC),
					}
				}
				return logs
			}(),
			width:      TestTerminalWidth,
			height:     20,
			totalLines: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create viewport
			vp := viewport.New(tt.width-4, 10)
			var contentLines []string
			for _, log := range tt.logs {
				lines := renderLogEntryImpl(log, tt.width-4, false, 0)
				contentLines = append(contentLines, lines...)
			}
			vp.SetContent(strings.Join(contentLines, "\n"))
			vp.GotoTop()

			output := RenderFocusViewWithViewport(tt.mysis, vp, tt.width, false, "⠋", false, tt.totalLines, 1, 1, 0, nil, 0, nil)

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
		name    string
		mysis   MysisInfo
		width   int
		loading bool
	}{
		{
			name: "with_account",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test-mysis",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "crab_miner",
				LastMessage:     "Mining asteroid",
				LastMessageAt:   time.Date(2026, 1, 15, 10, 5, 0, 0, time.UTC),
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
				Provider:        "ollama-qwen",
				AccountUsername: "",
				LastMessage:     "",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			width: 100,
		},
		{
			name: "long_name",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "mysis-with-a-very-long-name",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "crab_miner",
				LastMessage:     "Holding",
				LastMessageAt:   time.Date(2026, 1, 15, 10, 10, 0, 0, time.UTC),
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			width: 100,
		},
		{
			name: "errored_state",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test-mysis",
				State:           "errored",
				Provider:        "ollama-qwen",
				AccountUsername: "crab_miner",
				LastMessage:     "",
				LastError:       "connection lost",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			width: 100,
		},
		{
			name: "loading_state",
			mysis: MysisInfo{
				ID:              "abc123",
				Name:            "test-mysis",
				State:           "running",
				Provider:        "ollama-qwen",
				AccountUsername: "crab_miner",
				LastMessage:     "Processing",
				LastMessageAt:   time.Date(2026, 1, 15, 10, 15, 0, 0, time.UTC),
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			width:   100,
			loading: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := renderMysisLine(tt.mysis, false, tt.loading, "⬡", tt.width, 0)
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

// TestBroadcastLabels tests broadcast message sender labels
func TestBroadcastLabels(t *testing.T) {
	defer setupGoldenTest(t)()

	currentMysisID := "current-mysis"

	tests := []struct {
		name       string
		memory     *store.Memory
		senderName string
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
			senderName: "alpha",
		},
		{
			name: "broadcast_from_other",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: "other-mysis",
				Content:  "Another mysis's broadcast",
			},
			senderName: "beta",
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
			entry := LogEntryFromMemory(tt.memory, currentMysisID, tt.senderName)
			lines := renderLogEntryImpl(entry, 80, false, 0)
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
