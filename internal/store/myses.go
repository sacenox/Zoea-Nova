package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MysisState represents the current state of a mysis.
type MysisState string

const (
	MysisStateIdle    MysisState = "idle"
	MysisStateRunning MysisState = "running"
	MysisStateStopped MysisState = "stopped"
	MysisStateErrored MysisState = "errored"
)

// Mysis represents a stored mysis record.
type Mysis struct {
	ID        string
	Name      string
	Provider  string
	Model     string
	State     MysisState
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateMysis creates a new mysis record.
func (s *Store) CreateMysis(name, provider, model string) (*Mysis, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO myses (id, name, provider, model, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, name, provider, model, MysisStateIdle, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert mysis: %w", err)
	}

	return &Mysis{
		ID:        id,
		Name:      name,
		Provider:  provider,
		Model:     model,
		State:     MysisStateIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetMysis retrieves a mysis by ID.
func (s *Store) GetMysis(id string) (*Mysis, error) {
	row := s.db.QueryRow(`
		SELECT id, name, provider, model, state, created_at, updated_at
		FROM myses WHERE id = ?
	`, id)

	return scanMysis(row)
}

// ListMyses returns all myses.
func (s *Store) ListMyses() ([]*Mysis, error) {
	rows, err := s.db.Query(`
		SELECT id, name, provider, model, state, created_at, updated_at
		FROM myses ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query myses: %w", err)
	}
	defer rows.Close()

	var myses []*Mysis
	for rows.Next() {
		mysis, err := scanMysisRows(rows)
		if err != nil {
			return nil, err
		}
		myses = append(myses, mysis)
	}

	return myses, rows.Err()
}

// UpdateMysisState updates a mysis state.
func (s *Store) UpdateMysisState(id string, state MysisState) error {
	result, err := s.db.Exec(`
		UPDATE myses SET state = ?, updated_at = ? WHERE id = ?
	`, state, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update mysis state: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateMysisConfig updates a mysis provider and model.
func (s *Store) UpdateMysisConfig(id, provider, model string) error {
	result, err := s.db.Exec(`
		UPDATE myses SET provider = ?, model = ?, updated_at = ? WHERE id = ?
	`, provider, model, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update mysis config: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteMysis deletes a mysis and its memories (via CASCADE).
func (s *Store) DeleteMysis(id string) error {
	result, err := s.db.Exec(`DELETE FROM myses WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete mysis: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountMyses returns the total number of myses.
func (s *Store) CountMyses() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM myses`).Scan(&count)
	return count, err
}

func scanMysis(row *sql.Row) (*Mysis, error) {
	var m Mysis
	err := row.Scan(&m.ID, &m.Name, &m.Provider, &m.Model, &m.State, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func scanMysisRows(rows *sql.Rows) (*Mysis, error) {
	var m Mysis
	err := rows.Scan(&m.ID, &m.Name, &m.Provider, &m.Model, &m.State, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
