package core

import (
	"net/http"
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/provider"
)

// TestStopWithRealOllamaProvider tests the race condition with a REAL Ollama provider
// that has actual network latency. This test is skipped if Ollama is not available.
//
// HYPOTHESIS: The race condition happens because:
// 1. Start() spawns SendMessage(ContinuePrompt) in a goroutine
// 2. Test calls Stop() immediately (5ms window)
// 3. Real HTTP provider takes time to connect/send/receive
// 4. During this window, Stop() sets state=Stopped
// 5. But the LLM call completes and tries to update state
//
// This test should expose if HTTP timing is involved in the race.
func TestStopWithRealOllamaProvider(t *testing.T) {
	// Check if Ollama is available
	resp, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("Ollama not available")
	}
	resp.Body.Close()

	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	// Create mysis with real Ollama provider
	stored, err := s.CreateMysis("ollama-stop-test", "ollama", "qwen3:4b", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	// Use smallest/fastest model available (qwen3:4b)
	ollamaProvider := provider.NewOllama("http://localhost:11434", "qwen3:4b")
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, ollamaProvider, s, bus)

	// Start the mysis (spawns initial SendMessage with REAL HTTP call)
	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// IMMEDIATELY call Stop() during the HTTP request
	time.Sleep(5 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- m.Stop()
	}()

	// Stop should complete within a reasonable timeout
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Stop() - possible deadlock with real provider")
	}

	// CRITICAL CHECK: Verify final state is Stopped (not Errored)
	finalState := m.State()
	if finalState != MysisStateStopped {
		t.Errorf("RACE DETECTED: expected state=stopped after immediate Stop(), got %s (lastError: %v)", finalState, m.LastError())
	}

	// Verify no error was set
	if m.LastError() != nil {
		t.Errorf("RACE DETECTED: expected no lastError after immediate Stop(), got: %v", m.LastError())
	}
}

// TestStopDuringRealLLMCall tests stopping DURING an active LLM call.
// This is different from TestStopWithRealOllamaProvider because:
// - That test stops during INITIAL autonomous turn (Start -> SendMessage)
// - This test stops during an EXPLICIT message send
func TestStopDuringRealLLMCall(t *testing.T) {
	// Check if Ollama is available
	resp, err := http.Get("http://localhost:11434/api/tags")
	if err != nil {
		t.Skip("Ollama not available")
	}
	resp.Body.Close()

	s, bus, cleanup := setupMysisTest(t)
	defer cleanup()

	stored, err := s.CreateMysis("ollama-call-test", "ollama", "qwen3:4b", 0.7)
	if err != nil {
		t.Fatalf("CreateMysis() error: %v", err)
	}

	ollamaProvider := provider.NewOllama("http://localhost:11434", "qwen3:4b")
	m := NewMysis(stored.ID, stored.Name, stored.CreatedAt, ollamaProvider, s, bus)

	if err := m.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Wait for initial turn to complete
	time.Sleep(500 * time.Millisecond)

	// Send a message that will trigger a REAL LLM call
	go func() {
		_ = m.SendMessage("Hello, what's your status?", "direct")
	}()

	// Stop DURING the LLM call (after SendMessage is queued but before response)
	time.Sleep(10 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- m.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for Stop() during real LLM call")
	}

	// Verify final state
	finalState := m.State()
	if finalState != MysisStateStopped {
		t.Errorf("RACE DETECTED: expected state=stopped, got %s (lastError: %v)", finalState, m.LastError())
	}

	if m.LastError() != nil {
		t.Errorf("RACE DETECTED: expected no lastError, got: %v", m.LastError())
	}
}
