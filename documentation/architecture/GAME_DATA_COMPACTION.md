# Game Data Compaction

## Problem Statement

SpaceMolt game tools return large JSON payloads that can exceed LLM context windows.

**Evidence (2026-02-05):**
- Ollama logs: `prompt=41611 tokens`, `limit=32768 tokens`
- Result: `context deadline exceeded` errors
- Cause: Tool results (notifications, ship logs, market data) accumulating in memory

**Critical Constraint:**
Myses MUST see current game state to function. Blind truncation breaks gameplay.

**Large Tool Results:**
1. `get_notifications` - Battle reports, trade offers, discoveries
2. `get_ship_logs` - Complete action history
3. `get_market_data` - Full market listings for all systems
4. `get_system` - Detailed system scan results
5. `captains_log_list` - All stored notes

## Current Compression (v0.5.0)

From `CONTEXT_COMPRESSION.md`:
- Turn-aware composition (historical vs current turn)
- Snapshot compaction (removes redundant `get_*` tool results)
- Orphaned tool call removal

These strategies reduce message count but don't address individual message size.
