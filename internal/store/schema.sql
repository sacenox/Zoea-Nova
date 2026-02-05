-- Zoea Nova database schema

CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY
);

-- Schema v7 → v8 Migration:
-- Added sender_id column to memories table for broadcast tracking.
-- BREAKING CHANGE: Requires fresh database (make db-reset-accounts)
INSERT OR REPLACE INTO schema_version (version) VALUES (8);

CREATE TABLE IF NOT EXISTS myses (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    temperature REAL NOT NULL DEFAULT 0.7,
    state TEXT NOT NULL DEFAULT 'idle',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	mysis_id TEXT NOT NULL,
	role TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT 'direct',
	sender_id TEXT,
	content TEXT NOT NULL,
	reasoning TEXT,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (mysis_id) REFERENCES myses(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_memories_mysis_id ON memories(mysis_id);
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
CREATE INDEX IF NOT EXISTS idx_memories_source ON memories(source);

CREATE TABLE IF NOT EXISTS accounts (
	username TEXT PRIMARY KEY,
	password TEXT NOT NULL,
	in_use BOOLEAN NOT NULL DEFAULT 0,
	last_used_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_accounts_in_use ON accounts(in_use);
