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

The fix uses Oracle AQ's built-in `correlation` property and subscriber rules. `Enqueue_Event___` sets `message_properties_.correlation := subscriber_name_`. `Register_Subscriber` registers with rule `tab.CORRELATION IS NULL OR tab.CORRELATION = '<name>'`. This gives us both broadcast (NULL correlation → all subscribers) and subscriber-specific (non-NULL correlation → matching subscriber only) routing at the Oracle queue level with zero Go code changes.

## Impact Analysis

### Epic Impact

- **Epic 1** gains one new story (1-5: Correlation-Based Message Routing)
- **Epic 3** gains one new story (3-3: Safe Subscriber Unregistration on Shutdown)
- No scope expansion beyond these — the PL/SQL changes are already implemented

### Story Impact

- **New Story `1-5-correlation-based-message-routing`** added to Epic 1 (PL/SQL already done)
- **New Story `3-3-safe-subscriber-unregistration`** added to Epic 3 (future work)
- **Story `1-2`** is done — generated procedures correctly call `Enqueue_Event___` with `subscriber_name_`
- No changes to stories 1-3, 1-4, 2-1, 3-1, 3-2

### Artifact Impact

- Updated: `_bmad-output/planning-artifacts/architecture.md` (DEC-6 — correlation-based routing)
- Updated: `_bmad-output/planning-artifacts/epics.md` (Story 1.5, Story 3.3, FR-8, FR-9)
- Created: `_bmad-output/implementation-artifacts/stories/1-5-correlation-based-message-routing.md`
- Updated: `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Technical Impact

- `assets/sql/Omni_Tracer.sql`: `Enqueue_Event___` — replaced `recipient_list` with `message_properties_.correlation := subscriber_name_`
- `assets/sql/Omni_Tracer.sql`: `Register_Subscriber` — added `rule => 'tab.CORRELATION IS NULL OR tab.CORRELATION = ''<name>'''`
- **Zero Go code changes** — routing is handled entirely at the Oracle queue level
- **Zero C code changes**

## Recommended Approach

### Chosen Path: Correlation-Based Subscriber Rules

Route messages using Oracle AQ's `correlation` property and subscriber rule matching, instead of `recipient_list` (unsupported) or application-level filtering (unnecessary).

Rationale:
- Oracle AQ natively supports correlation on sharded queues
- Subscriber rules evaluated at queue level — no Go filtering needed
- Broadcast via NULL correlation + `IS NULL` rule — clean and simple
- Already implemented and tested by Basuruk

Risk: None — tested and working.

Timeline impact: PL/SQL changes already done. Story 1-5 is verification only.
