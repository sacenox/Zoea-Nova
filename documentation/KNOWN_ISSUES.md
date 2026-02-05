# Known Issues & Technical Debt

Active todo list of known issues, bugs, and planned improvements for Zoea Nova.

## High Priority

### Prompt & Behavior
- [ ] **State-aware ContinuePrompt**
  - **Impact:** Reduces redundant "waiting" responses during travel and cooldowns
  - **Needs:** Suppress or extend prompt intervals for known wait states; allow non-movement actions during long travel

- [ ] **Reinforce critical rules in ContinuePrompt**
  - **Impact:** Prevents drift when the system prompt falls out of the context window
  - **Needs:** Repeat collaboration rules, themed usernames, and memory search reminders

- [ ] **Remove real-time awareness**
  - **Impact:** Avoids references to real-world time; uses game tick time only

- [ ] **Add explicit `captains_log_add` guidance in prompts**
  - **Impact:** Prevents `empty_entry` errors
  - **Needs:** Include concise examples and constraints in SystemPrompt and ContinuePrompt

## Medium Priority

### TUI Enhancements

Ensure all changes stay consistent with the project's design rules.

- [ ] **Display reasoning in focus view** - Reasoning content is stored in database but not rendered in TUI
  - **Proposed:** Render reasoning messages using existing purple text color
  - **Location:** `internal/tui/focus.go`

- [ ] **Show account status in views** - Surface which game account username each mysis is currently using
  - **Locations:** Focus view header, commander dashboard
  - **Evidence:** Focus labels based on role only; account fields not present in TUI models

- [ ] **Render JSON as tree view** - Tool results with large JSON payloads should use Unicode tree rendering with smart truncation
  - **Format:** Show first 3 items, `[x more]`, last 3 items
  - **Enhancement:** Add verbose toggle for full output

- [ ] **Add scrollbar indicator** - Visual scrollbar for focus view conversation log
  - **Enhancement:** Improves navigation UX for long conversations

## Low Priority

### Provider Reliability
- [ ] **Investigate Ollama timeout errors** - Occasional "context deadline exceeded" errors when calling Ollama chat completions
  - **Error:** `Post "http://localhost:11434/v1/chat/completions": context deadline exceeded`
  - **Needs:** Root cause analysis (model size, request timeout configuration, rate limiting interaction)


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

- [x] **Track broadcast sender and suppress self-response** (2026-02-05) - Added sender_id to memories (schema v8), excluded sender from broadcast recipients, and updated focus view labels to distinguish swarm broadcasts from self broadcasts.

- [x] **Tool payload bloat removal** (2026-02-04) - Removed `provider` and `state` fields from MysisInfo struct and `zoea_list_myses` tool payload. Added `GetStateCounts()` method to Commander for `zoea_swarm_status`. Saves ~22 tokens per mysis, ~352 tokens for full swarm (16 myses).

- [x] **Context snapshot compaction** (2026-02-04) - Implemented snapshot compaction in `getContextMemories()` to keep only most recent result for each snapshot tool (get_ship, get_system, get_poi, get_nearby, get_cargo, zoea_swarm_status, zoea_list_myses). Added search tool reminders to SystemPrompt and ContinuePrompt. See `documentation/CONTEXT_COMPRESSION.md` for details.

- [x] **Database reset with account backup** - Added `make db-reset-accounts` target to safely wipe database while preserving account credentials via export/import cycle.
