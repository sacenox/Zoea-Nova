# Zoea Nova Development Workflow

**Version:** 1.1  
**Last updated:** 2026-02-08

---

## Quick Reference

**When to use:** Architectural changes, large refactors (10+ files), API changes, system redesigns

**Don't use for:** Simple bugs (1-2 files), docs-only, dependency updates, config tweaks

**Time budget:** 2-4 hours for medium refactors

**Minimum requirements:**
- Phase 1: Truth table document (abort if >90 min)
- Phase 2: 3+ agents for implementation
- Phase 4: 3+ agents for review
- Phase 6: User approval before commit

**Emergency stops:**
- Phase 1: Can't define truth table after 90 min
- Phase 2: >50% agents report blocking dependencies
- Phase 3: >20 compilation errors
- Phase 4: Review finds architectural flaws (not bugs)
- Phase 5: All 3 fix agents fail
- Phase 6: User finds critical missing functionality

**Quick links:** [Decision Tree](#decision-tree) | [Troubleshooting](#troubleshooting) | [Checklist](#template-checklist)

---

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
- You aren't confident in the approach

---

## Decision Tree

Use this flowchart to decide whether and how to proceed:

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

### Phase 1: Document the Truth

**Goal:** Create single source of truth before touching code.

**Estimated time:** 30-60 minutes

**Abort if:** Can't define clear truth table after 90 minutes (problem too complex/unclear)

**Steps:**

1. **Analyze the problem**
   - What's broken or needs changing?
   - What's the root cause?
   - What are the constraints?
   - How is it expected to work (do we have an existing truth table in our architecture: `documentation/architecture`?)

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

**Estimated time:** 30-90 minutes (depending on agent count and complexity)

**Agent count by change size:**
- Small refactor (5-10 files): 3 agents minimum
- Medium refactor (10-20 files): 5-6 agents recommended
- Large refactor (20+ files): 6-8 agents, consider splitting into sub-workflows

**Abort if:** >50% agents report blocking dependencies (task breakdown needs revision)

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
   - Use separate git branches per agent if changes overlap
   - Ask for test coverage when applicable
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

**Estimated time:** 20-40 minutes

**Abort if:** >20 compilation errors after integration (indicates fundamental design issue)

**Steps:**

1. **Check for conflicts**
   - Review overlapping changes
   - Verify no duplicate edits
   - Check for missing dependencies

2. **Verify truth table compliance**
   - Use Grep tool to find each state transition
   - Trace code path for each scenario
   - Run targeted tests for each row
   - Document verification in checklist
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

**Estimated time:** 30-45 minutes

**Review agents:** 3-5 (3 minimum, 5 for large changes, independent perspectives)

**Abort if:** Review agents find architectural flaws (not bugs) - indicates design needs revision

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

**Estimated time:** 20-60 minutes (depends on issue complexity)

**Approach:** Multiple agents with different solutions (if complex)

**Abort if:** All 3 fix agents fail to solve issue (problem needs redesign)

**Steps:**

1. **For complex fixes:**
   - Launch 3 agents with same requirements
   - Each agent implements independent solution
   - Compare approaches:
     - Truth table compliance (eliminates non-compliant)
     - Code simplicity (prefer simpler)
     - Test coverage (prefer better tested)
     - Performance
     - Maintainability
     - Risk level
   - Choose best solution (ask user if tied)
   - Never merge incompatible approaches

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

**Estimated time:** 15-30 minutes

**Abort if:** User finds critical missing functionality (scope was incomplete)

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

**Process metrics:**
- Phase 1 completion: <90 minutes
- Agent coordination: <3 conflicts requiring manual merge
- Review findings: <5 critical issues found
- Fix iterations: <2 rounds needed
- Total time: <4 hours for medium refactor

**Quality metrics:**
1. **Correctness:** All truth table scenarios verified in code
2. **Completeness:** No missing edge cases or half-implemented features
3. **Test coverage:** No decrease from baseline, all new behavior has tests
4. **Documentation:** Truth table doc created, outdated comments cleaned
5. **Code quality:** Net negative lines preferred (removed complexity)
6. **Build status:** All tests passing, zero warnings
7. **User approval:** First-try acceptance
8. **Efficiency:** Parallel agents complete in fraction of sequential time

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

## Handling Scope Changes

**If scope changes during Phase 1-2:**
- STOP all agents
- Update truth table
- Get user re-approval
- Restart from Phase 2

**If scope changes during Phase 3-5:**
- Document new scope in TODO
- Complete current workflow
- Start new workflow for additional scope

**Never:** Mix scope changes into in-flight workflow

---

## Rollback Procedure

**If workflow fails mid-execution:**

1. **Assess damage:**
   - Run `git status` to see uncommitted changes
   - Run `git diff --stat` to see scope
   - Check if any agents are still running

2. **Clean rollback:**
   - `git restore .` (discard all changes)
   - `git clean -fd` (remove untracked files)
   - Verify with `make build && make test`

3. **Document failure:**
   - Add to KNOWN_ISSUES.md
   - Note what went wrong
   - Note what was learned

4. **Restart decision:**
   - If truth table was wrong: Fix table, restart Phase 1
   - If agent coordination failed: Adjust task breakdown, restart Phase 2
   - If architectural flaw found: Escalate to user, new design needed

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

This workflow will evolve based on experience. After using this workflow multiple times to build confidence in the process, consider:

- [ ] Add metrics tracking (agent count, time per phase, bugs found)
- [ ] Refine agent task templates (reusable prompts)
- [ ] Improve review agent focus areas (coverage matrix)
- [ ] Streamline fix agent comparison (scoring rubric)
- [ ] Create decision tree for when to use which phase

---

## Agent Best Practices and Git Workflow

### Recommendations for Agent Prompts

**Phase 5 agent instructions must explicitly include:**

#### 1. Git Command Templates

**After making ANY code changes:**
```bash
# Stage changes
git add internal/path/to/changed_file.go internal/path/to/test_file.go

# Commit with descriptive message
git commit -m "fix(mysis): implement autonomous turn loop in run()

- Added select loop listening to nudgeChan in internal/mysis/mysis.go:234
- Loop calls handleNextTurn() when nudged
- Added test coverage in internal/mysis/mysis_test.go:456
- Tests: 91/91 passing"

# VERIFY commit was created
git log --oneline -1
# Expected output: <hash> fix(mysis): implement autonomous turn loop in run()
```

**Before reporting completion:**
```bash
# Verify commits exist on your branch
git log main..$(git branch --show-current) --oneline
# Expected output: One or more commit hashes with messages

# Verify changes are committed (should be empty if all committed)
git status
# Expected output: "nothing to commit, working tree clean"

# Verify diff exists between your branch and main
git diff main --stat
# Expected output: List of files with +/- line counts
```

#### 2. Mandatory Verification Checklist

**Every agent MUST complete this checklist before reporting "done":**

```markdown
- [ ] Created branch: `git branch --show-current` shows correct branch name
- [ ] Made code changes: Files modified in editor/IDE
- [ ] Staged changes: `git status` shows "Changes to be committed" OR already committed
- [ ] Created commit: `git log -1` shows my commit with correct message
- [ ] Verified commit exists on branch: `git log main..HEAD` shows my commit(s)
- [ ] Tests passing: `make test` output shows 0 failures
- [ ] Clean working tree: `git status` shows "nothing to commit, working tree clean"
- [ ] Changes visible in diff: `git diff main` shows my changes
- [ ] Commit hash reported: Included 7-char hash in completion message
```

#### 3. Commit Message Requirements

**Structure:**
```
<type>(<scope>): <summary>

<body with specifics>
<test results>
```

**Example (GOOD):**
```
fix(mysis): add autonomous turn loop in run() method

- Modified internal/mysis/mysis.go:234-256
- Added select{} loop with nudgeChan listener
- Calls handleNextTurn() on each nudge
- Updated internal/mysis/mysis_test.go:456-478
- Added TestRunMethodNudgeHandling test case

Tests: 91/91 passing (0.847s)
Coverage: internal/mysis 87.3% (+2.1%)
```

**Example (BAD - DO NOT USE):**
```
fix: implement feature

- Made changes to mysis
- Added tests
- Everything works now
```

**Rules:**
- Describe what code ACTUALLY does (not aspirational goals)
- Reference specific files and line numbers
- Include test results
- Use present tense ("add", "fix", "remove", not "added", "fixed", "removed")
- Be specific about behavior change

#### 4. Branch Verification Protocol

**Step-by-step verification (run ALL commands):**

```bash
# 1. Verify you're on correct branch
git branch --show-current
# Must output: Your expected branch name (e.g., "fix/autonomous-loop")

# 2. Verify commit exists
git log -1 --format="%H %s"
# Must output: Full hash + your commit message

# 3. Verify commit is on current branch (not on main)
git log main..HEAD --oneline
# Must output: One or more commits (NOT empty)

# 4. Verify changes are committed (not just staged)
git diff --cached
# Must output: Empty (no staged changes) if all committed

# 5. Verify working tree is clean
git status --short
# Must output: Empty (no uncommitted changes)

# 6. Verify diff against main
git diff main --stat
# Must output: File list with +/- counts
```

**If ANY check fails, DO NOT report completion. Fix the issue first.**

#### 5. Never Revert Previous Commits

**Before making changes, ALWAYS check history:**

```bash
# Check what commits exist on this branch
git log main..HEAD --oneline

# For each commit, check what it changed
git show <commit-hash> --stat

# Read the actual code changes
git show <commit-hash>
```

**Decision tree:**

```
Found previous commit with the fix?
├─ YES
│  ├─ Tests pass? → Keep the fix, report it works
│  └─ Tests fail?
│     ├─ Code matches truth table? → Fix the tests
│     └─ Code doesn't match? → Fix the code
└─ NO → Implement the fix
```

**Example (GOOD behavior):**
```bash
$ git log main..HEAD --oneline
788254c fix(mysis): add autonomous turn loop
$ git show 788254c | grep -A 10 "select {"
# See the loop exists
$ make test
# Tests fail
$ # Decision: Loop matches truth table, fix the tests
$ vim internal/mysis/mysis_test.go  # Fix test expectations
```

**Example (BAD behavior - DO NOT DO THIS):**
```bash
$ make test
# Tests fail
$ # Decision: Remove the loop to make tests pass
$ vim internal/mysis/mysis.go  # Delete the loop
# ❌ WRONG: Reverted correct implementation
```

#### 6. Evidence-Based Completion Reports

**When reporting completion, MUST include:**

1. **Branch name** (verified with `git branch --show-current`)
2. **Commit hash** (verified with `git log -1 --format="%h"`)
3. **Commit message** (first line of `git log -1 --format="%s"`)
4. **Files changed** (verified with `git diff main --stat`)
5. **Test results** (verified with `make test`)
6. **Verification commands** (paste output of checklist commands)

**Example (GOOD report):**
```
Completion Report:

Branch: fix/autonomous-loop-v2
Commit: a3f7c89
Message: fix(mysis): add autonomous turn loop in run()

Files changed:
 internal/mysis/mysis.go      | 23 +++++++++++++++++++++
 internal/mysis/mysis_test.go | 18 +++++++++++++++++
 2 files changed, 41 insertions(+)

Test results:
=== RUN   TestRunMethodNudgeHandling
--- PASS: TestRunMethodNudgeHandling (0.02s)
PASS
ok      github.com/example/zoea-nova/internal/mysis     0.847s

Verification:
$ git log main..HEAD --oneline
a3f7c89 fix(mysis): add autonomous turn loop in run()

$ git status
On branch fix/autonomous-loop-v2
nothing to commit, working tree clean

$ git diff main --stat
 internal/mysis/mysis.go      | 23 +++++++++++++++++++++
 internal/mysis/mysis_test.go | 18 +++++++++++++++++
 2 files changed, 41 insertions(+)
```

**Example (BAD report - DO NOT USE):**
```
Done! Created branch fix/autonomous-loop. All tests pass. Ready for review.

# ❌ Missing: commit hash, verification output, files changed, evidence
```

---

### Examples of Good vs Bad Agent Behavior

#### Example 1: Making Changes Without Committing

**BAD:**
```
Agent: "I've implemented the autonomous turn loop on branch fix/autonomous-loop. 
All tests pass. Ready for review."

User: "Show me the commit"
Agent: "Oh, I didn't commit yet. Let me do that now..."
```

**GOOD:**
```
Agent: "Completion Report:

Branch: fix/autonomous-loop-v2
Commit: a3f7c89
Message: fix(mysis): add autonomous turn loop in run()

Verification:
$ git log main..HEAD --oneline
a3f7c89 fix(mysis): add autonomous turn loop in run()

$ git status
On branch docs/workflow-completion-agent4
nothing to commit, working tree clean"
```

#### Example 2: Reverting Correct Implementations

**BAD:**
```
Agent 1: Commits working loop (788254c)
Agent 2: "Tests fail. Removing the loop to fix tests." (d2146af)

Result: Branch HEAD has no loop, fix is lost
```

**GOOD:**
```
Agent 2: 
$ git log main..HEAD --oneline
788254c fix(mysis): add autonomous turn loop

$ git show 788254c
# ... sees the loop exists and matches truth table

$ make test
# Tests fail with "unexpected goroutine running"

Analysis: Loop is correct per truth table. Tests have wrong expectations.
Action: Updating test to expect autonomous behavior.

$ git diff
# ... shows test changes, not code reversion
```

#### Example 3: Vague Commit Messages

**BAD:**
```
fix: implement feature

- Made changes
- Everything works
```

**GOOD:**
```
fix(mysis): add autonomous turn loop in run() method

- Modified internal/mysis/mysis.go:234-256
- Added select{} loop with nudgeChan listener
- Calls handleNextTurn() on each nudge
- Updated internal/mysis/mysis_test.go:456-478
- Added TestRunMethodNudgeHandling test case

Tests: 91/91 passing (0.847s)
Coverage: internal/mysis 87.3% (+2.1%)
```

#### Example 4: Incomplete Verification

**BAD:**
```
Agent: "Tests pass, so the implementation is correct!"

(Didn't verify:
- Commit exists
- Changes are on correct branch
- Working tree is clean
- Diff shows expected changes)
```

**GOOD:**
```
Agent: "Running verification checklist:

✓ Branch: git branch --show-current → fix/autonomous-loop-v2
✓ Commit: git log -1 --format='%h' → a3f7c89
✓ Commits on branch: git log main..HEAD --oneline → 1 commit found
✓ Tests: make test → 91/91 passing
✓ Clean tree: git status → nothing to commit, working tree clean
✓ Diff exists: git diff main --stat → 2 files, 41 insertions

All checks passed. Implementation complete."
```

#### Example 5: Testing Without Truth Table

**BAD:**
```
Agent: "Tests pass, so it must be correct!"

(Didn't check:
- Does behavior match truth table?
- Are tests encoding the right behavior?
- Did previous agent already fix this?)
```

**GOOD:**
```
Agent: "Verifying against truth table (MESSAGE_FORMAT_GUARANTEES.md):

Row 3: 'Running + no messages in 90s → Idle'
Expected: Mysis should NOT idle (encouragementCount prevents it)
Code: run() loop checks encouragementCount before idling ✓
Test: TestRunningWithRecentEncouragement passes ✓

Truth table compliance: VERIFIED
Tests encode correct behavior: VERIFIED"
```

---

### Action Items for Workflow Improvement

**Immediate (Before Next Workflow Execution):**

- [x] Document git command templates in this section
- [x] Create mandatory verification checklist
- [x] Add good vs bad behavior examples
- [ ] Create agent prompt template file (`templates/agent-prompt-fix-implementation.md`)
- [ ] Add pre-launch checklist for coordinators (`templates/coordinator-checklist.md`)
- [ ] Create post-completion verification script (`scripts/verify-agent-work.sh`)

**Short-term (After 2-3 Workflow Executions):**

- [ ] Build automated agent verification tool
  - Checks: branch exists, commits exist, tests pass, clean tree
  - Output: Pass/fail with specific failures highlighted
  - Integration: Run before accepting agent work
- [ ] Create agent performance metrics tracking
  - Track: completion rate, revert rate, verification pass rate
  - Report: Per-agent and aggregate statistics
  - Use: Identify agents needing prompt improvements
- [ ] Develop commit message linter for CI
  - Check: includes file paths, line numbers, test results
  - Enforce: conventional commits format
  - Reject: vague messages like "fix: implement feature"

**Medium-term (After 5+ Workflow Executions):**

- [ ] Build agent prompt library with versioning
  - Templates: implementation agents, review agents, fix agents
  - Includes: embedded checklists, command templates, examples
  - Versioned: track prompt effectiveness over time
- [ ] Create truth table compliance checker tool
  - Parse: truth table from architecture docs
  - Verify: each row has corresponding test
  - Report: coverage gaps and missing scenarios
- [ ] Develop agent coordination dashboard
  - Status: which agents running, completed, blocked
  - Conflicts: detect overlapping file changes early
  - Timeline: visualize parallel execution progress

**Long-term (Continuous Improvement):**

- [ ] Machine learning for agent prompt optimization
  - Train on: successful vs failed agent executions
  - Identify: patterns in prompt structure that correlate with success
  - Suggest: prompt improvements based on historical data
- [ ] Automated workflow phase detection system
  - Monitor: git state, test results, agent reports
  - Detect: current phase without manual declaration
  - Alert: when workflow deviates from standard path
- [ ] CI/CD integration for agent work
  - Hook: run verification on agent branch creation
  - Block: merges that fail verification checklist
  - Report: verification results in PR comments

---

**Version:** 1.1  
**Last updated:** 2026-02-08
