# LLM Loop Context Composition Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild LLM context composition to include only the system/prompt source plus the most recent tool-call/result loop, using a fixed priority for prompt sources.

**Architecture:** Replace the current “recent memories + compaction” approach with a deterministic loop slice: system prompt + chosen prompt source (commander → last commander broadcast → last swarm broadcast → nudge) + the tool call/result pairs from the most recent loop. This ensures stable, bounded context and eliminates orphaned tool sequencing.

**Tech Stack:** Go 1.22, Bubble Tea, SQLite store, zerolog

---

### Task 1: Add context selection tests (prompt source priority)

**Files:**
- Modify: `internal/core/mysis_test.go`

**Step 1: Write the failing test**

Add tests that build memories and assert the chosen prompt source:

```go
func TestContextPromptSourcePriority(t *testing.T) {
    // Arrange system prompt + commander direct + commander broadcast + swarm broadcast
    // Act: getContextMemories
    // Assert: context includes commander direct and excludes other prompt sources
}
```

Add separate cases for:
- commander direct missing → use last commander broadcast
- commander broadcasts missing → use last swarm broadcast
- no broadcasts → use synthetic nudge

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/core -run TestContextPromptSourcePriority
```
Expected: FAIL.

**Step 3: Commit (tests only)**

```bash
git add internal/core/mysis_test.go
git commit -m "test(core): cover prompt source selection"
```

---

### Task 2: Add loop slice tests (tool call/result inclusion)

**Files:**
- Modify: `internal/core/orphaned_tool_results_test.go`
- Modify: `internal/core/agent3_reproduction_test.go`

**Step 1: Write the failing tests**

Add a test that asserts:
- Only the *most recent* tool-call message and its tool results are present.
- Older tool loops are excluded.

Add a test that asserts:
- Tool results are present *only when* the matching tool call is present.
- No orphaned tool results exist.

**Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/core -run TestLoopContextSlice
```
Expected: FAIL.

**Step 3: Commit (tests only)**

```bash
git add internal/core/orphaned_tool_results_test.go internal/core/agent3_reproduction_test.go
git commit -m "test(core): cover loop slice tool pairing"
```

---

### Task 3: Implement new context composition

**Files:**
- Modify: `internal/core/mysis.go`

**Step 1: Implement helper functions**

Add helpers for:
- selecting the prompt source memory by priority
- extracting the most recent tool-call loop and its tool results

Example signatures:
```go
func (m *Mysis) selectPromptSource(memories []*store.Memory) *store.Memory
func (m *Mysis) extractLatestToolLoop(memories []*store.Memory) []*store.Memory
```

**Step 2: Update `getContextMemories`**

Replace existing compaction/orphan logic with:
- system prompt
- chosen prompt source
- latest tool-call + tool-result pair list

**Step 3: Run tests**

```bash
go test ./internal/core -run TestContextPromptSourcePriority
go test ./internal/core -run TestLoopContextSlice
```
Expected: PASS.

**Step 4: Commit**

```bash
git add internal/core/mysis.go
git commit -m "fix(core): compose context from prompt source and last tool loop"
```

---

### Task 4: Update docs for new loop model

**Files:**
- Modify: `documentation/architecture/CONTEXT_COMPRESSION.md`

**Step 1: Update description**

Document the new “loop slice” approach and prompt source priority:
- commander direct → last commander broadcast → last swarm broadcast → nudge (up to 3) → idle

**Step 2: Commit**

```bash
git add documentation/architecture/CONTEXT_COMPRESSION.md
git commit -m "docs: document loop slice context composition"
```

---

### Task 5: Run core tests

Run:
```bash
go test ./internal/core
```

---

## Notes

- Synthetic nudges are injected into the context for a single turn; they are not stored.
- Tool calls and tool results are preserved *only for the most recent loop*.
- If no prompt source is available after 3 nudges, transition to `idle` (already expected behavior).
