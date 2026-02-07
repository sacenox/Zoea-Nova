# Loop Context Composition - Execution Checklist

**Plan:** [2026-02-07-loop-context-composition.md](2026-02-07-loop-context-composition.md)
**Workflow:** [2026-02-07-loop-context-parallel-workflow.md](2026-02-07-loop-context-parallel-workflow.md)
**Diagrams:** [2026-02-07-loop-context-workflow-diagram.md](2026-02-07-loop-context-workflow-diagram.md)

---

## Phase 1: Test Infrastructure

### Implementation (Parallel)
- [ ] **Agent 1:** Write `TestContextPromptSourcePriority` in `internal/core/mysis_test.go`
  - [ ] Commander direct present case
  - [ ] Commander direct missing, broadcast present case
  - [ ] Commander broadcasts missing, swarm broadcast present case
  - [ ] No broadcasts, nudge generation case
  - [ ] Test compiles and fails

- [ ] **Agent 2:** Write `TestLoopContextSlice` in `internal/core/orphaned_tool_results_test.go`
  - [ ] Only most recent tool-call included
  - [ ] Tool results for most recent call included
  - [ ] Older tool loops excluded
  - [ ] Test compiles and fails

- [ ] **Agent 3:** Write tool pairing tests in `internal/core/agent3_reproduction_test.go`
  - [ ] Tool results only appear with matching tool calls
  - [ ] No orphaned tool results detected
  - [ ] Test compiles and fails

### Review Checkpoint 1
- [ ] **Review Agent A:** Test coverage complete
  - [ ] All priority edge cases covered
  - [ ] Tests are isolated (no shared state)
  - [ ] Test names follow convention

- [ ] **Review Agent B:** Test quality verified
  - [ ] Table-driven where appropriate
  - [ ] Assertions are specific and actionable
  - [ ] Setup/teardown correct
  - [ ] Tests fail for correct reasons

### Integration
- [ ] Run: `go test ./internal/core -run "TestContextPromptSourcePriority|TestLoopContextSlice" -v`
- [ ] Verify all tests fail as expected
- [ ] Commit: `git add internal/core/*_test.go`
- [ ] Commit message: "test(core): add loop context composition tests"

### Gate 1: Proceed? ☐ YES ☐ NO

---

## Phase 2: Helper Implementation

### Implementation (Parallel)
- [ ] **Agent 4:** Implement `selectPromptSource` in `internal/core/mysis.go`
  - [ ] Find most recent commander direct
  - [ ] Fallback to last commander broadcast
  - [ ] Fallback to last swarm broadcast
  - [ ] Return nil if none found
  - [ ] Unit test helpers pass

- [ ] **Agent 5:** Implement `extractLatestToolLoop` in `internal/core/mysis.go`
  - [ ] Reverse scan for most recent tool-call
  - [ ] Collect tool-call message
  - [ ] Collect subsequent tool results
  - [ ] Handle edge cases (no tool calls, tool call with no results)
  - [ ] Unit test helpers pass

### Review Checkpoint 2
- [ ] **Review Agent C:** Implementation correctness
  - [ ] `selectPromptSource` checks correct priority order
  - [ ] `extractLatestToolLoop` handles all edge cases
  - [ ] Error handling correct (nil checks, empty slices)
  - [ ] No mutations to input slices

- [ ] **Review Agent D:** Performance & safety
  - [ ] Efficient memory handling (no unnecessary copies)
  - [ ] Slice bounds checking correct
  - [ ] No nil pointer dereferences
  - [ ] Helper tests pass

### Integration
- [ ] Run: `go test ./internal/core -run "Helper" -v`
- [ ] Verify helper unit tests pass
- [ ] Commit: `git add internal/core/mysis.go`
- [ ] Commit message: "feat(core): add context composition helpers"

### Gate 2: Proceed? ☐ YES ☐ NO

---

## Phase 3: Context Composition Integration

### Implementation (Serial)
- [ ] **Agent 6:** Rewrite `getContextMemories` in `internal/core/mysis.go`
  - [ ] Filter for system prompt (source="system")
  - [ ] Call `selectPromptSource` for prompt memory
  - [ ] Generate synthetic nudge if prompt is nil (up to 3 attempts)
  - [ ] Call `extractLatestToolLoop` for tool messages
  - [ ] Compose: [system, prompt, ...toolLoop]
  - [ ] Remove old compaction/orphan logic

### Review Checkpoint 3
- [ ] **Review Agent E:** Integration review
  - [ ] All Task 1 & 2 tests now pass
  - [ ] Context size is bounded and predictable
  - [ ] No orphaned tool results possible
  - [ ] Synthetic nudge generation works (counter increment)
  - [ ] Idle transition after 3 nudges verified

- [ ] **Review Agent F:** Regression review
  - [ ] Full `internal/core` test suite passes
  - [ ] No existing tests broken
  - [ ] Mysis state machine tests still pass
  - [ ] Goroutine cleanup tests unaffected

### Integration
- [ ] Run: `go test ./internal/core -run "TestContext|TestLoopContext|TestOrphaned" -v`
- [ ] Run: `go test ./internal/core -v`
- [ ] Verify all tests pass, no regressions
- [ ] Commit: `git add internal/core/mysis.go`
- [ ] Commit message: "fix(core): compose context from prompt source and last tool loop"

### Gate 3: Proceed? ☐ YES ☐ NO

---

## Phase 4: Documentation

### Implementation (Parallel)
- [ ] **Agent 7:** Update `documentation/architecture/CONTEXT_COMPRESSION.md`
  - [ ] Describe loop slice model
  - [ ] Add prompt source priority diagram
  - [ ] Document loop extraction algorithm
  - [ ] Explain context composition order
  - [ ] Document bounded size guarantees

- [ ] **Agent 8:** Add code comments to `internal/core/mysis.go`
  - [ ] `selectPromptSource` documented (purpose, params, returns, edge cases)
  - [ ] `extractLatestToolLoop` documented
  - [ ] `getContextMemories` updated comments
  - [ ] Test file comments updated

### Review Checkpoint 4
- [ ] **Review Agent G:** Documentation review
  - [ ] Architecture doc matches implementation
  - [ ] Code comments are accurate
  - [ ] Examples are correct
  - [ ] KNOWN_ISSUES.md updated if relevant
  - [ ] README.md still accurate

### Integration
- [ ] Verify documentation accuracy
- [ ] Commit: `git add documentation/architecture/CONTEXT_COMPRESSION.md internal/core/mysis.go internal/core/mysis_test.go`
- [ ] Commit message: "docs: document loop slice context composition"

### Gate 4: Proceed? ☐ YES ☐ NO

---

## Phase 5: Full Validation

### Validation (Parallel)
- [ ] **Agent 9:** Unit test validation
  - [ ] Run: `go test ./internal/core -v -race -count=5`
  - [ ] No flaky tests detected
  - [ ] Race detector clean
  - [ ] 5 consecutive passes confirmed

- [ ] **Agent 10:** Integration test validation
  - [ ] Run: `make test`
  - [ ] Full test suite passes
  - [ ] No unexpected failures
  - [ ] Coverage maintained or improved

- [ ] **Agent 11:** Build validation
  - [ ] Run: `make clean && make build`
  - [ ] Clean build succeeds
  - [ ] Run: `./bin/zoea --offline`
  - [ ] Binary runs without crashes
  - [ ] Offline mode smoke test passes

### Final Review Checkpoint
- [ ] **Review Agent H:** Release readiness
  - [ ] All tests pass (unit + integration)
  - [ ] No race conditions detected
  - [ ] Documentation complete and accurate
  - [ ] KNOWN_ISSUES.md updated if needed
  - [ ] Commits follow conventional format
  - [ ] All plan tasks completed

### Gate 5: Proceed? ☐ YES ☐ NO

---

## Success Criteria

- [ ] All 5 plan tasks completed
- [ ] All new tests pass (100% success rate)
- [ ] No test regressions
- [ ] No race conditions detected
- [ ] Documentation updated and reviewed
- [ ] 8 review checkpoints passed
- [ ] Clean build on main branch
- [ ] Offline smoke test passes

---

## Post-Implementation

- [ ] Merge feature branch to main
- [ ] Tag release (if appropriate)
- [ ] Archive workflow document to `documentation/reports/`
- [ ] Update `documentation/current/TODO.md` to mark tasks complete
- [ ] Monitor production for unexpected behavior
- [ ] Update this checklist with lessons learned

---

## Notes Section

Use this space to track issues, decisions, or observations during execution:

```
[Date/Time] Phase X - Agent Y: <note>

Example:
[2026-02-07 14:30] Phase 2 - Agent 4: selectPromptSource needed additional nil check for empty memories slice
```

---

## Rollback Procedures

If any gate fails:

**Phase 1-4 Rollback:**
1. Identify failing agent's work
2. Run: `git diff HEAD`
3. Run: `git checkout -- <failing-file>`
4. Re-execute agent's task with fixes
5. Re-run review checkpoint
6. Update notes section with issue details

**Phase 5 Rollback (Complete Failure):**
1. Run: `git log --oneline -10`
2. Find commit before feature branch started
3. Run: `git reset --hard <commit-hash>`
4. Review plan for missing requirements
5. Add missing test cases to Phase 1
6. Re-execute from Phase 1

---

## Agent Commands Quick Reference

```bash
# Phase 1: Test verification
go test ./internal/core -run "TestContextPromptSourcePriority|TestLoopContextSlice" -v

# Phase 2: Helper verification
go test ./internal/core -run "Helper" -v

# Phase 3: Integration verification
go test ./internal/core -run "TestContext|TestLoopContext|TestOrphaned" -v
go test ./internal/core -v

# Phase 5: Full validation
go test ./internal/core -v -race -count=5
make test
make clean && make build
./bin/zoea --offline

# Format code
make fmt

# View recent commits
git log --oneline -10

# Check diff before commit
git diff
git diff --cached
```

---

Last Updated: 2026-02-07
Status: Ready for Execution
