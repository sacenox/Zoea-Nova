# Broadcast Sender Tracking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track broadcast message sender and suppress self-response to eliminate conversation loops and fix focus view labels.

**Architecture:** Add sender_id to broadcast messages in database and memory flow. Filter sender from broadcast recipients. Update TUI rendering to distinguish broadcast sources.

**Tech Stack:** Go 1.22+, SQLite (modernc.org/sqlite), Bubble Tea TUI framework

---

## Background

### Current Behavior

Broadcast messages have three problems:

1. **Self-response loops**: When a mysis broadcasts, it receives its own message and responds to it
2. **Missing sender info**: Database stores broadcasts without sender tracking
3. **Incorrect labels**: Focus view shows all broadcasts as "YOU:" regardless of source

### Current Flow

```
User Input → Commander.BroadcastAsync(content)
  ↓
For each running mysis (including sender):
  ↓
  mysis.SendMessage(content, source='broadcast')
  ↓
  Store memory: role='user', source='broadcast', sender_id=NULL
  ↓
  Mysis processes with LLM and responds
```

### Desired Flow

```
Mysis Tool Call → zoea_broadcast(message) → Commander.BroadcastFrom(senderID, content)
  ↓
Filter: recipients = all running myses EXCEPT sender
  ↓
For each recipient:
  ↓
  mysis.SendMessage(content, source='broadcast', senderID)
  ↓
  Store memory: role='user', source='broadcast', sender_id=<senderID>
  ↓
  TUI: Show "SWARM:" for other's broadcasts, "YOU (BROADCAST):" for own
```

### Files Affected

**Database:**
- `internal/store/schema.sql` - Add sender_id column
- `internal/store/memories.go` - Update structs and methods

**Core:**
- `internal/core/commander.go` - Filter sender from recipients
- `internal/core/mysis.go` - Pass sender info when storing

**MCP:**
- `internal/mcp/tools.go` - Update zoea_broadcast to capture sender
- `internal/mcp/proxy.go` - Pass mysis context to tool handlers

**TUI:**
- `internal/tui/focus.go` - Update rendering logic for sender-aware labels

**Tests:**
- `internal/core/commander_test.go` - Update broadcast tests
- `internal/store/memories_test.go` - Update broadcast storage tests
- `internal/tui/focus_test.go` - Add sender label rendering tests

---

## Task 1: Database Schema Migration

**Files:**
- Modify: `internal/store/schema.sql`
- Modify: `internal/store/migrations.go`

### Step 1: Add sender_id column to schema

Update `internal/store/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    mysis_id TEXT NOT NULL,
    role TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'direct',
    sender_id TEXT,  -- NEW: tracks who sent the message (for broadcasts)
    content TEXT NOT NULL,
    reasoning TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (mysis_id) REFERENCES myses(id) ON DELETE CASCADE
);
```

### Step 2: Update SchemaVersion constant

In `internal/store/migrations.go` (or schema.sql if version is there):

```go
// SchemaVersion is the current database schema version.
const SchemaVersion = 8
```

### Step 3: Document migration requirement

Add comment to schema.sql:

```sql
-- Schema v7 → v8 Migration:
-- Added sender_id column to memories table for broadcast tracking.
-- BREAKING CHANGE: Requires fresh database (make db-reset-accounts)
```

### Step 4: Verify schema change

```bash
# Check syntax
sqlite3 :memory: < internal/store/schema.sql
```

Expected: No errors

### Step 5: Commit schema changes

```bash
git add internal/store/schema.sql internal/store/migrations.go
git commit -m "feat(db): add sender_id to memories for broadcast tracking

Schema v7 → v8. Breaking change requires fresh database."
```

---

## Task 2: Update Store Layer - Structs

**Files:**
- Modify: `internal/store/memories.go:17-30` (Memory struct)
- Modify: `internal/store/memories.go:152-156` (BroadcastMessage struct)

### Step 1: Write test for Memory struct with sender_id

Add to `internal/store/memories_test.go`:

```go
func TestMemoryWithSenderID(t *testing.T) {
	db := setupTestDB(t)
	mysisID := "test-mysis"
	senderID := "sender-mysis"

	// Add broadcast memory with sender
	err := db.AddMemory(mysisID, MemoryRoleUser, MemorySourceBroadcast, "test broadcast", "", senderID)
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}

	// Retrieve and verify sender_id
	memories, err := db.GetRecentMemories(mysisID, 10)
	if err != nil {
		t.Fatalf("GetRecentMemories failed: %v", err)
	}

	if len(memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(memories))
	}

	if memories[0].SenderID != senderID {
		t.Errorf("expected sender_id %q, got %q", senderID, memories[0].SenderID)
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/store -run TestMemoryWithSenderID -v
```

Expected: Compilation error - SenderID field doesn't exist

### Step 3: Update Memory struct

In `internal/store/memories.go`:

```go
// Memory represents a stored conversation message with role and source tracking.
type Memory struct {
	ID        int64
	MysisID   string
	Role      MemoryRole
	Source    MemorySource
	SenderID  string    // NEW: For broadcasts, tracks who sent the message
	Content   string
	Reasoning string
	CreatedAt time.Time
}
```

### Step 4: Update BroadcastMessage struct

In `internal/store/memories.go`:

```go
// BroadcastMessage represents a broadcast message with sender tracking.
type BroadcastMessage struct {
	SenderID  string    // NEW: Mysis ID that sent the broadcast
	Content   string
	CreatedAt time.Time
}
```

### Step 5: Run test again

```bash
go test ./internal/store -run TestMemoryWithSenderID -v
```

Expected: Still fails - AddMemory signature doesn't accept sender_id

### Step 6: Commit struct changes

```bash
git add internal/store/memories.go
git commit -m "feat(store): add SenderID to Memory and BroadcastMessage structs"
```

---

## Task 3: Update Store Layer - AddMemory Method

**Files:**
- Modify: `internal/store/memories.go` (AddMemory method)

### Step 1: Update AddMemory signature and implementation

In `internal/store/memories.go`, find the AddMemory method and update:

```go
// AddMemory stores a new memory for a mysis.
// senderID is optional - empty string for direct messages, mysis ID for broadcasts.
func (s *Store) AddMemory(mysisID string, role MemoryRole, source MemorySource, content, reasoning, senderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		INSERT INTO memories (mysis_id, role, source, sender_id, content, reasoning)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query, mysisID, role, source, senderID, content, reasoning)
	return err
}
```

### Step 2: Run test to verify it compiles but still fails

```bash
go test ./internal/store -run TestMemoryWithSenderID -v
```

Expected: Compilation succeeds, test might pass now

### Step 3: Commit AddMemory changes

```bash
git add internal/store/memories.go
git commit -m "feat(store): update AddMemory to accept sender_id parameter"
```

---

## Task 4: Update Store Layer - Query Methods

**Files:**
- Modify: `internal/store/memories.go` (GetRecentMemories, GetRecentBroadcasts, SearchBroadcasts)

### Step 1: Update GetRecentMemories to retrieve sender_id

In `internal/store/memories.go`:

```go
func (s *Store) GetRecentMemories(mysisID string, limit int) ([]*Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT id, mysis_id, role, source, sender_id, content, reasoning, created_at
		FROM memories
		WHERE mysis_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, mysisID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := []*Memory{}
	for rows.Next() {
		m := &Memory{}
		var senderID sql.NullString
		err := rows.Scan(&m.ID, &m.MysisID, &m.Role, &m.Source, &senderID, &m.Content, &m.Reasoning, &m.CreatedAt)
		if err != nil {
			return nil, err
		}
		if senderID.Valid {
			m.SenderID = senderID.String
		}
		memories = append(memories, m)
	}

	// Reverse to chronological order
	for i, j := 0, len(memories)-1; i < j; i, j = i+1, j-1 {
		memories[i], memories[j] = memories[j], memories[i]
	}

	return memories, rows.Err()
}
```

### Step 2: Update GetRecentBroadcasts to include sender_id

In `internal/store/memories.go`:

```go
func (s *Store) GetRecentBroadcasts(limit int) ([]*BroadcastMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
		SELECT sender_id, content, MIN(created_at) as created_at
		FROM memories
		WHERE source = 'broadcast'
		GROUP BY content, sender_id
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	broadcasts := []*BroadcastMessage{}
	for rows.Next() {
		b := &BroadcastMessage{}
		var senderID sql.NullString
		err := rows.Scan(&senderID, &b.Content, &b.CreatedAt)
		if err != nil {
			return nil, err
		}
		if senderID.Valid {
			b.SenderID = senderID.String
		}
		broadcasts = append(broadcasts, b)
	}

	return broadcasts, rows.Err()
}
```

### Step 3: Update SearchBroadcasts to include sender_id

In `internal/store/memories.go`:

```go
func (s *Store) SearchBroadcasts(query string, limit int) ([]*BroadcastMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sql := `
		SELECT sender_id, content, MIN(created_at) as created_at
		FROM memories
		WHERE source = 'broadcast' AND content LIKE ?
		GROUP BY content, sender_id
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.Query(sql, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	broadcasts := []*BroadcastMessage{}
	for rows.Next() {
		b := &BroadcastMessage{}
		var senderID sql.NullString
		err := rows.Scan(&senderID, &b.Content, &b.CreatedAt)
		if err != nil {
			return nil, err
		}
		if senderID.Valid {
			b.SenderID = senderID.String
		}
		broadcasts = append(broadcasts, b)
	}

	return broadcasts, rows.Err()
}
```

### Step 4: Run all store tests

```bash
go test ./internal/store -v
```

Expected: TestMemoryWithSenderID should pass. Other tests may fail due to AddMemory signature change.

### Step 5: Commit query method updates

```bash
git add internal/store/memories.go
git commit -m "feat(store): update query methods to retrieve sender_id"
```

---

## Task 5: Fix Existing Store Test Failures

**Files:**
- Modify: `internal/store/memories_test.go`

### Step 1: Update all AddMemory calls in tests

Find all calls to `AddMemory` in test files and add empty string as sender_id:

```go
// Before:
db.AddMemory(mysisID, MemoryRoleUser, MemorySourceDirect, "test", "")

// After:
db.AddMemory(mysisID, MemoryRoleUser, MemorySourceDirect, "test", "", "")
```

### Step 2: Run store tests to verify fixes

```bash
go test ./internal/store -v
```

Expected: All tests pass

### Step 3: Commit test fixes

```bash
git add internal/store/memories_test.go
git commit -m "test(store): update AddMemory calls for sender_id parameter"
```

---

## Task 6: Update Core Layer - Mysis SendMessage

**Files:**
- Modify: `internal/core/mysis.go:342-375` (SendMessage method)

### Step 1: Write test for Mysis broadcast with sender

Add to `internal/core/mysis_test.go`:

```go
func TestMysisReceivesBroadcastWithSender(t *testing.T) {
	db := setupTestStore(t)
	bus := NewEventBus()
	
	receiver := createTestMysis(t, db, bus, "receiver")
	senderID := "sender-mysis"
	
	// Send broadcast message with sender ID
	err := receiver.SendMessageFrom("test broadcast", store.MemorySourceBroadcast, senderID)
	if err != nil {
		t.Fatalf("SendMessageFrom failed: %v", err)
	}
	
	// Verify memory was stored with sender_id
	memories, err := db.GetRecentMemories(receiver.ID(), 10)
	if err != nil {
		t.Fatalf("GetRecentMemories failed: %v", err)
	}
	
	if len(memories) == 0 {
		t.Fatal("expected at least 1 memory")
	}
	
	found := false
	for _, m := range memories {
		if m.Source == store.MemorySourceBroadcast && m.SenderID == senderID {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("broadcast memory with correct sender_id not found")
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/core -run TestMysisReceivesBroadcastWithSender -v
```

Expected: Compilation error - SendMessageFrom method doesn't exist

### Step 3: Add SendMessageFrom method to Mysis

In `internal/core/mysis.go`:

```go
// SendMessageFrom sends a message to this mysis with sender tracking.
// Used for broadcasts to track which mysis originated the message.
func (a *Mysis) SendMessageFrom(content string, source store.MemorySource, senderID string) error {
	a.mu.Lock()
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		return fmt.Errorf("mysis not running")
	}
	a.mu.Unlock()

	// Determine role
	role := store.MemoryRoleUser
	if source == store.MemorySourceSystem {
		role = store.MemoryRoleSystem
	}

	// Store message with sender ID
	if err := a.store.AddMemory(a.id, role, source, content, "", senderID); err != nil {
		return err
	}

	// Emit message event
	a.bus.Publish(Event{
		Type:      EventMysisMessage,
		MysisID:   a.id,
		MysisName: a.name,
		Data: MessageData{
			Role:    string(role),
			Content: content,
		},
		Timestamp: time.Now(),
	})

	// Process with LLM
	go a.processNextMessage()

	return nil
}
```

### Step 4: Update existing SendMessage to use SendMessageFrom

```go
// SendMessage sends a message to this mysis without sender tracking.
// Used for direct messages and system prompts.
func (a *Mysis) SendMessage(content string, source store.MemorySource) error {
	return a.SendMessageFrom(content, source, "")
}
```

### Step 5: Run test to verify it passes

```bash
go test ./internal/core -run TestMysisReceivesBroadcastWithSender -v
```

Expected: Test passes

### Step 6: Commit Mysis changes

```bash
git add internal/core/mysis.go internal/core/mysis_test.go
git commit -m "feat(core): add SendMessageFrom for broadcast sender tracking"
```

---

## Task 7: Update Core Layer - Commander Broadcast Filtering

**Files:**
- Modify: `internal/core/commander.go:312-345` (BroadcastAsync method)

### Step 1: Write test for sender exclusion

Add to `internal/core/commander_test.go`:

```go
func TestBroadcastExcludesSender(t *testing.T) {
	db := setupTestStore(t)
	bus := NewEventBus()
	cmd := NewCommander(db, bus)

	sender, _ := cmd.CreateMysis("sender", "mock")
	receiver, _ := cmd.CreateMysis("receiver", "mock")

	// Start both myses
	sender.Start()
	receiver.Start()
	defer sender.Stop()
	defer receiver.Stop()

	// Wait for myses to be running
	time.Sleep(100 * time.Millisecond)

	// Broadcast from sender
	err := cmd.BroadcastFrom(sender.ID(), "test broadcast")
	if err != nil {
		t.Fatalf("BroadcastFrom failed: %v", err)
	}

	// Wait for delivery
	time.Sleep(200 * time.Millisecond)

	// Verify sender did NOT receive its own broadcast
	senderMemories, err := db.GetRecentMemories(sender.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories for sender failed: %v", err)
	}

	for _, m := range senderMemories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "test broadcast" {
			t.Error("sender received its own broadcast - should be excluded")
		}
	}

	// Verify receiver DID receive the broadcast
	receiverMemories, err := db.GetRecentMemories(receiver.ID(), 50)
	if err != nil {
		t.Fatalf("GetRecentMemories for receiver failed: %v", err)
	}

	found := false
	for _, m := range receiverMemories {
		if m.Source == store.MemorySourceBroadcast && m.Content == "test broadcast" && m.SenderID == sender.ID() {
			found = true
			break
		}
	}

	if !found {
		t.Error("receiver did not receive broadcast with correct sender_id")
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/core -run TestBroadcastExcludesSender -v
```

Expected: Test fails - BroadcastFrom method doesn't exist

### Step 3: Add BroadcastFrom method to Commander

In `internal/core/commander.go`:

```go
// BroadcastFrom sends a message to all running myses except the sender.
func (c *Commander) BroadcastFrom(senderID, content string) error {
	c.mu.RLock()
	myses := make([]*Mysis, 0)
	for _, m := range c.myses {
		// Exclude sender and non-running myses
		if m.State() == MysisStateRunning && m.ID() != senderID {
			myses = append(myses, m)
		}
	}
	c.mu.RUnlock()

	if len(myses) == 0 {
		return fmt.Errorf("no recipients for broadcast (sender excluded)")
	}

	// Emit broadcast event
	c.bus.Publish(Event{
		Type:      EventBroadcast,
		Data: MessageData{
			Role:    "user",
			Content: content,
		},
		Timestamp: time.Now(),
	})

	// Send to each recipient with sender tracking
	var errs []error
	for _, m := range myses {
		if err := m.SendMessageFrom(content, store.MemorySourceBroadcast, senderID); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("broadcast failed for %d recipients: %w", len(errs), errors.Join(errs...))
	}

	return nil
}
```

### Step 4: Update BroadcastAsync to use BroadcastFrom with empty sender

In `internal/core/commander.go`:

```go
// BroadcastAsync sends a message to all running myses (legacy method).
// Use BroadcastFrom for sender tracking.
func (c *Commander) BroadcastAsync(content string) error {
	return c.BroadcastFrom("", content)
}
```

### Step 5: Run test to verify it passes

```bash
go test ./internal/core -run TestBroadcastExcludesSender -v
```

Expected: Test passes

### Step 6: Run all commander tests

```bash
go test ./internal/core -run TestCommander -v
```

Expected: Existing tests still pass (BroadcastAsync now uses BroadcastFrom with empty sender)

### Step 7: Commit Commander changes

```bash
git add internal/core/commander.go internal/core/commander_test.go
git commit -m "feat(core): add BroadcastFrom to exclude sender from recipients"
```

---

## Task 8: Update MCP Layer - Tool Handler Context

**Files:**
- Modify: `internal/mcp/proxy.go` (CallTool to accept caller context)
- Modify: `internal/mcp/tools.go` (zoea_broadcast handler)

### Step 1: Add context parameter to tool registration

In `internal/mcp/proxy.go`, update the Proxy struct to support caller context:

```go
// CallerContext provides information about who is calling a tool.
type CallerContext struct {
	MysisID   string // ID of the mysis making the call
	MysisName string // Name of the mysis making the call
}

// ToolHandlerWithContext is a function that handles a tool call with caller context.
type ToolHandlerWithContext func(ctx context.Context, caller CallerContext, arguments json.RawMessage) (*ToolResult, error)
```

### Step 2: Update RegisterTool to support context-aware handlers

Add new registration method:

```go
// RegisterToolWithContext registers a tool handler that receives caller context.
func (p *Proxy) RegisterToolWithContext(tool Tool, handler ToolHandlerWithContext) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.localTools[tool.Name] = tool
	p.contextHandlers[tool.Name] = handler
}
```

Update Proxy struct to include contextHandlers:

```go
type Proxy struct {
	mu             sync.RWMutex
	upstream       *Client
	localTools     map[string]Tool
	localHandlers  map[string]ToolHandler
	contextHandlers map[string]ToolHandlerWithContext // NEW
	accountStore   AccountStore
}
```

### Step 3: Update CallTool to pass caller context

In `internal/mcp/proxy.go`:

```go
// CallTool invokes a tool with caller context.
func (p *Proxy) CallTool(ctx context.Context, caller CallerContext, name string, arguments json.RawMessage) (*ToolResult, error) {
	p.mu.RLock()
	handler, isLocal := p.localHandlers[name]
	contextHandler, hasContext := p.contextHandlers[name]
	p.mu.RUnlock()

	// Try context-aware handler first
	if hasContext {
		return contextHandler(ctx, caller, arguments)
	}

	// Fall back to legacy handler
	if isLocal {
		return handler(ctx, arguments)
	}

	// Forward to upstream
	if p.upstream != nil {
		return p.upstream.CallTool(ctx, name, arguments)
	}

	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("tool %q not found", name)}},
		IsError: true,
	}, nil
}
```

### Step 4: Update Mysis to pass caller context to MCP

In `internal/core/mysis.go`, update executeToolCall:

```go
func (a *Mysis) executeToolCall(ctx context.Context, mcpProxy *mcp.Proxy, tc provider.ToolCall) (*mcp.ToolResult, error) {
	if mcpProxy == nil {
		return &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "MCP not configured"}},
			IsError: true,
		}, nil
	}

	// Create caller context
	caller := mcp.CallerContext{
		MysisID:   a.id,
		MysisName: a.name,
	}

	result, err := mcpProxy.CallTool(ctx, caller, tc.Name, tc.Arguments)
	// ... rest of method
}
```

### Step 5: Commit MCP context infrastructure

```bash
git add internal/mcp/proxy.go internal/core/mysis.go
git commit -m "feat(mcp): add caller context to tool handlers"
```

---

## Task 9: Update zoea_broadcast Tool Handler

**Files:**
- Modify: `internal/mcp/tools.go:145-182` (zoea_broadcast registration)

### Step 1: Update zoea_broadcast to use context-aware handler

In `internal/mcp/tools.go`:

```go
// Register zoea_broadcast with context
proxy.RegisterToolWithContext(
	Tool{
		Name:        "zoea_broadcast",
		Description: "Send a message to all running myses (you will not receive your own broadcast)",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {"type": "string", "description": "The message to broadcast"}
			},
			"required": ["message"]
		}`),
	},
	func(ctx context.Context, caller mcp.CallerContext, args json.RawMessage) (*mcp.ToolResult, error) {
		var params struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: fmt.Sprintf("invalid arguments: %v", err)}},
				IsError: true,
			}, nil
		}

		// Use BroadcastFrom with caller's mysis ID
		if err := orchestrator.BroadcastFrom(caller.MysisID, params.Message); err != nil {
			return &mcp.ToolResult{
				Content: []mcp.ContentBlock{{Type: "text", Text: fmt.Sprintf("broadcast failed: %v", err)}},
				IsError: true,
			}, nil
		}

		return &mcp.ToolResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: "broadcast sent to all running myses (excluding you)"}},
		}, nil
	},
)
```

### Step 2: Verify compilation

```bash
go build ./...
```

Expected: No compilation errors

### Step 3: Commit tool handler update

```bash
git add internal/mcp/tools.go
git commit -m "feat(mcp): update zoea_broadcast to use caller context and exclude sender"
```

---

## Task 10: Update TUI Layer - Focus View Rendering

**Files:**
- Modify: `internal/tui/focus.go:241-247` (LogEntryFromMemory)
- Modify: `internal/tui/focus.go:168-191` (renderConversation)

### Step 1: Write test for sender-aware label rendering

Create `internal/tui/focus_test.go` if it doesn't exist:

```go
package tui

import (
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/store"
)

func TestLogEntryFromMemoryWithSender(t *testing.T) {
	currentMysisID := "current-mysis"
	otherMysisID := "other-mysis"

	tests := []struct {
		name       string
		memory     *store.Memory
		currentID  string
		wantPrefix string
	}{
		{
			name: "direct message",
			memory: &store.Memory{
				Role:    store.MemoryRoleUser,
				Source:  store.MemorySourceDirect,
				Content: "direct msg",
			},
			currentID:  currentMysisID,
			wantPrefix: "YOU:",
		},
		{
			name: "broadcast from self",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: currentMysisID,
				Content:  "my broadcast",
			},
			currentID:  currentMysisID,
			wantPrefix: "YOU (BROADCAST):",
		},
		{
			name: "broadcast from other",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: otherMysisID,
				Content:  "other's broadcast",
			},
			currentID:  currentMysisID,
			wantPrefix: "SWARM:",
		},
		{
			name: "broadcast legacy (no sender)",
			memory: &store.Memory{
				Role:     store.MemoryRoleUser,
				Source:   store.MemorySourceBroadcast,
				SenderID: "",
				Content:  "legacy broadcast",
			},
			currentID:  currentMysisID,
			wantPrefix: "SWARM:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LogEntryFromMemory(tt.memory, tt.currentID)
			// Render logic tested separately - here just verify entry construction
			if entry.Role != string(tt.memory.Role) {
				t.Errorf("role: got %q, want %q", entry.Role, tt.memory.Role)
			}
			if entry.SenderID != tt.memory.SenderID {
				t.Errorf("sender_id: got %q, want %q", entry.SenderID, tt.memory.SenderID)
			}
		})
	}
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/tui -run TestLogEntryFromMemoryWithSender -v
```

Expected: Compilation error - LogEntry doesn't have SenderID field

### Step 3: Update LogEntry struct

In `internal/tui/focus.go`:

```go
type LogEntry struct {
	Role      string
	Source    string  // NEW: message source (direct/broadcast/system)
	SenderID  string  // NEW: for broadcasts, who sent it
	Content   string
	Timestamp time.Time
}
```

### Step 4: Update LogEntryFromMemory to accept current mysis ID

```go
// LogEntryFromMemory converts a store.Memory to a LogEntry with sender awareness.
func LogEntryFromMemory(m *store.Memory, currentMysisID string) LogEntry {
	return LogEntry{
		Role:      string(m.Role),
		Source:    string(m.Source),
		SenderID:  m.SenderID,
		Content:   m.Content,
		Timestamp: m.CreatedAt,
	}
}
```

### Step 5: Update renderConversation to use sender-aware prefixes

In `internal/tui/focus.go`:

```go
func (m *FocusModel) renderConversation() string {
	if len(m.conversation) == 0 {
		return dimStyle.Render("No conversation history yet")
	}

	lines := make([]string, 0, len(m.conversation))
	for _, entry := range m.conversation {
		var prefix string
		var style lipgloss.Style

		switch entry.Role {
		case "user":
			// Distinguish broadcast sources
			if entry.Source == "broadcast" {
				if entry.SenderID == m.mysisID {
					prefix = "YOU (BROADCAST): "
					style = userStyle
				} else {
					prefix = "SWARM: "
					style = swarmStyle // NEW: define this style
				}
			} else {
				prefix = "YOU: "
				style = userStyle
			}
		case "assistant":
			prefix = "AI:  "
			style = assistantStyle
		case "system":
			prefix = "SYS: "
			style = systemStyle
		case "tool":
			prefix = "TOOL:"
			style = toolStyle
		default:
			prefix = "???:"
			style = lipgloss.NewStyle()
		}

		lines = append(lines, style.Render(prefix+entry.Content))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
```

### Step 6: Define swarmStyle

In `internal/tui/focus.go`:

```go
var (
	// ... existing styles
	swarmStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange for swarm broadcasts
)
```

### Step 7: Update LoadConversation calls to pass mysis ID

Find all places that call `LogEntryFromMemory` and update:

```go
// Before:
entry := LogEntryFromMemory(memory)

// After:
entry := LogEntryFromMemory(memory, m.mysisID)
```

### Step 8: Run test to verify it passes

```bash
go test ./internal/tui -run TestLogEntryFromMemoryWithSender -v
```

Expected: Test passes

### Step 9: Commit TUI changes

```bash
git add internal/tui/focus.go internal/tui/focus_test.go
git commit -m "feat(tui): add sender-aware broadcast labels in focus view"
```

---

## Task 11: Integration Testing

**Files:**
- Create: `internal/integration/broadcast_sender_test.go`

### Step 1: Create end-to-end integration test

```go
package integration

import (
	"testing"
	"time"

	"github.com/xonecas/zoea-nova/internal/core"
	"github.com/xonecas/zoea-nova/internal/mcp"
	"github.com/xonecas/zoea-nova/internal/store"
)

func TestBroadcastSenderTracking(t *testing.T) {
	// Setup
	db := setupTestStore(t)
	bus := core.NewEventBus()
	cmd := core.NewCommander(db, bus)
	proxy := mcp.NewProxy("")
	
	// Register tools
	mcp.RegisterOrchestrationTools(proxy, cmd)

	// Create myses
	sender, _ := cmd.CreateMysis("sender", "mock")
	receiver1, _ := cmd.CreateMysis("receiver1", "mock")
	receiver2, _ := cmd.CreateMysis("receiver2", "mock")

	sender.Start()
	receiver1.Start()
	receiver2.Start()
	defer func() {
		sender.Stop()
		receiver1.Stop()
		receiver2.Stop()
	}()

	time.Sleep(100 * time.Millisecond)

	// Execute: Sender broadcasts via tool
	ctx := context.Background()
	caller := mcp.CallerContext{
		MysisID:   sender.ID(),
		MysisName: sender.Name(),
	}
	
	args := json.RawMessage(`{"message": "integration test broadcast"}`)
	result, err := proxy.CallTool(ctx, caller, "zoea_broadcast", args)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result)
	}

	// Wait for delivery
	time.Sleep(200 * time.Millisecond)

	// Verify: Sender did NOT receive
	senderMems, _ := db.GetRecentMemories(sender.ID(), 50)
	for _, m := range senderMems {
		if m.Source == store.MemorySourceBroadcast && m.Content == "integration test broadcast" {
			t.Error("sender received its own broadcast")
		}
	}

	// Verify: Receivers DID receive with sender ID
	for _, receiver := range []*core.Mysis{receiver1, receiver2} {
		mems, _ := db.GetRecentMemories(receiver.ID(), 50)
		found := false
		for _, m := range mems {
			if m.Source == store.MemorySourceBroadcast && 
			   m.Content == "integration test broadcast" && 
			   m.SenderID == sender.ID() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s did not receive broadcast with sender tracking", receiver.Name())
		}
	}

	// Verify: Broadcast appears in search results with sender
	broadcasts, _ := db.SearchBroadcasts("integration test", 10)
	if len(broadcasts) == 0 {
		t.Fatal("broadcast not found in search")
	}
	if broadcasts[0].SenderID != sender.ID() {
		t.Errorf("broadcast sender_id: got %q, want %q", broadcasts[0].SenderID, sender.ID())
	}
}
```

### Step 2: Run integration test

```bash
go test ./internal/integration -run TestBroadcastSenderTracking -v
```

Expected: Test passes

### Step 3: Commit integration test

```bash
git add internal/integration/broadcast_sender_test.go
git commit -m "test(integration): add end-to-end broadcast sender tracking test"
```

---

## Task 12: Update Documentation

**Files:**
- Modify: `documentation/KNOWN_ISSUES.md`
- Modify: `AGENTS.md` (if broadcast terminology needs update)

### Step 1: Mark issue as resolved in KNOWN_ISSUES.md

Update the first high-priority item:

```markdown
## Recently Resolved

- [x] **Track broadcast sender and suppress self-response** (2026-02-05) - Added sender_id to memories table (schema v8), updated Commander to filter sender from recipients, updated TUI focus view to show "SWARM:" for others' broadcasts and "YOU (BROADCAST):" for own. Prevents conversation loops and fixes label ambiguity.

[move the rest of the old entry to this resolved section]
```

### Step 2: Update AGENTS.md if needed

Check if Terminology section mentions broadcasts. Update if necessary:

```markdown
- **Broadcast**: A message sent to all running myses except the sender. Stored with sender_id for tracking.
```

### Step 3: Commit documentation updates

```bash
git add documentation/KNOWN_ISSUES.md AGENTS.md
git commit -m "docs: mark broadcast sender tracking as resolved"
```

---

## Task 13: Database Migration

**Files:**
- User action required

### Step 1: Document migration steps

Add to commit message or migration guide:

```
Schema v7 → v8 Migration:

BREAKING CHANGE: Requires fresh database.

Steps:
1. make db-reset-accounts  # Exports accounts, wipes DB, recreates schema, reimports
2. Restart application
3. Verify schema version: sqlite3 ~/.zoea-nova/zoea.db "PRAGMA user_version;"
   Expected: 8
```

### Step 2: Verify migration target exists

```bash
grep "db-reset-accounts" Makefile
```

Expected: Target exists as documented in AGENTS.md

### Step 3: User performs migration (not automated)

**Manual step - document only:**

```bash
make db-reset-accounts
```

---

## Task 14: Verification & Smoke Testing

**Files:**
- None (verification steps)

### Step 1: Build application

```bash
make build
```

Expected: No errors

### Step 2: Run full test suite

```bash
make test
```

Expected: All tests pass with >80% coverage

### Step 3: Run application with debug logging

```bash
./bin/zoea -debug
```

### Step 4: Manual verification checklist

1. **Create 2 myses** (n key twice)
2. **Start both** (select + r key)
3. **Focus on mysis 1** (select + Enter)
4. **Send broadcast from mysis 1** (m key, type message)
5. **Verify mysis 1's focus view shows "YOU (BROADCAST):"**
6. **Switch to mysis 2** (Esc, select mysis 2, Enter)
7. **Verify mysis 2's focus view shows "SWARM:" for mysis 1's broadcast**
8. **Verify mysis 2 didn't loop responding to its own broadcasts**
9. **Check logs** for any errors: `tail -f ~/.zoea-nova/zoea.log`

### Step 5: Verification complete

If all checks pass, feature is complete.

---

## Testing Strategy

### Unit Tests
- `internal/store`: Memory storage with sender_id
- `internal/core`: Commander filtering, Mysis SendMessageFrom
- `internal/tui`: Focus view label rendering

### Integration Tests
- End-to-end broadcast flow with sender tracking
- Tool handler receiving caller context
- Database query methods returning sender info

### Manual Tests
- TUI visual verification of labels
- No self-response loops
- Broadcast search includes sender

---

## Rollback Plan

If issues arise:

1. **Revert commits** in reverse order from Task 14 → Task 1
2. **Restore database** from backup (if accounts export succeeded)
3. **Rebuild** application at previous version

---

## Success Criteria

- [ ] Database schema v8 with sender_id column
- [ ] Store methods retrieve and store sender_id
- [ ] Commander filters sender from broadcast recipients
- [ ] MCP tool handlers receive caller context
- [ ] zoea_broadcast excludes sender from delivery
- [ ] TUI focus view shows "SWARM:" for others' broadcasts
- [ ] TUI focus view shows "YOU (BROADCAST):" for own broadcasts
- [ ] No self-response conversation loops
- [ ] All tests pass with >80% coverage
- [ ] Manual verification complete

---

## Notes

- **Schema version bump**: v7 → v8 (breaking change)
- **Migration**: Requires `make db-reset-accounts`
- **No backward compatibility**: Old databases cannot be read
- **Style color**: Orange (#FFA500) for SWARM prefix
- **Legacy handling**: Broadcasts with empty sender_id show as "SWARM:"
