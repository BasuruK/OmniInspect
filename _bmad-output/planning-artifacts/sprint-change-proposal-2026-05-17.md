---
stepsCompleted: [1, 2, 3, 4, 5, 6]
status: 'approved'
workflowType: 'correct-course'
trigger: 'User concern about security risk of FR-4 (drop entire OMNI_TRACER_API package) feature'
date: '2026-05-17'
project_name: OmniInspect
user_name: Basuruk
---

# Sprint Change Proposal — Park FR-4 / Story 3-2

## 1. Issue Summary

**Problem Statement**: The "Drop Entire Package" feature (FR-4 / Story 3-2) poses unacceptable security and operational risk. If one subscriber drops the `OMNI_TRACER_API` package, ALL subscribers lose their procedures immediately — affecting active users who haven't triggered the action.

**Context**:
- Epic 1 completed — per-subscriber procedure generation working ✅
- Story 3-1 completed — drop subscriber-specific procedure ✅
- Story 3-2 (drop entire package) was planned but never implemented
- User raised concern: giving non-admin users the ability to drop the entire package is dangerous

**Evidence**: User's direct observation during Epic 3 planning review — "I have a concern if we should give that feature. I feel like its a security risk giving access to such features to users."

---

## 2. Impact Analysis

### Epic Impact

| Epic | Status | Change |
|------|--------|--------|
| Epic 1 | ✅ COMPLETE | None — fully implemented |
| Epic 2 | ✅ COMPLETE | None — Story 2-1 done |
| Epic 3 | In Progress | Story 3-2 REMOVED; Stories 3-1 (done) and 3-3 (pending) remain viable |

**Epic 3 remains viable** — the primary danger zone use case (delete my procedure) is satisfied by Story 3-1. FR-9 (safe unregistration on shutdown) is independent and unaffected.

### Story Impact

| Story | Status | Change |
|-------|--------|--------|
| 3-1 | ✅ DONE | Unaffected — drop subscriber-specific procedure |
| 3-2 | 🛑 REMOVED | Never implemented — removed from plan |
| 3-3 | Pending | Unaffected — safe unregistration on shutdown |

### Artifact Conflicts

| Artifact | Conflict | Resolution |
|----------|---------|------------|
| `planning-artifacts/epics.md` | FR-4 listed, Story 3-2 defined | Remove FR-4, remove Story 3-2 section |
| `planning-artifacts/architecture.md` | Pattern 4 describes two danger zone options | Update to single option (drop my procedure); note FR-4 parked |
| `implementation-artifacts/sprint-status.yaml` | Story 3-2 status `ready-for-dev` | Mark as `cancelled` |
| `implementation-artifacts/stories/3-2-drop-entire-package.md` | File exists but story never implemented | Mark as deprecated |

### Technical Impact

**None** — Story 3-2 was never implemented. No code removal required.

---

## 3. Recommended Approach

**Selected Path**: Direct Adjustment — scope reduction by removing one feature from the plan.

**Rationale**:
- Story 3-2 was never built — no code to remove
- Epic 3 remains viable with Stories 3-1 and 3-3
- FR-3 (drop my procedure) satisfies the primary user need
- Risk of accidental mass deletion is eliminated
- Low effort — artifact updates only

**Alternatives Considered**:
- Option 2 (Rollback): N/A — Story 3-2 was never implemented
- Option 3 (MVP Review): Overkill — removing one feature doesn't require fundamental replan

**Effort**: Low (artifact updates only, ~1 hour)
**Risk**: Low (no code removal, just documentation)
**Timeline Impact**: None — Story 3-2 was not on the critical path

---

## 4. Detailed Change Proposals

### Change 4.1: `planning-artifacts/epics.md`

**Requirements Inventory — remove FR-4:**
```
REMOVE:
**FR-4:** Modify Settings UI - Add danger zone option to drop entire OMNI_TRACER_API package
```

**FR Coverage Map — remove FR-4 row:**
```
REMOVE row:
| FR-4 | Drop entire OMNI_TRACER_API package | Epic 3 |
```

**Epic 3 description — update user outcome and FR coverage:**
```
OLD:
### Epic 3: Danger Zone Implementation

**User Outcome:** Subscribers can clean up their procedures or the entire package when needed, and the application safely unregisters from Oracle AQ on shutdown.

**FRs Covered:** FR-3, FR-4, FR-9

NEW:
### Epic 3: Danger Zone Implementation

**User Outcome:** Subscribers can clean up their own procedures and safely unregister from Oracle AQ on shutdown.

**FRs Covered:** FR-3, FR-9
```

**Story 3.2 — remove entire section:**
```
REMOVE:
### Story 3.2: Drop Entire Package (Danger Zone)

As a subscriber,
I want to drop the entire OMNI_TRACER_API package,
So that I can remove all generated procedures at once.

[Full acceptance criteria and technical details — remove entirely]
```

---

### Change 4.2: `planning-artifacts/architecture.md`

**Requirements Overview — update FR-3, remove FR-4:**
```
OLD:
- FR-3: **Modify Settings UI** - Add danger zone option to drop subscriber-specific procedure
- FR-4: **Modify Settings UI** - Add danger zone option to drop entire OMNI_TRACER_API package

NEW:
- FR-3: **Modify Settings UI** - Add danger zone option to drop subscriber-specific procedure
- (FR-4 REMOVED: Package-drop poses unacceptable risk — accidental deletion affects all subscribers)
```

**Pattern 4: Settings UI Danger Zone — update description:**
```
OLD:
Danger zone section in Settings screen:
- Visual distinction (red/warning styling)
- Confirmation required before destructive actions
- Two options: Drop subscriber procedure OR Drop entire package

NEW:
Danger zone section in Settings screen:
- Visual distinction (red/warning styling)
- Confirmation required before destructive actions
- ONE option: Drop subscriber-specific procedure only

NOTE: Drop entire OMNI_TRACER_API package (FR-4) has been PARKED due to risk concerns.
If needed in the future, it should be implemented as an admin-only operation with separate access controls.
```

**Requirements to Structure Mapping — remove FR-4:**
```
OLD:
| FR-3, FR-4: Danger zone options | `internal/adapter/ui/database_settings.go` | **MODIFY** - Add "Drop my procedure" + "Drop all procedures" |

NEW:
| FR-3: Danger zone options | `internal/adapter/ui/database_settings.go` | **MODIFY** - Add "Drop my procedure" |
```

---

### Change 4.3: `implementation-artifacts/sprint-status.yaml`

**Story 3-2 status change:**
```
OLD:
  3-2-drop-entire-package:
    status: "ready-for-dev"
    title: "Drop Entire Package (Danger Zone)"
    epic: "Epic 3: Danger Zone Implementation"
    owner: "dev-agent"
    last_updated: "2026-04-28"
    notes: "Add danger zone option to drop entire OMNI_TRACER_API package"

NEW:
  3-2-drop-entire-package:
    status: "cancelled"
    title: "Drop Entire Package (Danger Zone) — PARKED"
    epic: "Epic 3: Danger Zone Implementation"
    owner: "dev-agent"
    last_updated: "2026-05-17"
    notes: "PARKED: Security risk — package drop affects all subscribers, not just the requestor. FR-4 removed from plan. Feature may be revisited as admin-only operation with proper access controls."
```

**Metrics update:**
```
OLD:
  total_stories: 9
  completed: 6
  ready_for_dev: 3

NEW:
  total_stories: 9
  completed: 6
  ready_for_dev: 2
  cancelled: 1
```

**Epic 3 summary update:**
```
OLD:
  epic-3:
    title: "Epic 3: Danger Zone Implementation"
    stories_total: 3
    stories_completed: 1
    stories_in_progress: 0
    status: "in-progress"

NEW:
  epic-3:
    title: "Epic 3: Danger Zone Implementation"
    stories_total: 3
    stories_completed: 1
    stories_in_progress: 0
    stories_cancelled: 1
    status: "in-progress"
```

---

## 5. Implementation Handoff

**Change Scope**: Minor — artifact updates only, no code changes

**Handoff**: Developer agent (bmad-dev-story) can apply directly

**Artifacts to Modify**:
1. `_bmad-output/planning-artifacts/epics.md`
2. `_bmad-output/planning-artifacts/architecture.md`
3. `_bmad-output/implementation-artifacts/sprint-status.yaml`

**Files to Deprecate**:
- `stories/3-2-drop-entire-package.md` — rename to `stories/3-2-drop-entire-package.md.deprecated` or add note

**Success Criteria**:
- epics.md no longer references FR-4 or Story 3.2
- architecture.md Pattern 4 reflects single danger zone option
- sprint-status.yaml shows Story 3-2 as `cancelled`
- Epic 3 remains viable with Stories 3-1 (done) and 3-3 (pending)

---

## 6. Resolution

**Approved by**: Basuruk
**Date**: 2026-05-17
**Approach**: Option 1 — Direct Adjustment (scope reduction)

**Next Steps**:
1. Apply artifact changes as detailed above
2. Epic 3 continues with remaining stories (3-1 done, 3-3 pending)
3. FR-4 may be revisited in future as admin-only operation with proper access controls

---

*Correct Course workflow complete — Basuruk!*