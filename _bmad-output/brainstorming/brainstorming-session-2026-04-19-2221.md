---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - docs/SUBSCRIBER_ISOLATION_SOLUTION.md
  - docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md
session_topic: 'Per-subscriber message isolation for OmniView/OmniInspect trace messages over Oracle AQ, in environments where many users share the ifsapp Oracle login and/or use personal Oracle accounts while the IFS Cloud application layer has its own user identity.'
session_goals: 'Generate a wide candidate list of architectural approaches (goal: beyond the 5 already drafted), surface cross-cutting mechanisms worth combining, include black-swan/unconventional ideas, capture risks and edge cases, and establish evaluation criteria for later Technical Research. No backward compatibility constraint. Modifying Trace_Message procedure is allowed (package invalidation on redeploy is acceptable).'
selected_approach: 'ai-recommended'
techniques_used:
  - First Principles Thinking
  - Morphological Analysis
  - Cross-Pollination
  - Reverse Brainstorming
  - SCAMPER
ideas_generated: 10
context_file: ''
session_status: 'complete'
---

# Brainstorming Session Results

**Facilitator:** Basuruk
**Date:** 2026-04-19
**Last Updated:** 2026-04-26 (resumed session)

---

## Session Overview

**Topic:** Per-subscriber message isolation for OmniView/OmniInspect trace messages over Oracle AQ, in environments where many users share the `ifsapp` Oracle login and/or use personal Oracle accounts while the IFS Cloud application layer has its own user identity.

**Goals:** Generate a wide candidate list of architectural approaches (goal: beyond the 5 already drafted), surface cross-cutting mechanisms worth combining, include black-swan/unconventional ideas, capture risks and edge cases, and establish evaluation criteria for later Technical Research.

---

## Context Summary

- **Environment:** IFS Cloud app on Oracle 19c backend. Intranet / test environments on internal servers.
- **Identity reality (three-way):**
  1. Oracle DB user OmniInspect subscriber connects as (often `ifsapp`, sometimes a personal account e.g. `basblk`, `tramlk`, `nipwlk`).
  2. Oracle DB user that the IFS Cloud application connects as when it executes code path calling `Omni_Tracer_API.Trace_Message()` (also often `ifsapp`, sometimes personal).
  3. IFS Cloud application user (application-level identity, distinct from the Oracle session user).
- **Scale:** 50+ concurrent users in worst case, mostly funneling through the shared `ifsapp` Oracle account.
- **Current behavior:** `Trace_Message` pushes to an AQ queue; every connected subscriber sees every message (broadcast).
- **Desired behavior:** each message deliverable to exactly the intended subscriber.
- **Degrees of freedom:** May change `Omni_Tracer_API.Trace_Message()` signature/body, queue topology, payload type, subscriber registration. Package invalidation on redeploy is acceptable.

---

## Alternatives Evaluated

| Approach | How it works | Problem for this case |
|----------|-------------|----------------------|
| **IP_ADDRESS filtering** | Capture caller's IP, filter at dequeue | IFS Cloud app server ≠ debugging user's IP |
| **Explicit subscriber parameter** | `Trace_Message('msg', 'INFO', 'SUB_BBB')` | Caller must pass name — human friction |
| **CLIENT_IDENTIFIER** | App sets ID at connection start | IFS Cloud starts its own sessions — OmniView can't pre-set |
| **Session Binding Table** | `Bind_Session()` before `Trace_Message()` | IFS Cloud starts its own sessions — binding doesn't correlate |
| **Rule-based subscriptions** | AQ rules filter by message content | Message still needs subscriber tag |
| **Dynamic procedure per subscriber** | `TRACE_MESSAGE_<name>('msg')` | ✅ No extra parameter, caller just uses their method |

**Decision: Dynamic procedure per subscriber approach is the best option.**

---

## Edge Cases Resolved

| # | Edge Case | Resolution |
|---|-----------|------------|
| 1 | **Name collision** | Use subscriber's existing unique ID for idempotent creation. No prefix needed. |
| 2 | **Oracle identifier limit** | Any name under 30 characters. |
| 3 | **SQL injection** | Strict validation: `^[A-Za-z_]+$` — letters and underscores only, no numbers. |
| 4 | **Package invalidation** | Accepted. App redeploys on restart if package was invalidated. |
| 5 | **Orphaned methods** | On restart: if method exists, skip creation. Danger zone options for selective/all deletion. |
| 6 | **IFS Cloud session crash** | No special handling. Clear error shown → user restarts OmniView → redeploys if needed. |
| 7 | **Permissions** | Any database user can call generated procedures. |
| 8 | **Duplicate registration** | Check and only create if missing. |
| 9 | **Scalability** | N subscribers, no hard limit. |
| 10 | **IFS Cloud compatibility** | No issue — adding procedures doesn't affect existing Trace_Message calls. |

### Additional Resolved Items

1. **Name format**: Less than 30 characters. Format: `^[A-Za-z_]+$` (letters and underscores only, no numbers).

2. **Danger zone options**:
   - Delete subscriber-specific method only (per-subscriber cleanup)
   - Drop entire `OMNI_TRACER_API` package (deletes ALL generated methods for all subscribers)

3. **Auto-redeploy on start**: Already implemented — if `OMNI_TRACER_API` package is missing on startup, OmniView redeploys it.

---

## Key Insights from Brainstorming

1. **The fundamental problem**: Caller (IFS Cloud) and subscriber (OmniView user) are completely decoupled — no shared identity, no shared context, no shared anything.

2. **Why existing solutions fail**: Every approach relying on call-time context (IP, CLIENT_IDENTIFIER, session binding) fails because IFS Cloud controls its own sessions — OmniView cannot influence them.

3. **Why dynamic procedure wins**: Moves subscriber identity to *compile-time* — the human picks a memorable name, embeds it in IFS code. No runtime coordination needed. The method name IS the routing key.

4. **Developer ergonomics**: `TRACE_MESSAGE_barnacle('test')` has same friction as `Trace_Message('test')` — one argument, no extra parameter to remember or pass.

---

## Next Step

**Recommendation:** Move to `bmad-create-architecture` in a fresh session to design the implementation.

---