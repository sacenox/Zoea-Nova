# Mysis State Alignment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Align mysis state semantics with the desired behavior (idle on 3 nudges, errored on provider/MCP failures, activities only for TUI/context).

**Architecture:** Keep `MysisState` for lifecycle (idle/running/stopped/errored) and use `ActivityState` only for UI/context. Adjust loop control to avoid activity gating, add explicit idle transition for the 3‑nudge breaker, and treat tool timeouts/failures as errored with persisted error context.

**Tech Stack:** Go 1.22, Bubble Tea, SQLite store, zerolog

---

## Task 1: Update state semantics documentation

**Files:**
- Modify: `documentation/architecture/MYSIS_STATE_MACHINE.md`

**Step 1: Write the doc change**

- Update definitions:
  - `idle`: set when 3 nudges fail; requires commander/broadcast to resume.
  - `thinking`: not a state; represent via `ActivityStateLLMCall`.
  - `running`: after reply, waiting for next nudge.
  - `errored`: provider/MCP failures after retries.
- Update transitions to include `running → idle` on 3‑nudge failure.

**Step 2: Review for consistency**

- Ensure doc does not claim activity gating for nudges.

**Step 3: Commit**

```bash
git add documentation/architecture/MYSIS_STATE_MACHINE.md
git commit -m "docs: align mysis state semantics"
```

---

## Task 2: Make activity non‑blocking for nudges

**Files:**
- Modify: `internal/core/mysis.go` (`shouldNudge`)

**Step 1: Write failing unit test**

- Add test in `internal/core/mysis_test.go`:
  - Set activity to `Traveling` or `Cooldown` with `activityUntil` in the future.
  - Assert `shouldNudge(...)` returns `true`.

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/core -run TestShouldNudge_IgnoresActivity
```
Expected: FAIL (returns false today).

**Step 3: Minimal implementation**

- Change `shouldNudge` to ignore activity state and always return true when running.
- Keep activity state for UI/context only.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/core -run TestShouldNudge_IgnoresActivity
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "fix: keep nudges active during in-game activity"
```

---

## Task 3: 3‑nudge breaker sets Idle (not Errored)

**Files:**
- Modify: `internal/core/mysis.go` (`run`, add `setIdle` helper)
- Modify: `internal/store/myses.go` if needed (state update helper)

**Step 1: Write failing unit test**

- Add test in `internal/core/mysis_test.go`:
  - Simulate 3 idle nudges with no turn in progress.
  - Assert state transitions to `idle` and store updated.

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/core -run TestNudgeBreakerSetsIdle
```
Expected: FAIL (currently sets errored).

**Step 3: Minimal implementation**

- Add `setIdle(reason string)` that:
  - Sets `state = idle`
  - Updates store
  - Emits `EventMysisStateChanged`
  - Publishes a non‑error event or log with reason
- Use `setIdle("Failed to respond after 3 nudges")` in `run` instead of `setError`.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/core -run TestNudgeBreakerSetsIdle
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "fix: mark mysis idle after nudge breaker"
```

---

## Task 4: Tool timeout/failure → errored + persisted tool error message

**Files:**
- Modify: `internal/core/mysis.go` (`SendMessageFrom`, `SendEphemeralMessage`, `executeToolCall`)
- Modify: `internal/constants/constants.go` (add error message template)

**Step 1: Write failing unit test**

- Add test in `internal/core/mysis_test.go`:
  - Simulate a tool call that times out (context deadline exceeded).
  - Assert:
    - `MysisState` becomes `errored`.
    - A tool result memory is stored that contains a clear timeout error string.

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/core -run TestToolTimeoutSetsErroredAndPersistsToolError
```
Expected: FAIL.

**Step 3: Minimal implementation**

- In tool execution loop:
  - If `executeToolCall` returns `ctx.DeadlineExceeded` or `ctx.Canceled`, store a tool result memory with `Error: tool call <name> timed out: <err>`.
  - Call `setError(err)` immediately after persisting the tool error memory.
- Ensure the error text is included in stored memory so a restarted mysis can read it.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/core -run TestToolTimeoutSetsErroredAndPersistsToolError
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go internal/constants/constants.go
git commit -m "fix: mark mysis errored on tool timeout and persist error"
```

---

## Task 5: MCP failure after retries → errored

**Files:**
- Modify: `internal/core/mysis.go` (tool call error handling)
- Modify: `internal/mcp/proxy.go` if needed for retry exhaust signal

**Step 1: Write failing unit test**

- Add test in `internal/core/mysis_test.go`:
  - Simulate MCP call returning a retried failure (tool error) without store errors.
  - Assert mysis transitions to `errored` and tool error message persisted.

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/core -run TestToolRetryFailureSetsErrored
```
Expected: FAIL.

**Step 3: Minimal implementation**

- Detect retry‑exhausted tool failure (add a sentinel error or wrapper in MCP proxy).
- When detected, store error tool result and call `setError(err)`.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/core -run TestToolRetryFailureSetsErrored
```
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/mcp/proxy.go internal/core/mysis_test.go
git commit -m "fix: set errored on MCP retry exhaustion"
```

---

## Task 6: Verify behavior end‑to‑end

**Step 1: Run unit tests**

Run:
```bash
go test ./internal/core
```

**Step 2: Run broader test suite**

Run:
```bash
make test
```

**Step 3: Build**

Run:
```bash
make build
```

---

## Notes / Decisions

- `Thinking` remains an activity indicator (`ActivityStateLLMCall`), not a state.
- Activities remain for UI/context and no longer block nudges.
- `Idle` is now the “needs attention” state after 3 failed nudges.
- Provider errors already set `Errored`; MCP failures will now do so when retries are exhausted.
