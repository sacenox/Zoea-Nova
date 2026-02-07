# Context Compaction & Search Reinforcement Plan

**Date:** 2026-02-04
**Status:** Pending Implementation
**Target Issue:** "Prefer recent state by compacting repeated snapshots; reinforce search tool use for older memory; consider longer tool loops."

## 1. Goal
Optimize the LLM context window by collapsing redundant tool results (snapshots) and ensuring the model is prompted to use search tools for historical data retrieval.

## 2. Proposed Changes

### Core Logic (`internal/core/mysis.go`)
- **Identify Snapshot Tools:** Define a list of tools that return state snapshots: `get_ship`, `get_system`, `get_poi`, `get_nearby`, `get_cargo`, `zoea_swarm_status`.
- **Compaction Algorithm:**
    - Update `getContextMemories()` to process the `MaxContextMessages` (20) window.
    - Track the most recent result for each snapshot tool.
    - If multiple results for the same tool exist in the window, keep only the latest one.
    - Ensure chronological order is maintained for non-snapshot messages (user/assistant/other tools).
    - Ensure the system prompt remains pinned at the top.
- **Tool Loop Tuning:** Review `MaxToolIterations` (currently 10). Compaction might free up context, allowing more tool steps in a single turn if needed.

### Prompt Updates (`internal/core/mysis.go`)
- **SystemPrompt:** Add explicit instructions on when and how to use `zoea_search_messages` and `zoea_search_reasoning` to recover context removed by compaction.
- **ContinuePrompt:** Append a reminder about the search tools: *"If you need past data, use zoea_search tools."*

## 3. Testing & Verification

### Unit Tests (`internal/core/mysis_test.go`)
- **`TestMysisContextCompaction` (New):**
    - Inject 15 `get_ship` tool results into a Mysis memory.
    - Verify `getContextMemories()` returns only the latest `get_ship` result.
    - Verify system prompt is still present.
    - Verify non-snapshot messages are unaffected.
- **Prompt Verification:** Ensure `TestSystemPromptContainsCaptainsLogExamples` and similar tests are updated/expanded to verify the new search guidance.

### Manual Verification
- Run `make test` to ensure no regressions in existing lifecycle or tool logic.
- Run `make build` to verify compilation.

## 4. Documentation
- Update `documentation/KNOWN_ISSUES.md` to mark the item as completed.
- Update `documentation/CONTEXT_COMPRESSION.md` to describe the new compaction behavior.
