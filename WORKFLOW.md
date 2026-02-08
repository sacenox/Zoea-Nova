# Zoea Nova Development Workflow

**Version:** 2.0  
**Last updated:** 2026-02-08

---

## When to Use

**Use for:** Architectural changes, large refactors (10+ files), API changes, system redesigns

**Don't use for:** Simple bugs (1-2 files), docs-only, dependency updates, config tweaks

**Time budget:** 2-4 hours for medium refactors

---

## Safe Parallel Execution Rule

**CRITICAL: One agent per file - never assign multiple agents to edit the same file**

This prevents merge conflicts and ensures clean integration. Prefer launching many small agents (each handling 2-3 files) over few large agents (each handling 10+ files).

---

## Decision Tree

```
Phase 1: Can define truth table <90 min? → YES → Phase 2
Phase 2: 3+ independent tasks identified? → YES → Phase 3
Phase 3: <20 compilation errors? → YES → Phase 4
Phase 4: Only bugs found (no arch flaws)? → YES → Phase 5
Phase 5: At least 1 solution works? → YES → Phase 6
Phase 6: User approved changes? → YES → Commit
```

---

## Workflow Phases

### Phase 1: Document the Truth (30-60 min)

**Goal:** Create single source of truth before touching code

**Abort if:** Can't define clear truth table after 90 minutes

**Steps:**
1. Analyze problem (what's broken, root cause, constraints)
2. Create truth table (states, transitions, triggers, counters)
3. Write architecture doc in `documentation/architecture/`
   - Truth table (state × condition → action)
   - Message flow diagrams
   - Code location references
   - Design rationale
   - Migration path
4. Get user approval before coding

**Deliverable:** Architecture doc with truth table

---

### Phase 2: Safe Parallel Agent Implementation (30-90 min)

**Goal:** Execute changes using coordinated agent fleet with no file conflicts

**Agent count:** 3 minimum (5-6 for 10-20 files, 6-8 for 20+ files)

**Abort if:** >50% agents report blocking dependencies

**CRITICAL: One agent per file - never assign multiple agents to edit the same file**

**Steps:**
1. Break work into independent tasks (isolated, testable, bounded)
   - Each agent owns specific files (no overlap)
   - Prefer many small agents over few large agents
   - Example: 6 agents each handling 2-3 files > 2 agents each handling 10 files
2. Launch agents in parallel
   - Reference truth table document
   - Specify exact files each agent should modify
   - Specify what NOT to change
   - Request specific deliverables
3. Monitor completion and review summaries

**Deliverable:** All tasks completed with summaries, no file conflicts

---

### Phase 3: Integration Review (20-40 min)

**Goal:** Verify agent work integrates correctly

**Abort if:** >20 compilation errors after integration

**Steps:**
1. Check for file conflicts (should be zero if Phase 2 done correctly)
2. Check for duplicate edits across different files
3. Verify truth table compliance (grep each state transition, trace code paths, test scenarios)
4. Build verification (`make build`)
5. Test verification (`make test`)
6. Fix integration issues

**Deliverable:** Clean build, passing tests, truth table verified, no file conflicts

---

### Phase 4: Regression Analysis (30-45 min)

**Goal:** Find issues the first pass missed

**Review agents:** 3 minimum (5 for large changes, independent perspectives)

**Abort if:** Review agents find architectural flaws (not bugs)

**Steps:**
1. Launch review agents in parallel (state transitions, trigger mechanisms, test coverage)
   - Each agent reviews different aspect
   - No file editing, only analysis
2. Compare findings (consensus vs unique)
3. Validate findings against truth table
4. Fix critical issues

**Deliverable:** All critical regressions found and fixed

---

### Phase 5: Fix Implementation (20-60 min)

**Goal:** Implement fixes from regression analysis

**Abort if:** All 3 fix agents fail to solve issue

**CRITICAL: Assign fixes to avoid file conflicts - one agent per file**

**Steps:**
1. For complex fixes spanning multiple files:
   - Launch 3 agents with same requirements but different files
   - OR launch 3 agents with same files sequentially (compare approaches)
   - Compare approaches (truth table compliance, simplicity, test coverage)
   - Choose best solution
2. For simple fixes: Implement directly, verify, test
3. Integration test (full suite, edge cases, performance)

**Deliverable:** Working implementation matching truth table, no file conflicts

---

### Phase 6: User Review (15-30 min)

**Goal:** Get user approval

**Abort if:** User finds critical missing functionality

**Steps:**
1. Present summary (changes, removals, additions, test status, truth table compliance)
2. User review and manual testing
3. User commits changes when satisfied

**Deliverable:** User approval

---

## Key Principles

**DON'T:**
- Code before documenting truth table
- Use single agent for large multi-file changes
- Assign multiple agents to edit the same file
- Skip regression analysis phase
- Commit without user review
- Mix unrelated changes in one commit

**DO:**
- Document state machines and truth tables upfront
- Use minimum 3 agents for complex changes
- Assign one agent per file (no file overlap)
- Prefer many small agents over few large agents
- Get explicit user approval
- Clean up as you go
- Write architecture docs that match implementation

---

## Scope Changes

**During Phase 1-2:** STOP all agents, update truth table, get user re-approval, restart from Phase 2

**During Phase 3-5:** Document new scope in TODO, complete current workflow, start new workflow for additional scope

---

## Rollback

**If workflow fails:**
1. User discards changes
2. Verify with `make build && make test`
3. Document failure in KNOWN_ISSUES.md
4. Decide: Fix truth table (restart Phase 1), adjust task breakdown (restart Phase 2), or escalate to user

---

## Checklist

- [ ] Phase 1: Truth table documented and user approved
- [ ] Phase 2: 3+ agents launched, all completed
- [ ] Phase 3: Build successful, tests passing, truth table verified
- [ ] Phase 4: 3+ review agents launched, findings documented
- [ ] Phase 5: Fixes implemented, tests passing
- [ ] Phase 6: User reviewed and approved

---

**Version:** 2.0  
**Last updated:** 2026-02-08
