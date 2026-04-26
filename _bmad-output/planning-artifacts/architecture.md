---
stepsCompleted: [1, 2]
inputDocuments:
  - _bmad-output/brainstorming/brainstorming-session-2026-04-19-2221.md
  - docs/SUBSCRIBER_ISOLATION_SOLUTION.md
  - docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md
  - DESIGN.md
workflowType: 'architecture'
project_name: OmniInspect
user_name: Basuruk
date: '2026-04-26'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Input Documents

| Document | Purpose |
|----------|---------|
| `brainstorming-session-2026-04-19-2221.md` | Solution decisions, edge cases resolved |
| `SUBSCRIBER_ISOLATION_SOLUTION.md` | Original solution options (IP filtering, explicit param, CLIENT_IDENTIFIER, session binding, hybrid) |
| `ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md` | Current architecture, obsolete components, multi-subscriber implementation plan |
| `DESIGN.md` | UX design specification (target visual language, layout rules, component standards, interaction rules) |

---

## Session Context

**Topic:** Per-subscriber message isolation for OmniView/OmniInspect trace messages over Oracle AQ

**Selected Solution:** Dynamic procedure per subscriber (`TRACE_MESSAGE_<name>('msg')`)

**Key Insight:** Moves subscriber identity to compile-time — the method name IS the routing key.

---

## Resolved Decisions (from Brainstorming)

| Decision | Value |
|----------|-------|
| Name format | `< 30 chars, `^[A-Za-z_]+$` (letters and underscores only) |
| Name uniqueness | Use subscriber's existing unique ID (e.g., `SUB_0CC283A4...`) |
| Creation | Idempotent - check if exists, only create if missing |
| SQL injection | Strict format validation before DDL generation |
| Package invalidation | Accepted risk - app redeploys on restart |
| Danger zone options | Per-subscriber method deletion OR drop entire OMNI_TRACER_API package |
| Auto-redeploy | Already implemented - if package missing, OmniView redeploys |
| Permissions | Any database user can call generated procedures |
| Scalability | N subscribers supported, no hard limit |

---

## Problem Summary

IFS Cloud executes trace calls under IFS app user identity, NOT the debugging OmniView user. No way to correlate:
- **Caller**: IFS Cloud session user (producer)
- **Subscriber**: OmniView user who wants to debug (listener)

**Current state:** All subscribers see all messages (broadcast)

**Desired state:** Each message delivered to exactly the intended subscriber

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
- FR-1: Generate unique `TRACE_MESSAGE_<subscriber_id>()` procedure per subscriber on registration
- FR-2: Idempotent creation - check if procedure exists before creating
- FR-3: **Modify Settings UI** - Add danger zone option to drop subscriber-specific procedure
- FR-4: **Modify Settings UI** - Add danger zone option to drop entire OMNI_TRACER_API package
- FR-5: Auto-redeploy package on startup if missing
- FR-6: Strict name format validation (`^[A-Za-z_]+$`) to prevent SQL injection
- FR-7: Display the subscriber's method name in TUI - Show the user the exact procedure they must call in their PL/SQL code to receive subscriber-isolated messages (e.g., `OMNI_TRACER_API.TRACE_MESSAGE_SUB_0CC283A4...`)

**Non-Functional Requirements:**
- NFR-1: Package invalidation is acceptable - app recovers on restart
- NFR-2: Any database user can call generated procedures
- NFR-3: Support N concurrent subscribers (no hard limit)
- NFR-4: Developer ergonomics - `TRACE_MESSAGE_xxx('msg')` same friction as `Trace_Message('msg')`

**Scale & Complexity:**
- Primary domain: Real-time TUI + Oracle AQ messaging
- Complexity level: Medium (feature enhancement to existing working system)
- Estimated new/modified components: Settings screen modification (2-3 new danger zone options)
- UI patterns: S key opens Settings (already implemented), webhook address currently saved there

### Technical Constraints & Dependencies
- Go + Bubble Tea v2 + Lip Gloss v2 (existing)
- Oracle 19c with ODPI-C for database connectivity
- Existing `OMNI_TRACER_API` package must be extended
- Existing hexagonal architecture must be respected
- Settings screen already exists at `internal/adapter/ui/database_settings.go`

### Cross-Cutting Concerns Identified
- SQL injection prevention via format validation
- Package invalidation recovery mechanism (already implemented)
- Backward compatibility with existing `Trace_Message()` callers

---

## Next Step

[C] Continue to evaluate starter templates

