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
