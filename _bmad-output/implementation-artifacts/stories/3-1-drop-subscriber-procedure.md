---
story_key: "3-1-drop-subscriber-procedure"
epic_key: "epic-3"
title: "Drop Subscriber Procedure (Danger Zone)"
status: "ready-for-dev"
priority: "high"
created_date: "2026-05-12"
last_updated: "2026-05-12"
---

# ==========================================
# STORY DEFINITION
# ==========================================

## Story

As a subscriber,
I want to delete my specific procedure,
So that I can clean up when I no longer need tracing.

## Background

Epic 1 implemented per-subscriber procedure generation using auto-assigned funny names (e.g., BARNACLE, PICKLES). Each subscriber gets a unique `TRACE_MESSAGE_<FUNNY_NAME>()` procedure inside the `OMNI_TRACER_API` package.

Epic 3 Story 3-1 adds a danger zone option in the Settings UI to delete only the subscriber's own procedure (not affecting other subscribers or the base package).

**Epic 1 status**: COMPLETE ✅ (Stories 1-1 through 1-5 all done, routing tested with 2 subscribers)

**Epic 2 status**: COMPLETE ✅ (Story 2-1 done - procedure name displayed in header)

**This story depends on**: Epic 1 completion — procedure generation is working and procedures can be deleted

---

## Acceptance Criteria

**Given** the subscriber is on the Settings screen
**When** they select "Delete My Procedure"
**Then** a confirmation dialog appears warning this action
**And** if confirmed, the `TRACE_MESSAGE_<FUNNY_NAME>()` procedure is removed from OMNI_TRACER_API package

**Given** the procedure is deleted
**When** OmniView restarts
**Then** if the subscriber is still registered, the procedure is regenerated

---

## Tasks/Subtasks

### Task 1: Study existing Settings UI and danger zone requirements

**Files**: `internal/adapter/ui/database_settings.go`, `_bmad-output/planning-artifacts/architecture.md`

- [x] Read `database_settings.go` to understand current Settings screen structure
- [x] Read architecture.md Pattern 4: Settings UI Danger Zone
- [x] Understand confirmation dialog pattern used in the app
- [x] Identify where danger zone options should be added

### Task 2: Add DropSubscriberProcedure method to ProcedureGenerator

**Files**: `internal/service/subscribers/procedure_generator.go`

**Implementation approach**:
- `DropSubscriberProcedure(subscriberName string) error` — removes the subscriber's procedure block from the package
- Uses the same section markers (`-- @SECTION: SUBSCRIBER_GENERATED_METHOD : <name>`) that are used during creation
- Idempotent: if procedure doesn't exist, return success
- Must use Oracle `EXECUTE IMMEDIATE` with proper DDL to remove the procedure

**DDL to drop procedure**:
```sql
ALTER PACKAGE OMNI_TRACER_API COMPILE BODY;
```
Or use dbms_utility to invalidate and recompile.

**Note**: In Oracle, you cannot directly "drop" a procedure from a package body — you must either:
1. Recompile the entire package body without the procedure (redeploy full package spec/body)
2. Use `ALTER PACKAGE ... DROP PROCEDURE` (Oracle 12c+)

Since OmniView uses package redeployment, the approach is:
- Fetch current package body source
- Remove the subscriber's procedure block (section between markers)
- Redeploy the updated package body

- [x] `DropSubscriberProcedure` already exists in procedure_generator.go (implemented in Epic 1)
- [x] Method uses section marker pattern to identify and remove procedure block
- [x] Handle case where procedure doesn't exist (idempotent - returns nil)
- [x] Handle case where procedure exists but ownership markers don't match
- [x] Unit tests for drop functionality already exist

### Task 3: Add danger zone UI to Settings screen

**Files**: `internal/adapter/ui/database_settings.go`, `internal/adapter/ui/styles/styles.go`, `internal/adapter/ui/model.go`

**Implementation approach**:
- Add "Danger Zone" section in Settings screen with red/warning styling
- Add "Delete My Procedure" option (activated via D key)
- Show confirmation dialog before executing
- Display current procedure name being deleted (e.g., `TRACE_MESSAGE_BARNACLE`)

**UI Flow**:
1. User presses S to open Settings
2. User scrolls to Danger Zone section
3. User presses D to select "Delete My Procedure"
4. Confirmation dialog shows: "This will delete your procedure `TRACE_MESSAGE_BARNACLE`. You can regenerate it by restarting OmniView."
5. If confirmed (Y key), call `DropSubscriberProcedure` and show success/error
6. If success, procedure is removed from package

- [x] Add `DangerZoneStyle` in `styles/styles.go` (red/warning styling)
- [x] Add danger zone section to Settings screen with hint "Press D to delete your subscriber procedure"
- [x] Add "D" key handler to trigger drop procedure confirmation
- [x] Add confirmation modal `viewDropProcedureConfirmModal()` showing procedure name
- [x] Wire up to `ProcedureGenerator.DropSubscriberProcedure()` via async command pattern
- [x] Handle success/error feedback to user via dialog
- [x] Add drop procedure confirm modal rendering in model.go View()

### Task 4: Handle edge cases and errors

- [x] Handle Oracle errors during procedure drop (package invalidation, etc.) - error shown in dialog
- [x] Handle case where subscriber has no procedure (no-op, show "no procedure to delete")
- [x] Handle case where subscriber is not registered (Danger zone section only shows when subscriber has funny name)
- [x] Unit tests for error scenarios already exist in procedure_generator_test.go

### Task 5: Verify build and tests

- [x] Run `make test` to verify all tests pass
- [x] Run `make build` to verify no compilation errors

---

## Dev Notes

### Technical Context

**From Epic 1 completed work**:
- `subscriber.FunnyName()` returns the auto-assigned funny name (e.g., "BARNACLE")
- Generated procedures are stored inside `OMNI_TRACER_API` package with section markers
- `ProcedureGenerator` handles procedure creation with `EnsureSubscriberProcedure()`
- The package uses `DeployFile()` to redeploy the full package when procedures change

**Ownership markers** (from procedure_generator.go):
```sql
-- @SECTION: SUBSCRIBER_GENERATED_METHOD : BARNACLE
PROCEDURE TRACE_MESSAGE_BARNACLE(
    message_   IN CLOB,
    log_level_ IN VARCHAR2 DEFAULT 'INFO',
    process_name_  IN VARCHAR2 DEFAULT NULL
)
IS
BEGIN
    Enqueue_Event___(
        process_name_     => process_name_,
        log_level_        => log_level_,
        payload           => message_,
        subscriber_name_  => 'BARNACLE'
    );
END TRACE_MESSAGE_BARNACLE;
-- @END_SECTION: SUBSCRIBER_GENERATED_METHOD : BARNACLE
```

**Drop approach**:
- Fetch current package body source via `FetchPackageBody()`
- Use `replaceProcedureBlock()` or similar to remove the subscriber's section
- Redeploy the updated package body via `DeployPackageBody()`

### File Structure

```
internal/
├── adapter/
│   └── ui/
│       ├── database_settings.go        # [MODIFY] Add danger zone
│       └── styles/
│           └── styles.go              # [MODIFY] Add DangerZoneStyle
├── service/
│   └── subscribers/
│       ├── procedure_generator.go     # [MODIFY] Add DropSubscriberProcedure
│       └── procedure_generator_test.go # [MODIFY] Add drop tests
```

### Testing Approach

- Test `DropSubscriberProcedure` with existing procedure (should succeed)
- Test `DropSubscriberProcedure` with non-existent procedure (should return nil - idempotent)
- Test `DropSubscriberProcedure` with procedure owned by another subscriber (should handle gracefully)
- Test Settings UI danger zone flow with confirmation dialog

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-3.1]
- [Source: _bmad-output/planning-artifacts/architecture.md#Pattern-4]
- [Source: internal/service/subscribers/procedure_generator.go] — existing procedure generation
- [Source: internal/adapter/ui/database_settings.go] — Settings screen
- [Source: docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md]

---

## Dev Agent Record

### Implementation Plan

**Approach**: Added danger zone UI to Settings screen with D key to trigger drop procedure confirmation.

**Files Modified**:
- `internal/adapter/ui/styles/styles.go` — added `DangerZoneStyle` (red/bold)
- `internal/adapter/ui/database_settings.go` — added danger zone state, D key handler, drop confirmation modal, async command wiring
- `internal/adapter/ui/messages.go` — added `dropSubscriberProcedureMsg` and `dropSubscriberProcedureResultMsg`
- `internal/adapter/ui/model.go` — added `showDropProcedureConfirm` handling in View()
- `internal/service/subscribers/subscriber_service.go` — added `ProcedureGenerator()` accessor method

**Key Implementation Details**:
- `DropSubscriberProcedure` was already implemented in Epic 1 (procedure_generator.go:217)
- Added `DangerZoneStyle` with ErrorColor and Bold for visual prominence
- "Danger Zone" section only appears when subscriber has a funny name
- D key triggers confirmation modal showing procedure name
- Y key confirms and calls `DropSubscriberProcedure` asynchronously
- Success/error shown in dialog with appropriate message

### Debug Log

- Added `showDropProcedureConfirm` and `dropProcedureConfirmMsg` fields to `databaseSettingsState`
- Added `dropSubscriberProcedureMsg` and `dropSubscriberProcedureResultMsg` message types
- Added `viewDropProcedureConfirmModal()` method for confirmation dialog rendering
- Added `ProcedureGenerator()` accessor to `SubscriberService` for UI access
- Modal rendering added to main View() in model.go after delete confirm check

### Completion Notes

✅ Story 3-1 completed: Drop Subscriber Procedure (Danger Zone) implemented with:
- `DangerZoneStyle` added to `styles/styles.go` (red foreground, bold)
- Danger zone section in Settings screen (only shown when subscriber has funny name)
- "Press D to delete your subscriber procedure" hint in UI
- D key handler triggers confirmation modal
- `viewDropProcedureConfirmModal()` shows procedure name and warning
- Y confirms and calls `DropSubscriberProcedure` asynchronously
- Error/success feedback via dialog
- `make test` ✅ — all tests pass
- `make build` ✅ — binary built successfully

---

## File List

**Modified:**
- `internal/adapter/ui/styles/styles.go` — added `DangerZoneStyle`
- `internal/adapter/ui/database_settings.go` — added danger zone UI, D key handler, confirmation modal
- `internal/adapter/ui/messages.go` — added `dropSubscriberProcedureMsg`, `dropSubscriberProcedureResultMsg`
- `internal/adapter/ui/model.go` — added drop procedure confirm modal rendering in View()
- `internal/service/subscribers/subscriber_service.go` — added `ProcedureGenerator()` accessor method

**Created:**
- (none — `DropSubscriberProcedure` already existed from Epic 1)

---

## Change Log

- **2026-05-12**: Story created — ready for dev-story execution
- **2026-05-12**: Implementation started — DangerZoneStyle, danger zone UI, D key handler, confirmation modal, async command wiring
- **2026-05-12**: Implementation complete — make test and make build passing
- **2026-05-12**: Story done — ready for review

---

## Status History

| Date | Status | Notes |
|------|--------|-------|
| 2026-05-12 | ready-for-dev | Story created from Epic 3 requirements |
| 2026-05-12 | in-progress | Implementation started |
| 2026-05-12 | done | Implementation complete; make test and make build passing |