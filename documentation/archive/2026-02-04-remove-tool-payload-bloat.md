# Remove Tool Payload Bloat Plan

**Date:** 2026-02-04
**Status:** Ready for Implementation
**Target Issue:** "Remove Zoea-only fields from tool payloads to reduce bloat for models"

## 1. Problem Statement

The `zoea_list_myses` tool returns internal Zoea infrastructure fields (`provider`, `state`) that provide zero value to LLMs but consume ~22 tokens per mysis. With a full swarm of 16 myses, this wastes ~352 tokens per tool call.

### Current Payload
```json
[
  {
    "id": "22c38707-f6a9-4643-8bc4-27b47fe31cbd",
    "name": "test",
    "provider": "ollama",    // BLOAT: not used by LLMs
    "state": "running"       // BLOAT: redundant with zoea_swarm_status
  }
]
```

### Target Payload
```json
[
  {
    "id": "22c38707-f6a9-4643-8bc4-27b47fe31cbd",
    "name": "test"
  }
]
```

## 2. Field Analysis

| Field | Keep? | Reason |
|-------|-------|--------|
| `id` | ✅ YES | Required for `zoea_send_message` tool |
| `name` | ✅ YES | Helps LLMs identify myses in coordination |
| `provider` | ❌ NO | Not referenced in prompts, not actionable |
| `state` | ❌ NO | Redundant with `zoea_swarm_status` aggregates |

## 3. Implementation Plan

### Phase 1: Update Data Structures

**File:** `internal/mcp/tools.go`

**Change MysisInfo struct:**
```go
// Before
type MysisInfo struct {
    ID        string
    Name      string
    State     string
    Provider  string
    LastError error
}

// After
type MysisInfo struct {
    ID        string
    Name      string
    LastError error  // Keep for error reporting in other contexts
}
```

### Phase 2: Update Tool Handler

**File:** `internal/mcp/tools.go` (lines 69-85)

**Update zoea_list_myses handler:**
```go
func(ctx context.Context, args json.RawMessage) (*ToolResult, error) {
    myses := orchestrator.ListMyses()
    var result []map[string]interface{}
    for _, m := range myses {
        result = append(result, map[string]interface{}{
            "id":   m.ID,
            "name": m.Name,
            // Remove: "state" and "provider"
        })
    }

    data, _ := json.MarshalIndent(result, "", "  ")
    return &ToolResult{
        Content: []ContentBlock{{Type: "text", Text: string(data)}},
    }, nil
}
```

### Phase 3: Update Commander Adapter

**File:** `cmd/zoea/main.go` (lines 245-258)

**Update ListMyses method:**
```go
func (a *commanderAdapter) ListMyses() []mcp.MysisInfo {
    myses := a.commander.ListMyses()
    result := make([]mcp.MysisInfo, len(myses))
    for i, m := range myses {
        result[i] = mcp.MysisInfo{
            ID:   m.ID,
            Name: m.Name,
            // Remove: State and Provider population
        }
    }
    return result
}
```

### Phase 4: Add Unit Tests

**File:** `internal/mcp/tools_test.go`

**Add test case:**
```go
func TestZoeaListMysesPayloadMinimal(t *testing.T) {
    // Create mock orchestrator that returns myses
    // Call zoea_list_myses tool
    // Parse JSON result
    // Verify only "id" and "name" fields are present
    // Verify "provider" and "state" are NOT present
}
```

## 4. Verification Steps

1. Run `make test` - all tests must pass
2. Run `make build` - compilation must succeed
3. Manual verification:
   - Start zoea with 2+ myses
   - Call `zoea_list_myses` from a mysis
   - Verify payload contains only `id` and `name`
4. Update `documentation/KNOWN_ISSUES.md` to mark item resolved

## 5. Expected Impact

**Token Savings:**
- Per mysis: ~22 tokens saved
- Full swarm (16 myses): ~352 tokens saved per call
- Context window: Less bloat in compacted snapshots

**No Breaking Changes:**
- `id` field preserved for `zoea_send_message` functionality
- `name` field preserved for human-readable coordination
- Internal TUI and debugging still have access to full mysis state via commander

## 6. Files Modified

| File | Action |
|------|--------|
| `internal/mcp/tools.go` | Modify MysisInfo struct, update tool handler |
| `cmd/zoea/main.go` | Update commanderAdapter.ListMyses |
| `internal/mcp/tools_test.go` | Add payload verification test |
| `documentation/KNOWN_ISSUES.md` | Mark issue resolved |

## 7. Rollback Plan

If issues arise, revert changes to `internal/mcp/tools.go` and `cmd/zoea/main.go`. The MysisInfo struct change is internal to the MCP layer and doesn't affect core or TUI.
