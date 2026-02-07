# Codebase and Documentation Cleanup Workflow

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove dead code, fix outdated tests, reorganize documentation, consolidate known issues, and organize TODO.md using 10 autonomous agents working in parallel.

**Architecture:** Split work into 10 independent audit tasks executed by parallel agents, each producing a report with recommended deletions/moves/merges, then execute all changes in a single verification phase.

**Tech Stack:** Go 1.22, ripgrep, git, markdown

---

## Phase 1: Parallel Code Audits (10 Agents)

Each agent audits a specific area and produces a report with actionable recommendations.

---

### Agent 1: Dead Code Detection

**Task:** Find unused functions, variables, and imports in production code.

**Search for:**
- Unexported functions with no references
- Unused constants
- Commented-out code blocks
- Imports marked as unused by LSP

**Exclude:**
- Test files (*_test.go)
- Exported functions (may be used externally)

**Output:** `reports/DEAD_CODE_AUDIT_2026-02-07.md` with:
- List of dead code with file:line
- Reason it's unused (no references found)
- Recommended deletion

**Verification command:**
```bash
rg "functionName" --type go
```

---

### Agent 2: Skipped and Disabled Tests

**Task:** Find all skipped tests and determine if they should be enabled or removed.

**Search for:**
- `t.Skip()`
- `t.SkipNow()`
- `// TODO: test`
- `// FIXME: test`
- Commented-out test functions

**Output:** `reports/SKIPPED_TESTS_AUDIT_2026-02-07.md` with:
- List of skipped tests with reason
- Recommendation: enable, fix, or remove
- Estimated effort to fix

**Verification:**
```bash
rg "t\.Skip|SkipNow" --type go
```

---

### Agent 3: Outdated Test Logic

**Task:** Find tests that verify removed or changed functionality.

**Search for:**
- Tests referencing removed functions (zoea_swarm_status, selectPromptSource, DefaultConfig)
- Tests using old architecture (loop-based context vs turn-aware)
- Tests with hardcoded provider names that should be dynamic

**Cross-reference:**
- Recent commits removing features
- Architecture changes in reports/

**Output:** `reports/OUTDATED_TESTS_AUDIT_2026-02-07.md` with:
- Tests that need updating
- What changed that invalidates them
- Recommended fix or removal

**Verification:**
```bash
git log --oneline --since="2026-02-06" | grep "remove\|delete\|refactor"
```

---

### Agent 4: Hardcoded Config References

**Task:** Find remaining hardcoded provider/model references that should use config.

**Search for:**
- String literals: "ollama", "opencode_zen" in non-provider implementation files
- Hardcoded model names in validation
- Provider switch statements

**Exclude:**
- internal/provider/*.go (provider implementations)
- Test files (allowed to use hardcoded fixtures)

**Output:** `reports/HARDCODED_CONFIG_AUDIT_2026-02-07.md` with:
- Hardcoded values with location
- Should be dynamic or is acceptable
- Recommended fix

**Verification:**
```bash
rg '"ollama"|"opencode_zen"' --type go | grep -v "_test.go" | grep -v "internal/provider"
```

---

### Agent 5: Documentation File Organization

**Task:** Audit documentation/ directory for misplaced, duplicate, or outdated files.

**Check:**
- Files in wrong directories (plans in reports/, reports in investigations/)
- Duplicate information across files
- Outdated reports superseded by newer implementations
- Missing index entries in documentation/README.md

**Output:** `reports/DOCUMENTATION_ORGANIZATION_AUDIT_2026-02-07.md` with:
- Files to move (source → destination)
- Files to merge (list duplicates)
- Files to archive
- Index updates needed

**Verification:**
```bash
ls -R documentation/
```

---

### Agent 6: Known Issues Consolidation

**Task:** Consolidate KNOWN_ISSUES.md, KNOWN_SERVER_ISSUES.md, and issues scattered in other docs.

**Check:**
- Issues mentioned in multiple places
- Fixed issues still listed as known issues
- Server issues that should be in KNOWN_SERVER_ISSUES.md
- Issues in reports/ that should be in KNOWN_ISSUES.md

**Cross-reference:**
- Recent commits fixing issues
- Git history for "fix:" commits

**Output:** `reports/KNOWN_ISSUES_CONSOLIDATION_2026-02-07.md` with:
- Issues to remove (fixed)
- Issues to add (found in other docs)
- Issues to move (wrong file)
- Proposed new KNOWN_ISSUES.md structure

**Verification:**
```bash
git log --oneline --grep="fix:" --since="2026-02-05"
```

---

### Agent 7: TODO.md Organization

**Task:** Organize TODO.md into categories and remove completed items.

**Check:**
- Completed items (marked with [x] or mentioned in recent commits)
- Duplicate items
- Vague items without clear success criteria
- Items that should be issues in KNOWN_ISSUES.md instead

**Output:** `reports/TODO_ORGANIZATION_2026-02-07.md` with:
- Items to remove (completed)
- Items to move to KNOWN_ISSUES.md
- Items to clarify
- Proposed new TODO.md structure with categories

**Verification:**
```bash
git log --oneline --since="2026-02-05" | grep -i "session\|context\|prompt"
```

---

### Agent 8: Test Coverage Gaps

**Task:** Find production code without test coverage.

**Check:**
- Functions in internal/ without corresponding tests
- Critical paths (CreateMysis, SendMessage, getContextMemories) test coverage
- Error paths not tested

**Output:** `reports/TEST_COVERAGE_GAPS_2026-02-07.md` with:
- Functions without tests
- Critical paths with insufficient coverage
- Recommended tests to add (with priority)

**Verification:**
```bash
go test ./... -cover
```

---

### Agent 9: Import Cleanup

**Task:** Find unused imports and missing imports across codebase.

**Check:**
- LSP diagnostics for unused imports
- Missing imports causing build failures
- Imports that could be stdlib instead of external

**Output:** `reports/IMPORT_CLEANUP_2026-02-07.md` with:
- Files with unused imports
- Recommended removals
- Build verification commands

**Verification:**
```bash
go build ./...
```

---

### Agent 10: Documentation Accuracy Audit

**Task:** Verify documentation matches current implementation.

**Check:**
- README.md - keyboard shortcuts, features, providers
- AGENTS.md - terminology, architecture, workflow
- architecture/*.md - diagrams, state machines, compression strategy
- guides/*.md - configuration examples, terminal requirements

**Cross-reference:**
- Current code (keyboard mappings, features)
- Recent architectural changes (turn-aware context)
- Provider list (config.toml)

**Output:** `reports/DOCUMENTATION_ACCURACY_AUDIT_2026-02-07.md` with:
- Inaccurate documentation with corrections
- Missing documentation
- Outdated examples
- Recommended updates

**Verification:**
```bash
diff <(actual_code) <(documented_behavior)
```

---

## Phase 2: Consolidation and Review

**Task:** Merge all 10 audit reports into execution plan.

**Agent:** Single coordinator agent

**Actions:**
1. Read all 10 audit reports
2. Identify conflicts (same file edited by multiple audits)
3. Prioritize changes (critical fixes first)
4. Create consolidated change list
5. Group by file for efficient editing

**Output:** `reports/CLEANUP_EXECUTION_PLAN_2026-02-07.md` with:
- Ordered list of all changes
- File-by-file change groups
- Conflict resolutions
- Estimated time per change

---

## Phase 3: Execution (Batched by File)

**Task:** Execute all changes in file-batched order.

**Agent:** Single execution agent

**Process:**
1. For each file with changes:
   - Read current state
   - Apply all changes to that file
   - Run relevant tests
   - Commit if tests pass
   - Rollback if tests fail

**Safety:**
- One file at a time
- Test after each file change
- Commit frequently
- Stop on first failure

**Commands:**
```bash
# For each change batch
go test ./path/to/affected/package -v
git add file.go
git commit -m "cleanup: [description]"
```

---

## Phase 4: Documentation Reorganization

**Task:** Move/merge/archive documentation files.

**Agent:** Documentation agent

**Actions:**
1. Execute moves (investigation → archive, etc.)
2. Merge duplicate content
3. Update documentation/README.md index
4. Update cross-references
5. Verify no broken links

**Verification:**
```bash
rg "]\(.*\.md\)" documentation/ --only-matching | sort | uniq
```

---

## Phase 5: Final Verification

**Task:** Verify entire codebase after cleanup.

**Agent:** Verification agent

**Checks:**
1. Full test suite passes
2. Build succeeds
3. No dead code remains
4. No skipped tests (unless documented)
5. Documentation accurate
6. TODO.md organized
7. KNOWN_ISSUES.md complete

**Commands:**
```bash
make test
make build
go vet ./...
```

**Output:** `reports/CLEANUP_VERIFICATION_2026-02-07.md` with:
- Test results
- Build status
- Remaining issues (if any)
- Cleanup statistics (lines removed, files moved, etc.)

---

## Success Criteria

**Code:**
- [ ] No dead code in production files
- [ ] No skipped tests without documentation
- [ ] No tests for removed functionality
- [ ] No hardcoded config outside provider implementations
- [ ] All tests passing
- [ ] Clean build

**Documentation:**
- [ ] All files in correct directories
- [ ] No duplicate content
- [ ] KNOWN_ISSUES.md accurate and complete
- [ ] TODO.md organized by category
- [ ] Index up to date
- [ ] No broken links

---

## Rollback Plan

If Phase 5 verification fails:

```bash
# List commits from cleanup
git log --oneline --since="[start_time]"

# Revert all cleanup commits
git revert [commit_range]

# Or reset to before cleanup
git reset --hard [pre_cleanup_commit]
```

---

## Execution Strategy

**Parallel Phase (Agents 1-10):**
```
Agent 1 (dead code)          │
Agent 2 (skipped tests)      │
Agent 3 (outdated tests)     ├─→ 10 parallel audits → 10 reports
Agent 4 (hardcoded config)   │
Agent 5 (doc organization)   │
Agent 6 (known issues)       │
Agent 7 (TODO organization)  │
Agent 8 (test coverage)      │
Agent 9 (imports)            │
Agent 10 (doc accuracy)      │
```

**Sequential Phase (Single agents):**
```
Consolidation agent → execution plan
    ↓
Execution agent → apply changes (file-by-file)
    ↓
Documentation agent → reorganize docs
    ↓
Verification agent → final checks
```

**Estimated time:** 30-45 minutes total

---

## Agent Prompts

### Launch Command for All 10 Agents

```
Launch agents 1-10 in parallel with their respective tasks from Phase 1.
Each agent produces a report in documentation/reports/.
Wait for all 10 reports before proceeding to Phase 2.
```

### After Reports Complete

```
Launch consolidation agent to merge reports into execution plan.
Then launch execution agent with the plan.
Then launch documentation agent for reorganization.
Finally launch verification agent for final checks.
```

---

**Plan Complete**  
**Ready for autonomous execution with 10 parallel agents**
