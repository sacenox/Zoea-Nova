# Mysis Account Management

**Version:** 2.0  
**Date:** 2026-02-08  
**Status:** Truth Document

---

## Problem Statement

Myses need game accounts to play SpaceMolt. We maintain a pool of accounts to control access and prevent account exhaustion.

**Constraint:** No changes to gameplay whatsoever. Myses must be able to call any game API freely.

---

## Truth Table

| Mysis Action | Pool State | System Behavior | Mysis Sees |
|--------------|------------|-----------------|------------|
| Calls `register(username, empire)` | Pool has available accounts | Proxy logs in with pool account | `session_id` (thinks it registered) |
| Calls `register(username, empire)` | Pool is empty | Proxy passes to game server, stores new account | `session_id` (actually registered) |
| Calls `login(username, password)` | Any | Proxy passes to game server, marks account in_use | `session_id` or error |
| Calls `logout()` | Any | Proxy passes to game server, releases account | Success |
| Lists tools | Any | `zoea_claim_account` filtered out | Only game tools visible |

---

## Behavior Rules

1. **Register interception:** When mysis calls `register()` and pool has accounts, proxy substitutes a `login()` call with pool credentials
2. **Register passthrough:** When mysis calls `register()` and pool is empty, proxy lets it go to game server normally
3. **Login passthrough:** Login calls always go to game server (no interception)
4. **Account locking:** Successful login (real or substituted) marks account as in_use
5. **Account release:** Logout releases account back to pool
6. **Tool filtering:** `zoea_claim_account` hidden from tool list

---

## State Transitions

```
POOL HAS ACCOUNTS:
Mysis: register() → Proxy: login(pool_account) → Game: session_id → Mysis: PLAYING

POOL EMPTY:
Mysis: register() → Game: session_id → Proxy: store_account → Mysis: PLAYING

MYSIS HAS CREDENTIALS:
Mysis: login() → Game: session_id → Proxy: mark_in_use → Mysis: PLAYING
```

---

## Verification

| Scenario | Expected Outcome |
|----------|------------------|
| Pool has 3 accounts, mysis calls register | Mysis gets session_id, pool has 2 available accounts |
| Pool empty, mysis calls register | Mysis gets session_id, pool has 1 account (the new one) |
| Mysis calls login with pool credentials | Mysis gets session_id, account marked in_use |
| Mysis calls logout | Account released, pool gains 1 available account |
| Mysis lists tools | `zoea_claim_account` not in list |

---

## Implementation Files

- `internal/mcp/proxy.go` - Register interception, account pool claim, tool filtering
- `internal/constants/constants.go` - System prompt (no auth instructions)
- `internal/store/accounts.go` - Account pool management

---

## Out of Scope

- Token budget management
- Structural JSON compaction
- Tool call merging
- Review of other `zoea_*` tools
