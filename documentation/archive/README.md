# Archive

Historical investigations, analyses, and reports that have been **completed**, **superseded**, or are now **obsolete**.

---

## Purpose

This directory contains documentation that:
- Solved a problem (implementation complete)
- Led to a decision (recorded in reports/)
- Is no longer relevant (obsolete)
- Provides historical context for future reference

**Do NOT use as active reference** - See parent README.md for current documentation.

---

## Categories

### Superseded Investigations
Investigations that led to implementations (code now shipped).

- **AUTO_SCROLL_INVESTIGATION.md** → Replaced by reports/SMART_AUTO_SCROLL_IMPLEMENTATION.md
- **CLEANUP_ORDER_ANALYSIS.md** → Implemented in commit 4613e12
- **GOROUTINE_CLEANUP_ANALYSIS.md** → Implemented in commit 4613e12
- **LLM_THINKING_STATE_INVESTIGATION.md** → Implemented in reports/LLM_THINKING_STATE_IMPLEMENTATION.md
- **TUI_QUIT_HANDLING_ANALYSIS.md** → Fixed in commit series

### Fixed Overflow/Layout Issues
Tests and investigations for layout problems (all resolved).

- **FOOTER_OVERFLOW_TEST_REPORT.md** - Fixed
- **INPUT_OVERFLOW_TEST_REPORT.md** - Fixed
- **STATUSBAR_OVERFLOW_TEST_REPORT.md** - Fixed
- **VIEW_HEIGHT_OVERFLOW_ANALYSIS.md** - Fixed

### Historical Reports
Early development phase reports (no longer relevant for current work).

- **PHASE_2_HEADER_INVESTIGATION.md** - Header design iterations (pre-v0.1.0)
- **PHASE_6_LAYOUT_VERIFICATION_SUMMARY.md** - Early layout validation
- **PHASE_13_PRE_RC_VERIFICATION.md** - Pre-release verification (superseded)
- **OLD_PROMPT_BASELINE_BEHAVIOR.md** - Historical AI behavior reference
- **STREAMING_TEST_REPORT.md** - Provider streaming tests (feature complete)

---

## When to Archive

**Archive a document when:**
1. Implementation is complete and merged
2. Investigation led to a decision (recorded elsewhere)
3. Problem is fully resolved
4. Report is superseded by newer analysis
5. Historical context only (no active reference needed)

**Do NOT archive if:**
1. Still referenced by active code
2. Contains unique information not captured elsewhere
3. Needed for current/future decisions
4. Part of active troubleshooting

---

## Finding Archived Content

**Why was X archived?**
- Check the parent README.md archive section
- Look for "Implemented in commit X" or "Replaced by Y"
- See git log for the file: `git log --follow archive/FILENAME.md`

**Need to reference archived content?**
- It's still in git history
- Read directly from archive/ directory
- Consider if content should be extracted to current docs

---

Last updated: 2026-02-06
