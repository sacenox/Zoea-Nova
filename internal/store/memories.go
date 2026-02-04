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
	MemoryRoleTool      MemoryRole = "tool"
)

// MemorySource indicates the origin of a message.
type MemorySource string

const (
	MemorySourceDirect    MemorySource = "direct"    // Direct message to specific agent
	MemorySourceBroadcast MemorySource = "broadcast" // Broadcast message to all agents
	MemorySourceSystem    MemorySource = "system"    // System prompts
	MemorySourceLLM       MemorySource = "llm"       // LLM-generated responses
	MemorySourceTool      MemorySource = "tool"      // Tool call results
)

// Memory represents a stored conversation message.
type Memory struct {
	ID        int64
	AgentID   string
	Role      MemoryRole
	Source    MemorySource
	Content   string
	CreatedAt time.Time
}

// AddMemory adds a memory entry for an agent.
func (s *Store) AddMemory(agentID string, role MemoryRole, source MemorySource, content string) (*Memory, error) {
	now := time.Now().UTC()

	result, err := s.db.Exec(`
		INSERT INTO memories (agent_id, role, source, content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, agentID, role, source, content, now)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Memory{
		ID:        id,
		AgentID:   agentID,
		Role:      role,
		Source:    source,
		Content:   content,
		CreatedAt: now,
	}, nil
}

// GetMemories retrieves all memories for an agent, ordered by creation time.
func (s *Store) GetMemories(agentID string) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, agent_id, role, source, content, created_at
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
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Source, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, &m)
	}

	return memories, rows.Err()
}

// GetSystemMemory retrieves the initial system prompt for an agent.
func (s *Store) GetSystemMemory(agentID string) (*Memory, error) {
	var m Memory
	err := s.db.QueryRow(`
		SELECT id, agent_id, role, source, content, created_at
		FROM memories
		WHERE agent_id = ? AND role = 'system' AND source = 'system'
		ORDER BY created_at ASC
		LIMIT 1
	`, agentID).Scan(&m.ID, &m.AgentID, &m.Role, &m.Source, &m.Content, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetRecentMemories retrieves the most recent N memories for an agent.
func (s *Store) GetRecentMemories(agentID string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, agent_id, role, source, content, created_at
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
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Source, &m.Content, &m.CreatedAt); err != nil {
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

// BroadcastMessage represents a unique broadcast message across all agents.
type BroadcastMessage struct {
	Content   string
	CreatedAt time.Time
}

// GetRecentBroadcasts retrieves the most recent N unique broadcast messages.
// Since broadcasts are stored per-agent, this groups by content to get unique messages.
func (s *Store) GetRecentBroadcasts(limit int) ([]*BroadcastMessage, error) {
	rows, err := s.db.Query(`
		SELECT content, MIN(created_at) as created_at
		FROM memories
		WHERE source = 'broadcast'
		GROUP BY content
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query broadcasts: %w", err)
	}
	defer rows.Close()

	var messages []*BroadcastMessage
	for rows.Next() {
		var content, createdAtStr string
		if err := rows.Scan(&content, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scan broadcast: %w", err)
		}
		// Parse the time string (SQLite returns aggregated times as strings)
		createdAt, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", createdAtStr)
		if createdAt.IsZero() {
			// Try alternate format
			createdAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		}
		messages = append(messages, &BroadcastMessage{
			Content:   content,
			CreatedAt: createdAt,
		})
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

// SearchMemories searches memories for an agent by content text.
// Returns memories where content contains the query string (case-sensitive).
func (s *Store) SearchMemories(agentID, query string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, agent_id, role, source, content, created_at
		FROM memories
		WHERE agent_id = ? AND content LIKE '%' || ? || '%'
		ORDER BY created_at DESC
		LIMIT ?
	`, agentID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.AgentID, &m.Role, &m.Source, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, &m)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}

	return memories, rows.Err()
}

// SearchBroadcasts searches unique broadcast messages by content text.
// Returns broadcasts where content contains the query string (case-sensitive).
func (s *Store) SearchBroadcasts(query string, limit int) ([]*BroadcastMessage, error) {
	rows, err := s.db.Query(`
		SELECT content, MAX(created_at) as created_at
		FROM memories
		WHERE source = 'broadcast' AND content LIKE '%' || ? || '%'
		GROUP BY content
		ORDER BY created_at DESC
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search broadcasts: %w", err)
	}
	defer rows.Close()

	var messages []*BroadcastMessage
	for rows.Next() {
		var content, createdAtStr string
		if err := rows.Scan(&content, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scan broadcast: %w", err)
		}
		createdAt, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", createdAtStr)
		if createdAt.IsZero() {
			createdAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		}
		messages = append(messages, &BroadcastMessage{
			Content:   content,
			CreatedAt: createdAt,
		})
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}
