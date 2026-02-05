# Zoea Nova Initial Design

## Idea

Control several automated players in the game https://www.spacemolt.com/. Each is its own AI. Be able to "drive" the swarm, acting as the swarm orchestrator. Have a TUI to control the swarm—a cool, command center 80s and 90s inspired interface with animations and movement.

## MVP

- **Tech Stack:** Go with the `Bubble Tea` (Charm.sh) framework for a fast, memory-efficient, and highly interactive TUI.
- **Model Connectivity:** Support for local (Ollama) and remote (OpenCode Zen) providers.
- **MCP Layer:**
  - Proxy the official SpaceMolt MCP server (`https://game.spacemolt.com/mcp`) using Streamable HTTP transport.
  - Custom framework to expose internal orchestration tools via MCP.
- **Orchestration:**
  - Capability to broadcast shared objectives to the entire swarm simultaneously.
  - Targeted messaging/tasking for individual Myses.
- **Command Center TUI:**
  - **Aggregated Dashboard:** High-level swarm status, health, swarm message history, and Mysis list.
  - **Focus Mode:** Detailed conversation logs and direct control for individual Myses.
  - **Aesthetic:** Retro-futuristic (80s/90s) CRT-style visuals with reactive animations.

### Dashboard Layout

```
╔═══ ZOEA NOVA COMMAND CENTER ═══╗
● 2  ○ 1  ◌ 0  ✖ 0              ← Mysis state counts (running/idle/stopped/errored)
─── Swarm Messages ───           ← Recent broadcast messages (up to 10)
14:30:05 Hello everyone, mine!
14:32:12 Check your inventories
─── Myses ───                   ← Mysis list with status and last message
⠋ mysis-1    running  [ollama] │ I found some ore...
○ mysis-2    idle     [ollama] │ Ready to start
Press ? for help
```

### Focus Mode Layout

Scrollable conversation viewport showing full message history with role-based styling:
- **System** messages in cyan
- **User** messages in green
- **Assistant** messages in magenta
- **Tool** calls/results in yellow

## Tech Stack & Dependencies

- **Core:** Go (Golang) for performance and concurrency.
- **TUI Framework:** `bubbletea` (The Elm Architecture for Go).
- **Styling & Components:** `lipgloss` for layouts/colors and `bubbles` for UI elements.
- **LLM Integration:** `go-openai` (OpenAI-compatible client for Ollama and OpenCode Zen).
- **MCP Protocol:** Custom MCP proxy and tool registry using Streamable HTTP.
- **Configuration:** `github.com/BurntSushi/toml` for TOML parsing.
- **Database:** `modernc.org/sqlite` (pure Go, no CGO).
- **Logging:** `zerolog` for structured, non-blocking logs.

## Configuration

- **Environment Variables:** Runtime overrides (`ZOEA_MAX_MYSES`, `ZOEA_MCP_ENDPOINT`, `ZOEA_OLLAMA_ENDPOINT`, `ZOEA_OLLAMA_MODEL`, `ZOEA_OLLAMA_TEMPERATURE`, `ZOEA_OLLAMA_RATE_LIMIT`, `ZOEA_OLLAMA_RATE_BURST`, `ZOEA_OPENCODE_ENDPOINT`, `ZOEA_OPENCODE_MODEL`, `ZOEA_OPENCODE_TEMPERATURE`, `ZOEA_OPENCODE_RATE_LIMIT`, `ZOEA_OPENCODE_RATE_BURST`).
- **Config File:** `config.toml` in project root for defaults and structure. Parsed via `github.com/BurntSushi/toml`.
- **Credentials:** Stored in `$HOME/.zoea-nova/credentials.json`. Contains API keys for LLM providers. File permissions should be `0600`.

Example `config.toml`:
```toml
[swarm]
max_myses = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "qwen3:4b"
temperature = 0.7
rate_limit = 2.0
rate_burst = 3

[providers.opencode_zen]
endpoint = "https://api.opencode.ai/v1"
model = "glm-4.7-free"
temperature = 0.7
rate_limit = 10.0
rate_burst = 5

[mcp]
upstream = "https://game.spacemolt.com/mcp"
```

## State & Persistence

- **Database:** Single SQLite file at `$HOME/.zoea-nova/zoea.db`.
- **Scope:** Mysis memories, conversation history, swarm state, per-Mysis provider config, and user preferences.
- **Migrations:** Schema managed via embedded SQL migrations (e.g., `golang-migrate` or manual versioning). Never support backwards, never write data migrations. If we change the schema we create a new db fresh.
- **Backup:** Consider periodic WAL checkpoints; SQLite handles crash recovery.

### Database Schema

```sql
-- Myses table
CREATE TABLE myses (
    id TEXT PRIMARY KY,
    name TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    temperature REAL NOT NULL DEFAULT 0.7,
    state TEXT NOT NULL DEFAULT 'idle',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Memories table (conversation history)
CREATE TABLE memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mysis_id TEXT NOT NULL,
    role TEXT NOT NULL,        -- system, user, assistant, tool
    source TEXT NOT NULL,      -- direct, broadcast, system, llm, tool
    content TEXT NOT NULL,
    reasoning TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mysis_id) REFERENCES myses(id) ON DELETE CASCADE
);

-- Swarm account credentials
CREATE TABLE accounts (
    username TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    in_use BOOLEAN NOT NULL DEFAULT 0,
    last_used_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**Schema v6 → v7 Migration:**
Requires fresh database. See `AGENTS.md` for the reset command and backup details.

**Memory Sources:**
See `AGENTS.md` for the canonical terminology.

## Mysis Lifecycle

- **Ownership:** The Commander (orchestrator) owns all Mysis lifecycles.
- **Creation:** User creates Myses via TUI controls.
- **Limits:** Configurable `max_myses` (default 16). Enforced at Commander level, but user controls count from TUI.
- **States:** `idle` → `running` → `stopped` / `errored`.
- **TUI Controls:**
  - **Create:** Spawn a new Mysis (if under limit).
  - **Relaunch:** Restart a stopped/errored Mysis.
  - **Force Stop:** Immediately halt a Mysis.
  - **Delete:** Remove Mysis and optionally purge its memories.
  - **Broadcast Message:** Send a message/objective to all Myses.
  - **Direct Message:** Send a message to a specific Mysis.
  - **Configure Mysis:** Set model/provider per Mysis (Ollama or OpenCode Zen).
- **Recovery:** On application restart, Myses are rehydrated from SQLite and auto-started.

## Keyboard Shortcuts

Minimal set matching TUI controls:
- `q` / `Ctrl+C` — Quit
- `n` — New Mysis
- `d` — Delete selected Mysis
- `r` — Relaunch selected Mysis
- `s` — Stop selected Mysis
- `b` — Broadcast message to all
- `m` — Message selected Mysis
- `c` — Configure selected Mysis
- `Tab` / `Shift+Tab` — Navigate between Myses
- `?` — Help overlay

## Build & Operations

Use `Makefile` targets documented in `AGENTS.md`.
Versioning follows the rules in `AGENTS.md`.

## Logging

- **Library:** `zerolog` for structured logs.
- **Output:** Truncated log file at `$HOME/.zoea-nova/zoea.log`. Rotated/truncated on startup to keep size manageable.
- **No metrics or tracing** — MVP scope.

## Code Practices & Organization

- **Separation of Concerns:**
  - `internal/core`: Pure orchestration logic, swarm management, and state.
  - `internal/tui`: Bubble Tea models, views, and update logic.
  - `internal/mcp`: MCP client/proxy implementation and tool definitions.
  - `internal/provider`: LLM provider implementations (Ollama, OpenCode Zen).
  - `internal/config`: Configuration loading (TOML + ENV merge).
  - `internal/store`: SQLite repository layer and migrations.
- **Concurrency Model:** Each Mysis operates in an isolated goroutine. State updates are pushed to the TUI via a central command/message bus to ensure thread-safety.
- **Interface-Driven:** All external dependencies (LLMs, MCP servers) must be hidden behind interfaces to facilitate unit testing and future-proofing.
- **Testing:** Mandatory unit tests for `internal/core` logic. TUI components should be tested using `bubbletea`'s testing utilities. Aim for above 80% test coverage. No flaky tests.
- **Warnings and Errors** Fix all that you see, even if it's from your current changes.
