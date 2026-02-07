# TODO Organization Audit - 2026-02-07

Comprehensive audit of `documentation/current/TODO.md` checking for completed items, duplicates, vague requirements, and organizational improvements.

## Summary

**Commits audited:** 133 commits since 2026-02-05  
**Items analyzed:** 8 active TODO items  
**Completed items:** 1  
**Items to clarify:** 4  
**Items to move to KNOWN_ISSUES:** 1  
**Proposed reorganization:** Category-based structure with priority levels

---

## Items to Remove (Completed)

### ✅ REGRESSION: broadcast doesn't start idle myses

**Status:** COMPLETED  
**Commit:** `71e119d` - "chore: final cleanup for idle state message fix" (2026-02-06)  
**Evidence:**
- `f7797cc` - "fix: allow broadcasts to idle myses"
- `036d4f7` - "fix: allow SendMessageFrom in idle state, improve error messages"
- `6cbbc79` - "fix: allow QueueBroadcast in idle state"
- `ff81459` - "feat: add state validation helper for message acceptance"

**Verification:**
- Updated state machine documentation confirms: "Messages can be sent to Myses in `idle` and `running` states"
- Implementation in `internal/core/mysis.go` allows broadcasts to idle myses
- Test coverage gap was addressed with state validation tests

**Action:** Remove from TODO.md

---

## Items to Clarify (Missing Success Criteria)

### 1. "Investigate json in tui, only some tool message render the json correctly"

**Current status:** PARTIALLY IMPLEMENTED  
**Completed work:**
- `8491b5b` - "fix(tui): improve dashboard header and json tree rendering" (2026-02-05)
- JSON tree rendering with `renderJSONTree()` function exists
- Verbose toggle (`v` key) implemented
- Golden tests for JSON rendering exist

**Missing clarity:**
- Which tool messages still render incorrectly?
- What is "correct" rendering? (Expected format/behavior)
- How to test/verify completion?

**Proposed rewrite:**
```markdown
- Audit all tool result rendering in focus view
  - Success criteria: All tool results with JSON payloads render as structured trees
  - Test: Create mysis, trigger all MCP tools, verify JSON rendering in focus view
  - Known working: get_status, get_system, get_notifications
  - Known broken: [NEEDS INVESTIGATION]
```

---

### 2. "tool messages need JSON rendering properly"

**Status:** DUPLICATE of item #1  
**Action:** Remove duplicate, merge with clarified version of item #1

---

### 3. "myses become idle even when there are broadcasts from both commander and other mysis"

**Current status:** UNCLEAR - needs verification  
**Related work:**
- Broadcast sender tracking implemented (`036d4f7`)
- Turn-aware context composition implemented (`7cf375c`)
- Activity-aware nudging implemented (`21c2c66`)
- Autonomous loop preservation (`09e496c` - "fix: preserve tool loop history for autonomous myses")

**Missing clarity:**
- Is this a bug or expected behavior?
- When should myses become idle vs stay running?
- Is this related to the 3-nudge circuit breaker?

**Proposed rewrite:**
```markdown
- Verify mysis autonomy behavior with broadcasts
  - Expected: Myses should stay running and respond to broadcasts
  - Current: Myses become idle despite pending broadcasts [NEEDS REPRO]
  - Related: 3-nudge circuit breaker (line 1691 in mysis.go)
  - Success criteria: Reproduce behavior, determine if bug or intended
  - If bug: Fix and add regression test
  - If intended: Document in state machine and close item
```

---

### 4. "when nudging, encourage broadcast to restart other idle mysis"

**Current status:** UNCLEAR - needs specification  
**Related work:**
- Activity-aware nudging exists (`21c2c66`)
- Continue prompts exist with escalation (gentle/firm/urgent)
- System prompt includes broadcast context

**Missing clarity:**
- Should myses be told to send broadcasts to wake idle peers?
- Or should Commander auto-start idle myses when broadcasts arrive?
- Is this a prompt change or a feature addition?

**Proposed rewrite:**
```markdown
- Add swarm coordination to nudge prompts
  - Option A: Prompt change - Add "coordinate with idle myses" to ContinuePrompt
  - Option B: Feature - Commander auto-starts idle myses on broadcast
  - Option C: Tool addition - Add zoea_start_mysis tool for peer management
  - Decision needed: Which approach aligns with autonomy goals?
  - Success criteria: [Define after approach chosen]
```

---

## Items to Move to KNOWN_ISSUES.md

### "When the app is exiting... show a splash screen with branding"

**Reasoning:**
- This is a quality-of-life enhancement, not a blocking issue
- Exit process already works correctly (goroutine cleanup implemented)
- No user complaints about exit UX
- Low priority polish feature

**Proposed KNOWN_ISSUES entry:**
```markdown
### User Experience
- [ ] **Add exit splash screen** - Show branded loading screen during graceful shutdown
  - **Purpose:** Provide visual feedback during connection cleanup
  - **Current behavior:** Terminal clears immediately on quit, connections close in background
  - **Proposed:** Show logo + "Closing connections..." + progress indicator
  - **Priority:** Low (polish for v1.1+)
  - **Effort:** 1-2 hours (new TUI view, progress tracking)
```

**Action:** Move to KNOWN_ISSUES.md under "User Experience" category

---

## Items Below Release Cutoff (Follow-up)

These items are explicitly marked as post-release and are correctly positioned:

### 1. "Refactor Help/controls panel and error message displays in TUI"
- **Status:** Valid post-release item
- **Clarity:** Good - has detailed specification
- **No action needed**

### 2. "when run with -debug, reset the log file for a clean run"
- **Status:** Valid post-release item
- **Clarity:** Good - clear success criteria
- **No action needed**

### 3. Follow-up checklist items
- Clean up skipped tests (8 tests with rationale)
- Investigate TestStateTransition_Running_To_Idle goroutine hang
- Fix TUI test environment issues
- **Status:** Valid tracking items
- **No action needed**

---

## Duplicate Items

### DUPLICATE: "tool messages need JSON rendering properly"
- Same as "Investigate json in tui, only some tool message render the json correctly"
- **Action:** Remove duplicate after clarifying the original

---

## Proposed Category Structure

Current TODO.md has flat structure. Proposed reorganization:

```markdown
# TODO

**RULES**
- Keep up to date.
- Delete completed items.
- Use blank lines to separate logical sections.
- Managed by the user, ask before edit

---

## Active Work (Pre-Release)

### High Priority
- [Items blocking release]

### Medium Priority
- [Items that should be addressed before release but not blockers]

### Low Priority
- [Nice-to-have improvements for release]

### Needs Investigation
- [Items requiring research/clarification before work can begin]

---

> RELEASE CUTOFF ----~ 8< ~---- STOP HERE **DON'T TOUCH** ----~ 8< ~----

## Post-Release Backlog

### UI/UX Improvements
- [Polish items for v1.1+]

### Testing & Quality
- [Test cleanup, coverage improvements]

### Documentation
- [Doc updates, guides, examples]

---

## Recently Completed
- [Last 3-5 completed items for reference, then move to git history]
```

---

## Priority Ordering

Based on impact and clarity:

### HIGH PRIORITY (Clear + Blocking)
None currently - all pre-release blockers resolved

### MEDIUM PRIORITY (Clear + Important)
None currently - needs clarification first

### LOW PRIORITY (Clear + Polish)
1. Exit splash screen → Move to KNOWN_ISSUES

### NEEDS INVESTIGATION (Unclear)
1. JSON rendering audit (needs scope definition)
2. Mysis idle behavior (needs reproduction)
3. Nudge broadcast coordination (needs specification)

---

## Recommended Actions

### Immediate (User Approval Required)

1. **Remove completed item:**
   - "REGRESSION: broadcast doesn't start idle myses" (completed in 71e119d)

2. **Remove duplicate:**
   - "tool messages need JSON rendering properly" (duplicate of JSON investigation)

3. **Move to KNOWN_ISSUES:**
   - Exit splash screen (polish item, not blocking)

### Clarification Needed (User Input)

4. **JSON rendering investigation:**
   - User needs to identify which tool messages render incorrectly
   - Define "correct" rendering expectations
   - Provide repro steps

5. **Mysis idle behavior:**
   - User needs to confirm if this is still occurring
   - Provide repro steps if bug
   - Or confirm if this is expected behavior from 3-nudge breaker

6. **Nudge broadcast coordination:**
   - User needs to specify desired behavior
   - Prompt change vs feature addition vs tool addition
   - Define success criteria

### Future Consideration

7. **Reorganize TODO.md with categories** (optional)
   - Current flat structure works for small list
   - Consider categories when list grows beyond 10 items
   - Current size (8 items) doesn't warrant reorganization yet

---

## Test Coverage Gap Analysis

While auditing, found these items were completed WITH test coverage:

✅ Broadcast to idle myses - `TestStateTransition_AllowBroadcastInIdle` exists  
✅ Tool loop preservation - `TestMysisContextMemory` covers this  
✅ Turn composition - `test(core): add loop context composition tests` (98fa0a9)

The original TODO claimed "test coverage gap" for broadcast regression, but tests were added during the fix.

---

## Related Documentation Updates Needed

If TODO items are clarified/removed, update:

1. **KNOWN_ISSUES.md** - Add exit splash screen under "User Experience"
2. **AGENTS.md** - No changes needed (terminology already accurate)
3. **README.md** - No changes needed (user-facing features documented)
4. **State Machine docs** - Already updated with broadcast acceptance rules

---

## Conclusion

**Summary of findings:**
- 1 completed item (broadcast regression)
- 1 duplicate item (JSON rendering)
- 1 polish item to move (exit splash screen)
- 4 items need clarification before work can proceed
- 5 post-release items correctly positioned

**Recommendation:** 
User should review the 4 "Needs Investigation" items and provide:
- Reproduction steps for idle behavior bug (or confirm it's not a bug)
- List of specific tool messages with broken JSON rendering
- Decision on nudge broadcast coordination approach
- Confirm whether the duplicate JSON item can be removed

After clarification, TODO.md will be in excellent shape for release.
