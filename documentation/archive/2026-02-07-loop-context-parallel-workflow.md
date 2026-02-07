# Parallel Workflow: Loop Context Composition Implementation

**Plan Reference:** [2026-02-07-loop-context-composition.md](2026-02-07-loop-context-composition.md)

**Objective:** Execute the loop context composition plan using parallel agent work with review checkpoints after each phase.

---

## Workflow Structure

Each phase follows this pattern:
1. **Parallel Work** - Multiple agents work independently on isolated tasks
2. **Review Checkpoint** - Review agents verify correctness before proceeding
3. **Integration** - Merge verified work
4. **Gate Decision** - Proceed to next phase only if review passes

---

## Phase 1: Test Infrastructure (Parallel)

**Agents:** 3 implementation agents + 2 review agents

### Implementation Agents (Parallel)

**Agent 1: Prompt Source Priority Tests**
- File: `internal/core/mysis_test.go`
- Task: Write `TestContextPromptSourcePriority` with all priority cases
- Deliverable: Failing test covering commander direct → commander broadcast → swarm broadcast → nudge priority

**Agent 2: Loop Slice Tests**
- File: `internal/core/orphaned_tool_results_test.go`
- Task: Write `TestLoopContextSlice` for most-recent-loop extraction
- Deliverable: Failing test verifying only latest tool-call + results are included

**Agent 3: Tool Pairing Tests**
- File: `internal/core/agent3_reproduction_test.go`
- Task: Write tests ensuring tool results only appear with matching tool calls
- Deliverable: Failing test detecting orphaned tool results

### Review Checkpoint 1

**Review Agent A: Test Coverage Review**
- Verify all priority edge cases covered:
  - Commander direct present
  - Commander direct missing, broadcast present
  - Commander broadcasts missing, swarm broadcast present
  - No broadcasts, nudge generation
- Verify test isolation (no shared state)
- Check test names follow convention

**Review Agent B: Test Quality Review**
- Verify tests use table-driven pattern where appropriate
- Check assertions are specific and actionable
- Verify setup/teardown is correct
- Ensure tests fail for the right reasons

### Integration Step
```bash
# Verify all tests fail as expected
go test ./internal/core -run "TestContextPromptSourcePriority|TestLoopContextSlice" -v

# Commit all test files together
git add internal/core/*_test.go
git commit -m "test(core): add loop context composition tests

- prompt source priority (commander → broadcast → swarm → nudge)
- loop slice extraction (most recent tool-call + results only)
- tool result pairing (no orphaned results)"
```

**Gate:** Proceed only if:
- All tests compile
- All tests fail
- Review agents approve test quality

---

## Phase 2: Helper Implementation (Parallel)

**Agents:** 2 implementation agents + 2 review agents

### Implementation Agents (Parallel)

**Agent 4: Prompt Source Selection**
- File: `internal/core/mysis.go`
- Task: Implement `selectPromptSource(memories []*store.Memory) *store.Memory`
- Logic:
  ```
  1. Find most recent commander direct (source="direct")
  2. If none, find last commander broadcast (source="broadcast", sender_id=commander)
  3. If none, find last swarm broadcast (source="broadcast", sender_id!=commander)
  4. If none, return nil (caller generates nudge)
  ```
- Include unit test helper verification

**Agent 5: Loop Extraction**
- File: `internal/core/mysis.go`
- Task: Implement `extractLatestToolLoop(memories []*store.Memory) []*store.Memory`
- Logic:
  ```
  1. Reverse scan for most recent tool-call (role="assistant", has tool_calls)
  2. Collect that tool-call message
  3. Collect all subsequent tool results (role="tool") until non-tool message
  4. Return [tool-call, result1, result2, ...]
  ```
- Include unit test helper verification

### Review Checkpoint 2

**Review Agent C: Implementation Correctness**
- Verify `selectPromptSource` checks in correct priority order
- Verify `extractLatestToolLoop` handles edge cases:
  - No tool calls in history
  - Tool call at end with no results yet
  - Multiple tool results for single call
- Check error handling (nil checks, empty slices)
- Verify no mutations to input slices

**Review Agent D: Performance & Safety**
- Check for efficient memory handling (no unnecessary copies)
- Verify slice bounds checking
- Check for potential nil pointer dereferences
- Verify helper tests pass

### Integration Step
```bash
# Run helper-specific tests
go test ./internal/core -run "Helper" -v

# Commit helpers
git add internal/core/mysis.go
git commit -m "feat(core): add context composition helpers

- selectPromptSource: priority-based prompt selection
- extractLatestToolLoop: most recent tool-call + results extraction"
```

**Gate:** Proceed only if:
- Helper unit tests pass
- Review agents approve implementation
- No regressions in existing tests

---

## Phase 3: Context Composition Integration (Serial)

**Agents:** 1 implementation agent + 2 review agents

### Implementation Agent (Single)

**Agent 6: Compose getContextMemories**
- File: `internal/core/mysis.go`
- Task: Rewrite `getContextMemories` using new helpers
- Steps:
  1. Filter for system prompt (source="system")
  2. Call `selectPromptSource` for prompt memory
  3. Generate synthetic nudge if prompt is nil (up to 3 attempts)
  4. Call `extractLatestToolLoop` for tool messages
  5. Compose: [system, prompt, ...toolLoop]
- Remove old compaction/orphan logic

### Review Checkpoint 3

**Review Agent E: Integration Review**
- Verify all Task 1 & 2 tests now pass
- Check context size is bounded and predictable
- Verify no orphaned tool results possible
- Test synthetic nudge generation (counter increment)
- Verify idle transition after 3 nudges

**Review Agent F: Regression Review**
- Run full `internal/core` test suite
- Check no existing tests broken
- Verify existing mysis state machine tests still pass
- Check goroutine cleanup tests unaffected

### Integration Step
```bash
# Run all context tests
go test ./internal/core -run "TestContext|TestLoopContext|TestOrphaned" -v

# Run full core suite
go test ./internal/core -v

# Commit integration
git add internal/core/mysis.go
git commit -m "fix(core): compose context from prompt source and last tool loop

Replaces compaction approach with deterministic loop slice:
- System prompt
- Chosen prompt source (commander → broadcast → swarm → nudge)
- Most recent tool-call + all its tool results

This eliminates orphaned tool results and provides stable, bounded context."
```

**Gate:** Proceed only if:
- All new tests pass
- No test regressions
- Both review agents approve

---

## Phase 4: Documentation & Validation (Parallel)

**Agents:** 2 implementation agents + 1 review agent

### Implementation Agents (Parallel)

**Agent 7: Architecture Documentation**
- File: `documentation/architecture/CONTEXT_COMPRESSION.md`
- Task: Update to describe loop slice model
- Include:
  - Prompt source priority diagram
  - Loop extraction algorithm
  - Context composition order
  - Bounded size guarantees

**Agent 8: Code Comment Documentation**
- Files: `internal/core/mysis.go`, `internal/core/mysis_test.go`
- Task: Add comprehensive comments to new functions
- Include:
  - Purpose and rationale
  - Parameter descriptions
  - Return value semantics
  - Edge case behavior

### Review Checkpoint 4

**Review Agent G: Documentation Review**
- Verify architecture doc matches implementation
- Check code comments are accurate
- Verify examples are correct
- Ensure KNOWN_ISSUES.md updated if relevant
- Check README.md still accurate

### Integration Step
```bash
# Verify documentation accuracy
git add documentation/architecture/CONTEXT_COMPRESSION.md
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "docs: document loop slice context composition

- Prompt source priority (commander → broadcast → swarm → nudge)
- Tool loop extraction (most recent only)
- Bounded context guarantees"
```

**Gate:** Proceed only if:
- Documentation review passes
- All links work
- No contradictions with code

---

## Phase 5: Full Validation (Parallel)

**Agents:** 3 validation agents

### Validation Agents (Parallel)

**Agent 9: Unit Test Validation**
```bash
go test ./internal/core -v -race -count=5
```
- Verify no flaky tests
- Check race detector clean
- Confirm 5 consecutive passes

**Agent 10: Integration Test Validation**
```bash
make test
```
- Full test suite passes
- No unexpected failures
- Coverage maintained or improved

**Agent 11: Build Validation**
```bash
make clean
make build
./bin/zoea --offline
```
- Clean build succeeds
- Binary runs without crashes
- Offline mode works (smoke test)

### Final Review Checkpoint

**Review Agent H: Release Readiness**
- All tests pass (unit + integration)
- No race conditions detected
- Documentation complete and accurate
- KNOWN_ISSUES.md updated if needed
- Commits follow conventional format
- Plan tasks all completed

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

## Agent Allocation Summary

**Implementation Agents:** 8
- 3 test writers (parallel)
- 2 helper implementers (parallel)
- 1 integration implementer (serial)
- 2 documentation writers (parallel)

**Review Agents:** 8
- 2 test quality reviewers
- 2 implementation correctness reviewers
- 2 integration reviewers
- 1 documentation reviewer
- 1 release readiness reviewer

**Validation Agents:** 3
- 1 unit test validator
- 1 integration validator
- 1 build validator

**Total:** 19 agent roles, maximum 6 agents working in parallel at peak (Phase 1)

---

## Execution Timeline

Assuming each phase takes T time units:

- **Phase 1:** T (3 agents parallel) + Review
- **Phase 2:** T (2 agents parallel) + Review
- **Phase 3:** 2T (1 agent serial, more complex) + Review
- **Phase 4:** T (2 agents parallel) + Review
- **Phase 5:** T (3 agents parallel) + Review

**Total:** 6T + 5 review checkpoints

**Sequential equivalent:** 8T (all agents serial)

**Speedup:** 25% faster with parallel work + continuous quality gates

---

## Risk Mitigation

**Risk:** Helper functions have bugs that propagate to integration
**Mitigation:** Phase 2 includes dedicated helper unit tests before integration

**Risk:** Context composition breaks existing functionality
**Mitigation:** Phase 3 review includes full regression suite

**Risk:** Documentation drifts from implementation
**Mitigation:** Phase 4 review agent verifies doc-code consistency

**Risk:** Race conditions in async context composition
**Mitigation:** Phase 5 runs race detector with 5 iterations

---

## Rollback Plan

If any phase fails review:
1. Identify failing agent's work
2. Rollback that agent's changes
3. Re-execute that agent's task with fixes
4. Re-run review checkpoint
5. Proceed only after approval

If Phase 5 validation fails:
1. Rollback entire feature branch
2. Review plan for missing requirements
3. Add missing test cases
4. Re-execute from Phase 1

---

## Post-Implementation

After all phases complete:
1. Merge feature branch to main
2. Tag release if appropriate
3. Archive this workflow document to `documentation/reports/`
4. Update `documentation/current/TODO.md` to mark tasks complete
5. Monitor production for unexpected behavior
