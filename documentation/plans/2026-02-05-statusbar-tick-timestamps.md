# Status Bar Tick + Timestamp Format Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor the footer/status bar to show animated state counts and the swarm aggregate tick; update all UI timestamps to `T#### ⬡ [HH:MM]`.

**Architecture:** Aggregate tick is computed in core (max of lastServerTick across myses) and exposed to TUI via the Model. Timestamps render via a shared formatting helper to keep UI consistent.

**Tech Stack:** Go 1.22, Bubble Tea, Lipgloss, Zerolog, golden tests.

---

## Parallel Track A — Core tick aggregation

### Task A1: Add aggregate tick accessor

**Files:**
- Modify: `internal/core/commander.go` (or core package that owns myses)
- Test: `internal/core/commander_test.go` (or new test file)

**Step 1: Write the failing test**

```go
func TestAggregateTick_MaxAcrossMyses(t *testing.T) {
    // setup commander with myses ticks 0, 120, 98
    // expect 120
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core -run TestAggregateTick_MaxAcrossMyses -v`
Expected: FAIL (method missing)

**Step 3: Write minimal implementation**

```go
func (c *Commander) AggregateTick() int64 {
    // max lastServerTick across myses
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core -run TestAggregateTick_MaxAcrossMyses -v`
Expected: PASS

---

### Task A2: Expose tick to TUI Model

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go` (or new test file)

**Step 1: Write the failing test**

```go
func TestModelRefreshTick(t *testing.T) {
    // model.refreshTick() pulls commander aggregate tick
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestModelRefreshTick -v`
Expected: FAIL (method missing)

**Step 3: Write minimal implementation**

- Add `currentTick int64` to `Model`
- Add `refreshTick()` or integrate into existing refresh cycle

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestModelRefreshTick -v`
Expected: PASS

---

## Parallel Track B — Status bar refactor

### Task B1: Define state counts view model

**Files:**
- Modify: `internal/tui/app.go` (Model fields)
- Test: `internal/tui/statusbar_test.go` (existing or new)

**Step 1: Write the failing test**

```go
func TestStatusBarStateCounts(t *testing.T) {
    // state counts render with icons + totals
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestStatusBarStateCounts -v`
Expected: FAIL

**Step 3: Write minimal implementation**

- Add per-state counts to Model (from Commander or existing state counts)
- Map states to icons (use existing spinner frames for running/loading)

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestStatusBarStateCounts -v`
Expected: PASS

---

### Task B2: Refactor `renderStatusBar()`

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/statusbar_test.go` + golden files

**Step 1: Write the failing golden test**

- Add/adjust test case for new status bar layout

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestStatusBar -v`
Expected: FAIL

**Step 3: Write minimal implementation**

- Compose: `[state icons + counts]  |  T#### ⬡ [HH:MM]  |  VIEW`
- Keep LLM/MCP indicator if required, or move to left segment

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestStatusBar -v`
Expected: PASS

---

## Parallel Track C — Timestamp format changes

### Task C1: Add timestamp formatter helper

**Files:**
- Modify: `internal/tui/styles.go` or new helper file
- Test: `internal/tui/timestamp_test.go`

**Step 1: Write the failing test**

```go
func TestFormatTickTimestamp(t *testing.T) {
    // T1234 ⬡ [09:41]
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestFormatTickTimestamp -v`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
func formatTickTimestamp(tick int64, ts time.Time) string
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tui -run TestFormatTickTimestamp -v`
Expected: PASS

---

### Task C2: Apply formatter to all timestamps

**Files:**
- Modify: `internal/tui/dashboard.go` (swarm messages + mysis last message)
- Modify: `internal/tui/focus.go` (log entry timestamps)

**Step 1: Update code to use formatter**

- Replace `.Format("15:04:05")` with `formatTickTimestamp(m.currentTick, ts)`.

**Step 2: Run targeted tests**

Run: `go test ./internal/tui -run TestDashboard -v`
Run: `go test ./internal/tui -run TestFocusView -v`

**Step 3: Update golden files**

Run: `go test ./internal/tui -update`

**Step 4: Verify**

Run: `go test ./internal/tui`

---

## Integration & Verification (sequential)

1) `go test ./internal/core -run TestAggregateTick -v`
2) `go test ./internal/tui -run TestStatusBar -v`
3) `go test ./internal/tui -update`
4) `go test ./internal/tui`
5) Manual (optional): `./bin/zoea --offline`
