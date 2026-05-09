---
story_key: "1-5-correlation-based-message-routing"
epic_key: "epic-1"
title: "Correlation-Based Message Routing"
status: "complete"
priority: "critical"
created_date: "2026-05-09"
last_updated: "2026-05-09"
---

# ==========================================
# STORY DEFINITION
# ==========================================

## Story

As a subscriber,
I want messages sent via my `TRACE_MESSAGE_<FUNNY_NAME>()` procedure to only appear in my OmniView instance,
So that I only see trace messages intended for me, while still seeing broadcast messages from `Trace_Message()`.

## Background

Oracle Sharded Queues do NOT support `recipient_list` on enqueue (`ORA-24205: feature not supported for sharded queues`). The fix uses Oracle AQ's built-in `correlation` property and subscriber rules to route messages at the queue level — no Go code changes needed.

See Architecture Decision DEC-6 for full rationale and rejected alternatives.

## Acceptance Criteria

**Given** a message was enqueued via `TRACE_MESSAGE_BARNACLE('msg')`
**When** subscriber BARNACLE's OmniView instance dequeues the message
**Then** the message is displayed in BARNACLE's TUI

**Given** a message was enqueued via `TRACE_MESSAGE_BARNACLE('msg')`
**When** subscriber PEBBLES's OmniView instance dequeues the message
**Then** the message is NOT delivered to PEBBLES (Oracle AQ filters it via subscriber rule)

**Given** a message was enqueued via `Trace_Message('msg')` (broadcast, NULL correlation)
**When** any subscriber's OmniView instance dequeues the message
**Then** the message is displayed in all subscribers' TUIs (NULL correlation matches the `IS NULL` part of the rule)

---

## Tasks/Subtasks

### Task 1: Set correlation on enqueue in `Enqueue_Event___`

**File**: `assets/sql/Omni_Tracer.sql` — `Enqueue_Event___` procedure

**ALREADY DONE** — Verify the following line is present (added by Basuruk):
```sql
message_properties_.correlation := subscriber_name_;
```

And the old `recipient_list` lines are removed or commented out:
```sql
-- IF subscriber_name_ IS NOT NULL THEN
--     message_properties_.recipient_list(1) := SYS.AQ$_AGENT(subscriber_name_, NULL, NULL);
-- END IF;
```

- [ ] Verify `message_properties_.correlation := subscriber_name_` is present in `Enqueue_Event___`
- [ ] Verify `recipient_list` assignment is removed/commented out

### Task 2: Add correlation rule to `Register_Subscriber`

**File**: `assets/sql/Omni_Tracer.sql` — `Register_Subscriber` procedure

**ALREADY DONE** — Verify the subscriber registration includes the correlation rule. The rule MUST be:
```sql
rule => 'tab.CORRELATION IS NULL OR tab.CORRELATION = ''' || subscriber_name_ || ''''
```

This ensures:
- NULL correlation (broadcast from `Trace_Message()`) → delivered to ALL subscribers
- Non-NULL correlation (from `TRACE_MESSAGE_<NAME>()`) → delivered ONLY to matching subscriber

- [ ] Verify `Register_Subscriber` includes the `tab.CORRELATION IS NULL OR tab.CORRELATION = '<name>'` rule
- [ ] Verify the rule handles both broadcast (NULL) and subscriber-specific (non-NULL) messages

### Task 3: Verify the `SUBSCRIBER` JSON field is still embedded (informational)

**File**: `assets/sql/Omni_Tracer.sql` — `Enqueue_Event___` procedure

The `SUBSCRIBER` JSON field in the payload was informational metadata (not used for routing). Verify it's present:

```sql
IF subscriber_name_ IS NOT NULL THEN
    message_.PUT('SUBSCRIBER', subscriber_name_);
END IF;
```

**Note:** This field is NOT consumed by dequeuing — routing is handled entirely by Oracle AQ `correlation` + subscriber rules. Nothing reads SUBSCRIBER from the JSON payload. Therefore this task is informational-only and marked N/A.

- [x] ~~Verify `SUBSCRIBER` JSON field embedding is still present in `Enqueue_Event___`~~ — **N/A: Field not consumed by dequeuing, routing via correlation only**

### Task 4: Redeploy and test

- [ ] Deploy the updated `OMNI_TRACER_API` package to a test database
- [ ] Register two subscribers (e.g., BARNACLE and PEBBLES) via the app or manually
- [ ] Enqueue via `TRACE_MESSAGE_BARNACLE('test')` — verify ONLY BARNACLE receives it
- [ ] Enqueue via `Trace_Message('broadcast')` — verify BOTH subscribers receive it
- [ ] Run `make build` — verify no compilation errors
- [ ] Run `make test` — verify all Go tests pass

---

## Technical Notes

### Files Changed

| File | Change Type | Lines Changed |
|------|-------------|---------------|
| `assets/sql/Omni_Tracer.sql` | `Enqueue_Event___`: remove `recipient_list`, add `correlation` | ~4 lines (already done) |
| `assets/sql/Omni_Tracer.sql` | `Register_Subscriber`: add correlation rule to `ADD_SUBSCRIBER` | ~1 line (already done) |

### Files NOT Changed

- **No Go code changes** — routing is handled entirely at the Oracle queue level
- `internal/core/domain/queue_message.go` — no `subscriber` field needed for routing
- `internal/service/tracer/tracer_service.go` — no `processBatch` filtering needed
- `internal/adapter/storage/oracle/dequeue_ops.c` — no C changes
- `internal/adapter/storage/oracle/oracle_adapter.go` — no adapter changes

### How It Works

```
TRACE_MESSAGE_BARNACLE('msg')
  → Enqueue_Event___(subscriber_name_ => 'BARNACLE')
    → message_properties_.correlation := 'BARNACLE'
    → DBMS_AQ.ENQUEUE(...)

Oracle AQ evaluates subscriber rules:
  BARNACLE rule: tab.CORRELATION IS NULL OR tab.CORRELATION = 'BARNACLE'  → MATCH
  PEBBLES rule:  tab.CORRELATION IS NULL OR tab.CORRELATION = 'PEBBLES'  → NO MATCH

Result: Only BARNACLE dequeues this message.

Trace_Message('broadcast')
  → Enqueue_Event___(subscriber_name_ => NULL)
    → message_properties_.correlation := NULL
    → DBMS_AQ.ENQUEUE(...)

Oracle AQ evaluates subscriber rules:
  BARNACLE rule: tab.CORRELATION IS NULL OR ...  → MATCH (IS NULL)
  PEBBLES rule:  tab.CORRELATION IS NULL OR ...  → MATCH (IS NULL)

Result: Both subscribers dequeue this message.
```

### Subscriber Identity Model

| Identifier | Example | Scope | Purpose |
|------------|---------|-------|---------|
| UUID Name (`name`) | `SUB_825418F0_...` | Go / BoltDB | Internal identity — BoltDB storage key, stable across restarts |
| FunnyName (`funnyName`) | `BARNACLE` | Oracle AQ | Oracle consumer name, correlation value, generated procedure name, subscriber rule |

`ConsumerName()` returns FunnyName when assigned (always after registration). The UUID is NOT redundant — it serves as the Go-side identity while FunnyName serves as the Oracle-side identity.

---

## Dependencies

- **Depends on**: Story 1-2 (Procedure Generation) — generated procedures must call `Enqueue_Event___` with `subscriber_name_`
- **Blocks**: Story 2-1 (Display Procedure Name in Header) — routing must work before the UI feature is useful
