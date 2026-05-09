---
story_key: "1-5-application-level-message-routing"
epic_key: "epic-1"
title: "Application-Level Message Routing (Payload Filtering)"
status: "ready-for-dev"
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

Oracle Sharded Queues do NOT support `recipient_list` on enqueue (`ORA-24205: feature not supported for sharded queues`). The original design in Story 1-2 assumed `message_properties_.recipient_list` could route messages to specific subscribers at the queue level. This story fixes the routing by moving message filtering to the Go application layer, using the `SUBSCRIBER` JSON field that `Enqueue_Event___()` already embeds in the payload.

See Architecture Decision DEC-6 for full rationale and rejected alternatives.

## Acceptance Criteria

**Given** a message was enqueued via `TRACE_MESSAGE_BARNACLE('msg')`
**When** subscriber BARNACLE's OmniView instance dequeues the message
**Then** the message is displayed in BARNACLE's TUI

**Given** a message was enqueued via `TRACE_MESSAGE_BARNACLE('msg')`
**When** subscriber PEBBLES's OmniView instance dequeues the message
**Then** the message is silently discarded (not displayed)

**Given** a message was enqueued via `Trace_Message('msg')` (broadcast, no SUBSCRIBER field)
**When** any subscriber's OmniView instance dequeues the message
**Then** the message is displayed in all subscribers' TUIs

---

## Tasks/Subtasks

### Task 1: Remove `recipient_list` from PL/SQL Enqueue_Event___

**File**: `assets/sql/Omni_Tracer.sql`

Remove 3 lines from `Enqueue_Event___` procedure that set `message_properties_.recipient_list`:

```sql
-- REMOVE these 3 lines (around line 232-234):
IF subscriber_name_ IS NOT NULL THEN
    message_properties_.recipient_list(1) := SYS.AQ$_AGENT(subscriber_name_, NULL, NULL);
END IF;
```

The `SUBSCRIBER` JSON field embedding (around line 268-270) must be KEPT:
```sql
-- KEEP this block (it already exists):
IF subscriber_name_ IS NOT NULL THEN
    message_.PUT('SUBSCRIBER', subscriber_name_);
END IF;
```

- [ ] Remove recipient_list assignment from Enqueue_Event___
- [ ] Verify SUBSCRIBER JSON field embedding is still present
- [ ] Redeploy package to test database

### Task 2: Add `subscriber` field to QueueMessage domain entity

**File**: `internal/core/domain/queue_message.go`

- [ ] Add `subscriber string` private field to `QueueMessage` struct
- [ ] Add `subscriber string` parameter to `NewQueueMessage` constructor (use variadic or option pattern to maintain backward compatibility)
- [ ] Add `Subscriber() string` getter method
- [ ] Add `Subscriber string` field to `queueMessageJSON` struct with JSON tag `"subscriber"`
- [ ] Update `UnmarshalJSON` to read `j.Subscriber` and pass to constructor (or set directly)
- [ ] Update `MarshalJSON` to include subscriber field
- [ ] Update existing tests if any reference `NewQueueMessage` signature

### Task 3: Filter messages in TracerService.processBatch

**File**: `internal/service/tracer/tracer_service.go`

In the `processBatch()` method, after JSON unmarshal and before `handleTracerMessage()`:

```go
// After unmarshal:
msg := &domain.QueueMessage{}
if err := json.Unmarshal([]byte(messages[i]), msg); err != nil {
    log.Printf("failed to unmarshal message ID %s: %v", msgIDs[i], err)
    continue
}

// NEW: Application-level routing filter (DEC-6)
// Skip messages targeted at a different subscriber
if msg.Subscriber() != "" && !strings.EqualFold(msg.Subscriber(), subscriber.FunnyName()) {
    continue
}

// Deliver while holding lock to preserve ordering
if !ts.handleTracerMessage(ctx, msg) {
    return ctx.Err()
}
```

- [ ] Add subscriber filtering logic after JSON unmarshal in processBatch
- [ ] Use case-insensitive comparison (funny names are uppercased in PL/SQL)
- [ ] Add `"strings"` import if not already present

### Task 4: Write tests

- [ ] Unit test: QueueMessage with subscriber field unmarshals correctly
- [ ] Unit test: QueueMessage without subscriber field unmarshals correctly (backward compat)
- [ ] Unit test: Subscriber() getter returns correct value
- [ ] Integration consideration: processBatch filtering logic (if testable in isolation)

### Task 5: Build and verify

- [ ] Run `make build` — verify no compilation errors
- [ ] Run `make test` — verify all tests pass
- [ ] Manual test: deploy updated package, enqueue via TRACE_MESSAGE_<NAME>, verify only matching subscriber sees it
- [ ] Manual test: enqueue via Trace_Message, verify all subscribers see it

---

## Technical Notes

### Files Changed

| File | Change Type | Lines Changed (est.) |
|------|-------------|---------------------|
| `assets/sql/Omni_Tracer.sql` | Remove recipient_list | -3 lines |
| `internal/core/domain/queue_message.go` | Add subscriber field + JSON | ~15 lines |
| `internal/service/tracer/tracer_service.go` | Add filter in processBatch | ~5 lines |

### Files NOT Changed

- `internal/adapter/storage/oracle/dequeue_ops.c` — no C changes
- `internal/adapter/storage/oracle/dequeue_ops.h` — no header changes
- `internal/adapter/storage/oracle/oracle_adapter.go` — no adapter changes
- Queue configuration — no queue-level changes

### Key Design Principle

The queue broadcasts all messages to all subscribers (this is how Oracle sharded queues work). Filtering happens cheaply at the Go application layer after JSON deserialization. This is the simplest approach that works within Oracle's sharded queue constraints.

### Backward Compatibility

- Messages from existing `Trace_Message()` calls have no `SUBSCRIBER` field → displayed to all subscribers (broadcast behavior preserved)
- Messages from `Trace_Message_To_Webhook()` also have no `SUBSCRIBER` field → broadcast + webhook behavior preserved
- Only messages from generated `TRACE_MESSAGE_<NAME>()` procedures contain the `SUBSCRIBER` field → filtered to matching subscriber only

---

## Dependencies

- **Depends on**: Story 1-2 (Procedure Generation) — must be complete so generated procedures already call `Enqueue_Event___` with `subscriber_name_`
- **Blocks**: Story 2-1 (Display Procedure Name in Header) — the routing must work before the UI feature is useful
