---
story_key: "2-1-display-procedure-name"
epic_key: "epic-2"
title: "Display Procedure Name in Header"
status: "done"
priority: "high"
created_date: "2026-05-11"
last_updated: "2026-05-11"
---

# ==========================================
# STORY DEFINITION
# ==========================================

## Story

As an IFS developer,
I want to see my procedure name in the OmniView header,
So that I know exactly which PL/SQL to call in my code.

## Background

Epic 1 implemented per-subscriber procedure generation using auto-assigned funny names (e.g., BARNACLE, PICKLES). Each subscriber gets a unique `TRACE_MESSAGE_<FUNNY_NAME>()` procedure that routes messages to only that subscriber via Oracle AQ correlation-based routing.

Epic 2 Story 2-1 makes this visible to the developer — showing the exact procedure they must call in their PL/SQL code.

**Epic 1 status**: COMPLETE ✅ (Stories 1-1 through 1-5 all done, routing tested with 2 subscribers)

**This story depends on**: Epic 1 completion — procedure generation and correlation routing are working

---

## Acceptance Criteria

**Given** a subscriber named BARNACLE is registered
**When** the Main Screen displays
**Then** the header shows `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`
**And** the name is visually prominent (e.g., different color, bold)

---

## Tasks/Subtasks

### Task 1: Identify where to display the procedure name in the TUI header

**Files**: `internal/adapter/ui/main_screen.go`, `internal/adapter/ui/chrome.go`, `internal/adapter/ui/styles/styles.go`

Study the existing header implementation to understand:
- How the header is currently structured
- Where subscriber context is available
- What styling conventions are used

- [x] Read `main_screen.go` to understand header layout
- [x] Read `chrome.go` to understand header chrome/structure
- [x] Read `styles/styles.go` to understand existing style tokens
- [x] Identify where subscriber's `FunnyName()` can be accessed
- [x] Identify the appropriate header location for the procedure call display

### Task 2: Add the procedure call display to the header

**AC**: Header shows `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')` with visual prominence

**Implementation approach**:
- The procedure call format is: `OMNI_TRACER_API.TRACE_MESSAGE_<FUNNY_NAME>('msg')`
- Example for subscriber BARNACLE: `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`
- Use `subscriber.FunnyName()` to get the funny name
- Format the display string: `OMNI_TRACER_API.TRACE_MESSAGE_` + `<funny_name>` + `('msg')`

**Files to modify**:
- `internal/adapter/ui/main_screen.go` — add the procedure call display to the header area
- `internal/adapter/ui/styles/styles.go` — add style token for the procedure call (e.g., a distinct color, maybe cyan/blue to indicate "call this")

**Design guidelines from DESIGN.md**:
- Visual prominence: different color, bold
- Position: in the Main Screen header area
- The display should be immediately visible to the developer

- [x] Add `ProcedureCallStyle` style token in `styles/styles.go` (distinct color for the procedure call, e.g., lipgloss.Color("99") or a blue)
- [x] In `main_screen.go`, add the procedure call display using the subscriber's `FunnyName()`
- [x] Format: `OMNI_TRACER_API.TRACE_MESSAGE_<FUNNY_NAME>('msg')`
- [x] Apply `ProcedureCallStyle` to make it visually prominent

### Task 3: Handle the case when no subscriber is registered

**Edge case**: The header should handle the case where no subscriber exists (e.g., before registration or if subscriber was deleted)

- [x] If no subscriber is registered, display a placeholder or nothing (not an error)
- [x] The procedure call should only display when a subscriber with a funny name is active

### Task 4: Write unit tests

- [x] Write test that verifies the procedure call display format: `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`
- [x] Write test for the case when no subscriber is registered (should not panic)
- [x] Run `make test` to verify all tests pass

### Task 5: Verify build

- [x] Run `make build` to verify no compilation errors

### Review Findings

- [x] [Review][Patch] Render the procedure call in the actual header, not the info bar [internal/adapter/ui/main_screen.go:723]
- [x] [Review][Patch] Add rendered-screen tests for header placement and narrow-width layout behavior [internal/adapter/ui/main_screen_test.go:20]
- [x] [Review][Patch] Check constructor errors in the new procedure-call tests [internal/adapter/ui/main_screen_test.go:24]

---

## Dev Notes

### Technical Context

**From Epic 1 completed work**:
- `subscriber.FunnyName()` returns the auto-assigned funny name (e.g., "BARNACLE")
- `ConsumerName()` returns `FunnyName` when assigned, falls back to UUID for legacy
- Generated procedure format: `TRACE_MESSAGE_<FUNNY_NAME>(message_, log_level_, process_name_?)`
- Full qualified call: `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`

**Subscriber availability in UI**:
- The model has access to subscriber service and current subscriber
- `model.subscriberService.GetSubscriber()` or similar can retrieve the current subscriber
- The subscriber entity has `FunnyName()` method

**Existing header structure** (from main_screen.go):
- Header area typically shows connection status, subscriber info
- Need to add the procedure call display without breaking existing layout

### File Structure

```
internal/adapter/ui/
├── main_screen.go        # [MODIFY] Add procedure call display to header
├── chrome.go             # Header chrome/structure
├── styles/
│   └── styles.go         # [MODIFY] Add ProcedureCallStyle token
```

### Styling Convention

From existing styles, follow the pattern:
```go
ProcedureCallStyle = lipgloss.NewStyle().
    Foreground(PrimaryColor).
    Bold(true)
```

### Testing Approach

- Test the view function output contains the procedure call string
- Test with subscriber having funny name "BARNACLE" → expects `TRACE_MESSAGE_BARNACLE`
- Test edge case: no subscriber → should not panic

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-2.1]
- [Source: _bmad-output/planning-artifacts/architecture.md#DEC-3]
- [Source: internal/adapter/ui/main_screen.go]
- [Source: internal/core/domain/subscriber.go] — `FunnyName()` method

---

## Dev Agent Record

### Implementation Plan

**Approach**: Added ProcedureCallStyle to styles.go using the shared primary accent token, added `mainProcedureCall()` in `main_screen.go`, and extended `renderScreenHeader()` to render the procedure call on its own header line.

**Files Modified**:
- `internal/adapter/ui/styles/styles.go` — added `ProcedureCallStyle`
- `internal/adapter/ui/chrome.go` — extended `renderScreenHeader()` with a dedicated detail line
- `internal/adapter/ui/main_screen.go` — added `mainProcedureCall()` and passed it into the header renderer

**Key Implementation Details**:
- `mainProcedureCall()` checks `m.subscriber != nil` and `subscriber.FunnyName()`
- If funny name is set, it returns `OMNI_TRACER_API.TRACE_MESSAGE_<NAME>('msg')` styled with `ProcedureCallStyle`
- `computeMainLayout()` passes that string into `renderScreenHeader()` so the procedure call renders in the actual header
- If no subscriber or no funny name, the procedure call is empty (no display)

### Debug Log

- Initial implementation put the procedure call into `mainStatusText()`, which satisfied visibility but not the story's explicit header requirement.
- Review feedback moved the procedure call into `renderScreenHeader()` and added rendered-screen coverage for placement and narrow-width safety.
- Procedure call styling now uses the shared `PrimaryColor` token with bold text for prominence.

### Completion Notes

✅ Story 2-1 completed: Display Procedure Name in Header implemented with:
- `ProcedureCallStyle` added to `styles/styles.go` (cyan foreground, bold)
- `renderScreenHeader()` now renders the procedure call in the actual header via `mainProcedureCall()`
- Format: `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`
- Edge cases handled: nil subscriber → no procedure call, subscriber without FunnyName → no procedure call
- 3 unit tests added covering: header placement, no funny name, and narrow-width layout safety
- make test ✅ — all tests pass
- make build ✅ — binary built successfully
- Review fixes applied ✅ — header placement corrected, rendered-screen tests added, constructor errors checked in test helpers

---

## File List

**Modified:**
- `internal/adapter/ui/chrome.go` — extended renderScreenHeader() to support a dedicated detail line
- `internal/adapter/ui/main_screen.go` — added mainProcedureCall() and restored status bar scope
- `internal/adapter/ui/styles/styles.go` — added ProcedureCallStyle using shared primary accent token
- `internal/adapter/ui/main_screen_test.go` — added rendered-header and narrow-width regression tests with checked constructors

**Created:**
- (none)

---

## Change Log

- **2026-05-11**: Story created — ready for dev-story execution
- **2026-05-11**: Implementation complete — ProcedureCallStyle added, mainStatusText() extended, tests added
- **2026-05-11**: Review — ready for review
- **2026-05-11**: Review fixes applied — procedure call moved into header, tests expanded, story marked done

---

## Status History

| Date | Status | Notes |
|------|--------|-------|
| 2026-05-11 | ready-for-dev | Story created from Epic 2 requirements |
| 2026-05-11 | in-progress | Implementation started and completed |
| 2026-05-11 | review | Implementation complete — ready for review |
| 2026-05-11 | done | Review findings fixed; make test and make build passing |
