package store

import (
	"path/filepath"
	"testing"
)

func setupStoreTest(t *testing.T) (*Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	return s, func() { s.Close() }
}

func TestOpenMemory(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory() error: %v", err)
	}
	defer s.Close()

	// Verify tables exist by querying them
	_, err = s.db.Exec("SELECT 1 FROM agents LIMIT 1")
	if err != nil {
		t.Errorf("agents table not created: %v", err)
	}

	_, err = s.db.Exec("SELECT 1 FROM memories LIMIT 1")
	if err != nil {
		t.Errorf("memories table not created: %v", err)
	}
}

func TestAgentCRUD(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create
	agent, err := s.CreateAgent("test-agent", "ollama", "llama3")
	if err != nil {
		t.Fatalf("CreateAgent() error: %v", err)
	}
	if agent.ID == "" {
		t.Error("expected non-empty agent ID")
	}
	if agent.Name != "test-agent" {
		t.Errorf("expected name=test-agent, got %s", agent.Name)
	}
	if agent.State != AgentStateIdle {
		t.Errorf("expected state=idle, got %s", agent.State)
	}

	// Get
	fetched, err := s.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("GetAgent() error: %v", err)
	}
	if fetched.ID != agent.ID {
		t.Errorf("expected ID=%s, got %s", agent.ID, fetched.ID)
	}

	// List
	agents, err := s.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents() error: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}

	// Update state
	if err := s.UpdateAgentState(agent.ID, AgentStateRunning); err != nil {
		t.Fatalf("UpdateAgentState() error: %v", err)
	}
	fetched, _ = s.GetAgent(agent.ID)
	if fetched.State != AgentStateRunning {
		t.Errorf("expected state=running, got %s", fetched.State)
	}

	// Update config
	if err := s.UpdateAgentConfig(agent.ID, "opencode_zen", "zen-model"); err != nil {
		t.Fatalf("UpdateAgentConfig() error: %v", err)
	}
	fetched, _ = s.GetAgent(agent.ID)
	if fetched.Provider != "opencode_zen" {
		t.Errorf("expected provider=opencode_zen, got %s", fetched.Provider)
	}

	// Count
	count, err := s.CountAgents()
	if err != nil {
		t.Fatalf("CountAgents() error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	// Delete
	if err := s.DeleteAgent(agent.ID); err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}
	count, _ = s.CountAgents()
	if count != 0 {
		t.Errorf("expected count=0 after delete, got %d", count)
	}
}

func TestMemoryCRUD(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	// Create agent first
	agent, err := s.CreateAgent("memory-test", "ollama", "llama3")
	if err != nil {
		t.Fatalf("CreateAgent() error: %v", err)
	}

	// Add memories
	m1, err := s.AddMemory(agent.ID, MemoryRoleSystem, MemorySourceSystem, "You are a helpful assistant.")
	if err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}
	if m1.ID == 0 {
		t.Error("expected non-zero memory ID")
	}

	m2, err := s.AddMemory(agent.ID, MemoryRoleUser, MemorySourceDirect, "Hello!")
	if err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	m3, err := s.AddMemory(agent.ID, MemoryRoleAssistant, MemorySourceLLM, "Hi there!")
	if err != nil {
		t.Fatalf("AddMemory() error: %v", err)
	}

	// Get all memories
	memories, err := s.GetMemories(agent.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 3 {
		t.Errorf("expected 3 memories, got %d", len(memories))
	}

	// Verify order (chronological)
	if memories[0].Role != MemoryRoleSystem {
		t.Errorf("expected first memory role=system, got %s", memories[0].Role)
	}

	// Get recent memories
	recent, err := s.GetRecentMemories(agent.ID, 2)
	if err != nil {
		t.Fatalf("GetRecentMemories() error: %v", err)
	}
	if len(recent) != 2 {
		t.Errorf("expected 2 recent memories, got %d", len(recent))
	}
	// Should be in chronological order (user, assistant)
	if recent[0].Role != MemoryRoleUser {
		t.Errorf("expected recent[0] role=user, got %s", recent[0].Role)
	}

	// Count
	count, err := s.CountMemories(agent.ID)
	if err != nil {
		t.Fatalf("CountMemories() error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}

	// Delete memories
	if err := s.DeleteMemories(agent.ID); err != nil {
		t.Fatalf("DeleteMemories() error: %v", err)
	}
	count, _ = s.CountMemories(agent.ID)
	if count != 0 {
		t.Errorf("expected count=0 after delete, got %d", count)
	}

	// Verify IDs for coverage
	_ = m2.ID
	_ = m3.ID
}

func TestCascadeDelete(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	agent, _ := s.CreateAgent("cascade-test", "ollama", "llama3")
	s.AddMemory(agent.ID, MemoryRoleUser, MemorySourceDirect, "test message")

	// Delete agent should cascade to memories
	if err := s.DeleteAgent(agent.ID); err != nil {
		t.Fatalf("DeleteAgent() error: %v", err)
	}

	// Memories should be gone
	memories, err := s.GetMemories(agent.ID)
	if err != nil {
		t.Fatalf("GetMemories() error: %v", err)
	}
	if len(memories) != 0 {
		t.Errorf("expected 0 memories after cascade delete, got %d", len(memories))
	}
}

func TestUpdateNonExistentAgent(t *testing.T) {
	s, cleanup := setupStoreTest(t)
	defer cleanup()

	err := s.UpdateAgentState("nonexistent-id", AgentStateRunning)
	if err == nil {
		t.Error("expected error updating non-existent agent")
	}

	err = s.UpdateAgentConfig("nonexistent-id", "ollama", "llama3")
	if err == nil {
		t.Error("expected error updating non-existent agent config")
	}

	err = s.DeleteAgent("nonexistent-id")
	if err == nil {
		t.Error("expected error deleting non-existent agent")
	}
}
