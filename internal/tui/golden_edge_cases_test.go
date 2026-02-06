package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
)

// TestDashboardEdgeCases tests dashboard rendering edge cases for visual regression.
func TestDashboardEdgeCases(t *testing.T) {
	defer setupGoldenTest(t)()

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
			name:        "full_swarm_16_myses",
			myses:       generateMyses(16, "running"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "full_swarm_with_10_broadcasts",
			myses:       generateMyses(5, "running"),
			swarmMsgs:   generateBroadcasts(10),
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "all_myses_errored",
			myses:       generateMyses(5, "errored"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "all_myses_stopped",
			myses:       generateMyses(5, "stopped"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "all_myses_idle",
			myses:       generateMyses(5, "idle"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "mixed_states",
			myses:       generateMixedStates(),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "very_long_broadcast",
			myses:       generateMyses(2, "running"),
			swarmMsgs:   generateLongBroadcasts(),
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "unicode_emoji_in_messages",
			myses:       generateMyses(2, "running"),
			swarmMsgs:   generateUnicodeBroadcasts(),
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "cjk_content",
			myses:       generateCJKMyses(),
			swarmMsgs:   generateCJKBroadcasts(),
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "extremely_narrow_terminal",
			myses:       generateMyses(2, "running"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       60,
			height:      20,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "extremely_wide_terminal",
			myses:       generateMyses(3, "running"),
			swarmMsgs:   generateBroadcasts(3),
			selectedIdx: 0,
			width:       240,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "extremely_tall_terminal",
			myses:       generateMyses(10, "running"),
			swarmMsgs:   generateBroadcasts(5),
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      100,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "all_loading",
			myses:       generateMyses(5, "running"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 0,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  generateAllLoadingSet(5),
		},
		{
			name:        "selection_at_bottom",
			myses:       generateMyses(10, "running"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 9,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
		{
			name:        "selection_middle",
			myses:       generateMyses(10, "running"),
			swarmMsgs:   []SwarmMessageInfo{},
			selectedIdx: 5,
			width:       TestTerminalWidth,
			height:      TestTerminalHeight,
			loadingSet:  map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderDashboard(tt.myses, tt.swarmMsgs, tt.selectedIdx, tt.width, tt.height, tt.loadingSet, "⠋", 0)

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

// TestFocusViewEdgeCases tests focus view edge cases for visual regression.
func TestFocusViewEdgeCases(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name   string
		mysis  MysisInfo
		logs   []LogEntry
		width  int
		height int
	}{
		{
			name: "no_logs_empty_state",
			mysis: MysisInfo{
				ID:              "empty-mysis",
				Name:            "empty-test",
				State:           "idle",
				Provider:        "ollama",
				AccountUsername: "",
				CreatedAt:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs:   []LogEntry{},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "very_long_log_entry",
			mysis: MysisInfo{
				ID:        "long-mysis",
				Name:      "long-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   strings.Repeat("This is a very long message that will need to be wrapped across multiple lines. ", 10),
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "very_long_reasoning",
			mysis: MysisInfo{
				ID:        "reasoning-mysis",
				Name:      "reasoning-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "I will execute the plan.",
					Reasoning: strings.Repeat("This reasoning is extremely long and contains many detailed thoughts about the decision process. ", 20),
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "unicode_emoji_logs",
			mysis: MysisInfo{
				ID:        "emoji-mysis",
				Name:      "emoji-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "Mining asteroid 🪨 found precious metals 💎 in sector 7 🚀",
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					Role:      "user",
					Source:    "direct",
					Content:   "Good job! 👍 Continue exploring 🔭",
					Timestamp: time.Date(2026, 1, 15, 10, 1, 0, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "cjk_logs",
			mysis: MysisInfo{
				ID:        "cjk-mysis",
				Name:      "cjk-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "採掘完了。鉱石を発見しました。",
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					Role:      "user",
					Source:    "direct",
					Content:   "很好！继续探索。",
					Timestamp: time.Date(2026, 1, 15, 10, 1, 0, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "huge_json_tool_result",
			mysis: MysisInfo{
				ID:        "json-mysis",
				Name:      "json-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "tool",
					Source:    "tool",
					Content:   generateLargeJSON(),
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "many_logs_100_entries",
			mysis: MysisInfo{
				ID:        "many-logs-mysis",
				Name:      "many-logs-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs:   generateManyLogs(100),
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
		{
			name: "narrow_focus_view",
			mysis: MysisInfo{
				ID:        "narrow-focus-mysis",
				Name:      "narrow-focus",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "Short",
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
			width:  60,
			height: 20,
		},
		{
			name: "all_role_types",
			mysis: MysisInfo{
				ID:        "roles-mysis",
				Name:      "roles-test",
				State:     "running",
				Provider:  "ollama",
				CreatedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			logs: []LogEntry{
				{
					Role:      "system",
					Source:    "system",
					Content:   "System initialization message",
					Timestamp: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					Role:      "user",
					Source:    "direct",
					Content:   "Direct user message",
					Timestamp: time.Date(2026, 1, 15, 10, 1, 0, 0, time.UTC),
				},
				{
					Role:      "user",
					Source:    "broadcast",
					Content:   "Broadcast from swarm",
					Timestamp: time.Date(2026, 1, 15, 10, 2, 0, 0, time.UTC),
				},
				{
					Role:      "assistant",
					Source:    "llm",
					Content:   "AI response",
					Reasoning: "Because reasons",
					Timestamp: time.Date(2026, 1, 15, 10, 3, 0, 0, time.UTC),
				},
				{
					Role:      "tool",
					Source:    "tool",
					Content:   `{"result": "success"}`,
					Timestamp: time.Date(2026, 1, 15, 10, 4, 0, 0, time.UTC),
				},
			},
			width:  TestTerminalWidth,
			height: TestTerminalHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := RenderFocusView(tt.mysis, tt.logs, tt.width, tt.height, false, "⠋", false, 1, 1, 0)

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

// TestNetIndicatorEdgeCases tests net indicator rendering edge cases.
func TestNetIndicatorEdgeCases(t *testing.T) {
	defer setupGoldenTest(t)()

	tests := []struct {
		name     string
		activity NetActivity
		position int
		compact  bool
	}{
		{
			name:     "idle_full",
			activity: NetActivityIdle,
			position: 0,
			compact:  false,
		},
		{
			name:     "idle_compact",
			activity: NetActivityIdle,
			position: 0,
			compact:  true,
		},
		{
			name:     "llm_start",
			activity: NetActivityLLM,
			position: 0,
			compact:  false,
		},
		{
			name:     "llm_middle",
			activity: NetActivityLLM,
			position: 6,
			compact:  false,
		},
		{
			name:     "llm_end",
			activity: NetActivityLLM,
			position: 11,
			compact:  false,
		},
		{
			name:     "llm_compact",
			activity: NetActivityLLM,
			position: 2,
			compact:  true,
		},
		{
			name:     "mcp_start",
			activity: NetActivityMCP,
			position: 0,
			compact:  false,
		},
		{
			name:     "mcp_compact",
			activity: NetActivityMCP,
			position: 1,
			compact:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNetIndicator()
			n.SetActivity(tt.activity)
			n.position = tt.position

			var output string
			if tt.compact {
				output = n.ViewCompact()
			} else {
				output = n.View()
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

// Helper functions for test data generation

func generateMyses(count int, state string) []MysisInfo {
	myses := make([]MysisInfo, count)
	for i := 0; i < count; i++ {
		myses[i] = MysisInfo{
			ID:              generateID(i),
			Name:            generateName(i),
			State:           state,
			Provider:        getProvider(i),
			AccountUsername: generateAccount(i),
			LastMessage:     generateMessage(i),
			LastMessageAt:   time.Date(2026, 1, 15, 10, i, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		}
		if state == "errored" {
			myses[i].LastError = "Connection timeout"
			myses[i].LastMessage = ""
		}
	}
	return myses
}

func generateMixedStates() []MysisInfo {
	return []MysisInfo{
		{ID: "mysis-1", Name: "alpha", State: "running", Provider: "ollama", AccountUsername: "crab_1", LastMessage: "Running", LastMessageAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC), CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)},
		{ID: "mysis-2", Name: "beta", State: "idle", Provider: "ollama", AccountUsername: "crab_2", LastMessage: "", CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)},
		{ID: "mysis-3", Name: "gamma", State: "stopped", Provider: "opencode_zen", AccountUsername: "crab_3", LastMessage: "", CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)},
		{ID: "mysis-4", Name: "delta", State: "errored", Provider: "ollama", AccountUsername: "crab_4", LastError: "Network error", CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)},
		{ID: "mysis-5", Name: "epsilon", State: "running", Provider: "opencode_zen", AccountUsername: "", LastMessage: "No account", LastMessageAt: time.Date(2026, 1, 15, 10, 5, 0, 0, time.UTC), CreatedAt: time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)},
	}
}

func generateBroadcasts(count int) []SwarmMessageInfo {
	msgs := make([]SwarmMessageInfo, count)
	for i := 0; i < count; i++ {
		msgs[i] = SwarmMessageInfo{
			SenderID:   generateID(i % 3),
			SenderName: generateName(i % 3),
			Content:    generateBroadcastContent(i),
			CreatedAt:  time.Date(2026, 1, 15, 11, i, 0, 0, time.UTC),
		}
	}
	return msgs
}

func generateLongBroadcasts() []SwarmMessageInfo {
	return []SwarmMessageInfo{
		{
			SenderID:   "mysis-1",
			SenderName: "alpha",
			Content:    strings.Repeat("This is a very long broadcast message that should be truncated when displayed in the UI. ", 5),
			CreatedAt:  time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		},
	}
}

func generateUnicodeBroadcasts() []SwarmMessageInfo {
	return []SwarmMessageInfo{
		{
			SenderID:   "mysis-1",
			SenderName: "alpha",
			Content:    "Found asteroid 🪨 with precious cargo 💎 heading to base 🚀",
			CreatedAt:  time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			SenderID:   "mysis-2",
			SenderName: "beta",
			Content:    "Enemy detected ⚠️ requesting backup 🆘",
			CreatedAt:  time.Date(2026, 1, 15, 11, 1, 0, 0, time.UTC),
		},
	}
}

func generateCJKBroadcasts() []SwarmMessageInfo {
	return []SwarmMessageInfo{
		{
			SenderID:   "mysis-1",
			SenderName: "alpha",
			Content:    "採掘完了。鉱石を発見しました。",
			CreatedAt:  time.Date(2026, 1, 15, 11, 0, 0, 0, time.UTC),
		},
		{
			SenderID:   "mysis-2",
			SenderName: "beta",
			Content:    "很好！继续探索。",
			CreatedAt:  time.Date(2026, 1, 15, 11, 1, 0, 0, time.UTC),
		},
	}
}

func generateCJKMyses() []MysisInfo {
	return []MysisInfo{
		{
			ID:              "mysis-1",
			Name:            "探索者",
			State:           "running",
			Provider:        "ollama",
			AccountUsername: "crab_explorer",
			LastMessage:     "探索中",
			LastMessageAt:   time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		},
		{
			ID:              "mysis-2",
			Name:            "采矿机",
			State:           "running",
			Provider:        "opencode_zen",
			AccountUsername: "crab_miner",
			LastMessage:     "采矿完成",
			LastMessageAt:   time.Date(2026, 1, 15, 10, 1, 0, 0, time.UTC),
			CreatedAt:       time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC),
		},
	}
}

func generateAllLoadingSet(count int) map[string]bool {
	set := make(map[string]bool)
	for i := 0; i < count; i++ {
		set[generateID(i)] = true
	}
	return set
}

func generateManyLogs(count int) []LogEntry {
	logs := make([]LogEntry, count)
	roles := []string{"user", "assistant", "system", "tool"}
	sources := []string{"direct", "llm", "system", "tool"}
	for i := 0; i < count; i++ {
		roleIdx := i % len(roles)
		logs[i] = LogEntry{
			Role:      roles[roleIdx],
			Source:    sources[roleIdx],
			Content:   generateLogContent(i),
			Timestamp: time.Date(2026, 1, 15, 10, i%60, i/60, 0, time.UTC),
		}
	}
	return logs
}

func generateLargeJSON() string {
	var sb strings.Builder
	sb.WriteString(`{"ship": {"name": "Explorer", "fuel": 100, "cargo": [`)
	for i := 0; i < 20; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"item": "ore_`)
		sb.WriteString(string(rune('A' + i)))
		sb.WriteString(`", "quantity": `)
		sb.WriteString(string(rune('0' + (i % 10))))
		sb.WriteString(`}`)
	}
	sb.WriteString(`]}, "location": {"x": 1000, "y": 2000, "z": 3000}, "status": "active"}`)
	return sb.String()
}

func generateID(i int) string {
	return "mysis-" + string(rune('a'+i%26)) + string(rune('0'+(i/26)%10))
}

func generateName(i int) string {
	names := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa"}
	if i < len(names) {
		return names[i]
	}
	return "mysis-" + string(rune('0'+i%10))
}

func getProvider(i int) string {
	if i%2 == 0 {
		return "ollama"
	}
	return "opencode_zen"
}

func generateAccount(i int) string {
	accounts := []string{"crab_warrior", "crab_trader", "crab_explorer", "crab_miner", "crab_scout"}
	return accounts[i%len(accounts)]
}

func generateMessage(i int) string {
	messages := []string{
		"Mining asteroid belt",
		"Traveling to sector 7",
		"Trading at station",
		"Scanning for targets",
		"Docked at base",
	}
	return messages[i%len(messages)]
}

func generateBroadcastContent(i int) string {
	contents := []string{
		"All units: proceed to sector 7",
		"Target rich environment detected",
		"Enemy spotted in quadrant 4",
		"Request backup at coordinates",
		"Mission accomplished",
		"Low on fuel, returning to base",
		"Hostile contact, engaging",
		"Cargo hold full, heading back",
		"New waypoint marked",
		"All clear, resuming patrol",
	}
	return contents[i%len(contents)]
}

func generateLogContent(i int) string {
	contents := []string{
		"Executing command",
		"Processing request",
		"Analyzing data",
		"Computing trajectory",
		"Optimizing route",
	}
	return contents[i%len(contents)]
}
