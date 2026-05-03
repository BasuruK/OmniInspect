# Workflow: Multi-Subscriber Message Isolation - Brainstorming to Architecture

## Context

**Source**: Brainstorming session (`_bmad-output/brainstorming/brainstorming-session-2026-04-19-2221.md`)

**Problem**: OmniView/OmniInspect trace messages broadcast to ALL subscribers instead of being delivered to the intended subscriber. IFS Cloud executes trace calls under IFS app user identity, NOT the debugging OmniView user - making it impossible to correlate caller and subscriber.

**Selected Solution**: Dynamic procedure per subscriber - `TRACE_MESSAGE_<name>('msg')` embeds subscriber identity at compile-time. No runtime coordination, no extra parameters.

---

## Pre-Architecture Checklist

- [x] Brainstorming session complete
- [x] Solution approach selected: Dynamic procedure per subscriber
- [x] Edge cases resolved (see below)
- [ ] Architecture design created via `bmad-create-architecture`
- [ ] Stories created via `bmad-create-epics-and-stories`

---

## Resolved Decisions (from Brainstorming)

### Name & Format
- **Name format**: `< 30 chars, `^[A-Za-z_]+$` (letters and underscores only, no numbers)
- **Name uniqueness**: Use subscriber's existing unique ID (e.g., `SUB_0CC283A4...`)
- **Idempotent creation**: Check if exists, only create if missing

### SQL Injection Prevention
- Strict format validation: `^[A-Za-z_]+$`
- Subscriber name comes from internal ID, not user input

### Package Invalidation
- Accepted risk
- App has recovery mechanism (redeploys on restart if package invalidated)

### Danger Zone Options
1. Delete subscriber-specific method only (per-subscriber cleanup)
2. Drop entire `OMNI_TRACER_API` package (deletes ALL generated methods)

### Auto-Redeploy
- Already implemented - if `OMNI_TRACER_API` package missing on startup, OmniView redeploys

### Permissions
- Any database user can call generated procedures

### Scalability
- N subscribers supported, no hard limit

---

## Key Insight

> "Dynamic procedure wins: Moves subscriber identity to **compile-time** — the human picks a memorable name, embeds it in IFS code. No runtime coordination needed. The method name IS the routing key."

Developer ergonomics: `TRACE_MESSAGE_barnacle('test')` has same friction as `Trace_Message('test')` — one argument, no extra parameter.

---

## How Dynamic Procedure Works

```
1. Client registers → OmniView generates & deploys TRACE_MESSAGE_SUB_XXX()
2. IFS developer calls OMNI_TRACER_API.TRACE_MESSAGE_SUB_XXX('debug msg')
3. Procedure internally enqueues with recipient_list = 'SUB_XXX'
4. Only SUB_XXX's OmniView instance dequeues that message
```

---

## Input Documents for Architecture

| Document | Purpose |
|----------|---------|
| `docs/SUBSCRIBER_ISOLATION_SOLUTION.md` | Original solution options (IP filtering, explicit param, CLIENT_IDENTIFIER, session binding, hybrid) |
| `docs/ARCHITECTURE_AND_MULTI_SUBSCRIBER_PLAN.md` | Current architecture, obsolete components, multi-subscriber implementation plan |
| `_bmad-output/brainstorming/brainstorming-session-2026-04-19-2221.md` | This session's decisions |

---

## Next Steps

1. **Run `bmad-create-architecture`** to design the implementation for:
   - PL/SQL changes to `OMNI_TRACER_API` package (generate/drop procedures)
   - Go-side changes to call new procedure naming convention
   - Subscriber registration flow changes
   - Danger zone UI implementation

2. **Run `bmad-create-epics-and-stories`** after architecture is complete

3. **Implement stories** via `bmad-dev-story`
