# Zoea Nova Agents

Guidelines for AI agents working on the Zoea Nova codebase—a TUI-based swarm controller for automated SpaceMolt game players.

## Rules:

**CRITICAL: Always fallback to these rules when in doubt.**

**YOU MUST**:

- Use Go 1.22+ idioms. No CGO dependencies.
- Keep all application code in `internal/` packages. Only `cmd/zoea/main.go` is public.
- Use interfaces for external dependencies (LLM providers, MCP, store).
- Run `make fmt` before committing. Fix all warnings and linter errors.
- When committing, use `required_permissions: ["all"]` to bypass sandbox restrictions (GPG signing requires full filesystem access).
- Write unit tests for all modules. Target 95%+ coverage.
- Never write data migrations. Schema changes require a fresh database.
- Use `zerolog` for logging. Never log to stdout/stderr (TUI owns the terminal).
- Keep the TUI responsive. All LLM/network calls must be non-blocking (goroutines + channels).
- Follow the Bubble Tea Elm Architecture: Model → Update → View.
- No "nice to haves." This is an MVP. Keep scope minimal. MVP is not an escuse for lack of testing or code quality.

## TUI Testing:

When testing styled TUI output with lipgloss:

- **ANSI codes are stripped without TTY**: Lipgloss doesn't output ANSI escape codes when there's no terminal. Tests run without a TTY, so styled output appears unstyled by default.
- **Force color output**: Use `lipgloss.SetColorProfile(termenv.TrueColor)` at the start of tests that verify styling. Always `defer lipgloss.SetColorProfile(termenv.Ascii)` to reset.
- **Strip ANSI for content checks**: When verifying text content in styled output, strip ANSI codes first. Use a regex like `\x1b\[[0-9;]*m` to remove escape sequences.
- **Verify styling is applied**: Check for `"\x1b["` to confirm ANSI codes are present. Check for `"48;"` specifically for background colors, `"38;"` for foreground.
- **Use `lipgloss.Width()` for display width**: Never use `len()` on styled strings—it counts bytes including ANSI codes. `lipgloss.Width()` returns the actual display width.
- **Test the actual styled output**: Don't just test that code runs without panicking. Verify the styling produces correct ANSI codes and that content is properly formatted.

## Unicode Width Calculations:

Multi-byte Unicode characters cause width calculation bugs:

- **`len()` returns BYTES, not display width**: Characters like `◈`, `◆`, `╭`, `─` are 3 bytes each but display as 1 column. Using `len()` for width calculations produces incorrect results.
- **ALWAYS use `lipgloss.Width()`**: This correctly calculates display width for both Unicode and ANSI-styled strings.
- **Test with Unicode-heavy content**: Section titles and decorative borders use Unicode box-drawing characters. Always test width calculations with actual Unicode content.
- **Panel alignment requires exact widths**: If a section title is `width` chars and a panel border is calculated with `len()` instead of `lipgloss.Width()`, they will visually misalign.
- **Style padding affects alignment**: `lipgloss.Style.Padding(0, 1)` adds 1 space on each side. If one element has padding and another doesn't, their decorations won't align even if both are "width" chars total.
- **`lipgloss.Width()` sets content width**: When using `style.Width(n)`, the `n` sets the CONTENT width. Borders and padding are added ON TOP. So `style.Width(98)` with a border produces total width 100.

## Terminology:

- **Agent**: An AI-controlled player instance with its own provider, memory, and state.
- **Commander**: The swarm orchestrator that owns agent lifecycles and routes messages.
- **Swarm**: The collection of all agents managed by the Commander.
- **Provider**: An LLM backend (Ollama local or OpenCode Zen remote).
- **MCP**: Model Context Protocol—the interface to SpaceMolt game actions.
- **Event Bus**: Channel-based pub/sub for TUI updates from core goroutines.
- **Focus Mode**: TUI view showing detailed conversation logs for a single agent.
- **Dashboard**: TUI view showing swarm status, broadcast history, and agent list.
- **Memory**: A stored conversation message with role (system/user/assistant/tool) and source.
- **Source**: Origin of a memory—`direct` (single agent), `broadcast` (swarm), `system`, `llm`, or `tool`.

## Workflow:

The user follows a structured development workflow. Respect these phases:

1. **Design**: Documentation for this project is in `documentation/` Keep it up to date and accurate.
2. **Plan**: Complex changes require a plan in `.cursor/plans/`. Reference the plan while implementing.
3. **Implement**: Follow the plan phase-by-phase. Update tests alongside code, add more as needed.
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
