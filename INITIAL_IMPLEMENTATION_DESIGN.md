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
  - Targeted messaging/tasking for individual swarm members.
- **Command Center TUI:**
  - **Aggregated Dashboard:** High-level swarm status, health, and positioning to prevent UI noise.
  - **Focus Mode:** Detailed logs and direct control for individual agents.
  - **Aesthetic:** Retro-futuristic (80s/90s) CRT-style visuals with reactive animations.

## Tech Stack & Dependencies

- **Core:** Go (Golang) for performance and concurrency.
- **TUI Framework:** `bubbletea` (The Elm Architecture for Go).
- **Styling & Components:** `lipgloss` for layouts/colors and `bubbles` for UI elements.
- **LLM Integration:** `go-openai` (OpenAI-compatible client for Ollama and OpenCode Zen).
- **MCP Protocol:** Researching `mcp-golang` or similar for robust Model Context Protocol support.
- **Configuration:** `github.com/BurntSushi/toml` for TOML parsing.
- **Database:** `modernc.org/sqlite` (pure Go, no CGO) or `github.com/mattn/go-sqlite3` (CGO).
- **Logging:** `zerolog` for structured, non-blocking logs.

## Configuration

- **Environment Variables:** Primary mechanism for runtime overrides (e.g., `ZOEA_LOG_LEVEL`, `ZOEA_MCP_ENDPOINT`).
- **Config File:** `config.toml` in project root for defaults and structure. Parsed via `github.com/BurntSushi/toml`.
- **Credentials:** Stored in `$HOME/.zoea-nova/credentials.json`. Contains API keys for LLM providers. File permissions should be `0600`.

Example `config.toml`:
```toml
[swarm]
default_agents = 4
max_agents = 16

[providers.ollama]
endpoint = "http://localhost:11434"
model = "llama3"

[providers.opencode_zen]
endpoint = "https://api.opencode.ai/v1"  # confirm from Zen dashboard
model = "zen-default"

[mcp]
upstream = "https://game.spacemolt.com/mcp"
```

## State & Persistence

- **Database:** Single SQLite file at `$HOME/.zoea-nova/zoea.db`.
- **Scope:** Agent memories, conversation history, swarm state, per-agent provider config, and user preferences.
- **Migrations:** Schema managed via embedded SQL migrations (e.g., `golang-migrate` or manual versioning). Never support backwards, never write data migrations. If we change the scheme we crate a new db fresh.
- **Backup:** Consider periodic WAL checkpoints; SQLite handles crash recovery.

## Agent Lifecycle

- **Ownership:** The Commander (orchestrator) owns all agent lifecycles.
- **Creation:** User creates agents via TUI controls. Default swarm size: **4 agents**.
- **Limits:** Configurable `max_agents` (default 16). Enforced at Commander level, but user controls count from TUI.
- **States:** `idle` → `running` → `stopped` / `errored`.
- **TUI Controls:**
  - **Create:** Spawn a new agent (if under limit).
  - **Relaunch:** Restart a stopped/errored agent.
  - **Force Stop:** Immediately halt an agent.
  - **Delete:** Remove agent and optionally purge its memories.
  - **Broadcast Message:** Send a message/objective to all agents.
  - **Direct Message:** Send a message to a specific agent.
  - **Configure Agent:** Set model/provider per agent (Ollama or OpenCode Zen).
- **Recovery:** On application restart, agents are rehydrated from SQLite in `stopped` state; user must explicitly relaunch.

## Keyboard Shortcuts

Minimal set matching TUI controls:
- `q` / `Ctrl+C` — Quit
- `n` — New agent
- `d` — Delete selected agent
- `r` — Relaunch selected agent
- `s` — Stop selected agent
- `b` — Broadcast message to all
- `m` — Message selected agent
- `c` — Configure selected agent
- `Tab` / `Shift+Tab` — Navigate between agents
- `?` — Help overlay

## Build & Operations

- **Task Runner:** `Makefile` for standard commands.
- **Commands:**
  - `make fmt` — Format code (`go fmt ./...`)
  - `make build` — Compile binary
  - `make run` — Build and start
  - `make test` — Run tests
- **Versioning:** Semver. Version injected at build time via `-ldflags`.

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
- **Concurrency Model:** Each agent operates in an isolated goroutine. State updates are pushed to the TUI via a central command/message bus to ensure thread-safety.
- **Interface-Driven:** All external dependencies (LLMs, MCP servers) must be hidden behind interfaces to facilitate unit testing and future-proofing.
- **Testing:** Mandatory unit tests for `internal/core` logic. TUI components should be tested using `bubbletea`'s testing utilities. Aim for above 80% test coverage. No flaky tests.
- **Warnings and Errors** Fix all that you see, even if it's from your current changes.
