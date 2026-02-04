package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AgentState represents the current state of an agent.
type AgentState string

const (
	AgentStateIdle    AgentState = "idle"
	AgentStateRunning AgentState = "running"
	AgentStateStopped AgentState = "stopped"
	AgentStateErrored AgentState = "errored"
)

// Agent represents a stored agent record.
type Agent struct {
	ID        string
	Name      string
	Provider  string
	Model     string
	State     AgentState
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateAgent creates a new agent record.
func (s *Store) CreateAgent(name, provider, model string) (*Agent, error) {
	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO agents (id, name, provider, model, state, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, name, provider, model, AgentStateIdle, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	return &Agent{
		ID:        id,
		Name:      name,
		Provider:  provider,
		Model:     model,
		State:     AgentStateIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetAgent retrieves an agent by ID.
func (s *Store) GetAgent(id string) (*Agent, error) {
	row := s.db.QueryRow(`
		SELECT id, name, provider, model, state, created_at, updated_at
		FROM agents WHERE id = ?
	`, id)

	return scanAgent(row)
}

// ListAgents returns all agents.
func (s *Store) ListAgents() ([]*Agent, error) {
	rows, err := s.db.Query(`
		SELECT id, name, provider, model, state, created_at, updated_at
		FROM agents ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		agent, err := scanAgentRows(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// UpdateAgentState updates an agent's state.
func (s *Store) UpdateAgentState(id string, state AgentState) error {
	result, err := s.db.Exec(`
		UPDATE agents SET state = ?, updated_at = ? WHERE id = ?
	`, state, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update agent state: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UpdateAgentConfig updates an agent's provider and model.
func (s *Store) UpdateAgentConfig(id, provider, model string) error {
	result, err := s.db.Exec(`
		UPDATE agents SET provider = ?, model = ?, updated_at = ? WHERE id = ?
	`, provider, model, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update agent config: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteAgent deletes an agent and its memories (via CASCADE).
func (s *Store) DeleteAgent(id string) error {
	result, err := s.db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete agent: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountAgents returns the total number of agents.
func (s *Store) CountAgents() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&count)
	return count, err
}

func scanAgent(row *sql.Row) (*Agent, error) {
	var a Agent
	err := row.Scan(&a.ID, &a.Name, &a.Provider, &a.Model, &a.State, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanAgentRows(rows *sql.Rows) (*Agent, error) {
	var a Agent
	err := rows.Scan(&a.ID, &a.Name, &a.Provider, &a.Model, &a.State, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
