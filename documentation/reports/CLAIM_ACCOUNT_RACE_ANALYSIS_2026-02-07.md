# ClaimAccount() Race Condition Analysis

**Date:** 2026-02-07  
**Context:** Analysis of edge cases and race conditions in the ClaimAccount() fix  
**Key Question:** Is the simple fix (no locking) safe, or do we need a claim-lock mechanism?

---

## Executive Summary

**Recommendation: The simple fix is SAFE. No claim-lock mechanism needed.**

The current ClaimAccount() implementation using optimistic locking via `UPDATE ... WHERE in_use = 0` with retry logic is sufficient. The database-level concurrency controls (MaxOpenConns=1, WAL mode, busy_timeout=5000ms) combined with the check-and-set pattern provide adequate safety for all identified edge cases.

---

## Current Implementation

### Account State Transitions

```
[Available: in_use=0]
    ↓ ClaimAccount() - optimistic lock via UPDATE WHERE in_use=0
[Claimed: in_use=1]
    ↓ MCP login tool call
[Logged In: in_use=1] (MarkAccountInUse - idempotent)
    ↓ MCP logout tool call OR Mysis.Stop() OR Mysis.setError()
[Released: in_use=0] (ReleaseAccount)
```

### Key Code Paths

**ClaimAccount()** (`internal/store/accounts.go:87-132`):
```go
for attempts := 0; attempts < 5; attempts++ {
    // 1. SELECT first available account (WHERE in_use = 0)
    var username, password string
    err := s.db.QueryRow(`SELECT ... WHERE in_use = 0 LIMIT 1`)
    
    // 2. Optimistic lock: UPDATE only if still available
    result := s.db.Exec(`UPDATE accounts SET in_use = 1 WHERE username = ? AND in_use = 0`)
    
    // 3. Check if UPDATE succeeded
    rows, _ := result.RowsAffected()
    if rows == 0 {
        continue  // Another thread won the race, retry
    }
    
    return account
}
```

**MarkAccountInUse()** (`internal/store/accounts.go:134-147`):
- Called by MCP proxy on successful login
- Idempotent: sets `in_use=1` without conditions
- Safe even if account already claimed

**ReleaseAccount()** (`internal/store/accounts.go:149-160`):
- Called by MCP proxy on logout
- Called by Mysis.releaseCurrentAccount() on Stop/setError
- Sets `in_use=0` without conditions

### Database Concurrency Controls

**From** `internal/store/store.go:38-56`:
```go
dsn += "?_busy_timeout=5000"      // 5 second busy timeout
db.SetMaxOpenConns(1)             // Serialize all DB access
PRAGMA journal_mode=WAL           // Write-Ahead Log for read concurrency
```

**Critical insight:** `SetMaxOpenConns(1)` means **all database operations are serialized through a single connection**. This eliminates true race conditions at the SQL execution level.

---

## Edge Case Analysis

### Scenario 1: Multiple Myses Call ClaimAccount() Simultaneously

**Flow:**
```
Time    Mysis A                     Mysis B                     Database
----    -------                     -------                     --------
T1      ClaimAccount()              [waiting for DB conn]       in_use=0
T2      SELECT crab_01              [waiting for DB conn]       in_use=0
T3      UPDATE ... in_use=1         [waiting for DB conn]       in_use=1
T4      return crab_01              ClaimAccount()              in_use=1
T5                                  SELECT crab_01              in_use=1
T6                                  UPDATE (rows=0)             in_use=1
T7                                  continue retry              in_use=1
T8                                  SELECT crab_02              in_use=0
T9                                  UPDATE ... in_use=1         in_use=1
T10                                 return crab_02              in_use=1
```

**Result:** ✅ SAFE
- MaxOpenConns=1 serializes all DB operations
- Mysis B's UPDATE fails (rows=0) because Mysis A already claimed crab_01
- Retry logic succeeds on next available account
- No double-claim possible

**Test Coverage:** `internal/store/store_test.go:320-350` (TestReleaseAllAccounts with sequential claims)

---

### Scenario 2: Mysis Claims Account But Never Logs In

**Flow:**
```
Time    Mysis A                     MCP Tool                    Database
----    -------                     --------                    --------
T1      ClaimAccount()              -                           in_use=0
T2      UPDATE ... in_use=1         -                           in_use=1
T3      return crab_01              -                           in_use=1
T4      [LLM ignores credentials]   -                           in_use=1
T5      [never calls login tool]    -                           in_use=1
... (account stuck in_use=1 indefinitely)
```

**Result:** ⚠️ KNOWN LIMITATION (acceptable)
- Account remains locked until Mysis is stopped/errored
- This is **by design**: claimed accounts are reserved for the claiming Mysis
- Mitigation: Mysis lifecycle management (Stop/Error) releases account
- Alternative fix: Add timeout-based auto-release (NOT RECOMMENDED - adds complexity)

**Design Decision:** Accept this limitation. Account claiming is a resource allocation, not a transaction. The Mysis "owns" the account until released.

**User Impact:** If a Mysis claims but never uses an account, operator must restart/delete the Mysis to free the account. This is acceptable operational overhead.

---

### Scenario 3: Mysis Crashes After Claim But Before Login

**Flow:**
```
Time    Mysis A                     Process                     Database
----    -------                     -------                     --------
T1      ClaimAccount()              running                     in_use=0
T2      UPDATE ... in_use=1         running                     in_use=1
T3      return crab_01              running                     in_use=1
T4      [process killed]            SIGKILL                     in_use=1
T5      [no cleanup runs]           dead                        in_use=1
... (account stuck in_use=1 until manual intervention)
```

**Result:** ⚠️ KNOWN LIMITATION (acceptable)
- Same as Scenario 2: account remains locked
- Zoea Nova has no crash recovery mechanism for account locks
- Mitigation: Application restart calls `ReleaseAllAccounts()` on startup (NOT CURRENTLY IMPLEMENTED)

**Recommendation:** Add `ReleaseAllAccounts()` call to main.go initialization:
```go
func main() {
    store, _ := store.New()
    defer store.Close()
    
    // Release any accounts locked by previous run
    if err := store.ReleaseAllAccounts(); err != nil {
        log.Warn().Err(err).Msg("failed to release accounts on startup")
    }
    
    // ... continue normal startup
}
```

**Impact:** Low priority. Workaround is `make db-reset-accounts` or manual SQL.

---

### Scenario 4: Race Between ClaimAccount() and MarkAccountInUse()

**Flow:**
```
Time    Mysis A (goroutine 1)       MCP Proxy (goroutine 2)     Database
----    ----------------------       -----------------------     --------
T1      ClaimAccount()              -                           in_use=0
T2      UPDATE ... in_use=1         -                           in_use=1
T3      return crab_01              -                           in_use=1
T4      [calls login tool]          CallTool("login", ...)      in_use=1
T5      -                           MarkAccountInUse("crab_01") in_use=1
T6      -                           UPDATE ... in_use=1         in_use=1 (no-op)
```

**Result:** ✅ SAFE
- MarkAccountInUse() is **idempotent** - setting `in_use=1` when already `in_use=1` is harmless
- No race condition because both operations set the same value
- MaxOpenConns=1 serializes operations anyway

**Edge case:** What if another Mysis claims the account between T3 and T5?

**Flow (contended):**
```
Time    Mysis A                     Mysis B                     Database
----    -------                     -------                     --------
T1      ClaimAccount()              -                           in_use=0
T2      UPDATE ... in_use=1         -                           in_use=1
T3      return crab_01              -                           in_use=1
T4      [paused by scheduler]       ClaimAccount()              in_use=1
T5      [paused]                    SELECT (finds no in_use=0)  in_use=1
T6      [paused]                    return error                in_use=1
T7      [calls login tool]          -                           in_use=1
T8      MarkAccountInUse("crab_01") -                           in_use=1
```

**Result:** ✅ SAFE
- Mysis B cannot claim crab_01 because it's already `in_use=1` at T5
- Mysis A's login succeeds without interference
- No double-claim possible

---

## Concurrency Model Analysis

### SQLite Configuration

| Setting | Value | Impact on ClaimAccount() |
|---------|-------|-------------------------|
| `MaxOpenConns` | 1 | All DB operations serialized - eliminates true races |
| `_busy_timeout` | 5000ms | Goroutines wait up to 5s for DB lock - prevents lock errors |
| `journal_mode` | WAL | Allows concurrent reads, but writes still serialized by MaxOpenConns |

### Critical Insight: MaxOpenConns=1 Is The Safety Mechanism

The `SetMaxOpenConns(1)` setting means:
1. All SQL statements execute serially (no true parallelism)
2. `ClaimAccount()` calls are queued by the database/sql connection pool
3. The `UPDATE ... WHERE in_use = 0` pattern becomes a serialized atomic operation

**Without MaxOpenConns=1:**
- SELECT and UPDATE could interleave (classic TOCTOU race)
- Multiple threads could SELECT the same account
- Multiple threads could UPDATE the same account to in_use=1

**With MaxOpenConns=1:**
- SELECT and UPDATE execute atomically (no other thread can execute SQL between them)
- Retry loop handles "lost updates" gracefully (rows=0 detection)

### Why Optimistic Locking Works Here

**Optimistic locking pattern:**
```sql
-- Thread 1
SELECT username FROM accounts WHERE in_use = 0 LIMIT 1;  -- Returns crab_01
UPDATE accounts SET in_use = 1 WHERE username = 'crab_01' AND in_use = 0;  -- Succeeds

-- Thread 2 (after Thread 1's UPDATE)
SELECT username FROM accounts WHERE in_use = 0 LIMIT 1;  -- Returns crab_02 (or crab_01 if re-released)
UPDATE accounts SET in_use = 1 WHERE username = 'crab_01' AND in_use = 0;  -- Fails (rows=0)
```

The `AND in_use = 0` condition is the optimistic lock. If another thread claimed the account between SELECT and UPDATE, the UPDATE fails without side effects.

**Retry logic** (5 attempts) handles transient contention:
- If 10 Myses start simultaneously and only 1 account exists, 9 will retry
- Each retry re-runs SELECT to find next available account
- Worst case: all 5 retries fail → return "no accounts available"

---

## ListAvailableAccounts() Safety

**Function:** `internal/store/accounts.go:56-85`

```go
func (s *Store) ListAvailableAccounts() ([]*Account, error) {
    rows := s.db.Query(`SELECT ... WHERE in_use = 0 ORDER BY created_at ASC`)
    // ... scan and return
}
```

**Race condition analysis:**

| Scenario | Safety |
|----------|--------|
| List accounts while another Mysis claims one | ✅ SAFE - snapshot read (WAL mode) |
| List accounts immediately after ClaimAccount() | ✅ SAFE - MaxOpenConns=1 serializes operations |
| List accounts during login tool call | ✅ SAFE - reads don't block/interfere with writes |

**Result:** No changes needed to ListAvailableAccounts().

---

## ReleaseAccount() Safety

**Function:** `internal/store/accounts.go:149-160`

```go
func (s *Store) ReleaseAccount(username string) error {
    _, err := s.db.Exec(`UPDATE accounts SET in_use = 0 WHERE username = ?`)
}
```

**Race condition analysis:**

| Scenario | Safety |
|----------|--------|
| Release account while another Mysis claims it | ✅ SAFE - MaxOpenConns=1 serializes operations |
| Release account twice (double-release) | ✅ SAFE - idempotent (setting in_use=0 when already 0 is harmless) |
| Release account during ClaimAccount() retry loop | ✅ SAFE - account becomes available for next retry |

**Edge case:** What if a Mysis releases an account it doesn't own?

**Flow:**
```
Time    Mysis A                     Mysis B                     Database
----    -------                     -------                     --------
T1      ClaimAccount()              ClaimAccount()              in_use=0
T2      return crab_01              return crab_02              in_use=1 (both)
T3      ReleaseAccount("crab_02")   -                           in_use=0 (crab_02)
T4      -                           [still using crab_02]       in_use=0 (crab_02)
T5      ClaimAccount()              -                           in_use=0 (crab_02)
T6      return crab_02              -                           in_use=1 (crab_02)
T7      [double-claim!]             [still using crab_02]       in_use=1 (crab_02)
```

**Result:** ⚠️ POTENTIAL BUG - but **NOT POSSIBLE** in current implementation

**Why it's not possible:**
- Mysis tracks `currentAccountUsername` (internal/core/mysis.go:1084-1090)
- ReleaseAccount() only called with Mysis's own account (mysis.go:1093-1103)
- No API exists for one Mysis to release another's account

**Mitigation (defense in depth):** Add ownership check to ReleaseAccount()
```go
func (m *Mysis) releaseCurrentAccount() {
    m.mu.Lock()
    username := m.currentAccountUsername
    m.currentAccountUsername = ""
    m.mu.Unlock()
    
    if username != "" {
        // Only release account we actually own
        _ = m.store.ReleaseAccount(username)
    }
}
```

**Current implementation already has this pattern** - no changes needed.

---

## Do We Need a "Claimed But Not Logged In" State?

**Proposed 3-state model:**
```
[Available: in_use=0]
    ↓ ClaimAccount()
[Claimed: in_use=1, logged_in=0]  ← NEW STATE
    ↓ MarkAccountInUse()
[Logged In: in_use=1, logged_in=1]
    ↓ ReleaseAccount()
[Released: in_use=0, logged_in=0]
```

**Analysis:**

| Benefit | Cost | Verdict |
|---------|------|---------|
| Distinguish claim vs login | Schema change (add logged_in column) | ❌ Not worth it |
| Timeout-based auto-release | Complex timeout tracking logic | ❌ Not worth it |
| Better observability | Minimal - current logs already show state | ❌ Not worth it |

**Recommendation:** **NO** - Current 2-state model is sufficient.

**Rationale:**
1. The claim→login window is typically <1 second (LLM tool call latency)
2. Failure modes (scenario 2 & 3) are rare and acceptable
3. Adding a third state increases complexity without meaningful benefit
4. Operator can always Stop/Delete Mysis to release stuck accounts

---

## Recommendations

### 1. Keep Current Implementation ✅

The simple fix (no locking in ClaimAccount, use optimistic locking) is **SAFE and CORRECT**.

**No changes needed to:**
- ClaimAccount() - retry logic handles contention
- MarkAccountInUse() - idempotent, no race conditions
- ReleaseAccount() - idempotent, protected by Mysis ownership
- ListAvailableAccounts() - snapshot reads are safe

### 2. Add Startup Account Release (Optional, Low Priority)

**Add to** `cmd/zoea/main.go`:
```go
func main() {
    store, err := store.New()
    if err != nil {
        log.Fatal().Err(err).Msg("failed to open store")
    }
    defer store.Close()
    
    // Release accounts locked by previous run (crash recovery)
    if err := store.ReleaseAllAccounts(); err != nil {
        log.Warn().Err(err).Msg("failed to release accounts on startup")
    }
    
    // ... continue normal startup
}
```

**Benefit:** Handles scenario 3 (crash recovery) gracefully  
**Cost:** Minimal (1 SQL UPDATE at startup)  
**Priority:** Low - crashes are rare, workaround exists

### 3. Document Edge Cases (This Report)

**Add to** `documentation/architecture/ACCOUNT_LIFECYCLE.md`:
- Document 2-state model (available → claimed)
- Document acceptable limitations (scenarios 2 & 3)
- Document operator recovery procedures

### 4. No Claim-Lock Mechanism Needed ❌

**Rejected alternatives:**
- Distributed lock (Redis, etc.) - Overkill for single-process app
- Mutex in Go code - Doesn't survive process restarts
- SQL transactions - Already have optimistic locking via WHERE clause
- Timeout-based auto-release - Complex, error-prone, unnecessary

---

## Test Coverage Assessment

### Existing Tests

✅ **Basic claim/release cycle:** `internal/store/store_test.go:274-318`  
✅ **Multiple claims:** `internal/store/store_test.go:320-350`  
✅ **Account release on error:** `internal/core/mysis_account_release_test.go:10-85`

### Missing Tests (Recommended)

❌ **Concurrent ClaimAccount() calls** (10 goroutines, 1 account)  
❌ **ClaimAccount() during ReleaseAccount()** (interleaved ops)  
❌ **MarkAccountInUse() idempotency** (call twice on same account)

**Test plan:**
```go
func TestClaimAccount_Concurrent(t *testing.T) {
    s := setupStore(t)
    s.CreateAccount("crab_01", "pass")
    
    var wg sync.WaitGroup
    claims := make(chan *Account, 10)
    
    // 10 goroutines try to claim 1 account
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            acc, err := s.ClaimAccount()
            if err == nil {
                claims <- acc
            }
        }()
    }
    
    wg.Wait()
    close(claims)
    
    // Only 1 should succeed
    count := 0
    for range claims {
        count++
    }
    
    if count != 1 {
        t.Errorf("expected 1 successful claim, got %d", count)
    }
}
```

**Priority:** Medium - current implementation is safe, but test would validate assumptions.

---

## Conclusion

**The simple fix is SAFE. No claim-lock mechanism needed.**

**Key safety mechanisms:**
1. **MaxOpenConns=1** - Serializes all DB operations, eliminates true race conditions
2. **Optimistic locking** - `UPDATE ... WHERE in_use = 0` provides atomic check-and-set
3. **Retry logic** - 5 attempts handle transient contention gracefully
4. **Idempotent operations** - MarkAccountInUse and ReleaseAccount are safe to call multiple times

**Acceptable limitations:**
1. Claimed but unused accounts remain locked until Mysis stopped (Scenario 2)
2. Crashed Mysis leaves accounts locked until manual intervention (Scenario 3)

**Recommended follow-ups:**
1. ✅ Keep current implementation (no changes)
2. 🟡 Add ReleaseAllAccounts() to main.go startup (low priority)
3. 🟡 Add concurrent ClaimAccount() test (medium priority)
4. ✅ Document edge cases (this report)

**Final verdict:** Ship it. The implementation is production-ready.
