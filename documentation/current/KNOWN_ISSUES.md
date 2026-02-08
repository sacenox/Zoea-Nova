# Known Issues & Technical Debt

Active todo list of known issues, bugs, and planned improvements for Zoea Nova.

## High Priority

## Medium Priority

### Testing / Concurrency

- [ ] **TestStateTransition_Running_To_Idle hangs** - Test hangs during cleanup
  - **Location:** `internal/core/state_machine_test.go:260`
  - **Skip Reason:** "Goroutine not exiting after idle transition"
  - **Action:** Investigate potential goroutine leak in idle transition

## Low Priority

### TUI / Rendering

- [ ] **Exit splash screen** - Display branding, connection cleanup status, and animated loading bar during shutdown
  - **Trigger:** User presses q/ESC/CTRL+C
  - **Purpose:** Provide visual feedback during graceful shutdown
  - **Needs:** Splash screen design with loading animation

- [ ] **Inconsistent JSON rendering in tool messages** - Some tool results render as raw text with tool call ID prefix
  - **Example:** `chatcmpl-tool-XXX:{"player": {...}}` instead of tree view
  - **Location:** `internal/tui/focus.go` - `renderLogEntry()`
  - **Action:** Audit JSON detection logic for consistency

### Testing / Coverage

- [ ] **TUI integration tests skipped (config setup)** - Two tests skipped due to missing provider config
  - **Tests:** `TestIntegration_NewMysisInput`, `TestIntegration_CreateAndStartMysis`
  - **Root Cause:** Tests need temp config file after DefaultConfig() removal (commit 068a5a6)
  - **Action:** Update tests to use setupTUITest() pattern from tui_test.go:32-56

### Provider Reliability

- [ ] **OpenCode Zen endpoint workaround for gpt-5-nano** - Using `/chat/completions` instead of documented `/responses` endpoint
  - **Issue:** Official docs specify `gpt-5-nano` uses `/v1/responses` endpoint, but this returns HTTP 500 errors
  - **Error:** `request failed after 3 retries: chat completion status 500` with server error mentioning `input_tokens` (Anthropic field)
  - **Root cause:** `/responses` endpoint appears to expect non-OpenAI request format; our code uses OpenAI SDK with Chat Completions format
  - **Workaround:** Hardcoded `gpt-5-nano` to use `/chat/completions` endpoint (works but contradicts official docs)
  - **Location:** `internal/provider/opencode.go` line 53
  - **Action needed:** Contact OpenCode support to clarify `/responses` endpoint format or confirm `/chat/completions` is acceptable
  - **Reference:** https://opencode.ai/docs/zen/#models

- [ ] **Investigate Ollama timeout errors** - Occasional "context deadline exceeded" errors when calling Ollama chat completions
  - **Error:** `Post "http://localhost:11434/v1/chat/completions": context deadline exceeded`
  - **Needs:** Root cause analysis (model size, request timeout configuration, rate limiting interaction)
  - **Recent evidence (2026-02-05):** Ollama logs show prompt truncation (`limit=32768`, `prompt=41611`) followed by bursts of `400` responses on `/v1/chat/completions` and one `500` response. No corresponding errors in `~/.zoea-nova/zoea.log`.

### Documentation & Tooling

- [ ] **Add plan enforcement command** - OpenCode slash command to require plan/todo creation before implementation
  - **Purpose:** Enforce workflow discipline for complex changes

- [ ] **Add documentation audit command** - OpenCode slash command to audit AGENTS.md and README.md against codebase using @explore
  - **Purpose:** Keep documentation in sync with code changes

### Operations

- [ ] **Validate game server API changes** - Monitor and validate MCP and SpaceMolt game server updates for breaking changes
  - **Reference:** `documentation/current/KNOWN_SERVER_ISSUES.md`
  - **Process:** Periodic checks against upstream API

---

## Recently Resolved

- [x] **Network indicator shows idle despite active Myses** (2026-02-07) - Fixed missing `EventNetworkIdle` after successful MCP tool execution and added counter tracking for network operations. Root cause: `EventNetworkMCP` was published for each tool call but `EventNetworkIdle` was only published on errors, causing counter to grow unbounded. Added `activeNetworkOps` counter in TUI that increments on LLM/MCP events and decrements on idle events. Indicator now correctly shows activity for both user-initiated and autonomous operations. See `documentation/architecture/NETWORK_EVENT_GUARANTEES.md` for event contract. Coverage improved from 85.0% to 85.3%.

- [x] **Config validation and type safety** (2026-02-05) - Added comprehensive config validation with aggregated errors for provider/swarm settings. Replaced Event.Data interface{} with typed fields for type safety. Standardized Mysis receiver names. Coverage improved from 61% to 71.4%.

- [x] **Testing coverage expansion** (2026-02-05) - Added config validation tests (11 subtests), provider error handling tests, HTTP mocking for tool calls, MCP proxy tests, and concurrent write benchmark (p50: 0.3ms, p99: 1.9ms).

- [x] **Memory growth analysis** (2026-02-05) - Documented memory growth rate (279 memories/hour, 0.96 MB/hour DB growth). DB size is not a concern for v1. See `documentation/reports/MEMORY_GROWTH_REPORT.md`.

- [x] **Ollama reliability investigation** (2026-02-05) - Analyzed 24h of Ollama logs. Found 65 HTTP 500s, 19 HTTP 400s, 3 prompt truncations, 23 client disconnects. Evidence documented in KNOWN_ISSUES.md for future investigation.

- [x] **State-aware ContinuePrompt** (2026-02-05) - Implemented activity state tracking (idle, traveling, mining, in_combat, cooldown) to suppress nudges during known wait states. Parses arrival_tick and cooldown_ticks from tool results.

- [x] **Prompt reinforcement and time awareness** (2026-02-05) - Reinforced critical rules in ContinuePrompt with drift detection. Removed real-time awareness, replaced with game tick time instructions. Added captain's log guidance with limits and examples.

- [x] **TUI Enhancements** (2026-02-05) - Implemented display reasoning in focus view, account status in dashboard and focus header, JSON tree rendering with verbose toggle, and visual scrollbar indicator. Improves readability and navigation UX.

- [x] **Track broadcast sender and suppress self-response** (2026-02-05) - Added sender_id to memories (schema v8), excluded sender from broadcast recipients, and updated focus view labels to distinguish swarm broadcasts from self broadcasts.

- [x] **Tool payload bloat removal** (2026-02-04) - Removed `provider` and `state` fields from MysisInfo struct and `zoea_list_myses` tool payload. Added `GetStateCounts()` method to Commander for `zoea_swarm_status`. Saves ~22 tokens per mysis, ~352 tokens for full swarm (16 myses).

- [x] **Context snapshot compaction** (2026-02-04) - Implemented snapshot compaction in `getContextMemories()` to keep only most recent result for each snapshot tool (get_ship, get_system, get_poi, get_nearby, get_cargo, zoea_swarm_status, zoea_list_myses). Added search tool reminders to SystemPrompt and ContinuePrompt. See `documentation/architecture/CONTEXT_COMPRESSION.md` for details.

- [x] **Database reset with account backup** (2026-02-05) - Added `make db-reset-accounts` target to safely wipe database while preserving account credentials via export/import cycle.

- [x] **Goroutine cleanup on exit** (2026-02-06) - Fixed terminal hangs and errored state on quit. Implemented complete goroutine cleanup with timeouts, WaitGroup tracking, and graceful shutdown sequence. See `documentation/plans/2026-02-06-goroutine-cleanup-fixes.md`.

- [x] **OpenCode Zen 500 errors** (2026-02-07) - Fixed system-only message crashes, added Stream parameter, improved message merging, added validation. See `documentation/reports/OPENCODE_ZEN_FIX_2026-02-07.md`.

- [x] **Session ID claim→login loops** (2026-02-07) - Fixed infinite loops caused by session_required errors. Added prompt reinforcement and error message interception. See `documentation/reports/SESSION_ID_LOOP_FIX_IMPLEMENTATION_2026-02-07.md`.
