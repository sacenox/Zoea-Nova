# Known Issues & Technical Debt

Active todo list of known issues, bugs, and planned improvements for Zoea Nova.

## High Priority

### Context & Memory Management
- [x] **Remove internal fields from tool payloads** - Strip Zoea-only fields (`provider`, `state`) from orchestrator tool results to reduce token bloat for LLM context. **RESOLVED**: Removed `provider` and `state` fields from MysisInfo struct and `zoea_list_myses` tool payload. Added `GetStateCounts()` method to Commander for `zoea_swarm_status`. Saves ~22 tokens per mysis, ~352 tokens for full swarm (16 myses).

### Prompt & Behavior
- [ ] **Fix self-response to broadcasts** - Myses respond to their own broadcast messages instead of recognizing them as sent by themselves
  - **Impact:** Creates unnecessary conversation loops
  - **Root Cause:** Broadcast messages lack sender tracking
  
- [ ] **Reduce cognitive looping during waits** - ContinuePrompt fires every 30 seconds regardless of mysis state (travel, cooldown), causing redundant "waiting" responses
  - **Current:** Fixed 30-second ticker with no state awareness
  - **Proposed:** Suppress prompts during known wait states or extend interval dynamically
  
- [ ] **Reinforce critical rules in ContinuePrompt** - Important rules (collaboration, themed usernames) only appear in SystemPrompt, which may fall out of the 20-message context window
  - **Impact:** Myses may forget critical behaviors during long sessions
  - **Evidence:** ContinuePrompt includes limited reminders; enforcement depends on SystemPrompt being in context

### Provider Issues
- [ ] **Investigate Ollama timeout errors** - Occasional "context deadline exceeded" errors when calling Ollama chat completions
  - **Error:** `Post "http://localhost:11434/v1/chat/completions": context deadline exceeded`
  - **Needs:** Root cause analysis (model size, request timeout configuration, rate limiting interaction)

## Medium Priority

### Gameplay Improvements
- [ ] **Enable multi-tasking during travel** - Myses should use ticks for non-movement actions (checking cargo, planning routes) during long travel periods
  - **Current:** ContinuePrompt fires but myses just report "waiting for travel"
  - **Evidence:** ContinuePrompt always fires while running, even during travel or cooldown
  
- [ ] **Remove real-time awareness** - Myses reference real-world time instead of game tick time
  - **Impact:** Breaks immersion, may cause confusion about game state timing

### TUI Enhancements
- [ ] **Display reasoning in focus view** - Reasoning content is stored in database but not rendered in TUI
  - **Proposed:** Render reasoning messages using existing purple text color
  - **Location:** `internal/tui/focus.go`
  
- [ ] **Fix broadcast sender labels** - Broadcast messages show "YOU" in focus view regardless of actual sender
  - **Needs:** Track sender ID in broadcast messages and render with consistent style
  
- [ ] **Show account status in views** - Surface which game account username each mysis is currently using
  - **Locations:** Focus view header, commander dashboard
  - **Evidence:** Focus labels based on role only; account fields not present in TUI models
  
- [ ] **Render JSON as tree view** - Tool results with large JSON payloads should use Unicode tree rendering with smart truncation
  - **Format:** Show first 3 items, `[x more]`, last 3 items
  - **Enhancement:** Add verbose toggle for full output

- [ ] **Add scrollbar indicator** - Visual scrollbar for focus view conversation log
  - **Enhancement:** Improves navigation UX for long conversations

## Low Priority

### Documentation & Tooling
- [ ] **Document OpenCode Zen API key path** - Confirm and document the configuration path for OpenCode Zen API keys in user-facing documentation
  - **Target:** README.md or new setup guide section

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

- [x] **Tool payload bloat removal** (2026-02-04) - Removed `provider` and `state` fields from MysisInfo struct and `zoea_list_myses` tool payload. Added `GetStateCounts()` method to Commander for `zoea_swarm_status`. Saves ~22 tokens per mysis, ~352 tokens for full swarm (16 myses).

- [x] **Context snapshot compaction** (2026-02-04) - Implemented snapshot compaction in `getContextMemories()` to keep only most recent result for each snapshot tool (get_ship, get_system, get_poi, get_nearby, get_cargo, zoea_swarm_status, zoea_list_myses). Added search tool reminders to SystemPrompt and ContinuePrompt. See `documentation/CONTEXT_COMPRESSION.md` for details.

- [x] **Database reset with account backup** - Added `make db-reset-accounts` target to safely wipe database while preserving account credentials via export/import cycle.
