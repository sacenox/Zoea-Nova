# Captain's Log Bug Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Ensure Myses reliably use `captains_log_add` with valid arguments and receive actionable guidance on server errors, without modifying the SpaceMolt API.

**Architecture:** Improve LLM guidance via prompt examples and better error messaging. Document server-side constraints in `documentation/KNOWN_SERVER_ISSUES.md`. No client-side validation of upstream tool arguments.

**Tech Stack:** Go 1.22+, Bubble Tea, MCP proxy, SQLite, zerolog

---

### Task 1: Update system prompts with explicit examples

**Files:**
- Modify: `internal/core/mysis.go:53-90`
- Test: `internal/core/mysis_test.go`

**Step 1: Write the failing test**

Add tests that assert the SystemPrompt and ContinuePrompt include explicit `captains_log_add` guidance and reminders.

```go
func TestSystemPromptContainsCaptainsLogExamples(t *testing.T) {
    if !strings.Contains(SystemPrompt, "captains_log_add({\"entry\":") {
        t.Fatal("SystemPrompt missing captains_log_add example")
    }
    if !strings.Contains(SystemPrompt, "non-empty entry field") {
        t.Fatal("SystemPrompt missing non-empty entry reminder")
    }
}

func TestContinuePromptContainsCriticalReminders(t *testing.T) {
    if !strings.Contains(ContinuePrompt, "zoea_list_myses") {
        t.Fatal("ContinuePrompt missing mysis ID reminder")
    }
    if !strings.Contains(ContinuePrompt, "captains_log_add") {
        t.Fatal("ContinuePrompt missing captains_log_add reminder")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core -run TestSystemPromptContainsCaptainsLogExamples -v`
Expected: FAIL with missing text in prompts.

**Step 3: Write minimal implementation**

Update `SystemPrompt` memory section to include explicit examples and warnings. Update `ContinuePrompt` to include critical reminders about `captains_log_add` and correct mysis ID usage.

```go
## Memory
Use captain's log to persist important information across sessions.

CRITICAL: captains_log_add requires a non-empty entry field:
CORRECT: captains_log_add({"entry": "Discovered iron ore at Sol-3. Coordinates: X:1234 Y:5678"})
WRONG: captains_log_add({"entry": ""})
WRONG: captains_log_add({})
```

```go
const ContinuePrompt = `Turn complete. What is your next move?

CRITICAL REMINDERS:
- When using captains_log_add, entry field must be non-empty
- Never calculate ticks, use every turn to progress

If waiting for something, describe what and why. Otherwise, continue your mission.`
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core -run TestSystemPromptContainsCaptainsLogExamples -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "feat: add captains log guidance to prompts"
```

---

### Task 2: Improve error messages for upstream empty_entry errors

**Files:**
- Modify: `internal/core/mysis.go:565-583`
- Test: `internal/core/mysis_test.go`

**Step 1: Write the failing test**

```go
func TestFormatToolResult_EmptyEntryError(t *testing.T) {
    m := &Mysis{}
    result := &mcp.ToolResult{
        Content: []mcp.ContentBlock{{Type: "text", Text: "empty_entry"}},
        IsError: true,
    }
    got := m.formatToolResult("call_1", "captains_log_add", result, nil)
    if !strings.Contains(got, "entry field must contain non-empty text") {
        t.Fatal("expected actionable guidance for empty_entry")
    }
}
```

Add an assertion update in `internal/core/mysis_tools_test.go` for the new error format when tool execution returns an error.

```go
if m.Role == store.MemoryRoleTool && strings.Contains(m.Content, "call_err:Error calling error_tool: tool failed") {
    foundError = true
    break
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core -run TestFormatToolResult_EmptyEntryError -v`
Expected: FAIL with missing guidance.

**Step 3: Write minimal implementation**

Update `formatToolResult` to detect `empty_entry` in tool error content and include guidance and a correct example for `captains_log_add`.

```go
if result.IsError {
    if strings.Contains(content, "empty_entry") {
        content = fmt.Sprintf("Error: %s. The entry field must contain non-empty text. Example: captains_log_add({\"entry\": \"Your message here\"})", content)
    } else {
        content = "Error: " + content
    }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core -run TestFormatToolResult_EmptyEntryError -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "fix: add guidance for captains log empty_entry"
```

---

### Task 3: Document server-side behavior and update known issues

**Files:**
- Create: `documentation/KNOWN_SERVER_ISSUES.md`
- Modify: `documentation/KNOWN_ISSUES.md`

**Step 1: Write the documentation**

```markdown
# Known SpaceMolt Server Issues

This document tracks known issues with the upstream SpaceMolt MCP server API. We do not modify or validate the game server's API behavior. Issues are documented here, and we improve prompts and error handling instead.

## captains_log_add: empty_entry Error

**Issue:** The `captains_log_add` tool returns an `empty_entry` error when:
- The `entry` field is an empty string
- The `entry` field is missing
- The `entry` field contains only whitespace

**Server Response:**
```json
{"error":{"code":0,"message":"empty_entry"}}
```

**Workaround:**
- System prompt includes explicit examples of correct usage
- Error messages provide actionable guidance when this error occurs
```

**Step 2: Update known issues**

Mark the Captain's Log bug as addressed and reference `documentation/KNOWN_SERVER_ISSUES.md`.

**Step 3: Run docs checks**

Run: `git diff -- documentation/KNOWN_ISSUES.md documentation/KNOWN_SERVER_ISSUES.md`
Expected: both files updated with consistent text.

**Step 4: Commit**

```bash
git add documentation/KNOWN_SERVER_ISSUES.md documentation/KNOWN_ISSUES.md
git commit -m "docs: record captains log server issue"
```

---

### Task 4: Verification

**Step 1: Run tests**

Run: `make test`
Expected: PASS

**Step 2: Build**

Run: `make build`
Expected: PASS

**Step 3: Manual sanity check**

1. Start app: `./bin/zoea`
2. Create a Mysis.
3. Send: "Remember iron ore sells well."
4. Observe that `captains_log_add` calls include non-empty entry text or error guidance is shown.

---

## Notes

- Do not add client-side validation for upstream tools.
- Do not modify the game server API.
- Keep all application code under `internal/`.
- Update tests alongside changes.
