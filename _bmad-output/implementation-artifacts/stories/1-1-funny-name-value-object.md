---
story_key: "1-1-funny-name-value-object"
epic_key: "epic-1"
title: "Funny Name Value Object"
status: "done"
priority: "high"
created_date: "2026-04-28"
last_updated: "2026-04-28"
---

# ==========================================
# STORY DEFINITION
# ==========================================

## Story

As a system,
I want to auto-assign funny cartoon character names to subscribers,
So that procedure names are memorable and unique.

## Acceptance Criteria

**Given** a new subscriber registers with OmniView
**When** the system generates their procedure
**Then** a funny name (e.g., BARNACLE, PICKLES, NIBBLES) is automatically assigned from the curated list
**And** the resulting procedure is `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`

**Given** a funny name is already assigned to another subscriber
**When** a new subscriber registers
**Then** the system picks another available name to avoid collision
**And** no two subscribers have the same procedure name

---

## Tasks/Subtasks

- [x] Create `internal/core/domain/funny_names.go` with curated list of 120+ cartoon character names
- [x] Implement `FunnyNameGenerator` struct with thread-safe random name assignment
- [x] Add collision handling - system picks another available name when name is already used
- [x] Implement format validation (`^[A-Za-z_]+$`) for SQL injection prevention
- [x] Add error types: `ErrInvalidFunnyName`, `ErrNoAvailableNames`, `ErrFunnyNameTooLong`, `ErrFunnyNameTooShort`
- [x] Write comprehensive unit tests covering all functionality
- [x] Verify all tests pass

---

## Dev Notes

### Implementation Decisions

1. **Name Format**: Less than 30 characters, format `^[A-Za-z_]+$` (letters and underscores only, no numbers)
2. **Name Storage**: Curated list of 120+ cartoon character names stored as constant array in `funny_names.go`
3. **Collision Handling**: Uses `FunnyNameGenerator` with `used` map tracking assigned names; picks another if collision occurs
4. **Thread Safety**: Generator uses `sync.Mutex` for safe concurrent access
5. **Validation**: Two-layer validation - format check then list membership check

### Project Context (from SuperMemory)

- Dynamic procedure generation: Uses ExecuteStatement() with runtime-generated DDL strings, NOT embedded files
- Funny name auto-assignment system: Curated list of 100+ cartoon character names (Mickey, Donald, Bugs, Daffy, Scooby, Tom, Jerry, etc.)
- System auto-assigns on subscriber creation, user has NO visibility into or choice over the name
- Collision handling: system picks another available name
- Full procedure name: `TRACE_MESSAGE_<FUNNY_NAME>` (e.g., `TRACE_MESSAGE_BARNACLE`)

### Key Files

- `internal/core/domain/funny_names.go` - FunnyNameGenerator, validation functions, curated name list
- `internal/core/domain/funny_names_test.go` - 16 test cases covering validation, generation, collision handling

---

## Dev Agent Record

### Implementation Plan

**Approach**: Created value object following existing domain patterns (BatchSize, WaitTime)

**Files Created**:
- `internal/core/domain/funny_names.go` (331 lines)
- `internal/core/domain/funny_names_test.go` (333 lines)

**Key Implementation Details**:
1. `FunnyName` struct with `String()` and `IsValid()` methods
2. `FunnyNameGenerator` with `GetRandomName()`, `MarkAsUsed()`, `MarkAsAvailable()`, `Reset()`
3. Global `ValidateFunnyNameFormat()` for format validation
4. Global `IsValidFunnyName()` for list membership validation
5. Singleton pattern via `DefaultFunnyNameGenerator()` with `sync.Once`
6. Domain sentinel errors registered in `internal/core/domain/errors.go`
7. Fixed duplicate "Nibbles" entry in curated list
8. Added missing "Tom" entry

**Tests**: 16 tests covering format validation, list validation, generator functionality, thread safety, case insensitivity

### Debug Log

- Initial build had compilation error (`_, err = gen.GetRandomName()` missing `:=`)
- Fixed by changing to `_, err := gen.GetRandomName()`
- Duplicate "Nibbles" in name list caused test failure in `TestFunnyNameGenerator_NoDuplicates`
- Missing "Tom" caused test failure in `TestIsValidFunnyName_FromList`

### Completion Notes

✅ Story 1-1 completed: FunnyName value object implemented with:
- 120+ curated cartoon character names
- Thread-safe `FunnyNameGenerator` with collision handling
- Format validation (`^[A-Za-z_]+$`)
- List membership validation
- Comprehensive unit tests (16 tests, all passing)

---

## File List

**Created:**
- `internal/core/domain/funny_names.go`
- `internal/core/domain/funny_names_test.go`

**Modified:**
- `internal/core/domain/errors.go`

---

## Change Log

- **2026-04-28**: Initial implementation of FunnyName value object with curated name list and collision handling
- **2026-04-28**: Added comprehensive unit tests, all 16 tests passing

---

## Status History

| Date | Status | Notes |
|------|--------|-------|
| 2026-04-28 | ready-for-dev | Story created from epics |
| 2026-04-28 | in-progress | Development started |
| 2026-04-28 | review | Implementation complete, tests passing |
| 2026-04-28 | done | Completed |

---

## Senior Developer Review (AI)

### Review Outcome

**Approved** - Story 1.1 is complete and ready to close.

### Review Date

2026-04-28

### Findings

No remaining blocking findings.

Issues found during review were fixed before approval:
- Generated names now normalize to uppercase for procedure-safe names such as `TRACE_MESSAGE_BARNACLE`
- `IsValidFunnyName()` no longer trims leading/trailing spaces before validation
- Funny name sentinel errors now live in `internal/core/domain/errors.go`
- Nondeterministic availability test assertion was removed

### Verification

- `go test ./internal/core/domain -run "FunnyName"` passed
- `make test` passed
- `make lint` passed

### Action Items

- [x] Normalize generated funny names to uppercase
- [x] Reject leading/trailing spaces in `IsValidFunnyName()`
- [x] Move funny name sentinel errors to domain errors
- [x] Remove nondeterministic test expectation
- [x] Add doc comments to all exported functions and methods

### Additional Changes After Review

- Added doc comments to all exported functions and methods in `funny_names.go`
- Documented domain sentinel error location rule in `project-context.md`

---

## Review Follow-ups (AI)

*To be added if review requires changes*
