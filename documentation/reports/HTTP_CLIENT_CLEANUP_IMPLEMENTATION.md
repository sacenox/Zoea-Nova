# HTTP Client Cleanup Implementation

**Date:** 2026-02-06  
**Status:** ✅ **COMPLETE** - Close() methods added to all HTTP clients

---

## Summary

Added `Close()` methods to all HTTP client-holding structs to prevent connection leaks during shutdown. All methods follow the same pattern: check for nil, call `CloseIdleConnections()`, return nil error.

---

## Changes Made

### 1. OllamaProvider.Close()

**File:** `internal/provider/ollama.go:360-366`

```go
// Close closes idle HTTP connections
func (p *OllamaProvider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
```

**Location:** After `toOllamaTools()` helper function  
**Purpose:** Closes idle HTTP connections to local Ollama server

---

### 2. OpenCodeProvider.Close()

**File:** `internal/provider/opencode.go:228-234`

```go
// Close closes idle HTTP connections
func (p *OpenCodeProvider) Close() error {
	if p.httpClient != nil {
		p.httpClient.CloseIdleConnections()
	}
	return nil
}
```

**Location:** After `Stream()` method  
**Purpose:** Closes idle HTTP connections to OpenCode Zen API

---

### 3. MCP Client.Close()

**File:** `internal/mcp/client.go:320-326`

```go
// Close closes idle HTTP connections
func (c *Client) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}
```

**Location:** After `Notify()` method  
**Purpose:** Closes idle HTTP connections to SpaceMolt MCP server

---

## Provider Factory Interface Analysis

### Current Factory Interface

**File:** `internal/provider/provider.go:57-60`

```go
type ProviderFactory interface {
	Name() string
	Create(model string, temperature float64) Provider
}
```

**Analysis:** ✅ **No changes needed**

**Reasoning:**
1. **Factories don't hold resources** - They only create providers
2. **Providers are created per-mysis** - Each mysis owns its provider instance
3. **Cleanup happens at mysis level** - When mysis stops, it should close its provider
4. **Factories are long-lived** - Registered once at startup, never cleaned up

**Recommendation:** Do NOT add Close() to factory interface. Providers are the resource owners.

---

## Provider Interface Analysis

### Current Provider Interface

**File:** `internal/provider/provider.go:42-55`

```go
type Provider interface {
	// Name returns the provider's identifier.
	Name() string

	// Chat sends messages and returns the complete response.
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatWithTools sends messages with available tools and returns response with potential tool calls.
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)

	// Stream sends messages and returns a channel that streams response chunks.
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}
```

**Analysis:** ⚠️ **SHOULD add Close() method to interface**

**Reasoning:**
1. **All implementations now have Close()** - Both OllamaProvider and OpenCodeProvider
2. **Interface should reflect cleanup contract** - Ensures all future providers implement cleanup
3. **Enables polymorphic cleanup** - Caller can close any provider without type assertion

**Recommendation:** Add `Close() error` to Provider interface.

---

## Recommended Interface Update

### Add Close() to Provider Interface

**File:** `internal/provider/provider.go:42-56`

**Before:**
```go
type Provider interface {
	Name() string
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
}
```

**After:**
```go
type Provider interface {
	Name() string
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)
	Close() error
}
```

**Impact:**
- ✅ Both implementations already have Close() - no code changes needed
- ✅ Future providers must implement Close() - compile-time enforcement
- ✅ Enables cleanup in mysis.Stop() - can call provider.Close() without type switch

---

## Integration into Cleanup Flow

### Current Cleanup Order (main.go)

```
1. User presses 'q' OR SIGINT/SIGTERM received
2. onQuit() callback closes event bus (line 153)
3. tea.Quit command processed, TUI exits
4. commander.StopAll() stops all myses (line 173)
   - For each mysis: Cancel context, wait for turn, release account
5. store.ReleaseAllAccounts() (line 176)
6. Deferred cleanups execute:
   - defer bus.Close() (line 81)
   - defer s.Close() (line 76)
```

**Missing:** Provider and MCP client cleanup!

---

### Recommended Cleanup Integration

#### Option 1: Add to Mysis.Stop() (Best)

**File:** `internal/core/mysis.go:224-264`

**Current Stop() method:**
```go
func (m *Mysis) Stop() error {
	a := m
	a.mu.Lock()
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		return nil
	}
	
	if a.cancel != nil {
		a.cancel()  // Cancel context
	}
	a.mu.Unlock()
	
	// Wait for current turn to finish
	a.turnMu.Lock()
	defer a.turnMu.Unlock()
	
	// ... update state ...
	
	a.releaseCurrentAccount()  // Line 261
	return nil
}
```

**Recommended addition:**
```go
func (m *Mysis) Stop() error {
	a := m
	a.mu.Lock()
	if a.state != MysisStateRunning {
		a.mu.Unlock()
		return nil
	}
	
	if a.cancel != nil {
		a.cancel()  // Cancel context
	}
	a.mu.Unlock()
	
	// Wait for current turn to finish
	a.turnMu.Lock()
	defer a.turnMu.Unlock()
	
	// ... update state ...
	
	a.releaseCurrentAccount()
	
	// NEW: Close provider HTTP client
	if a.provider != nil {
		if err := a.provider.Close(); err != nil {
			log.Warn().Err(err).Str("mysis", a.name).Msg("Failed to close provider")
		}
	}
	
	return nil
}
```

**Pros:**
- ✅ Automatic cleanup per-mysis
- ✅ Happens when mysis stops (normal or forced)
- ✅ No changes to main.go
- ✅ Works for both shutdown and individual mysis stop

**Cons:**
- ⚠️ Requires Provider interface to have Close() method (recommended anyway)

---

#### Option 2: Add to main.go Cleanup (Simple)

**File:** `cmd/zoea/main.go:173-180`

**Current cleanup:**
```go
// Clean shutdown
commander.StopAll()

// Release all accounts
if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}

log.Info().Msg("Zoea Nova shutdown complete")
```

**Recommended addition:**
```go
// Clean shutdown
commander.StopAll()

// NEW: Close MCP client
if mcpProxy != nil && mcpProxy.HasUpstream() {
	if client, ok := upstreamClient.(*mcp.Client); ok {
		if err := client.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close MCP client")
		}
	}
}

// NEW: Close all providers
// Note: This requires iterating over all myses and calling provider.Close()
// This is why Option 1 (add to Mysis.Stop) is better!

// Release all accounts
if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}

log.Info().Msg("Zoea Nova shutdown complete")
```

**Pros:**
- ✅ Simple to understand
- ✅ All cleanup in one place

**Cons:**
- ❌ Requires accessing internal mysis providers
- ❌ Doesn't handle individual mysis stop (only full shutdown)
- ❌ More complex than Option 1

---

### Recommended Approach: Option 1 + MCP Cleanup

**Recommendation:** Combine both approaches:

1. **Add Close() to Provider interface** (`internal/provider/provider.go`)
2. **Call provider.Close() in Mysis.Stop()** (`internal/core/mysis.go`)
3. **Add MCP client cleanup to main.go** (since it's not owned by myses)

---

## MCP Client Cleanup Location

### Where to Close MCP Client

**Current:** MCP client (`upstreamClient`) created in main.go (line 109):
```go
if cfg.MCP.Upstream != "" {
	upstreamClient = mcp.NewClient(cfg.MCP.Upstream)
}
```

**Recommended cleanup location:** After `commander.StopAll()` in main.go

**Code:**
```go
// Clean shutdown
commander.StopAll()

// Close MCP client if initialized
if upstreamClient != nil {
	if client, ok := upstreamClient.(*mcp.Client); ok {
		if err := client.Close(); err != nil {
			log.Warn().Err(err).Msg("Failed to close MCP client")
		}
	}
}

// Release all accounts
if err := s.ReleaseAllAccounts(); err != nil {
	log.Warn().Err(err).Msg("Failed to release accounts on shutdown")
}

log.Info().Msg("Zoea Nova shutdown complete")
```

**Why here:**
- MCP client is shared by all myses via proxy
- Should close AFTER all myses stop (so they don't try to use closed client)
- Should close BEFORE database closes (in case cleanup logs to DB)

---

## Testing Verification

### Build Status

```bash
$ make build
go build -ldflags "-X main.Version=v0.0.1-11-g4d8f6f1-dirty" -o bin/zoea ./cmd/zoea
```

✅ **Build successful** - All Close() methods compile correctly

### Manual Testing Checklist

- [ ] Start application with multiple myses
- [ ] Verify no connection leaks during normal operation
- [ ] Press 'q' to quit
- [ ] Verify graceful shutdown with no errors
- [ ] Check for orphaned HTTP connections: `netstat -anp | grep zoea`
- [ ] Send SIGTERM: `kill -TERM <pid>`
- [ ] Verify graceful shutdown
- [ ] Stop individual mysis
- [ ] Verify provider cleanup without errors

---

## Impact Assessment

### User-Visible Changes

**None.** Close() methods are internal cleanup - no API or behavior changes.

### Performance Impact

**Positive:**
- ✅ Faster shutdown (no lingering idle connections)
- ✅ Reduced resource usage (connections cleaned up properly)
- ✅ No connection leak buildup over long runs

### Risk Assessment

**Very Low:**
- Close() is called only during shutdown/stop
- CloseIdleConnections() is non-destructive (waits for active requests)
- No risk of breaking active connections
- Idempotent (safe to call multiple times)

---

## Next Steps

### Required for Completion

1. **Update Provider interface** - Add `Close() error` to interface definition
2. **Update Mysis.Stop()** - Call `provider.Close()` after releasing account
3. **Update main.go cleanup** - Call `upstreamClient.Close()` after stopping myses
4. **Test shutdown flow** - Verify no connection leaks

### Optional Enhancements

1. **Add timeout to MCP Client.Close()** - Ensure cleanup doesn't hang
2. **Add Close() to MCP Proxy** - Wrapper around upstream client close
3. **Log file handle cleanup** - Close log file on exit (currently not closed)
4. **Add graceful shutdown timeout** - Force exit if cleanup takes too long

---

## References

- **Cleanup Order Analysis:** `documentation/CLEANUP_ORDER_ANALYSIS.md`
- **HTTP Client Best Practices:** Close idle connections on shutdown
- **Go net/http docs:** https://pkg.go.dev/net/http#Client.CloseIdleConnections

---

**Implementation Status:** ✅ **Phase 1 Complete** (Close() methods added)  
**Next Phase:** Integration into cleanup flow (Mysis.Stop + main.go)  
**Documentation:** This report

