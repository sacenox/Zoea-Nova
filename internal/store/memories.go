package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
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
	MemorySourceDirect    MemorySource = "direct"    // Direct message to specific mysis
	MemorySourceBroadcast MemorySource = "broadcast" // Broadcast message to all myses
	MemorySourceSystem    MemorySource = "system"    // System prompts
	MemorySourceLLM       MemorySource = "llm"       // LLM-generated responses
	MemorySourceTool      MemorySource = "tool"      // Tool call results
)

// Memory represents a stored conversation message.
type Memory struct {
	ID        int64
	MysisID   string
	Role      MemoryRole
	Source    MemorySource
	SenderID  string
	Content   string
	Reasoning string
	CreatedAt time.Time
}

// AddMemory adds a memory entry for a mysis.
func (s *Store) AddMemory(mysisID string, role MemoryRole, source MemorySource, content string, reasoning string, senderID string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		INSERT INTO memories (mysis_id, role, source, sender_id, content, reasoning, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, mysisID, role, source, senderID, content, reasoning, now)
	return err
}

// GetMemories retrieves all memories for a mysis, ordered by creation time.
func (s *Store) GetMemories(mysisID string) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ?
		ORDER BY created_at ASC
	`, mysisID)
	if err != nil {
		return nil, fmt.Errorf("query memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		var senderID sql.NullString
		if err := rows.Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if senderID.Valid {
			m.SenderID = senderID.String
		}
		memories = append(memories, &m)
	}

	return memories, rows.Err()
}

// GetSystemMemory retrieves the initial system prompt for a mysis.
func (s *Store) GetSystemMemory(mysisID string) (*Memory, error) {
	var m Memory
	var senderID sql.NullString
	err := s.db.QueryRow(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ? AND role = 'system' AND source = 'system'
		ORDER BY created_at ASC
		LIMIT 1
	`, mysisID).Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	if senderID.Valid {
		m.SenderID = senderID.String
	}
	return &m, nil
}

// DeleteSystemMemory deletes the system memory for a mysis
func (s *Store) DeleteSystemMemory(mysisID string) error {
	_, err := s.db.Exec(`DELETE FROM memories WHERE mysis_id = ? AND role = 'system'`, mysisID)
	return err
}

// GetRecentMemories retrieves the most recent N memories for a mysis.
func (s *Store) GetRecentMemories(mysisID string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, mysisID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		var senderID sql.NullString
		if err := rows.Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if senderID.Valid {
			m.SenderID = senderID.String
		}
		memories = append(memories, &m)
	}

	// Reverse to get chronological order
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}

	return memories, rows.Err()
}

// DeleteMemories deletes all memories for a mysis.
func (s *Store) DeleteMemories(mysisID string) error {
	_, err := s.db.Exec(`DELETE FROM memories WHERE mysis_id = ?`, mysisID)
	if err != nil {
		return fmt.Errorf("delete memories: %w", err)
	}
	return nil
}

// CountMemories returns the number of memories for a mysis.
func (s *Store) CountMemories(mysisID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM memories WHERE mysis_id = ?`, mysisID).Scan(&count)
	return count, err
}

// BroadcastMessage represents a unique broadcast message across all myses.
type BroadcastMessage struct {
	SenderID  string
	Content   string
	CreatedAt time.Time
}

var broadcastTimeLayouts = []string{
	"2006-01-02 15:04:05.999999999 -0700 MST",
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
}

func parseBroadcastTime(value string) (time.Time, error) {
	for _, layout := range broadcastTimeLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}

	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0).UTC(), nil
	}

	return time.Time{}, fmt.Errorf("parse broadcast time: %q", value)
}

// GetRecentBroadcasts retrieves the most recent N unique broadcast messages.
// Since broadcasts are stored per-mysis, this groups by content to get unique messages.
func (s *Store) GetRecentBroadcasts(limit int) ([]*BroadcastMessage, error) {
	rows, err := s.db.Query(`
		SELECT sender_id, content, MIN(created_at) as created_at
		FROM memories
		WHERE source = 'broadcast'
		GROUP BY content, sender_id
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
		var senderID sql.NullString
		if err := rows.Scan(&senderID, &content, &createdAtStr); err != nil {
			return nil, fmt.Errorf("scan broadcast: %w", err)
		}
		createdAt, err := parseBroadcastTime(createdAtStr)
		if err != nil {
			log.Warn().
				Err(err).
				Str("created_at", createdAtStr).
				Msg("failed to parse broadcast time")
		}
		message := &BroadcastMessage{
			Content:   content,
			CreatedAt: createdAt,
		}
		if senderID.Valid {
			message.SenderID = senderID.String
		}
		messages = append(messages, message)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}

// GetMostRecentBroadcast retrieves the most recent broadcast message for a specific mysis.
// If no broadcast exists for this mysis, falls back to the most recent broadcast in the
// entire system (global swarm mission). This ensures new myses inherit the current mission.
// Returns nil only if no broadcasts exist anywhere in the system.
func (s *Store) GetMostRecentBroadcast(mysisID string) (*Memory, error) {
	var m Memory
	var senderID sql.NullString

	// First, try to find a broadcast for this specific mysis
	err := s.db.QueryRow(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ? AND source = 'broadcast'
		ORDER BY created_at DESC
		LIMIT 1
	`, mysisID).Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt)

	if err == nil {
		// Found a broadcast for this mysis
		if senderID.Valid {
			m.SenderID = senderID.String
		}
		return &m, nil
	}

	if err != sql.ErrNoRows {
		// Real database error
		return nil, fmt.Errorf("query mysis-specific broadcast: %w", err)
	}

	// No broadcast for this mysis - fall back to most recent global broadcast
	// This handles new myses created after a broadcast was sent
	err = s.db.QueryRow(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE source = 'broadcast'
		ORDER BY created_at DESC
		LIMIT 1
	`).Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt)

	if err == sql.ErrNoRows {
		// No broadcasts anywhere in the system
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query global broadcast: %w", err)
	}
	if senderID.Valid {
		m.SenderID = senderID.String
	}

	// Return the global broadcast (with original mysis_id preserved for tracking)
	return &m, nil
}

// SearchMemories searches memories for a mysis by content text.
// Returns memories where content contains the query string (case-sensitive).
func (s *Store) SearchMemories(mysisID, query string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ? AND content LIKE '%' || ? || '%'
		ORDER BY created_at DESC
		LIMIT ?
	`, mysisID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		var senderID sql.NullString
		if err := rows.Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if senderID.Valid {
			m.SenderID = senderID.String
		}
		memories = append(memories, &m)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}

	return memories, rows.Err()
}

func (s *Store) SearchReasoning(mysisID, query string, limit int) ([]*Memory, error) {
	rows, err := s.db.Query(`
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ? AND reasoning LIKE '%' || ? || '%'
		ORDER BY created_at DESC
		LIMIT ?
	`, mysisID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search reasoning: %w", err)
	}
	defer rows.Close()

	var memories []*Memory
	for rows.Next() {
		var m Memory
		var senderID sql.NullString
		if err := rows.Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if senderID.Valid {
			m.SenderID = senderID.String
		}
		memories = append(memories, &m)
	}

	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}

	return memories, rows.Err()
}
