---
stepsCompleted: [1, 2, 3, 4]
status: 'complete'
inputDocuments:
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/brainstorming/brainstorming-session-2026-04-19-2221.md
  - docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md
  - docs/SUBSCRIBER_ISOLATION_SOLUTION.md
  - DESIGN.md
workflowType: 'epics-and-stories'
project_name: OmniInspect
user_name: Basuruk
date: '2026-04-27'
---

# OmniInspect - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for OmniInspect, decomposing the requirements from the Architecture and design decisions into implementable stories.

## Requirements Inventory

### Functional Requirements

**FR-1:** Generate unique `TRACE_MESSAGE_<subscriber_id>()` procedure per subscriber on registration

**FR-2:** Idempotent creation - check if procedure exists before creating

**FR-3:** Modify Settings UI - Add danger zone option to drop subscriber-specific procedure

**FR-4:** Modify Settings UI - Add danger zone option to drop entire OMNI_TRACER_API package

**FR-5:** Auto-redeploy package on startup if missing

**FR-6:** Strict name format validation (`^[A-Za-z_]+$`) to prevent SQL injection

**FR-7:** Display the subscriber's method name in TUI - Show the user the exact procedure they must call in their PL/SQL code to receive subscriber-isolated messages (e.g., `OMNI_TRACER_API.TRACE_MESSAGE_SUB_0CC283A4...`)

### NonFunctional Requirements

**NFR-1:** Package invalidation is acceptable - app recovers on restart

**NFR-2:** Any database user can call generated procedures

**NFR-3:** Support N concurrent subscribers (no hard limit)

**NFR-4:** Developer ergonomics - `TRACE_MESSAGE_xxx('msg')` same friction as `Trace_Message('msg')`

### Additional Requirements

- **Funny Name System:** Auto-assign curated cartoon character names (e.g., Mickey, Donald, Bugs, Daffy, Scooby, Tom, Jerry) to subscribers for procedure naming
- **Name Collision Handling:** System automatically picks another available name if collision occurs
- **Enqueue_For_Subscriber():** Must be added to base OMNI_TRACER_API package as prerequisite for generated procedures
- **SQL Injection Prevention:** Strict format validation on all funny names before DDL generation
- **Idempotent Procedure Creation:** Check if procedure exists before creating (skip if already exists)
- **Backwards Compatibility:** Existing `Trace_Message()` callers remain unaffected

### UX Design Requirements

**UX-DR1:** Display subscriber's procedure name (`OMNI_TRACER_API.TRACE_MESSAGE_<NAME>('msg')`) in Main Screen header with visual emphasis

**UX-DR2:** Add danger zone section in Settings screen with:
- Clear visual distinction (red/warning styling)
- Confirmation required before destructive actions
- Option to drop subscriber-specific procedure
- Option to drop entire OMNI_TRACER_API package

**UX-DR3:** Settings screen keyboard shortcut (S key) already implemented - verify danger zone integration works correctly

### FR Coverage Map

| FR | Description | Epic |
|----|-------------|------|
| FR-1 | Generate TRACE_MESSAGE_<id>() procedure per subscriber | Epic 1 |
| FR-2 | Idempotent creation | Epic 1 |
| FR-3 | Drop subscriber-specific procedure | Epic 3 |
| FR-4 | Drop entire OMNI_TRACER_API package | Epic 3 |
| FR-5 | Auto-redeploy on startup | Epic 1 |
| FR-6 | Strict name format validation | Epic 1 |
| FR-7 | Display procedure name in TUI | Epic 2 |

## Epic List

### Epic 1: Multi-Subscriber Procedure Generation

**User Outcome:** IFS developers can call subscriber-specific procedures that route messages to only their OmniView instance.

**FRs Covered:** FR-1, FR-2, FR-5, FR-6

### Epic 2: TUI Procedure Name Display

**User Outcome:** Developers can see exactly which PL/SQL procedure to call in their code.

**FRs Covered:** FR-7

### Epic 3: Danger Zone Implementation

**User Outcome:** Subscribers can clean up their procedures or the entire package when needed.

**FRs Covered:** FR-3, FR-4

## Epic 1: Multi-Subscriber Procedure Generation

### Epic Goal

Implement dynamic procedure generation system that creates per-subscriber `TRACE_MESSAGE_<name>()` procedures inside the OMNI_TRACER_API package, enabling message isolation between subscribers.

## Epic 2: TUI Procedure Name Display

### Epic Goal

Display each subscriber's unique procedure name in the TUI header, making it easy for developers to know exactly which PL/SQL procedure to call.

## Epic 3: Danger Zone Implementation

### Epic Goal

Add Settings UI options for subscribers to clean up their procedure or drop the entire OMNI_TRACER_API package.

<!-- Repeat for each story (M = 1, 2, 3...) within epic N -->

### Story 1.1: Funny Name Value Object

As a system,
I want to auto-assign funny cartoon character names to subscribers,
So that procedure names are memorable and unique.

**Acceptance Criteria:**

**Given** a new subscriber registers with OmniView
**When** the system generates their procedure
**Then** a funny name (e.g., BARNACLE, PICKLES, NIBBLES) is automatically assigned from the curated list
**And** the resulting procedure is `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`

**Given** a funny name is already assigned to another subscriber
**When** a new subscriber registers
**Then** the system picks another available name to avoid collision
**And** no two subscribers have the same procedure name

---

### Story 1.2: Procedure Generation with Enqueue_For_Subscriber

As a system,
I want to generate `TRACE_MESSAGE_<name>()` procedures that call `Enqueue_For_Subscriber()`,
So that messages are routed to the correct subscriber.

**Acceptance Criteria:**

**Given** a subscriber with name BARNACLE is registered
**When** OmniView generates their procedure
**Then** the procedure `TRACE_MESSAGE_BARNACLE(message_, log_level_)` is created inside OMNI_TRACER_API package
**And** it calls `OMNI_TRACER_API.Enqueue_For_Subscriber(subscriber_name_ => 'BARNACLE', message_ => message_, log_level_ => log_level_)`

**Given** the subscriber's procedure already exists
**When** OmniView starts
**Then** no new procedure is created (idempotent)

---

### Story 1.3: SQL Injection Prevention

As a system,
I want to validate all funny names against a strict format before DDL generation,
So that SQL injection attacks are prevented.

**Acceptance Criteria:**

**Given** a name from the funny name list
**When** validation runs
**Then** it passes if it matches `^[A-Za-z_]+$` (letters and underscores only)
**And** it fails if it contains numbers, special characters, or spaces

**Given** an invalid name somehow reaches the DDL generator
**When** the system attempts to create a procedure
**Then** an error is returned and no procedure is created

---

### Story 1.4: Auto-Deploy Package on Startup

As a system,
I want to check if OMNI_TRACER_API package exists on startup,
So that I can redeploy it if missing.

**Acceptance Criteria:**

**Given** OmniView starts
**When** the system checks for OMNI_TRACER_API package
**Then** if missing, it deploys the base package with `Enqueue_For_Subscriber()`
**And** it then generates all subscriber procedures

**Given** OmniView starts and OMNI_TRACER_API package exists
**When** the system checks
**Then** it skips deployment and uses existing package
**And** it generates any missing subscriber procedures

---

### Story 2.1: Display Procedure Name in Header

As an IFS developer,
I want to see my procedure name in the OmniView header,
So that I know exactly which PL/SQL to call in my code.

**Acceptance Criteria:**

**Given** a subscriber named BARNACLE is registered
**When** the Main Screen displays
**Then** the header shows `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg')`
**And** the name is visually prominent (e.g., different color, bold)

---

### Story 3.1: Drop Subscriber Procedure (Danger Zone)

As a subscriber,
I want to delete my specific procedure,
So that I can clean up when I no longer need tracing.

**Acceptance Criteria:**

**Given** the subscriber is on the Settings screen
**When** they select "Delete My Procedure"
**Then** a confirmation dialog appears warning this action
**And** if confirmed, the `TRACE_MESSAGE_<name>()` procedure is removed from OMNI_TRACER_API package

**Given** the procedure is deleted
**When** OmniView restarts
**Then** if the subscriber is still registered, the procedure is regenerated

---

### Story 3.2: Drop Entire Package (Danger Zone)

As a subscriber,
I want to drop the entire OMNI_TRACER_API package,
So that I can remove all generated procedures at once.

**Acceptance Criteria:**

**Given** the subscriber is on the Settings screen
**When** they select "Drop All Procedures"
**Then** a confirmation dialog appears with strong warning
**And** if confirmed, the entire OMNI_TRACER_API package is dropped from the database

**Given** the package is dropped
**When** OmniView restarts
**Then** it redeploys the base package with `Enqueue_For_Subscriber()`
**And** it regenerates all subscriber procedures
