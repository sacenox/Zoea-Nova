# Mysis Account Management

**Version:** 3.0  
**Date:** 2026-02-09  
**Status:** Truth Document

---

## Problem Statement

Myses need game accounts to play SpaceMolt. Each mysis gets one account permanently assigned to eliminate timing races and enable reliable re-login. We need this account management to avoid polluting the server with more test accounts.

**Constraint:** No changes to gameplay whatsoever. Myses must be able to call any game API freely.

---

## Truth Table

| Mysis Action | Mysis State | Pool State | System Behavior | Mysis Sees |
|--------------|-------------|------------|-----------------|------------|
| Calls `register(username, empire)` | No assigned account | Pool has available accounts | Proxy assigns pool account permanently, mimics MCP response with pool account details | `{result: {password, player_id}, session: {id, player_id, ...}}` (thinks it registered) |
| Calls `register(username, empire)` | No assigned account | Pool is empty | Proxy passes to game server, captures response, stores new account, assigns permanently | `{result: {password, player_id}, session: {...}}` (actually registered) |
| Calls `register(username, empire)` | Has assigned account | Any | Proxy returns error pointing to `login()` with assigned credentials | Error response |
| Calls `login(username, password)` | Has assigned account | Any | Proxy substitutes assigned account credentials, passes to game server | `{result: {player, ship, system, poi, ...}, session: {...}}` (logged into assigned account) |
| Calls `logout()` | Any | Any | Proxy passes to game server (account remains assigned) | Success |
| Lists tools | Any | Any | N/A - `zoea_claim_account` removed entirely | Only game tools visible |

---

## Behavior Rules

1. **Permanent assignment:** Once a mysis gets an account (via register), that account is permanently bound to `mysis_id` (stored in `assigned_to` field)
2. **First register - pool has accounts:** Proxy assigns pool account permanently, mimics game server `registered` response with pool account's password and player_id (never forwards to game server)
3. **First register - pool empty:** Proxy forwards register request to game server, captures response (password and player_id), stores new account in pool, assigns permanently to mysis
4. **Subsequent register:** When mysis with assigned account calls `register()`, proxy returns error: "Already have account. Use login(username, password)."
5. **Login substitution:** All `login()` calls from mysis with assigned account are silently substituted with assigned account credentials before passing to game server
6. **Account persistence:** Logout does NOT release account assignment - mysis keeps same account forever
7. **Account release:** Accounts return to pool ONLY when mysis is deleted/destroyed (assigned_to cleared)
8. **Internal claiming:** Account assignment is handled internally by proxy - no `zoea_claim_account` tool exists

---

## State Transitions

```
FIRST REGISTER - POOL HAS ACCOUNTS:
Mysis: register(username, empire) → Proxy: assign_account(pool) → Proxy: mimic_MCP_response({result: {password, player_id}, session: {...}}) → Mysis: REGISTERED
(Proxy never forwards to game server; mimics full MCP response structure with pool account credentials; account permanently bound to mysis_id)

FIRST REGISTER - POOL EMPTY:
Mysis: register(username, empire) → Game/MCP: {result: {password, player_id}, session: {...}} → Proxy: capture_response + store_account + assign_to_mysis → Mysis: REGISTERED
(Request forwarded to game server; new account created; response captured; account stored in pool and permanently bound to mysis_id)

SUBSEQUENT REGISTER:
Mysis: register() → Proxy: error_response → Mysis: ERROR
(Proxy returns MCP error: "Already have account. Use login(username, password).")

LOGIN WITH ASSIGNED ACCOUNT:
Mysis: login(any_user, any_pass) → Proxy: substitute(assigned_credentials) → Game/MCP: {result: {player, ship, system, poi, ...}, session: {...}} → Mysis: LOGGED_IN
(Always logs into assigned account regardless of provided credentials; full game state returned)

LOGOUT:
Mysis: logout() → Game/MCP: success → Mysis: LOGGED_OUT
(Account remains assigned to mysis_id for future login)

MYSIS DELETION:
Commander: delete_mysis() → Store: clear assigned_to → Pool gains available account
```

---

## Verification

| Scenario | Expected Outcome |
|----------|------------------|
| Pool has 3 accounts, new mysis calls register | Proxy mimics full MCP response with pool account's password/player_id/session (no game server call), account permanently assigned to mysis_id, pool has 2 available accounts |
| Pool empty, new mysis calls register | Request forwarded to game server, MCP response captured (password/player_id/session), new account stored in pool and permanently assigned to mysis_id |
| Mysis with assigned account calls register again | Mysis receives MCP error: "Already have account. Use login(username, password)." |
| Mysis with assigned account calls login(wrong_user, wrong_pass) | Proxy substitutes assigned credentials, mysis gets full MCP response with `{result: {player, ship, system, poi, ...}, session: {...}}` for assigned account |
| Mysis with assigned account calls logout | Mysis logged out, account remains assigned to mysis_id (can login again later) |
| Mysis is deleted | Account's assigned_to cleared, account returns to available pool |
| `make db-reset-accounts` run | All mysis assignments cleared (assigned_to = NULL), accounts return to pool, account details preserved |
| Mysis lists tools | Only game tools visible (no `zoea_claim_account` - tool removed) |

---

## Implementation Files

- `internal/mcp/proxy.go` - Register interception, permanent assignment, login credential substitution, tool filtering
- `internal/constants/constants.go` - System prompt (no auth instructions)
- `internal/store/accounts.go` - Account pool management with `assigned_to` field
- `Makefile` - `db-reset-accounts` target must clear `assigned_to` while preserving account details

---

## Out of Scope

- Review of other `zoea_*` tools (note: `zoea_claim_account` was removed entirely)
- Account sharing between myses (one account = one mysis permanently)
- Manual account reassignment (only via mysis deletion)
