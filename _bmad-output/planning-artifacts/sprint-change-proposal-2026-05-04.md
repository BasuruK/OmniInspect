# Sprint Change Proposal

## Issue Summary

During Story `1-2-procedure-generation`, the implementation introduced a separate public PL/SQL helper, `Enqueue_For_Subscriber()`, even though the existing private helper `Enqueue_Event___()` already handled nearly all enqueue behavior.

The identified issue was architectural duplication:
- two enqueue paths had to stay behaviorally aligned
- generated procedures depended on an extra public helper that was not fundamentally required
- the simpler design is to keep one enqueue implementation and add optional subscriber routing to it

## Impact Analysis

### Epic Impact

- **Epic 1** is affected directly.
- No scope expansion is required.
- This is a design simplification inside an already approved story.

### Story Impact

- **Story `1-2-procedure-generation`** wording must change from `Enqueue_For_Subscriber()` to subscriber-routed `Enqueue_Event___()`.
- **Story `1-4-auto-deploy-package`** references to base package deployment must describe subscriber-routed `Enqueue_Event___()` support instead of a separate helper.

### Artifact Impact

- Updated: `_bmad-output/implementation-artifacts/stories/1-2-procedure-generation.md`
- Updated: `_bmad-output/planning-artifacts/epics.md`
- Updated: `_bmad-output/planning-artifacts/architecture.md`
- Updated: `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Technical Impact

- `assets/sql/Omni_Tracer.sql` now keeps a single enqueue helper.
- Generated procedures call `Enqueue_Event___(..., subscriber_name_ => '<FUNNY_NAME>')`.
- Oracle AQ routing remains subscriber-specific through `recipient_list`.
- Existing `Trace_Message()` and `Trace_Message_To_Webhook()` behavior remains unchanged.

## Recommended Approach

### Chosen Path: Direct Adjustment

Keep one enqueue implementation and extend it.

Rationale:
- less PL/SQL duplication
- easier maintenance
- same runtime behavior
- smaller long-term review surface

Risk:
- low

Timeline impact:
- minimal, same sprint

## Detailed Change Proposals

### Story 1.2

OLD:
- `Procedure Generation with Enqueue_For_Subscriber`
- Generated procedures call `OMNI_TRACER_API.Enqueue_For_Subscriber(...)`

NEW:
- `Procedure Generation with Subscriber-Routed Enqueue`
- Generated procedures call `Enqueue_Event___(..., subscriber_name_ => '<FUNNY_NAME>')`

Rationale:
- reflects the simplified single-helper design

### Architecture

OLD:
- base package must add `Enqueue_For_Subscriber()`

NEW:
- base package must extend `Enqueue_Event___()` with optional `subscriber_name_`

Rationale:
- one enqueue path handles broadcast and subscriber-specific routing

### Sprint Status

OLD:
- Story title referenced `Enqueue_For_Subscriber`

NEW:
- Story title and notes reference subscriber-routed enqueue

Rationale:
- tracking artifacts should match shipped design

## Implementation Handoff

### Scope Classification

- **Minor**

### Handoff

- Developer: refactor PL/SQL helper and generated procedure body
- Developer: update tests
- Developer: update story and architecture artifacts

### Success Criteria

- no remaining `Enqueue_For_Subscriber()` dependency in code or BMAD planning artifacts
- generated procedures still route only to the matching subscriber
- `make test` passes
- `make build` passes

---

# Sprint Change Proposal — 2026-05-09: ORA-24205 Fix (Application-Level Message Routing)

## Issue Summary

Story `1-2-procedure-generation` implemented subscriber routing via Oracle AQ `recipient_list` in `Enqueue_Event___()`. This causes `ORA-24205: feature not supported for sharded queues` because Oracle Sharded Queues / TxEventQ do NOT support the `recipient_list` feature on enqueue.

The `SUBSCRIBER` JSON field was already being embedded in the message payload by `Enqueue_Event___()`. The fix is to remove the broken `recipient_list` line and filter messages at the Go application layer using the existing `SUBSCRIBER` payload field.

## Impact Analysis

### Epic Impact

- **Epic 1** gains one new story (1-5)
- No scope expansion beyond this fix — no new Oracle features, no new queue types

### Story Impact

- **New Story `1-5-application-level-message-routing`** added to Epic 1
- **Story `1-2`** is already done — the generated procedures correctly call `Enqueue_Event___` with `subscriber_name_`, which embeds the `SUBSCRIBER` JSON field. The only broken line is the `recipient_list` assignment.
- No changes to stories 1-3, 1-4, 2-1, 3-1, 3-2

### Artifact Impact

- Updated: `_bmad-output/planning-artifacts/architecture.md` (DEC-6 added)
- Updated: `_bmad-output/planning-artifacts/epics.md` (Story 1-5 + FR-8)
- Created: `_bmad-output/implementation-artifacts/stories/1-5-application-level-message-routing.md`
- Updated: `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Technical Impact

- `assets/sql/Omni_Tracer.sql`: remove 3 lines (`recipient_list` assignment)
- `internal/core/domain/queue_message.go`: add `subscriber` field + JSON
- `internal/service/tracer/tracer_service.go`: add filtering in `processBatch()`
- Zero changes to C code, Oracle adapter, or queue configuration

## Recommended Approach

### Chosen Path: Application-Level Payload Filtering

Route messages by checking the `SUBSCRIBER` field in the JSON payload after dequeue, instead of trying to route at the Oracle queue level.

Rationale:
- Oracle sharded queues broadcast to all subscribers by design
- Filtering in Go is cheap, testable, and transparent
- The `SUBSCRIBER` JSON field is already embedded by `Enqueue_Event___`
- Zero Oracle infrastructure changes needed

Risk:
- Negligible — each subscriber deserializes all messages including non-matching ones. For < 20 subscribers with ephemeral trace messages, CPU cost is trivial.

Timeline impact:
- Estimated 25 lines of code changes across 3 files. Same sprint.
