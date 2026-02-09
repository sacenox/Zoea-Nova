package core

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/config"
	"github.com/xonecas/zoea-nova/internal/provider"
	"github.com/xonecas/zoea-nova/internal/store"
)

// setupTickTest creates a test environment for tick testing
func setupTickTest(t *testing.T) (*Commander, *store.Store, *EventBus, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	bus := NewEventBus(100)

	reg := provider.NewRegistry()
	reg.RegisterFactory("mock", provider.NewMockFactory("mock", "mock response"))

	cfg := &config.Config{
		Swarm: config.SwarmConfig{
			MaxMyses: 16,
		},
		Providers: map[string]config.ProviderConfig{
			"mock": {Endpoint: "http://mock", Model: "mock-model", Temperature: 0.7},
		},
	}

	cmd := NewCommander(s, reg, bus, cfg, "")

	cleanup := func() {
		cmd.StopAll()
		bus.Close()
		s.Close()
	}

	return cmd, s, bus, cleanup
}

// TestTickExtractionFromToolResults tests that current_tick is extracted
// from various JSON payload formats returned by MCP tools.
func TestTickExtractionFromToolResults(t *testing.T) {
	testCases := []struct {
		name          string
		toolResult    *mcp.ToolResult
		expectedTick  int64
		shouldExtract bool
	}{
		{
			name: "top_level_current_tick",
			toolResult: &mcp.ToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: `{"current_tick": 42, "status": "ok"}`},
				},
			},
			expectedTick:  42,
			shouldExtract: true,
		},
		{
			name: "nested_in_data_wrapper",
			toolResult: &mcp.ToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: `{"data": {"current_tick": 88, "ship": {}}}`},
				},
			},
			expectedTick:  88,
			shouldExtract: true,
		},
		{
			name: "tick_field_instead_of_current_tick",
			toolResult: &mcp.ToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: `{"tick": 123, "status": "ok"}`},
				},
			},
			expectedTick:  123,
			shouldExtract: true,
		},
		{
			name: "no_tick_field",
			toolResult: &mcp.ToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: `{"status": "ok", "message": "success"}`},
				},
			},
			expectedTick:  0,
			shouldExtract: false,
		},
		{
			name: "error_result",
			toolResult: &mcp.ToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: `{"error": "failed"}`},
				},
				IsError: true,
			},
			expectedTick:  0,
			shouldExtract: false,
		},
		{
			name: "multiple_content_blocks_with_tick",
			toolResult: &mcp.ToolResult{
				Content: []mcp.ContentBlock{
					{Type: "text", Text: `{"status": "ok", "current_tick": 200}`},
				},
			},
			expectedTick:  200,
			shouldExtract: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			_, testStore, testBus, cleanup := setupTickTest(t)
			defer cleanup()

			mysis := NewMysis("test-id", "test-mysis", time.Now(), nil, testStore, testBus, "")

			// Get initial tick
			mysis.mu.RLock()
			initialTick := mysis.lastServerTick
			mysis.mu.RUnlock()

			if initialTick != 0 {
				t.Fatalf("Initial tick should be 0, got %d", initialTick)
			}

			// Call updateActivityFromToolResult
			mysis.updateActivityFromToolResult(tc.toolResult, nil)

			// Check if tick was updated
			mysis.mu.RLock()
			finalTick := mysis.lastServerTick
			mysis.mu.RUnlock()

			if tc.shouldExtract {
				if finalTick == 0 {
					t.Errorf("ISSUE FOUND: Tick was not extracted from tool result")
					t.Logf("Expected tick: %d, got: %d", tc.expectedTick, finalTick)
					t.Logf("Tool result content: %+v", tc.toolResult.Content)

					// Debug: Try to manually parse the payload
					payload, ok := parseToolResultPayload(tc.toolResult)
					if !ok {
						t.Logf("DEBUG: parseToolResultPayload returned false")
					} else {
						t.Logf("DEBUG: Parsed payload: %+v", payload)
						tick, found := findCurrentTick(payload)
						if !found {
							t.Logf("DEBUG: findCurrentTick returned false")
						} else {
							t.Logf("DEBUG: findCurrentTick found tick: %d", tick)
						}
					}
				} else if finalTick != tc.expectedTick {
					t.Errorf("Tick extracted incorrectly: expected %d, got %d", tc.expectedTick, finalTick)
				} else {
					t.Logf("✓ Tick correctly extracted: %d", finalTick)
				}
			} else {
				if finalTick != 0 {
					t.Errorf("Tick was extracted when it shouldn't be: expected 0, got %d", finalTick)
				} else {
					t.Logf("✓ Tick correctly not extracted (no tick field or error)")
				}
			}
		})
	}
}

// TestAggregateTick_WithRealToolResults tests that AggregateTick returns
// the correct value after myses process tool results with ticks.
func TestAggregateTick_WithRealToolResults(t *testing.T) {
	commander, _, _, cleanup := setupTickTest(t)
	defer cleanup()

	// Create three myses
	m1, err := commander.CreateMysis("mysis-1", "mock")
	if err != nil {
		t.Fatalf("Failed to create mysis-1: %v", err)
	}

	m2, err := commander.CreateMysis("mysis-2", "mock")
	if err != nil {
		t.Fatalf("Failed to create mysis-2: %v", err)
	}

	m3, err := commander.CreateMysis("mysis-3", "mock")
	if err != nil {
		t.Fatalf("Failed to create mysis-3: %v", err)
	}

	// Initially, all ticks should be 0
	tick := commander.AggregateTick()
	if tick != 0 {
		t.Errorf("Initial aggregate tick should be 0, got %d", tick)
	}

	// Simulate tool results with different ticks
	toolResult1 := &mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: `{"current_tick": 98, "status": "ok"}`},
		},
	}

	toolResult2 := &mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: `{"current_tick": 120, "ship": {"id": "test"}}`},
		},
	}

	toolResult3 := &mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: `{"status": "ok"}`}, // No tick
		},
	}

	// Update each mysis with tool results
	m1.updateActivityFromToolResult(toolResult1, nil)
	m2.updateActivityFromToolResult(toolResult2, nil)
	m3.updateActivityFromToolResult(toolResult3, nil)

	// Check individual ticks
	m1.mu.RLock()
	m1Tick := m1.lastServerTick
	m1.mu.RUnlock()

	m2.mu.RLock()
	m2Tick := m2.lastServerTick
	m2.mu.RUnlock()

	m3.mu.RLock()
	m3Tick := m3.lastServerTick
	m3.mu.RUnlock()

	t.Logf("Mysis ticks: m1=%d, m2=%d, m3=%d", m1Tick, m2Tick, m3Tick)

	if m1Tick == 0 {
		t.Errorf("ISSUE FOUND: mysis-1 tick is 0, expected 98")
		t.Logf("This indicates updateActivityFromToolResult is not extracting current_tick")
	}

	if m2Tick == 0 {
		t.Errorf("ISSUE FOUND: mysis-2 tick is 0, expected 120")
	}

	if m3Tick != 0 {
		t.Errorf("mysis-3 tick should remain 0 (no tick in result), got %d", m3Tick)
	}

	// Check aggregate tick
	aggregateTick := commander.AggregateTick()
	t.Logf("Aggregate tick: %d", aggregateTick)

	if aggregateTick == 0 {
		t.Errorf("ISSUE FOUND: AggregateTick returns 0, expected 120 (max of 98, 120, 0)")
		t.Logf("Possible causes:")
		t.Logf("  1. updateActivityFromToolResult is not updating lastServerTick")
		t.Logf("  2. AggregateTick is not reading lastServerTick correctly")
		t.Logf("  3. There's a race condition with mutex locking")
	} else if aggregateTick != 120 {
		t.Errorf("AggregateTick = %d, expected 120 (max tick)", aggregateTick)
	} else {
		t.Logf("✓ AggregateTick correctly returns %d", aggregateTick)
	}

	// Verify the ticks are persisted in the store (if applicable)
	// Note: lastServerTick is runtime-only, not persisted to DB
}

// TestTickUpdateFlow_EndToEnd tests the complete flow from tool result to UI display
func TestTickUpdateFlow_EndToEnd(t *testing.T) {
	commander, _, _, cleanup := setupTickTest(t)
	defer cleanup()

	// Create a mysis
	mysis, err := commander.CreateMysis("test-mysis", "mock")
	if err != nil {
		t.Fatalf("Failed to create mysis: %v", err)
	}

	// Verify initial state
	initialTick := commander.AggregateTick()
	if initialTick != 0 {
		t.Errorf("Initial aggregate tick should be 0, got %d", initialTick)
	}

	// Simulate a tool result with a tick
	toolResult := &mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: `{"current_tick": 42, "status": "operational"}`},
		},
	}

	// Update the mysis
	mysis.updateActivityFromToolResult(toolResult, nil)

	// Check that the tick was updated
	mysis.mu.RLock()
	mysisT := mysis.lastServerTick
	mysis.mu.RUnlock()

	if mysisT == 0 {
		t.Errorf("ISSUE FOUND: Mysis tick is 0 after tool result, expected 42")
		t.Logf("Flow broken at: updateActivityFromToolResult -> lastServerTick")
	}

	// Check that AggregateTick reflects the update
	aggregateTick := commander.AggregateTick()
	if aggregateTick == 0 {
		t.Errorf("ISSUE FOUND: AggregateTick is 0 after tool result, expected 42")
		t.Logf("Flow broken at: lastServerTick -> AggregateTick")
	} else if aggregateTick != 42 {
		t.Errorf("AggregateTick = %d, expected 42", aggregateTick)
	} else {
		t.Logf("✓ Complete flow working: tool result -> lastServerTick -> AggregateTick")
	}

	// Simulate another tool result with a higher tick
	toolResult2 := &mcp.ToolResult{
		Content: []mcp.ContentBlock{
			{Type: "text", Text: `{"current_tick": 100, "data": {"ship": {}}}`},
		},
	}

	mysis.updateActivityFromToolResult(toolResult2, nil)

	// Verify tick increased
	mysis.mu.RLock()
	mysisT2 := mysis.lastServerTick
	mysis.mu.RUnlock()

	if mysisT2 != 100 {
		t.Errorf("Mysis tick after second update: expected 100, got %d", mysisT2)
	}

	aggregateTick2 := commander.AggregateTick()
	if aggregateTick2 != 100 {
		t.Errorf("AggregateTick after second update: expected 100, got %d", aggregateTick2)
	} else {
		t.Logf("✓ Tick correctly updated from 42 to 100")
	}
}
