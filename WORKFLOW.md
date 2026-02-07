# Zoea Nova Development Workflow

## Purpose

This document defines the standard workflow for complex refactoring and architectural changes in Zoea Nova. It codifies the process used successfully for the nudge-to-encouragement system migration (commit 82df0ce, -785 lines, 11 agents coordinated).

---

## When to Use This Workflow

Use this workflow for:
- **Architectural changes** - Replacing core systems or patterns
- **Large refactoring** - Changes spanning 10+ files or 500+ lines
- **API changes** - Modifications to core interfaces or contracts
- **System redesign** - Changing fundamental behavior or state machines

Do NOT use for:
- Simple bug fixes (1-2 files, clear scope)
- Documentation-only changes
- Dependency updates
- Configuration tweaks

---

## Workflow Phases

### Phase 1: Document the Truth

**Goal:** Create single source of truth before touching code.

**Steps:**

1. **Analyze the problem**
   - What's broken or needs changing?
   - What's the root cause?
   - What are the constraints?

2. **Create truth table**
   - Document all possible states
   - Document all transitions
   - Document trigger conditions
   - Document counter/flag behavior

3. **Write architecture document**
   - Location: `documentation/architecture/`
   - Include:
     - Truth table (state × condition → action)
     - Message flow diagrams
     - Code location references
     - Design rationale (why this approach)
     - Migration path (current → target)
   - Format: Clear, specific, testable assertions

4. **Review with user**
   - Present truth table
   - Verify assumptions
   - Get explicit approval before coding

**Deliverable:** Architecture doc with truth table in `documentation/architecture/`

**Example:** `MESSAGE_FORMAT_GUARANTEES.md` with state/message/counter truth table

---

### Phase 2: Parallel Agent Implementation

**Goal:** Execute changes efficiently using coordinated agent fleet.

**Minimum agents:** 3 (recommended: 5-6 for large changes)

**Steps:**

1. **Break work into independent tasks**
   - Each task should be:
     - Isolated (no dependencies on other agents)
     - Testable (can verify independently)
     - Bounded (clear start/end)
   - Example tasks:
     - Agent 1: Remove obsolete code (ticker, channels)
     - Agent 2: Remove provider fallbacks
     - Agent 3: Rename variables across codebase
     - Agent 4: Implement counter increment logic
     - Agent 5: Implement counter reset logic
     - Agent 6: Update tests

2. **Launch agents in parallel**
   - Use task tool with clear instructions
   - Reference the truth table document
   - Specify what NOT to change (avoid conflicts)
   - Request specific deliverables (summary with line numbers)

3. **Monitor agent completion**
   - Agents return task_id for resumption
   - Review each agent's summary as it completes
   - Note any unexpected issues

**Deliverable:** All assigned tasks completed, summaries from each agent

**Example:** 6 agents removed ticker code, renamed variables, updated tests in parallel (~15 min total vs ~90 min sequential)

---

### Phase 3: Integration Review

**Goal:** Verify agent work integrates correctly and matches truth table.

**Steps:**

1. **Check for conflicts**
   - Review overlapping changes
   - Verify no duplicate edits
   - Check for missing dependencies

2. **Verify truth table compliance**
   - For each row in truth table:
     - Find corresponding code
     - Verify behavior matches spec
     - Test the scenario

3. **Build verification**
   - Run `make build`
   - Fix compilation errors
   - Check for LSP errors

4. **Test verification**
   - Run `make test`
   - Review failures
   - Categorize: real bugs vs outdated tests

5. **Fix integration issues**
   - Address conflicts between agents
   - Remove duplicate code
   - Fix broken tests
   - Handle edge cases

**Deliverable:** Clean build, passing tests, truth table verified

**Example:** Found and fixed encouragementCount reset in wrong location, removed duplicate test code, updated provider tests

---

### Phase 4: Regression Analysis

**Goal:** Find issues the first agent pass missed.

**Minimum review agents:** 3 (independent perspectives)

**Steps:**

1. **Launch review agents**
   - Each agent reviews different aspect:
     - Agent 1: State transition correctness
     - Agent 2: Trigger mechanisms and flow
     - Agent 3: Test coverage and edge cases
   - Reference truth table for verification
   - Request specific issue analysis format

2. **Compare findings**
   - Look for consensus (all agents found same issue)
   - Look for unique findings (one agent caught something)
   - Prioritize by severity

3. **Validate findings**
   - Check if issue is real or false positive
   - Verify against truth table
   - Test the scenario if unclear

4. **Fix critical issues**
   - Use additional agents if fix is complex
   - Or fix manually if straightforward
   - Re-verify after fix

**Deliverable:** All critical regressions found and fixed

**Example:** 3 review agents found run() loop was empty (critical), Start() didn't reset counter (high), SendMessageFrom didn't auto-start idle myses (high)

---

### Phase 5: Fix Implementation

**Goal:** Implement fixes for issues found in regression analysis.

**Approach:** Multiple agents with different solutions (if complex)

**Steps:**

1. **For complex fixes:**
   - Launch 3 agents with same requirements
   - Each agent implements independent solution
   - Compare approaches:
     - Code simplicity
     - Performance
     - Maintainability
     - Risk level
   - Choose best solution or merge ideas

2. **For simple fixes:**
   - Implement directly
   - Verify against truth table
   - Test immediately

3. **Integration test**
   - Run full test suite
   - Verify all edge cases
   - Check performance

**Deliverable:** Working implementation matching truth table

**Example:** 3 agents proposed different run() loop fixes (direct call, extracted helper, polling), chose simplest working solution

---

### Phase 6: User Review and Commit

**Goal:** Get user approval and create clean commit history.

**Steps:**

1. **Present summary**
   - What changed (high level)
   - What was removed (line counts)
   - What was added (new behavior)
   - Test status (coverage, pass rate)
   - Truth table compliance verification

2. **User review**
   - User verifies changes match expectations
   - User tests manually if needed
   - User documents any unrelated issues found

3. **Create commit**
   - Use conventional commits format
   - Include:
     - Type (refactor, feat, fix)
     - Short summary (what changed)
     - Detailed body (what/why for each area)
     - Statistics (lines changed, files affected)
     - Verification (tests passing, coverage)
   - Reference truth table document

4. **Final verification**
   - `git show --stat HEAD`
   - Verify commit message accuracy
   - Verify all changes included

**Deliverable:** Clean commit matching conventional commits style, user approval

**Example:** Single comprehensive commit with detailed breakdown of all changes, +888 -1673 lines

---

## Success Metrics

A successful workflow execution should achieve:

1. **Correctness:** All truth table scenarios verified in code
2. **Completeness:** No missing edge cases or half-implemented features
3. **Test coverage:** All new behavior has tests, obsolete tests removed
4. **Documentation:** Truth table doc created, outdated comments cleaned
5. **Code quality:** Net negative lines (removed complexity)
6. **Build status:** All tests passing, clean compilation
7. **Efficiency:** Parallel agents complete in fraction of sequential time

---

## Anti-Patterns to Avoid

**DON'T:**
- Code before documenting the truth table
- Use single agent for large multi-file changes
- Skip regression analysis phase
- Commit without user review
- Leave TODO comments for "later cleanup"
- Mix unrelated changes in one commit
- Write tests after implementation (write truth table first)

**DO:**
- Document state machines and truth tables upfront
- Use minimum 3 agents for complex changes
- Review agent work before moving forward
- Get explicit user approval
- Clean up as you go (remove obsolete code immediately)
- Split commits by logical concern
- Write architecture docs that match implementation

---

## Tool Usage

### Agent Coordination

**Parallel execution:**
```
Agent 1: Remove subsystem A
Agent 2: Remove subsystem B  
Agent 3: Rename variables
Agent 4: Implement new behavior X
Agent 5: Implement new behavior Y
Agent 6: Update tests
```

**Sequential execution:**
```
Phase 1: Agents 1-6 (implementation)
Phase 2: Review integration
Phase 3: Agents 7-9 (regression review)
Phase 4: Agents 10-12 (fix implementation)
Phase 5: Final verification
```

### Review Agents

Always launch minimum 3 review agents with:
- Different focus areas (state, flow, tests)
- Independent analysis (no communication)
- Specific deliverables (issue list with line numbers)
- Truth table reference

### Fix Agents

For complex fixes, launch 3 agents with:
- Same requirements
- Different approaches encouraged
- Complete implementation expected
- Comparison matrix requested

Choose best solution or merge ideas.

---

## Template Checklist

Use this checklist for each workflow execution:

- [ ] Phase 1: Truth table documented in `documentation/architecture/`
- [ ] Phase 1: User approved truth table
- [ ] Phase 2: Minimum 3 agents launched for implementation
- [ ] Phase 2: All agents completed and summarized work
- [ ] Phase 3: Build successful (`make build`)
- [ ] Phase 3: Initial test run completed
- [ ] Phase 3: Integration issues fixed
- [ ] Phase 4: Minimum 3 review agents launched
- [ ] Phase 4: All review findings documented
- [ ] Phase 4: Critical issues prioritized
- [ ] Phase 5: Fixes implemented (3 agents if complex)
- [ ] Phase 5: Final test run passing
- [ ] Phase 5: Truth table scenarios verified
- [ ] Phase 6: User reviewed changes
- [ ] Phase 6: Commit created with detailed message
- [ ] Phase 6: Commit verified with `git show --stat`

---

## Real-World Example: Nudge System Refactor

**Phase 1:**
- Problem: Myses auto-idling after 90s despite processing broadcasts
- Truth table: Created MESSAGE_FORMAT_GUARANTEES.md with 7-row state table
- User approval: Confirmed new encouragement-based approach

**Phase 2:**
- 6 agents launched in parallel
- Agent 1: Removed ticker (450 lines)
- Agent 2: Removed provider fallbacks (24 lines)
- Agent 3: Renamed variables (28 occurrences)
- Agent 4: Implemented counter increment
- Agent 5: Implemented counter reset
- Agent 6: Updated initial tests

**Phase 3:**
- Build: Successful
- Tests: 88/91 passing (3 timeout failures)
- Integration: Fixed duplicate code, updated provider tests
- Time: ~30 minutes

**Phase 4:**
- 3 review agents launched
- All found: run() loop empty (critical bug)
- Findings: 4 issues (1 critical, 2 high, 1 low)
- Prioritized: Fix run() loop immediately

**Phase 5:**
- 3 fix agents launched (different approaches)
- Agent 1: Direct call (simple, working)
- Agent 2: Extracted helper (clean but complex)
- Agent 3: Polling loop (works but inefficient)
- Chose: Agent 1 solution (already tested)

**Phase 6:**
- User review: Manual testing revealed unrelated issues (documented in TODO)
- Commit: Single comprehensive refactor commit
- Result: 21 files, -785 lines, all tests passing

**Total time:** ~2 hours (vs estimated 8-12 hours sequential)

---

## Lessons Learned

1. **Truth table prevents scope creep** - Clear boundaries for what changes
2. **Parallel agents are 4-6x faster** - Independent work, no blocking
3. **Review agents catch critical bugs** - Fresh perspective finds issues
4. **Multiple fix approaches reveal best solution** - Compare, don't guess
5. **User review at end prevents wasted work** - Manual testing finds real issues
6. **Single commit preferred for refactors** - Atomic, easy to revert

---

## Workflow Evolution

This workflow will evolve based on experience. Future improvements:

- [ ] Add metrics tracking (agent count, time per phase, bugs found)
- [ ] Refine agent task templates (reusable prompts)
- [ ] Optimize agent count (minimum effective team size)
- [ ] Improve review agent focus areas (coverage matrix)
- [ ] Streamline fix agent comparison (scoring rubric)

---

## Notes for Revision

**User requested:** Review and refine this workflow for general use

**Questions to address:**
- Optimal agent count per phase?
- How to handle mid-flight scope changes?
- When to abort and restart vs push through?
- How to handle conflicting agent solutions?
- Metrics for "is this working well"?

**Future additions:**
- Phase timing guidelines (how long should each phase take?)
- Agent prompt templates for common tasks
- Decision tree for when to use which phase
- Rollback procedure if workflow fails mid-execution

---

Last updated: 2026-02-07
