package store

import (
	"fmt"
	"time"
)

// MemoryRole represents the role in a conversation.
type MemoryRole string

const (
	MemoryRoleSystem    MemoryRole = "system"
	MemoryRoleUser      MemoryRole = "user"
	MemoryRoleAssistant MemoryRole = "assistant"
)

// Memory represents a stored conversation message.
type Memory struct {
	ID        int64
	AgentID   string
	Role      MemoryRole
	Content   string
	CreatedAt time.Time
}

// AddMemory adds a memory entry for an agent.
func (s *Store) AddMemory(agentID string, role MemoryRole, content string) (*Memory, error) {
	now := time.Now().UTC()

	result, err := s.db.Exec(`
		INSERT INTO memories (agent_id, role, content, created_at)
		VALUES (?, ?, ?, ?)
	`, agentID, role, content, now)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Memory{
		ID:        id,
		AgentID:   agentID,
		Role:      role,
		Content:   content,
		CreatedAt: now,
	}, nil
}

// GetMemories retrieves all memories for an agent, ordered by creation time.
func (s *Store) GetMemories(agentID string) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, agent_id, role, content, created_at
		FROM memories
		WHERE agent_id = ?
		ORDER BY created_at ASC
	`, agentID)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, &m)
	}

	return memories, rows.Err()
}

// GetRecentMemories retrieves the most recent N memories for an agent.
func (s *Store) GetRecentMemories(agentID string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, agent_id, role, content, created_at
		FROM memories
		WHERE agent_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, &m)
	}

	// Reverse to get chronological order
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}

	return memories, rows.Err()
}

// DeleteMemories deletes all memories for an agent.
func (s *Store) DeleteMemories(agentID string) error {
	_, err := s.db.Exec(`DELETE FROM memories WHERE agent_id = ?`, agentID)
	if err != nil {
		return fmt.Errorf("delete memories: %w", err)
	}
	return nil
}

// CountMemories returns the number of memories for an agent.
func (s *Store) CountMemories(agentID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM memories WHERE agent_id = ?`, agentID).Scan(&count)
	return count, err
}
