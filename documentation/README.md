# Zoea Nova Documentation Index

Central index for all Zoea Nova documentation organized by category.

---

## Quick Links

**Current Status:**
- [TODO](current/TODO.md) - Active work items
- [Known Issues](current/KNOWN_ISSUES.md) - Bugs and technical debt
- [Known Server Issues](current/KNOWN_SERVER_ISSUES.md) - SpaceMolt API problems

**For Developers:**
- [Go Compilation Guide](guides/GO_COMPILATION.md)
- [TUI Testing Guide](guides/TUI_TESTING.md)
- [Terminal Compatibility](guides/TERMINAL_COMPATIBILITY.md)
- [Shell Commands](guides/SHELL_COMMANDS_AND_COMPLETIONS.md)

**For Users:**
- [Ollama Models](guides/OLLAMA_MODELS.md)
- [OpenCode Zen Models](guides/OPENCODE_ZEN_MODELS.md)

---

## Directory Structure

### `architecture/`
Core system design and architectural decisions.

- **[Initial Implementation Design](architecture/INITIAL_IMPLEMENTATION_DESIGN.md)** - Original system architecture and rationale
- **[Mysis State Machine](architecture/MYSIS_STATE_MACHINE.md)** - Valid state transitions and triggers
- **[Context Compression](architecture/CONTEXT_COMPRESSION.md)** - Memory management and sliding window
- **[Rate Limiting](architecture/RATE_LIMITING_PLAN.md)** - Provider rate limiting strategy
- **[Offline Mode](architecture/OFFLINE_MODE.md)** - Stub MCP server for development
- **[OpenAI Compatibility](architecture/OPENAI_COMPATIBILITY.md)** - OpenAI API compliance and provider guidelines

### `guides/`
How-to guides and reference material for developers and users.

**Development:**
- **[Go Compilation](guides/GO_COMPILATION.md)** - Build instructions and requirements
- **[TUI Testing](guides/TUI_TESTING.md)** - Testing patterns for Bubble Tea UI
- **[Terminal Compatibility](guides/TERMINAL_COMPATIBILITY.md)** - Supported terminals and font requirements
- **[Shell Commands](guides/SHELL_COMMANDS_AND_COMPLETIONS.md)** - CLI reference

**Configuration:**
- **[Ollama Models](guides/OLLAMA_MODELS.md)** - Local model configuration
- **[OpenCode Zen Models](guides/OPENCODE_ZEN_MODELS.md)** - Remote provider setup

### `current/`
Active tracking documents (frequently updated).

- **[TODO](current/TODO.md)** - Current work items
- **[Known Issues](current/KNOWN_ISSUES.md)** - Active bugs and technical debt
- **[Known Server Issues](current/KNOWN_SERVER_ISSUES.md)** - SpaceMolt API quirks

### `reports/`
Completed implementation and analysis reports.

**Recent (2026-02-07):**
- **[Cleanup Execution Plan](reports/CLEANUP_EXECUTION_PLAN_2026-02-07.md)** - Phase 1 user-facing docs and quick wins
- **[Dead Code Audit](reports/DEAD_CODE_AUDIT_2026-02-07.md)** - Unused code identification and removal
- **[Documentation Accuracy Audit](reports/DOCUMENTATION_ACCURACY_AUDIT_2026-02-07.md)** - Verification of docs against codebase
- **[Documentation Organization Audit](reports/DOCUMENTATION_ORGANIZATION_AUDIT_2026-02-07.md)** - Structure and navigation improvements
- **[Hardcoded Config Audit](reports/HARDCODED_CONFIG_AUDIT_2026-02-07.md)** - Configuration consistency check
- **[Import Cleanup](reports/IMPORT_CLEANUP_2026-02-07.md)** - Unused import removal
- **[Known Issues Consolidation](reports/KNOWN_ISSUES_CONSOLIDATION_2026-02-07.md)** - Issue tracking organization
- **[LLM Behavior Analysis Session ID](reports/LLM_BEHAVIOR_ANALYSIS_SESSION_ID_2026-02-07.md)** - Session ID error investigation
- **[Login Flow Verification](reports/LOGIN_FLOW_VERIFICATION_2026-02-07.md)** - Account claiming flow analysis
- **[Loop Context Composition](reports/LOOP_CONTEXT_COMPOSITION_IMPLEMENTATION_2026-02-07.md)** - Context building refactor
- **[OpenCode Zen Fix](reports/OPENCODE_ZEN_FIX_2026-02-07.md)** - System-only message crash fix
- **[Outdated Tests Audit](reports/OUTDATED_TESTS_AUDIT_2026-02-07.md)** - Comprehensive test cleanup recommendations
- **[Prompt Reconstruction Tool](reports/PROMPT_RECONSTRUCTION_TOOL_2026-02-07.md)** - Debug tool for LLM prompt analysis
- **[Session ID Error Message Audit](reports/SESSION_ID_ERROR_MESSAGE_AUDIT_2026-02-07.md)** - Error message improvement analysis
- **[Session ID Loop Fix](reports/SESSION_ID_LOOP_FIX_IMPLEMENTATION_2026-02-07.md)** - Infinite loop fix for session errors
- **[Skipped Tests Audit](reports/SKIPPED_TESTS_AUDIT_2026-02-07.md)** - Analysis of t.Skip() tests
- **[Test Coverage Gaps](reports/TEST_COVERAGE_GAPS_2026-02-07.md)** - Missing test identification
- **[TODO Organization](reports/TODO_ORGANIZATION_2026-02-07.md)** - Task tracking cleanup
- **[Turn-Aware Context](reports/TURN_AWARE_CONTEXT_IMPLEMENTATION_2026-02-07.md)** - Context management improvements
- **[ClaimAccount Race Analysis](reports/CLAIM_ACCOUNT_RACE_ANALYSIS_2026-02-07.md)** - Edge case analysis and concurrency safety validation

**Recent (2026-02-06):**
- **[Restart Errored Mysis Investigation](reports/RESTART_ERRORED_MYSIS_INVESTIGATION_2026-02-06.md)** - 10-agent async race fix
- **[Production Stop Failure Tests](reports/PRODUCTION_STOP_FAILURE_TESTS_2026-02-06.md)** - Long-running stability tests
- **[Context Cancellation Review](reports/REVIEW_AGENT_4_CONTEXT_CANCELLATION_REPORT.md)** - Context chain verification
- **[Code Quality Review](reports/CODE_QUALITY_REVIEW_2026-02-06.md)** - 5-agent review of recent changes
- **[Goroutine Cleanup Security](reports/GOROUTINE_CLEANUP_SECURITY_REVIEW.md)** - Shutdown safety analysis
- **[Smart Auto-Scroll](reports/SMART_AUTO_SCROLL_IMPLEMENTATION.md)** - Stateless auto-scroll design

**Feature Implementations:**
- **[LLM Thinking State](reports/LLM_THINKING_STATE_IMPLEMENTATION.md)** - Busy state indicator
- **[HTTP Client Cleanup](reports/HTTP_CLIENT_CLEANUP_IMPLEMENTATION.md)** - Connection pooling
- **[Scrollbar Gap Fix](reports/SCROLLBAR_GAP_FIX_REPORT.md)** - Layout arithmetic correction

**Analysis:**
- **[Memory Growth](reports/MEMORY_GROWTH_REPORT.md)** - Memory usage patterns
- **[UI Layout](reports/UI_LAYOUT_REPORT.md)** - Comprehensive layout analysis
- **[Post-v0.4.4 Cleanup](reports/POST_V044_CLEANUP_SUMMARY.md)** - Refactoring summary

### `plans/`
Implementation plans for features and fixes.

**2026-02-05:**
- **[Statusbar Tick Timestamps](plans/2026-02-05-statusbar-tick-timestamps.md)**
- **[TUI Enhancements](plans/2026-02-05-tui-enhancements.md)**

**Milestone Plans:**
- **[UI Fixes for RC](plans/ui-fixes-for-rc.md)**
- **[V1 Complete](plans/v1-complete.md)**

### `investigations/`
Active investigations and research (work in progress).

- **[Auto Polling Debug](investigations/AUTO_POLLING_DEBUG_REPORT.md)**
- **[Get Notifications API](investigations/GET_NOTIFICATIONS_API_INVESTIGATION.md)**
- **[Get Notifications Plan](investigations/GET_NOTIFICATIONS_IMPLEMENTATION_PLAN.md)**
- **[Tick Investigation](investigations/TICK_INVESTIGATION_FINDINGS.md)**
- **[TUI Tick Display](investigations/TUI_TICK_DISPLAY_INVESTIGATION.md)**

See `investigations/README.md` for details.

### `archive/`
Historical investigations and reports (completed, superseded, or obsolete).

**Completed Plans (2026-02-04 to 2026-02-07):**
- **[Loop Context Checklist](archive/2026-02-07-loop-context-checklist.md)** - Completed
- **[Loop Context Composition](archive/2026-02-07-loop-context-composition.md)** - Completed
- **[Loop Context Parallel Workflow](archive/2026-02-07-loop-context-parallel-workflow.md)** - Completed
- **[Loop Context README](archive/2026-02-07-loop-context-README.md)** - Completed
- **[Loop Context Workflow Diagram](archive/2026-02-07-loop-context-workflow-diagram.md)** - Completed
- **[Goroutine Cleanup Fixes](archive/2026-02-06-goroutine-cleanup-fixes.md)** - Completed
- **[Broadcast Sender Tracking](archive/2026-02-05-broadcast-sender-tracking.md)** - Completed
- **[Context Size Logging](archive/2026-02-05-context-size-logging.md)** - Completed
- **[Captain's Log Bug](archive/2026-02-04-captains-log-bug.md)** - Completed
- **[Context Compaction](archive/2026-02-04-context-compaction-plan.md)** - Completed
- **[Tool Payload Bloat](archive/2026-02-04-remove-tool-payload-bloat.md)** - Completed

**Resolved Investigations (2026-02-06 to 2026-02-07):**
- **[OpenCode Zen API Tests](archive/OPENCODE_ZEN_API_TESTS_2026-02-06.md)** - System-only message bug confirmed
- **[OpenCode Zen Bug Verdict](archive/OPENCODE_ZEN_BUG_VERDICT_2026-02-06.md)** - API limitation verified
- **[Real Provider Race Investigation](archive/AGENT6_REAL_PROVIDER_RACE_INVESTIGATION.md)** - Race condition resolved

**Restart Bug Investigation Reports (2026-02-06):**
- **[Restart Bug Index](archive/RESTART_BUG_INDEX.md)** - Investigation coordination
- **[Goroutine Cleanup Verification](archive/AGENT7_GOROUTINE_CLEANUP_VERIFICATION.md)** - Cleanup order verified
- **[Race Reproduction Report](archive/AGENT_3_RACE_REPRODUCTION_REPORT.md)** - Race condition reproduced
- **[Timing Test Report](archive/TIMING_TEST_REPORT.md)** - Timing analysis
- **[State Machine Test Report](archive/STATE_MACHINE_TEST_REPORT.md)** - State transition analysis

**Superseded Investigations:**
- **[Auto-Scroll Investigation](archive/AUTO_SCROLL_INVESTIGATION.md)** - Replaced by reports/SMART_AUTO_SCROLL_IMPLEMENTATION.md
- **[Cleanup Order Analysis](archive/CLEANUP_ORDER_ANALYSIS.md)** - Implemented in commit 4613e12
- **[Goroutine Cleanup Analysis](archive/GOROUTINE_CLEANUP_ANALYSIS.md)** - Implemented in commit 4613e12
- **[LLM Thinking Investigation](archive/LLM_THINKING_STATE_INVESTIGATION.md)** - Implemented
- **[TUI Quit Handling](archive/TUI_QUIT_HANDLING_ANALYSIS.md)** - Fixed

**Overflow/Layout Fixes:**
- **[Footer Overflow](archive/FOOTER_OVERFLOW_TEST_REPORT.md)** - Fixed
- **[Input Overflow](archive/INPUT_OVERFLOW_TEST_REPORT.md)** - Fixed
- **[Statusbar Overflow](archive/STATUSBAR_OVERFLOW_TEST_REPORT.md)** - Fixed
- **[View Height Overflow](archive/VIEW_HEIGHT_OVERFLOW_ANALYSIS.md)** - Fixed

**Historical Reports:**
- **[Phase 2 Header](archive/PHASE_2_HEADER_INVESTIGATION.md)** - Early development
- **[Phase 6 Layout](archive/PHASE_6_LAYOUT_VERIFICATION_SUMMARY.md)** - Early development
- **[Phase 13 Pre-RC](archive/PHASE_13_PRE_RC_VERIFICATION.md)** - Pre-release check
- **[Old Prompt Baseline](archive/OLD_PROMPT_BASELINE_BEHAVIOR.md)** - Historical reference
- **[Streaming Test](archive/STREAMING_TEST_REPORT.md)** - Provider testing

---

## Documentation Guidelines

**When adding new docs:**

1. **Current work** → `current/` (TODO, KNOWN_ISSUES)
2. **Feature implementation** → Start in `plans/`, finish in `reports/`
3. **Active research** → `investigations/`
4. **Architecture decisions** → `architecture/`
5. **How-to guides** → `guides/`
6. **Completed/obsolete** → `archive/`

**Naming conventions:**
- Plans: `YYYY-MM-DD-feature-name.md`
- Reports: `FEATURE_NAME_IMPLEMENTATION.md` or `FEATURE_NAME_REPORT.md`
- Guides: `TOPIC_NAME.md` (uppercase with underscores)
- Architecture: `CONCEPT_NAME.md` (uppercase with underscores)

**File lifecycle:**
1. Investigation starts in `investigations/`
2. Plan created in `plans/`
3. Implementation happens (code commits)
4. Report written in `reports/`
5. Investigation/plan archived in `archive/`

---

## Finding Documentation

**By Topic:**
- **Build/compile issues** → guides/GO_COMPILATION.md
- **Test failures** → guides/TUI_TESTING.md
- **Terminal rendering** → guides/TERMINAL_COMPATIBILITY.md
- **State transitions** → architecture/MYSIS_STATE_MACHINE.md
- **Memory management** → architecture/CONTEXT_COMPRESSION.md
- **Offline development** → architecture/OFFLINE_MODE.md
- **Known bugs** → current/KNOWN_ISSUES.md
- **SpaceMolt quirks** → current/KNOWN_SERVER_ISSUES.md

**By Date:**
- **Recent work** → `reports/` (sorted by date in filename)
- **Current sprint** → `current/TODO.md`
- **Past investigations** → `archive/` (historical context)

---

## Maintenance

**Keep current:**
- Update `current/TODO.md` as work completes
- Move completed items from TODO to appropriate category
- Archive investigations after implementation
- Consolidate related reports periodically

**Avoid duplication:**
- Check existing docs before creating new ones
- Reference other docs with relative links
- Consolidate overlapping content
- Remove obsolete docs after archiving

---

Last updated: 2026-02-07
