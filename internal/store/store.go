// Package store provides SQLite-based persistence for agents and memories.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/xonecas/zoea-nova/internal/config"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

const currentSchemaVersion = 1

// Store provides access to the SQLite database.
type Store struct {
	db *sql.DB
}

// New creates a new Store with the database at the default location.
func New() (*Store, error) {
	dir, err := config.EnsureDataDir()
	if err != nil {
		return nil, fmt.Errorf("ensure data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "zoea.db")
	return Open(dbPath)
}

// Open opens a database at the given path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// OpenMemory opens an in-memory database for testing.
func OpenMemory() (*Store, error) {
	return Open(":memory:")
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs schema migrations.
func (s *Store) migrate() error {
	// Check current version
	var version int
	err := s.db.QueryRow("SELECT version FROM schema_version LIMIT 1").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		// Table doesn't exist, create fresh schema
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		return nil
	}

	// If version matches, nothing to do
	if version == currentSchemaVersion {
		return nil
	}

	// Per design: forward-only migrations, create fresh DB on schema change
	// For MVP, we just recreate
	if version < currentSchemaVersion {
		// Drop all tables and recreate
		if _, err := s.db.Exec(`
			DROP TABLE IF EXISTS memories;
			DROP TABLE IF EXISTS agents;
			DROP TABLE IF EXISTS schema_version;
		`); err != nil {
			return fmt.Errorf("drop tables: %w", err)
		}
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("recreate schema: %w", err)
		}
	}

	return nil
}

// DB returns the underlying database connection for advanced queries.
func (s *Store) DB() *sql.DB {
	return s.db
}
