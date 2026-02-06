# Restart Bug Investigation - Document Index

## Quick Start

**Want to understand the bug?**  
→ Read `RESTART_BUG_FINAL_SUMMARY.md` (5 min)

**Want to reproduce it?**  
→ Follow `RESTART_BUG_REPRODUCTION.md` (30 sec test)

**Want technical details?**  
→ Read `AGENT_10_INTEGRATION_FINDINGS.md` (complete analysis)

**Want visual explanation?**  
→ See `RACE_CONDITION_DIAGRAM.md` (timelines)

**Ready to fix?**  
→ Apply 5-line fix from `RESTART_BUG_FINAL_SUMMARY.md`

## All Documents

### 1. RESTART_BUG_FINAL_SUMMARY.md
**Purpose**: Executive summary and quick reference  
**Audience**: Anyone  
**Content**:
- The bug in 3 sentences
- The fix in 5 lines
- Quick reproduction (30 sec)
- Verification steps
- Impact analysis

**Read this first!**

---

### 2. RESTART_BUG_REPRODUCTION.md
**Purpose**: Detailed reproduction guide  
**Audience**: Developers, testers  
**Content**:
- 3 test scenarios (LLM call, tool call, stress test)
- Step-by-step TUI instructions
- Instrumentation patch application
- Log analysis commands
- Expected findings

**Use this to reproduce and verify fix.**

---

### 3. AGENT_10_INTEGRATION_FINDINGS.md
**Purpose**: Complete technical analysis  
**Audience**: Developers  
**Content**:
- Root cause with code locations (file:line)
- Race condition timeline (both scenarios)
- Fix recommendation with rationale
- Alternative solutions considered
- Edge case analysis
- Verification checklist
- Confidence levels

**Use this for deep understanding.**

---

### 4. RACE_CONDITION_DIAGRAM.md
**Purpose**: Visual explanation  
**Audience**: Visual learners  
**Content**:
- ASCII timeline diagrams
- Thread 1 vs Thread 2 execution
- Timeout scenario
- No-timeout scenario
- With fix (correct behavior)
- State transition diagrams

**Use this to visualize the race.**

---

### 5. restart_debug.patch
**Purpose**: Instrumentation for verification  
**Audience**: Developers  
**Content**:
- 20+ debug log statements
- State transition tracking
- Lock acquisition logging
- Race detection warnings

**Apply this to see the race in action.**

---

### 6. RESTART_BUG_INDEX.md
**Purpose**: Navigation guide  
**Audience**: You  
**Content**: This file

---

## Information Flow

```
Quick Overview
    ↓
RESTART_BUG_FINAL_SUMMARY.md
    ↓
    ├──→ Want to reproduce?
    │        ↓
    │   RESTART_BUG_REPRODUCTION.md
    │        ↓
    │   restart_debug.patch
    │
    ├──→ Want technical details?
    │        ↓
    │   AGENT_10_INTEGRATION_FINDINGS.md
    │
    └──→ Want visual explanation?
             ↓
        RACE_CONDITION_DIAGRAM.md
```

## Background Documents

From previous agents:
- `/tmp/stop_bug_investigation.md` - Initial investigation plan
- `/tmp/stop_bug_root_cause.md` - Original root cause analysis

## The Fix (Quick Reference)

**File**: `internal/core/mysis.go`  
**Line**: 774 (in `setError()` method)  
**Change**: Add 5-line check

```go
if a.state == MysisStateStopped {
    a.mu.Unlock()
    log.Debug().Str("mysis", a.name).Err(err).
        Msg("Ignoring error - mysis was intentionally stopped")
    return
}
```

## Recommended Reading Order

1. **RESTART_BUG_FINAL_SUMMARY.md** - Get the overview (5 min)
2. **RACE_CONDITION_DIAGRAM.md** - Visualize the problem (5 min)
3. **RESTART_BUG_REPRODUCTION.md** - Test it yourself (30 sec)
4. **AGENT_10_INTEGRATION_FINDINGS.md** - Deep dive (optional)

## TL;DR

**Bug**: Stop() → errored state (wrong, should be stopped)  
**Cause**: Race between Stop() and setError()  
**Fix**: 5 lines in setError() to check for stopped state  
**Test**: 30 seconds with offline mode  
**Impact**: Clean shutdown semantics, better UX

## Questions?

All answers are in these documents:
- What is the bug? → FINAL_SUMMARY
- How do I reproduce? → REPRODUCTION
- Why does it happen? → INTEGRATION_FINDINGS
- How does the race work? → RACE_CONDITION_DIAGRAM
- What's the fix? → FINAL_SUMMARY
- How do I verify? → REPRODUCTION + restart_debug.patch
