# UI Fixes for RC Release

Plan to address UI/UX issues discovered during v1 completion and prepare for release candidate.

**Created:** 2026-02-05  
**Target:** RC release (v0.1.0)

---

## Overview

During v1 completion and UI documentation audit, we identified several discrepancies between intended design, actual implementation, and user experience. This plan addresses visual consistency, border rendering issues, and test coverage gaps.

---

## Phase 1: Border Rendering Investigation

**Goal:** Understand why mysis list borders don't render in actual terminal but appear in golden tests.

### 1.1 Reproduce Border Issue

**Owner:** Can run in parallel with other phases  
**Estimated time:** 30 minutes

- [ ] **Test border rendering across terminals**
  - Test in: kitty, alacritty, gnome-terminal, iTerm2, Windows Terminal
  - Document which terminals show borders correctly
  - Capture screenshots from each terminal
  - File: `documentation/TERMINAL_COMPATIBILITY.md`

- [ ] **Test with different color schemes**
  - Test with light and dark backgrounds
  - Check if borders blend into background
  - Verify border color (`colorBorder = #2A2A55`) is visible
  - Document findings in compatibility doc

- [ ] **Compare golden test rendering vs actual terminal**
  - Run: `go test ./internal/tui -run TestDashboard -update`
  - Compare golden file output with actual terminal display
  - Check if ANSI codes differ between test and runtime

**Deliverables:**
- `documentation/TERMINAL_COMPATIBILITY.md` with findings
- Screenshots from 3+ terminals
- Recommendation: keep borders, adjust colors, or remove borders

---

## Phase 2: Focus View Header Investigation

**Goal:** Determine why focus view header doesn't appear in screenshots.

### 2.1 Verify Header Rendering

**Owner:** Can run in parallel with Phase 1  
**Estimated time:** 20 minutes

- [ ] **Trace header rendering in code**
  - Verify `renderFocusHeader()` is called (line 182 in `focus.go`)
  - Add debug logging to confirm header content
  - Check if header is rendered but scrolled off-screen
  - Verify header height is included in layout calculations

- [ ] **Test header visibility**
  - Run app with small terminal height (20 lines)
  - Run app with normal terminal height (40 lines)
  - Check if header appears in both cases
  - Capture screenshots showing header

- [ ] **Check viewport offset**
  - Verify viewport doesn't start above header
  - Check if header is included in `sections` array
  - Confirm `lipgloss.JoinVertical` includes header

**Deliverables:**
- Confirmation that header renders or bug report if it doesn't
- Screenshots showing header in focus view
- Fix if header is missing (update `RenderFocusViewWithViewport`)

---

## Phase 3: Unicode Character Consistency

**Goal:** Document and verify Unicode character rendering across terminals.

### 3.1 Character Rendering Audit

**Owner:** Can run in parallel with Phases 1-2  
**Estimated time:** 30 minutes

- [ ] **Document all Unicode characters used**
  - List all decorative characters: `⬥`, `⬧`, `⬡`, `⬢`, `⬦`, `◦`, `◌`, `✖`
  - Document Unicode codepoints and names
  - Add to `documentation/UI_LAYOUT_REPORT.md` (already done)
  - Create visual reference chart

- [ ] **Test character rendering**
  - Test in 3+ terminals with different fonts
  - Document which characters render correctly
  - Identify problematic characters (e.g., ambiguous width)
  - Check if any characters cause alignment issues

- [ ] **Verify spinner animation**
  - Confirm all 8 frames render correctly
  - Check animation speed (125ms per frame)
  - Test in different terminals
  - Verify no frame causes layout shift

**Deliverables:**
- Character compatibility matrix (terminal × character)
- Recommendations for fallback characters if needed
- Update `TUI_TESTING.md` with character testing guidelines

---

## Phase 4: Spinner Animation Testing

**Goal:** Add comprehensive tests for spinner animation and state indicators.

### 4.1 Spinner Unit Tests

**Owner:** Can run in parallel with other phases  
**Estimated time:** 45 minutes

- [ ] **Add spinner frame tests**
  - Test all 8 frames render without errors
  - Test frame cycling logic
  - Test FPS timing (125ms per frame)
  - File: `internal/tui/spinner_test.go` (new file)

- [ ] **Add state indicator tests**
  - Test running state shows spinner
  - Test idle state shows `◦`
  - Test stopped state shows `◌`
  - Test errored state shows `✖`
  - Test loading state shows spinner
  - File: `internal/tui/dashboard_test.go` (add to existing)

- [ ] **Add golden tests for spinner states**
  - Create golden files for each state indicator
  - Test with spinner at different frames
  - Verify alignment doesn't shift between frames
  - Files: `testdata/TestMysisLine/spinner_frame_*/*.golden`

**Deliverables:**
- `internal/tui/spinner_test.go` with 10+ tests
- 8 new golden files for spinner frames
- Coverage increase: `internal/tui` from 83% to 85%+

---

## Phase 5: Border Style Alternatives

**Goal:** Provide alternative border styles if DoubleBorder doesn't render well.

### 5.1 Implement Border Style Options

**Owner:** Depends on Phase 1 findings  
**Estimated time:** 1 hour

- [ ] **Add border style configuration**
  - Add `border_style` to `config.toml`
  - Options: `double`, `rounded`, `thick`, `none`
  - Default: `double` (current behavior)
  - File: `internal/config/config.go`

- [ ] **Implement border style switching**
  - Update `mysisListStyle` to use configured border
  - Update `logStyle` to use configured border
  - Update `helpStyle` to use configured border
  - File: `internal/tui/styles.go`

- [ ] **Test each border style**
  - Create golden tests for each style
  - Test in multiple terminals
  - Document visual differences
  - Files: `testdata/TestDashboard/border_style_*/*.golden`

- [ ] **Add environment override**
  - `ZOEA_BORDER_STYLE` env var
  - Validate values: double, rounded, thick, none
  - Document in `AGENTS.md` and `README.md`

**Deliverables:**
- Border style configuration option
- 4 border style variants tested
- Golden files for each border style
- Documentation updates

**Note:** Only implement if Phase 1 identifies terminal compatibility issues.

---

## Phase 6: Layout Calculation Verification

**Goal:** Ensure layout calculations are correct and don't cause overflow/underflow.

### 6.1 Layout Math Audit

**Owner:** Can run in parallel with other phases  
**Estimated time:** 1 hour

- [ ] **Audit dashboard height calculations**
  - Verify `usedHeight` calculation (dashboard.go:116-120)
  - Test with various terminal heights (20, 30, 40, 60 lines)
  - Ensure mysis list never has negative height
  - Add assertions for minimum heights
  - File: `internal/tui/dashboard.go`

- [ ] **Audit focus view height calculations**
  - Verify viewport height calculation
  - Test with various terminal heights
  - Ensure viewport never has negative height
  - Check if header is included in calculations
  - File: `internal/tui/focus.go`

- [ ] **Add layout calculation tests**
  - Test dashboard with 0, 1, 5, 10, 20 myses
  - Test with 0, 1, 5, 10 swarm messages
  - Test with terminal heights: 20, 30, 40, 60, 100
  - Test with terminal widths: 80, 120, 160, 200
  - File: `internal/tui/layout_test.go` (new file)

- [ ] **Add minimum dimension guards**
  - Ensure minimum terminal size (80x20)
  - Display warning if terminal too small
  - Gracefully degrade layout if needed
  - File: `internal/tui/app.go`

**Deliverables:**
- `internal/tui/layout_test.go` with 20+ test cases
- Minimum dimension guards in place
- Documentation of minimum terminal requirements
- Coverage increase: `internal/tui` from 85% to 87%+

---

## Phase 7: Width Calculation Verification

**Goal:** Ensure all width calculations use `lipgloss.Width()` for Unicode safety.

### 7.1 Width Calculation Audit

**Owner:** Can run in parallel with Phase 6  
**Estimated time:** 45 minutes

- [ ] **Audit all width calculations**
  - Search for `len(` in TUI code
  - Replace with `lipgloss.Width()` where needed
  - Verify truncation uses `truncateToWidth()`
  - Check padding calculations
  - Files: `internal/tui/*.go`

- [ ] **Add width calculation tests**
  - Test with ASCII strings
  - Test with Unicode strings (emoji, CJK, box-drawing)
  - Test with ANSI-styled strings
  - Test truncation at various widths
  - File: `internal/tui/width_test.go` (new file)

- [ ] **Test mysis line width calculations**
  - Test with long mysis names (16+ chars)
  - Test with long provider names
  - Test with long account names
  - Test with long last messages
  - Verify no overflow at various terminal widths
  - File: `internal/tui/dashboard_test.go`

**Deliverables:**
- All `len()` calls replaced with `lipgloss.Width()` where appropriate
- `internal/tui/width_test.go` with 15+ tests
- Golden tests for edge cases (long names, Unicode)
- Coverage increase: `internal/tui` from 87% to 89%+

---

## Phase 8: Input Prompt Documentation

**Goal:** Document input prompt behavior and add tests.

### 8.1 Input Prompt Tests

**Owner:** Can run in parallel with other phases  
**Estimated time:** 30 minutes

- [ ] **Add input prompt rendering tests**
  - Test broadcast mode prompt (`⬧`)
  - Test message mode prompt (`⬥`)
  - Test prompt with various input lengths
  - Test prompt width at various terminal widths
  - File: `internal/tui/input_test.go` (new file)

- [ ] **Add input prompt golden tests**
  - Golden file for broadcast prompt
  - Golden file for message prompt
  - Golden file for long input text
  - Files: `testdata/TestInputPrompt/*/*.golden`

- [ ] **Document input prompt behavior**
  - Add to `UI_LAYOUT_REPORT.md` (already done)
  - Document keyboard shortcuts
  - Document prompt indicators
  - File: `documentation/UI_LAYOUT_REPORT.md`

**Deliverables:**
- `internal/tui/input_test.go` with 8+ tests
- 3 new golden files for input prompts
- Coverage increase: `internal/tui` from 89% to 90%+

---

## Phase 9: Status Bar Testing

**Goal:** Add comprehensive tests for status bar rendering.

### 9.1 Status Bar Tests

**Owner:** Can run in parallel with other phases  
**Estimated time:** 45 minutes

- [ ] **Add status bar rendering tests**
  - Test LLM indicator with 0%, 50%, 100% progress
  - Test dashboard view name
  - Test focus view name (with mysis ID)
  - Test mysis count display (0/0, 1/3, 16/16)
  - File: `internal/tui/statusbar_test.go` (new file)

- [ ] **Add status bar golden tests**
  - Golden file for dashboard status
  - Golden file for focus status
  - Golden file for various mysis counts
  - Golden file for LLM progress states
  - Files: `testdata/TestStatusBar/*/*.golden`

- [ ] **Test status bar width handling**
  - Test at various terminal widths (80, 120, 160)
  - Verify no overflow
  - Test truncation of long mysis IDs
  - File: `internal/tui/statusbar_test.go`

**Deliverables:**
- `internal/tui/statusbar_test.go` with 12+ tests
- 4 new golden files for status bar states
- Coverage increase: `internal/tui` from 90% to 91%+

---

## Phase 10: Integration Test Expansion

**Goal:** Add integration tests for UI flows not currently covered.

### 10.1 New Integration Tests

**Owner:** Can run after Phases 1-9 complete  
**Estimated time:** 1.5 hours

- [ ] **Add border rendering integration test**
  - Test dashboard with borders visible
  - Test focus view with borders visible
  - Verify borders don't cause layout issues
  - File: `internal/tui/integration_test.go`

- [ ] **Add spinner animation integration test**
  - Test spinner updates during mysis execution
  - Verify spinner doesn't cause flicker
  - Test multiple myses with spinners
  - File: `internal/tui/integration_test.go`

- [ ] **Add input prompt integration test**
  - Test entering broadcast message
  - Test entering direct message
  - Test canceling input (ESC)
  - Test submitting input (Enter)
  - File: `internal/tui/integration_test.go`

- [ ] **Add status bar integration test**
  - Test status bar updates during mysis state changes
  - Test LLM indicator during provider calls
  - Test view name changes (dashboard ↔ focus)
  - File: `internal/tui/integration_test.go`

- [ ] **Add resize integration test**
  - Test window resize during dashboard view
  - Test window resize during focus view
  - Verify layout recalculates correctly
  - Test minimum size handling
  - File: `internal/tui/integration_test.go`

**Deliverables:**
- 5 new integration tests
- All integration tests passing
- Coverage increase: `internal/tui` from 91% to 93%+

---

## Phase 11: Visual Regression Prevention

**Goal:** Ensure future changes don't break UI rendering.

### 11.1 Golden File Coverage

**Owner:** Can run after Phases 1-10 complete  
**Estimated time:** 1 hour

- [ ] **Audit golden file coverage**
  - List all rendering functions
  - Identify functions without golden tests
  - Create missing golden tests
  - Document golden file naming convention
  - File: `documentation/TUI_TESTING.md`

- [ ] **Add edge case golden tests**
  - Empty states (no myses, no messages)
  - Full states (16 myses, 10 messages)
  - Error states (all myses errored)
  - Long content (truncation edge cases)
  - Unicode content (emoji, CJK, box-drawing)
  - Files: `testdata/TestDashboard/edge_cases/*/*.golden`

- [ ] **Add golden test CI check**
  - Fail CI if golden files are stale
  - Require explicit `-update` flag to update
  - Document golden file update process
  - File: `.github/workflows/test.yml` (if using GitHub Actions)

**Deliverables:**
- 10+ new golden files for edge cases
- Golden file coverage report
- CI check for golden file freshness
- Coverage increase: `internal/tui` from 93% to 95%+

---

## Phase 12: Documentation Updates

**Goal:** Ensure all documentation reflects actual implementation.

### 12.1 Documentation Audit

**Owner:** Can run after Phases 1-11 complete  
**Estimated time:** 45 minutes

- [ ] **Update UI_LAYOUT_REPORT.md**
  - Add findings from Phase 1 (border rendering)
  - Add findings from Phase 2 (focus header)
  - Add findings from Phase 3 (Unicode characters)
  - Add terminal compatibility notes
  - File: `documentation/UI_LAYOUT_REPORT.md` (already updated)

- [ ] **Update TUI_TESTING.md**
  - Add spinner testing guidelines
  - Add layout testing guidelines
  - Add width calculation testing guidelines
  - Add golden file update process
  - File: `documentation/TUI_TESTING.md`

- [ ] **Update AGENTS.md**
  - Add border style configuration
  - Add minimum terminal requirements
  - Add terminal compatibility notes
  - File: `AGENTS.md`

- [ ] **Update README.md**
  - Add terminal requirements section
  - Add troubleshooting section (borders not visible)
  - Add configuration section (border style)
  - File: `README.md`

**Deliverables:**
- All documentation files updated
- Terminal compatibility documented
- Configuration options documented
- Troubleshooting guide added

---

## Phase 13: Pre-RC Verification

**Goal:** Final verification before RC release.

### 13.1 Manual Testing Checklist

**Owner:** Must run after all phases complete  
**Estimated time:** 1 hour

- [ ] **Test in 3+ terminals**
  - kitty, alacritty, gnome-terminal (or equivalents)
  - Verify borders render correctly (or document if not)
  - Verify Unicode characters render correctly
  - Verify spinner animation is smooth
  - Capture screenshots for documentation

- [ ] **Test at various terminal sizes**
  - Minimum size (80x20)
  - Small size (100x30)
  - Medium size (120x40)
  - Large size (160x60)
  - Verify layout adapts correctly

- [ ] **Test all UI flows**
  - Create mysis → verify dashboard updates
  - Send broadcast → verify all myses receive
  - Send message → verify target mysis receives
  - Focus on mysis → verify focus view renders
  - Scroll conversation → verify scrollbar updates
  - Toggle verbose → verify reasoning display
  - Resize window → verify layout recalculates

- [ ] **Test error states**
  - Mysis in errored state → verify error display
  - No myses → verify empty state message
  - No broadcasts → verify empty state message
  - Terminal too small → verify warning/degradation

**Deliverables:**
- Manual testing report with screenshots
- List of any remaining issues
- Go/no-go decision for RC release

---

## Success Criteria

- [ ] All phases complete
- [ ] Test coverage ≥ 95% for `internal/tui`
- [ ] All integration tests passing
- [ ] No visual regressions (golden tests pass)
- [ ] Documentation accurate and complete
- [ ] Manual testing successful in 3+ terminals
- [ ] No critical bugs identified

---

## Tracking

| Phase | Tasks | Estimated Time | Can Run in Parallel |
|-------|-------|----------------|---------------------|
| 1. Border Rendering | 3 | 30 min | Yes |
| 2. Focus Header | 3 | 20 min | Yes |
| 3. Unicode Characters | 3 | 30 min | Yes |
| 4. Spinner Animation | 3 | 45 min | Yes |
| 5. Border Alternatives | 4 | 1 hour | Depends on Phase 1 |
| 6. Layout Calculations | 4 | 1 hour | Yes |
| 7. Width Calculations | 3 | 45 min | Yes |
| 8. Input Prompt | 3 | 30 min | Yes |
| 9. Status Bar | 3 | 45 min | Yes |
| 10. Integration Tests | 5 | 1.5 hours | After 1-9 |
| 11. Visual Regression | 3 | 1 hour | After 1-10 |
| 12. Documentation | 4 | 45 min | After 1-11 |
| 13. Pre-RC Verification | 4 | 1 hour | After all |
| **Total** | **45** | **10.5 hours** | **~4 hours with parallelization** |

---

## Parallel Execution Strategy

**Group A (can run simultaneously):**
- Phase 1: Border Rendering Investigation
- Phase 2: Focus View Header Investigation
- Phase 3: Unicode Character Consistency
- Phase 4: Spinner Animation Testing
- Phase 6: Layout Calculation Verification
- Phase 7: Width Calculation Verification
- Phase 8: Input Prompt Documentation
- Phase 9: Status Bar Testing

**Group B (depends on Group A):**
- Phase 5: Border Style Alternatives (depends on Phase 1 findings)
- Phase 10: Integration Test Expansion (after Group A)

**Group C (depends on Group B):**
- Phase 11: Visual Regression Prevention (after Group B)

**Group D (depends on Group C):**
- Phase 12: Documentation Updates (after Group C)
- Phase 13: Pre-RC Verification (after all)

**Estimated timeline with parallelization:**
- Group A: ~1 hour (longest task in parallel)
- Group B: ~1.5 hours (sequential after A)
- Group C: ~1 hour (sequential after B)
- Group D: ~1.75 hours (sequential after C)
- **Total: ~5.25 hours** (vs 10.5 hours sequential)

---

## Notes

- Phases 1-9 are highly parallelizable - can be delegated to multiple agents
- Phase 5 is optional - only implement if Phase 1 identifies issues
- Phase 10-13 must run sequentially after earlier phases
- All test additions should update coverage reports
- Golden file updates require explicit `-update` flag
- Manual testing (Phase 13) is critical - don't skip

---

## Risk Assessment

**Low Risk:**
- Phases 3, 4, 8, 9 (pure testing additions)

**Medium Risk:**
- Phases 6, 7 (layout/width calculations - could break existing rendering)

**High Risk:**
- Phase 5 (border style changes - could affect all views)

**Mitigation:**
- Run full test suite after each phase
- Update golden files carefully
- Test in multiple terminals before committing
- Keep Phase 5 optional and behind configuration flag
