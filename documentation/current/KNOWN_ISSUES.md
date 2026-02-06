# Known Issues & Technical Debt

Active todo list of known issues, bugs, and planned improvements for Zoea Nova.

## High Priority

None currently.

## Medium Priority

## Low Priority

### Provider Reliability
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
  - **Reference:** `documentation/KNOWN_SERVER_ISSUES.md`
  - **Process:** Periodic checks against upstream API

---

## Recently Resolved

- [x] **Config validation and type safety** (2026-02-05) - Added comprehensive config validation with aggregated errors for provider/swarm settings. Replaced Event.Data interface{} with typed fields for type safety. Standardized Mysis receiver names. Coverage improved from 61% to 71.4%.

- [x] **Testing coverage expansion** (2026-02-05) - Added config validation tests (11 subtests), provider error handling tests, HTTP mocking for tool calls, MCP proxy tests, and concurrent write benchmark (p50: 0.3ms, p99: 1.9ms).

- [x] **Memory growth analysis** (2026-02-05) - Documented memory growth rate (279 memories/hour, 0.96 MB/hour DB growth). DB size is not a concern for v1. See `documentation/MEMORY_GROWTH_REPORT.md`.

- [x] **Ollama reliability investigation** (2026-02-05) - Analyzed 24h of Ollama logs. Found 65 HTTP 500s, 19 HTTP 400s, 3 prompt truncations, 23 client disconnects. Evidence documented in KNOWN_ISSUES.md for future investigation.

- [x] **State-aware ContinuePrompt** (2026-02-05) - Implemented activity state tracking (idle, traveling, mining, in_combat, cooldown) to suppress nudges during known wait states. Parses arrival_tick and cooldown_ticks from tool results.

- [x] **Prompt reinforcement and time awareness** (2026-02-05) - Reinforced critical rules in ContinuePrompt with drift detection. Removed real-time awareness, replaced with game tick time instructions. Added captain's log guidance with limits and examples.

- [x] **TUI Enhancements** (2026-02-05) - Implemented display reasoning in focus view, account status in dashboard and focus header, JSON tree rendering with verbose toggle, and visual scrollbar indicator. Improves readability and navigation UX.

- [x] **Track broadcast sender and suppress self-response** (2026-02-05) - Added sender_id to memories (schema v8), excluded sender from broadcast recipients, and updated focus view labels to distinguish swarm broadcasts from self broadcasts.

- [x] **Tool payload bloat removal** (2026-02-04) - Removed `provider` and `state` fields from MysisInfo struct and `zoea_list_myses` tool payload. Added `GetStateCounts()` method to Commander for `zoea_swarm_status`. Saves ~22 tokens per mysis, ~352 tokens for full swarm (16 myses).

- [x] **Context snapshot compaction** (2026-02-04) - Implemented snapshot compaction in `getContextMemories()` to keep only most recent result for each snapshot tool (get_ship, get_system, get_poi, get_nearby, get_cargo, zoea_swarm_status, zoea_list_myses). Added search tool reminders to SystemPrompt and ContinuePrompt. See `documentation/CONTEXT_COMPRESSION.md` for details.

- [x] **Database reset with account backup** - Added `make db-reset-accounts` target to safely wipe database while preserving account credentials via export/import cycle.
