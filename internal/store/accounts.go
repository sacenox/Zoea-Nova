package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Account represents a stored game account credential.
type Account struct {
	ID         int64
	Username   string
	Password   string
	Empire     string
	InUse      bool
	ClaimedBy  string // mysis_id or empty
	LastUsedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateAccount stores a new account credential.
func (s *Store) CreateAccount(username, password, empire string) (*Account, error) {
	now := time.Now().UTC()

	result, err := s.db.Exec(`
		INSERT INTO accounts (username, password, empire, in_use, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, ?)
	`, username, password, empire, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}

	id, _ := result.LastInsertId()
	return &Account{
		ID:        id,
		Username:  username,
		Password:  password,
		Empire:    empire,
		InUse:     false,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// GetAccount retrieves an account by username.
func (s *Store) GetAccount(username string) (*Account, error) {
	var acc Account
	var lastUsedAt sql.NullTime
	var claimedBy sql.NullString

	err := s.db.QueryRow(`
		SELECT id, username, password, empire, in_use, claimed_by, last_used_at, created_at, updated_at
		FROM accounts WHERE username = ?
	`, username).Scan(&acc.ID, &acc.Username, &acc.Password, &acc.Empire, &acc.InUse, &claimedBy, &lastUsedAt, &acc.CreatedAt, &acc.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if claimedBy.Valid {
		acc.ClaimedBy = claimedBy.String
	}
	if lastUsedAt.Valid {
		acc.LastUsedAt = lastUsedAt.Time
	}

	return &acc, nil
}

// ListAvailableAccounts returns all unclaimed accounts.
func (s *Store) ListAvailableAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password, empire, in_use, claimed_by, last_used_at, created_at, updated_at
		FROM accounts
		WHERE in_use = 0
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
		var claimedBy sql.NullString

		if err := rows.Scan(&acc.ID, &acc.Username, &acc.Password, &acc.Empire, &acc.InUse, &claimedBy, &lastUsedAt, &acc.CreatedAt, &acc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}

		if claimedBy.Valid {
			acc.ClaimedBy = claimedBy.String
		}
		if lastUsedAt.Valid {
			acc.LastUsedAt = lastUsedAt.Time
		}

		accounts = append(accounts, &acc)
	}

	return accounts, rows.Err()
}

// ClaimAccount marks an account as in use by a mysis.
// Returns error if already claimed or doesn't exist.
func (s *Store) ClaimAccount(username, mysisID string) error {
	now := time.Now().UTC()

	// Check if account exists first
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM accounts WHERE username = ?)`, username).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check account exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("account %s does not exist", username)
	}

	result, err := s.db.Exec(`
		UPDATE accounts
		SET in_use = 1, claimed_by = ?, updated_at = ?
		WHERE username = ? AND in_use = 0
	`, mysisID, now, username)
	if err != nil {
		return fmt.Errorf("claim account: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("account %s is already claimed", username)
	}

	return nil
}

// ReleaseAccount marks an account as available.
func (s *Store) ReleaseAccount(username string) error {
	now := time.Now().UTC()

	result, err := s.db.Exec(`
		UPDATE accounts
		SET in_use = 0, claimed_by = NULL, updated_at = ?
		WHERE username = ?
	`, now, username)
	if err != nil {
		return fmt.Errorf("release account: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ReleaseAccountsByMysis releases all accounts claimed by a mysis.
func (s *Store) ReleaseAccountsByMysis(mysisID string) error {
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		UPDATE accounts
		SET in_use = 0, claimed_by = NULL, updated_at = ?
		WHERE claimed_by = ?
	`, now, mysisID)
	if err != nil {
		return fmt.Errorf("release accounts by mysis: %w", err)
	}

	return nil
}

// UpdateAccountLastUsed updates the last_used_at timestamp.
func (s *Store) UpdateAccountLastUsed(username string) error {
	now := time.Now().UTC()

	result, err := s.db.Exec(`
		UPDATE accounts
		SET last_used_at = ?, updated_at = ?
		WHERE username = ?
	`, now, now, username)
	if err != nil {
		return fmt.Errorf("update account last used: %w", err)
	}

	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}

	return nil
}
