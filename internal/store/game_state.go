package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GameStateSnapshot represents a cached game state from a tool call.
type GameStateSnapshot struct {
	ID         string
	Username   string
	ToolName   string
	Content    string
	GameTick   int64
	CapturedAt time.Time
}

// StoreGameStateSnapshot stores or updates a game state snapshot for a username+tool combination.
// Uses UPSERT to replace old snapshots with new ones.
func (s *Store) StoreGameStateSnapshot(username, toolName, content string, gameTick int64) error {
	id := uuid.New().String()
	capturedAt := time.Now().UTC().Unix()

	_, err := s.db.Exec(`
		INSERT INTO game_state_snapshots (id, username, tool_name, content, game_tick, captured_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(username, tool_name) DO UPDATE SET
			id = excluded.id,
			content = excluded.content,
			game_tick = excluded.game_tick,
			captured_at = excluded.captured_at
	`, id, username, toolName, content, gameTick, capturedAt)

	return err
}

// GetGameStateSnapshot retrieves a specific snapshot for a username+tool combination.
func (s *Store) GetGameStateSnapshot(username, toolName string) (*GameStateSnapshot, error) {
	var snapshot GameStateSnapshot
	var gameTick sql.NullInt64
	var capturedAtUnix int64

	err := s.db.QueryRow(`
		SELECT id, username, tool_name, content, game_tick, captured_at
		FROM game_state_snapshots
		WHERE username = ? AND tool_name = ?
	`, username, toolName).Scan(
		&snapshot.ID,
		&snapshot.Username,
		&snapshot.ToolName,
		&snapshot.Content,
		&gameTick,
		&capturedAtUnix,
	)

	if err != nil {
		return nil, err
	}

	if gameTick.Valid {
		snapshot.GameTick = gameTick.Int64
	}
	snapshot.CapturedAt = time.Unix(capturedAtUnix, 0)

	return &snapshot, nil
}

// GetAllGameStateSnapshots retrieves all snapshots for a username, ordered by tool name.
func (s *Store) GetAllGameStateSnapshots(username string) ([]*GameStateSnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, username, tool_name, content, game_tick, captured_at
		FROM game_state_snapshots
		WHERE username = ?
		ORDER BY tool_name ASC
	`, username)
	if err != nil {
		return nil, fmt.Errorf("query game state snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*GameStateSnapshot
	for rows.Next() {
		var snapshot GameStateSnapshot
		var gameTick sql.NullInt64
		var capturedAtUnix int64

		if err := rows.Scan(
			&snapshot.ID,
			&snapshot.Username,
			&snapshot.ToolName,
			&snapshot.Content,
			&gameTick,
			&capturedAtUnix,
		); err != nil {
			return nil, fmt.Errorf("scan game state snapshot: %w", err)
		}

		if gameTick.Valid {
			snapshot.GameTick = gameTick.Int64
		}
		snapshot.CapturedAt = time.Unix(capturedAtUnix, 0)

		snapshots = append(snapshots, &snapshot)
	}

	return snapshots, rows.Err()
}

// DeleteGameStateSnapshotsForUsername deletes all snapshots for a username (e.g., on logout).
func (s *Store) DeleteGameStateSnapshotsForUsername(username string) error {
	_, err := s.db.Exec(`
		DELETE FROM game_state_snapshots
		WHERE username = ?
	`, username)
	return err
}

// DeleteGameStateSnapshot deletes a specific snapshot for a username+tool combination.
func (s *Store) DeleteGameStateSnapshot(username, toolName string) error {
	_, err := s.db.Exec(`
		DELETE FROM game_state_snapshots
		WHERE username = ? AND tool_name = ?
	`, username, toolName)
	return err
}

// RecencyMessage returns a human-readable recency string for a snapshot.
func (s *GameStateSnapshot) RecencyMessage(currentTick int64) string {
	if currentTick > 0 && s.GameTick > 0 {
		ticksAgo := currentTick - s.GameTick
		if ticksAgo == 0 {
			return "just now"
		}
		if ticksAgo < 0 {
			// Future tick (shouldn't happen, but handle gracefully)
			return "just now"
		}
		return fmt.Sprintf("T%d ago (%ds)", ticksAgo, ticksAgo*10)
	}

	// Fallback to wall clock time
	elapsed := time.Since(s.CapturedAt)
	if elapsed < time.Minute {
		return fmt.Sprintf("%.0fs ago", elapsed.Seconds())
	}
	if elapsed < time.Hour {
		return fmt.Sprintf("%.0f m ago", elapsed.Minutes())
	}
	return fmt.Sprintf("%.1f h ago", elapsed.Hours())
}
