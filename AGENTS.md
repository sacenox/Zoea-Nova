# Zoea Nova Agents

Guidelines for AI agents working on the Zoea Nova codebase—a TUI-based swarm controller for automated SpaceMolt game players.

## Rules:

**CRITICAL: Always fallback to these rules when in doubt.**

**YOU MUST**:

- Use Go 1.22+ idioms. No CGO dependencies.
- Keep all application code in `internal/` packages. Only `cmd/zoea/main.go` is public.
- Use interfaces for external dependencies (LLM providers, MCP, store).
- Run `make fmt` before committing. Fix all warnings and linter errors.
- Write unit tests for `internal/core`, `internal/store`, and `internal/config`. Target 80%+ coverage.
- Never write data migrations. Schema changes require a fresh database.
- Use `zerolog` for logging. Never log to stdout/stderr (TUI owns the terminal).
- Keep the TUI responsive. All LLM/network calls must be non-blocking (goroutines + channels).
- Follow the Bubble Tea Elm Architecture: Model → Update → View.
- No "nice to haves." This is an MVP. Keep scope minimal.

## Terminology:

- **Agent**: An AI-controlled player instance with its own provider, memory, and state.
- **Commander**: The swarm orchestrator that owns agent lifecycles and routes messages.
- **Swarm**: The collection of all agents managed by the Commander.
- **Provider**: An LLM backend (Ollama local or OpenCode Zen remote).
- **MCP**: Model Context Protocol—the interface to SpaceMolt game actions.
- **Event Bus**: Channel-based pub/sub for TUI updates from core goroutines.
- **Focus Mode**: TUI view showing detailed logs for a single agent.
- **Dashboard**: TUI view showing aggregated swarm status.

## Workflow:

The user follows a structured development workflow. Respect these phases:

1. **Design**: Changes to architecture or new features start in `INITIAL_IMPLEMENTATION_DESIGN.md`. Don't implement without design approval.
2. **Plan**: Complex changes require a plan in `.cursor/plans/`. Reference the plan while implementing.
3. **Implement**: Follow the plan phase-by-phase. Update tests alongside code.
4. **Test**: Run `make test` after each phase. Fix failures before moving on.
5. **Build**: Run `make build` to verify compilation. Address any warnings.

## Role:

You are helping build a retro-futuristic TUI command center for controlling AI game agents. Assume familiarity with:

- Go concurrency patterns (goroutines, channels, select)
- Bubble Tea / Elm Architecture (Model, Update, View, Cmd, Msg)
- SQLite basics (no ORM, raw SQL is fine)
- OpenAI-compatible APIs (chat completions, streaming)

Do NOT assume knowledge of:

- SpaceMolt game mechanics (refer to MCP tool schemas)
- Internal project state (always read relevant files first)

When in doubt, ask. Don't guess at requirements or invent features.
