# Active and Historical Investigations

This folder contains investigation reports for issues both active and resolved. Active investigations document ongoing research, while resolved issues are preserved for historical context and understanding of the debugging process.

## Active Investigations

### OpenCode Zen API System-Only Messages Bug

**Issue:** OpenCode Zen API returns `"Cannot read properties of undefined (reading 'input_tokens')"` error when messages contain only system messages (no user or assistant turns).

**Investigation Documents:**
- `OPENCODE_ZEN_API_TESTS_2026-02-06.md` - Direct curl testing proving system-only requests crash

**Status:** Confirmed as API limitation. Workaround implemented in `internal/provider/opencode.go` (fallback to add dummy user message).

**Next Steps:** Consider reporting to OpenCode team.

---

## Resolved Issues

### get_notifications Missing Tick Field

**Issue:** SpaceMolt MCP server did not return `current_tick` in `get_notifications` responses  
**Resolution Date:** 2026-02-06  
**Server Version Fixed:** v0.44.4

**Investigation Documents:**
- `GET_NOTIFICATIONS_API_INVESTIGATION.md` - Comprehensive API testing proving tick was missing
- `TICK_INVESTIGATION_FINDINGS.md` - Root cause analysis
- `GET_NOTIFICATIONS_IMPLEMENTATION_PLAN.md` - Implementation plan for workarounds
- `AUTO_POLLING_DEBUG_REPORT.md` - Automatic polling placement fix
- `TUI_TICK_DISPLAY_INVESTIGATION.md` - TUI refresh investigation

**Outcome:** Server fix in v0.44.4 resolved the issue. Zoea Nova's implementation was correct and worked immediately after server update.

## Purpose

These documents demonstrate:
- Systematic debugging methodology
- API investigation techniques
- Workaround strategies
- Resolution verification process

They serve as reference for future similar investigations.
