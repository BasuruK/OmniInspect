---
story_key: "1-2-procedure-generation"
epic_key: "epic-1"
title: "Procedure Generation with Enqueue_For_Subscriber"
status: "done"
priority: "high"
created_date: "2026-04-28"
last_updated: "2026-05-03"
---

# ==========================================
# STORY DEFINITION
# ==========================================

## Story

As a system,
I want to generate `TRACE_MESSAGE_<FUNNY_NAME>()` procedures that call `Enqueue_For_Subscriber()`,
So that messages are routed to the correct subscriber.

## Acceptance Criteria

**Given** a subscriber with name BARNACLE is registered
**When** OmniView generates their procedure
**Then** the procedure `TRACE_MESSAGE_BARNACLE(message_, log_level_)` is created inside OMNI_TRACER_API package
**And** it calls `OMNI_TRACER_API.Enqueue_For_Subscriber(subscriber_name_ => 'BARNACLE', message_ => message_, log_level_ => log_level_)`

**Given** the subscriber's procedure already exists
**When** OmniView starts
**Then** no new procedure is created (idempotent)

---

## Tasks/Subtasks

- [x] Create `internal/service/subscribers/procedure_generator.go` with DDL generation logic for TRACE_MESSAGE_<name> procedures
- [x] Add `ProcedureExists` method to OracleAdapter to check if a procedure already exists in the package
- [x] Implement `GenerateSubscriberProcedure` method that generates and executes DDL for subscriber procedure
- [x] Add error type `ErrProcedureGeneration` for procedure generation failures
- [x] Implement idempotent creation - check procedure exists before creating
- [x] Call `GenerateSubscriberProcedure` during subscriber registration in `RegisterSubscriber`
- [x] Write unit tests for procedure generation logic
- [x] Verify build passes (make build)
- [x] Verify tests pass (make test)

### Review Findings

- [x] [Review][Patch] Generated procedure routes messages to an unregistered consumer [internal/service/subscribers/procedure_generator.go:101]
- [x] [Review][Patch] Funny name is not persisted through BoltDB serialization, breaking restart idempotence [internal/core/domain/subscriber.go:172]
- [x] [Review][Patch] Dynamic DDL is executed in a form Oracle cannot apply as written [internal/service/subscribers/procedure_generator.go:95]
- [x] [Review][Patch] Base package still does not define `Enqueue_For_Subscriber`, so procedure creation cannot succeed [assets/sql/Omni_Tracer.sql:101]
- [x] [Review][Patch] Registration can leave a partially saved subscriber after procedure generation fails [internal/service/subscribers/subscriber_service.go:52]

---

## Dev Notes

### Implementation Decisions

1. **DDL Generation**: Build updated package specification/body text in Go and deploy it via `DeployFile()`
2. **Static base package (Omni_Tracer.sql)** provides `Enqueue_For_Subscriber()` and serves as the fallback template
3. **Per-subscriber procedures** are injected into the current `OMNI_TRACER_API` package source and redeployed as packaged procedures
4. **Consumer Routing**: Oracle subscriber registration and dequeue use the assigned funny name as the AQ consumer alias
5. **Idempotent**: Check `user_procedures` for existing packaged procedures before creating and preserve persisted funny names across restarts
6. **Name validation**: Validate funny names through the existing `FunnyName` domain rules before any package update

### SQL for Checking Procedure Existence

```sql
SELECT COUNT(1) FROM user_procedures 
WHERE object_name = 'OMNI_TRACER_API' 
AND procedure_name = UPPER(:procedureName)
AND object_type = 'PACKAGE'
```

### DDL Template for Generated Procedure

```sql
PROCEDURE TRACE_MESSAGE_BARNACLE(
    message_   IN CLOB,
    log_level_ IN VARCHAR2 DEFAULT 'INFO'
);

PROCEDURE TRACE_MESSAGE_BARNACLE(
    message_   IN CLOB,
    log_level_ IN VARCHAR2 DEFAULT 'INFO'
)
IS
BEGIN
    OMNI_TRACER_API.Enqueue_For_Subscriber(
        subscriber_name_ => 'BARNACLE',
        message_         => message_,
        log_level_       => log_level_
    );
END TRACE_MESSAGE_BARNACLE;
```

### Key Files

- `internal/service/subscribers/procedure_generator.go` - DDL generation and execution
- `internal/adapter/storage/oracle/oracle_adapter.go` - Added `ProcedureExists()` method
- `internal/service/subscribers/subscriber_service.go` - Call procedure generation on registration
- `internal/core/domain/errors.go` - Added procedure-related errors
- `internal/core/domain/subscriber.go` - Added funny-name persistence and consumer alias handling
- `internal/adapter/storage/oracle/subscriptions.go` - Register Oracle subscribers with the funny-name consumer alias
- `internal/adapter/storage/oracle/queue.go` - Dequeue with the funny-name consumer alias
- `assets/sql/Omni_Tracer.sql` - Added `Enqueue_For_Subscriber()` to the base package

### Dependencies

- Story 1-1 (Funny Name Value Object) must be completed first
- Depends on `Enqueue_For_Subscriber()` being present in base OMNI_TRACER_API package

---

## Dev Agent Record

### Implementation Plan

**Approach**: Created a new procedure_generator.go file in the subscribers service package that handles DDL generation. Added ProcedureExists to oracle adapter. Wired into subscriber registration.

**Files Created**:
- `internal/service/subscribers/procedure_generator.go` (123 lines)
- `internal/service/subscribers/procedure_generator_test.go` (227 lines)
- `internal/core/domain/subscriber_test.go`

**Files Modified**:
- `internal/adapter/storage/oracle/oracle_adapter.go` - Added `ProcedureExists()`
- `internal/adapter/storage/oracle/subscriptions.go` - Register Oracle subscribers with the persisted funny-name alias
- `internal/adapter/storage/oracle/queue.go` - Dequeue with the persisted funny-name alias
- `internal/service/subscribers/subscriber_service.go` - Call procedure generation on registration
- `internal/core/ports/repository.go` - Added `ProcedureExists` interface method
- `internal/core/domain/errors.go` - Added `ErrProcedureGeneration`, `ErrProcedureNotFound`, `ErrInvalidProcedureName`
- `internal/core/domain/subscriber.go` - Added persisted funny-name alias support and JSON round-trip handling
- `assets/sql/Omni_Tracer.sql` - Added `Enqueue_For_Subscriber()` to the base package
- `internal/adapter/ui/model.go` - Create ProcedureGenerator with subscriber service
- `internal/adapter/ui/welcome_test.go` - Updated NewSubscriberService call
- `internal/adapter/ui/loading_test.go` - Updated NewSubscriberService call
- `internal/adapter/ui/webhook_settings_test.go` - Updated NewSubscriberService call
- `internal/adapter/ui/database_settings_test.go` - Added ProcedureExists to MockDatabaseRepository
- `internal/service/tracer/tracer_service_test.go` - Added ProcedureExists to stubDatabaseRepository

### Debug Log

- Fixed duplicate DefaultBatchSize declaration in subscriber.go constants block
- Added ProcedureExists to all mock/stub implementations in test files
- Fixed test assertions to be deterministic (use HasPrefix instead of exact name match)

### Completion Notes

✅ Story 1-2 completed: Procedure Generation with Enqueue_For_Subscriber implemented with:
- `ProcedureGenerator` struct with `GenerateSubscriberProcedure()` and `DropSubscriberProcedure()`
- `ProcedureExists()` method in OracleAdapter checking `user_procedures` table
- Packaged procedure generation for `TRACE_MESSAGE_<FUNNY_NAME>` procedures calling `Enqueue_For_Subscriber()`
- Idempotent creation and restart-safe funny-name persistence
- Oracle AQ registration/dequeue routed through the persisted funny-name consumer alias
- Wire into `RegisterSubscriber()` without persisting partial subscriber state on failure
- Comprehensive unit tests covering package deployment, persistence, and registration flow
- Build passes (make build)
- Tests pass (make test)

---

## File List

**Created:**
- `internal/service/subscribers/procedure_generator.go`
- `internal/service/subscribers/procedure_generator_test.go`
- `internal/core/domain/subscriber_test.go`

**Modified:**
- `internal/adapter/storage/oracle/oracle_adapter.go`
- `internal/adapter/storage/oracle/subscriptions.go`
- `internal/adapter/storage/oracle/queue.go`
- `internal/service/subscribers/subscriber_service.go`
- `internal/core/ports/repository.go`
- `internal/core/domain/errors.go`
- `internal/core/domain/subscriber.go`
- `assets/sql/Omni_Tracer.sql`
- `internal/adapter/ui/model.go`
- `internal/adapter/ui/welcome_test.go`
- `internal/adapter/ui/loading_test.go`
- `internal/adapter/ui/webhook_settings_test.go`
- `internal/adapter/ui/database_settings_test.go`
- `internal/service/tracer/tracer_service_test.go`

---

## Change Log

- **2026-05-03**: Implementation started
- **2026-05-03**: Story complete - procedure generation implemented with all ACs satisfied
- **2026-05-03**: Review fixes applied for packaged DDL, funny-name persistence, and alias-based routing

---

## Status History

| Date | Status | Notes |
|------|--------|-------|
| 2026-04-28 | ready-for-dev | Story created from epics |
| 2026-04-28 | in-progress | Development started |
| 2026-05-03 | review | Implementation complete, tests passing |
| 2026-05-03 | done | Review findings fixed; make test and make build passing |
