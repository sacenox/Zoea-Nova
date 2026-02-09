// Package store provides SQLite-based persistence for Myses and memories.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xonecas/zoea-nova/internal/config"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

const currentSchemaVersion = 12

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
	// Add busy timeout to handle concurrent access
	dsn := path
	if !strings.Contains(path, "?") {
		dsn += "?_busy_timeout=5000"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Limit to 1 connection to avoid "database is locked" errors with modernc.org/sqlite
	// especially during tests with concurrent writes.
	db.SetMaxOpenConns(1)

	// Enable WAL mode for better concurrent access (though limited by MaxOpenConns=1)
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
	if err != nil {
		// Table doesn't exist or is empty, create fresh schema
		if _, err := s.db.Exec(schema); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		return nil
	}

	// If version matches, nothing to do
	if version == currentSchemaVersion {
		return nil
	}

	// Per design: no data migrations. Schema changes require manual database deletion.
	if version < currentSchemaVersion {
		return fmt.Errorf(
			"database schema version %d is outdated (current: %d)\n"+
				"Reset the database to continue:\n"+
				"  make db-reset-accounts",
			version, currentSchemaVersion,
		)
	}

	// Future schema version (downgrade not supported)
	if version > currentSchemaVersion {
		return fmt.Errorf(
			"database schema version %d is newer than supported version %d\n"+
				"Upgrade Zoea Nova or reset the database:\n"+
				"  make db-reset-accounts",
			version, currentSchemaVersion,
		)
	}

	return nil
}

// DB returns the underlying database connection for advanced queries.
func (s *Store) DB() *sql.DB {
	return s.db
}
