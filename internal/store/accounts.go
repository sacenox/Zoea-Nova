package store

import (
	"database/sql"
	"fmt"
	"time"
)

type Account struct {
	Username   string
	Password   string
	InUse      bool
	LastUsedAt time.Time
	CreatedAt  time.Time
}

func (s *Store) CreateAccount(username, password string) (*Account, error) {
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		INSERT INTO accounts (username, password, in_use, created_at, last_used_at)
		VALUES (?, ?, 1, ?, ?)
	`, username, password, now, now)
	if err != nil {
		return nil, fmt.Errorf("insert account: %w", err)
	}

	return &Account{
		Username:   username,
		Password:   password,
		InUse:      true,
		CreatedAt:  now,
		LastUsedAt: now,
	}, nil
}

func (s *Store) GetAccount(username string) (*Account, error) {
	var acc Account
	var lastUsedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT username, password, in_use, last_used_at, created_at
		FROM accounts WHERE username = ?
	`, username).Scan(&acc.Username, &acc.Password, &acc.InUse, &lastUsedAt, &acc.CreatedAt)
	if err != nil {
		return nil, err
	}

	if lastUsedAt.Valid {
		acc.LastUsedAt = lastUsedAt.Time
	}

	return &acc, nil
}

func (s *Store) ListAvailableAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`
		SELECT username, password, in_use, last_used_at, created_at
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

		if err := rows.Scan(&acc.Username, &acc.Password, &acc.InUse, &lastUsedAt, &acc.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}

		if lastUsedAt.Valid {
			acc.LastUsedAt = lastUsedAt.Time
		}

		accounts = append(accounts, &acc)
	}

	return accounts, rows.Err()
}

func (s *Store) ClaimAccount() (*Account, error) {
	now := time.Now().UTC()
	var username, password string
	var createdAt time.Time
	var lastUsedAt sql.NullTime

	// Atomically claim an account by updating in_use flag and returning the row
	// This prevents race conditions where multiple myses claim the same account
	err := s.db.QueryRow(`
		UPDATE accounts
		SET in_use = 1, last_used_at = ?
		WHERE username = (
			SELECT username
			FROM accounts
			WHERE in_use = 0
			ORDER BY created_at ASC
			LIMIT 1
		)
		RETURNING username, password, created_at, last_used_at
	`, now).Scan(&username, &password, &createdAt, &lastUsedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no accounts available")
	}
	if err != nil {
		return nil, fmt.Errorf("claim account: %w", err)
	}

	acc := &Account{
		Username:  username,
		Password:  password,
		InUse:     true,
		CreatedAt: createdAt,
	}

	if lastUsedAt.Valid {
		acc.LastUsedAt = lastUsedAt.Time
	}

	return acc, nil
}

func (s *Store) MarkAccountInUse(username string) error {
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		UPDATE accounts
		SET in_use = 1, last_used_at = ?
		WHERE username = ?
	`, now, username)
	if err != nil {
		return fmt.Errorf("mark account in use: %w", err)
	}

	return nil
}

func (s *Store) ReleaseAccount(username string) error {
	_, err := s.db.Exec(`
		UPDATE accounts
		SET in_use = 0
		WHERE username = ?
	`, username)
	if err != nil {
		return fmt.Errorf("release account: %w", err)
	}

	return nil
}

func (s *Store) ReleaseAllAccounts() error {
	_, err := s.db.Exec(`UPDATE accounts SET in_use = 0`)
	if err != nil {
		return fmt.Errorf("release all accounts: %w", err)
	}

	return nil
}
