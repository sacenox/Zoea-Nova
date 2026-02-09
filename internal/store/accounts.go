package store

import (
	"database/sql"
	"fmt"
	"time"
)

type Account struct {
	Username   string
	Password   string
	AssignedTo string
	LastUsedAt time.Time
	CreatedAt  time.Time
}

func (s *Store) CreateAccount(username, password string, mysisID ...string) (*Account, error) {
	now := time.Now().UTC()
	assignedTo := ""
	var assignedToParam interface{}
	if len(mysisID) > 0 && mysisID[0] != "" {
		assignedTo = mysisID[0]
		assignedToParam = mysisID[0]
	} else {
		assignedToParam = nil
	}

	_, err := s.db.Exec(`
		INSERT INTO accounts (username, password, assigned_to, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?)
	`, username, password, assignedToParam, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}

	return &Account{
		Username:   username,
		Password:   password,
		AssignedTo: assignedTo,
		CreatedAt:  now,
		LastUsedAt: now,
	}, nil
}

func (s *Store) GetAccount(username string) (*Account, error) {
	var acc Account
	var lastUsedAt sql.NullTime
	var assignedTo sql.NullString

	err := s.db.QueryRow(`
		SELECT username, password, assigned_to, last_used_at, created_at
		FROM accounts WHERE username = ?
	`, username).Scan(&acc.Username, &acc.Password, &assignedTo, &lastUsedAt, &acc.CreatedAt)
	if err != nil {
		return nil, err
	}

	if assignedTo.Valid {
		acc.AssignedTo = assignedTo.String
	}
	if lastUsedAt.Valid {
		acc.LastUsedAt = lastUsedAt.Time
	}

	return &acc, nil
}

func (s *Store) GetAccountByMysisID(mysisID string) (*Account, error) {
	var acc Account
	var lastUsedAt sql.NullTime
	var assignedTo sql.NullString

	err := s.db.QueryRow(`
		SELECT username, password, assigned_to, last_used_at, created_at
		FROM accounts WHERE assigned_to = ?
	`, mysisID).Scan(&acc.Username, &acc.Password, &assignedTo, &lastUsedAt, &acc.CreatedAt)
	if err != nil {
		return nil, err
	}

	if assignedTo.Valid {
		acc.AssignedTo = assignedTo.String
	}
	if lastUsedAt.Valid {
		acc.LastUsedAt = lastUsedAt.Time
	}

	return &acc, nil
}

func (s *Store) ListAvailableAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`
		SELECT username, password, assigned_to, last_used_at, created_at
		FROM accounts
		WHERE assigned_to IS NULL
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query available accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		var acc Account
		var lastUsedAt sql.NullTime
		var assignedTo sql.NullString

		if err := rows.Scan(&acc.Username, &acc.Password, &assignedTo, &lastUsedAt, &acc.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}

		if assignedTo.Valid {
			acc.AssignedTo = assignedTo.String
		}
		if lastUsedAt.Valid {
			acc.LastUsedAt = lastUsedAt.Time
		}

		accounts = append(accounts, &acc)
	}

	return accounts, rows.Err()
}

func (s *Store) ClaimAccount(mysisID string) (*Account, error) {
	if mysisID == "" {
		return nil, fmt.Errorf("mysisID required to claim account")
	}

	now := time.Now().UTC()
	var username, password string
	var createdAt time.Time
	var lastUsedAt sql.NullTime

	// Atomically assign an account permanently to this mysis
	// This prevents race conditions where multiple myses claim the same account
	err := s.db.QueryRow(`
		UPDATE accounts
		SET assigned_to = ?, last_used_at = ?
		WHERE username = (
			SELECT username
			FROM accounts
			WHERE assigned_to IS NULL
			ORDER BY created_at ASC
			LIMIT 1
		)
		RETURNING username, password, created_at, last_used_at
	`, mysisID, now).Scan(&username, &password, &createdAt, &lastUsedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no accounts available")
	}
	if err != nil {
		return nil, fmt.Errorf("claim account: %w", err)
	}

	acc := &Account{
		Username:   username,
		Password:   password,
		AssignedTo: mysisID,
		CreatedAt:  createdAt,
	}

	if lastUsedAt.Valid {
		acc.LastUsedAt = lastUsedAt.Time
	}

	return acc, nil
}

// AssignAccount permanently assigns an account to a mysis
func (s *Store) AssignAccount(username, mysisID string) error {
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		UPDATE accounts
		SET assigned_to = ?, last_used_at = ?
		WHERE username = ?
	`, mysisID, now, username)
	if err != nil {
		return fmt.Errorf("assign account: %w", err)
	}

	return nil
}

// ReleaseAccount clears the permanent assignment of an account (returns it to pool)
func (s *Store) ReleaseAccount(username string) error {
	_, err := s.db.Exec(`
		UPDATE accounts
		SET assigned_to = NULL
		WHERE username = ?
	`, username)
	if err != nil {
		return fmt.Errorf("release account: %w", err)
	}

	return nil
}

// ReleaseAccountByMysisID clears the permanent assignment for a mysis's account
func (s *Store) ReleaseAccountByMysisID(mysisID string) error {
	_, err := s.db.Exec(`
		UPDATE accounts
		SET assigned_to = NULL
		WHERE assigned_to = ?
	`, mysisID)
	if err != nil {
		return fmt.Errorf("release account by mysis: %w", err)
	}

	return nil
}

func (s *Store) ReleaseAllAccounts() error {
	_, err := s.db.Exec(`UPDATE accounts SET assigned_to = NULL`)
	if err != nil {
		return fmt.Errorf("release all accounts: %w", err)
	}

	return nil
}
