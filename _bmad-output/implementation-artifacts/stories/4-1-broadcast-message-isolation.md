# Story 4.1: Broadcast Message Isolation

**Status:** PENDING

---

## User Story

As a subscriber,
I want to toggle which messages I see in the TUI (all, subscriber-only, or broadcast-only),
So that I can reduce noise and focus on the messages relevant to my current debugging context.

---

## Problem Statement

Currently, when two users collaborate on the same Oracle database:
- User A using `TRACE_MESSAGE_BARNACLE('msg')` (subscriber-isolated)
- User B using `Trace_Message('msg')` (global broadcast)

User A sees ALL messages — both the subscriber-specific ones AND the global broadcasts. This is noisy.

The routing at Oracle AQ level already works correctly (correlation-based). The gap is purely **client-side display filtering**.

---

## Solution Overview

Three-mode toggle at the TUI client level:

| Mode | Shows | Detection |
|------|-------|-----------|
| **Global** (default) | All messages | No filtering |
| **SubscriberOnly** | Only named-subscriber messages | `SUBSCRIBER` field matches current subscriber |
| **BroadcastOnly** | Only global messages | `SUBSCRIBER` field is NULL/absent |

**Key binding:** `b` cycles through modes

**Storage:** BoltDB, per-client setting (same storage as subscriber config)

---

## Acceptance Criteria

**AC1: Three Broadcast Modes**

**Given** a subscriber named BARNACLE is registered
**When** the mode is set to **Global**
**Then** the TUI displays ALL messages (both broadcast via `Trace_Message()` and subscriber-specific via `TRACE_MESSAGE_BARNACLE()`)

**Given** a subscriber named BARNACLE is registered
**When** the mode is set to **SubscriberOnly**
**Then** the TUI displays ONLY messages where `SUBSCRIBER = 'BARNACLE'`
**And** messages where `SUBSCRIBER IS NULL` (broadcast) are hidden

**Given** a subscriber named BARNACLE is registered
**When** the mode is set to **BroadcastOnly**
**Then** the TUI displays ONLY messages where `SUBSCRIBER IS NULL`
**And** messages where `SUBSCRIBER = 'BARNACLE'` (subscriber-specific) are hidden

**AC2: Keyboard Shortcut**

**Given** the user is on the Main Screen
**When** they press `b`
**Then** the mode cycles: Global → SubscriberOnly → BroadcastOnly → Global
**And** the current mode is visually indicated in the TUI

**AC3: Mode Persistence**

**Given** the user sets a broadcast mode
**When** OmniView restarts
**Then** the mode is restored to the saved value (default: Global)

**AC4: Mode Indicator**

**Given** the user is on the Main Screen
**When** any mode is active
**Then** a status indicator shows the current mode (e.g., `[GLOBAL]` / `[SUB ONLY]` / `[BCAST ONLY]`)

---

## Technical Approach

### 1. PL/SQL Change — Add SUBSCRIBER to JSON Payload

**File:** `assets/sql/Omni_Tracer.sql`

Modify `Enqueue_Event___()` to stamp subscriber name into the JSON payload:

```sql
-- In Enqueue_Event___, after building the message JSON object:
message_.PUT('SUBSCRIBER', subscriber_name_);  -- NULL for broadcast, name for subscriber-isolated
```

This is a one-line addition in the existing `Enqueue_Event___()` procedure. The `subscriber_name_` parameter is already declared and passed by generated procedures.

**Rationale:** Correlation-based routing at Oracle AQ level is already working and unchanged. Adding `SUBSCRIBER` to the JSON payload is purely for Go client's display filtering — no routing logic changes.

### 2. Go Domain — BroadcastMode Value Object

**File:** `internal/core/domain/broadcast_mode.go` (NEW)

```go
package domain

type BroadcastMode int

const (
    BroadcastModeGlobal        BroadcastMode = 0  // Default — show all messages
    BroadcastModeSubscriber   BroadcastMode = 1  // Show only subscriber-isolated
    BroadcastModeBroadcast    BroadcastMode = 2  // Show only global broadcasts
)

func (m BroadcastMode) String() string { ... }
func (m BroadcastMode) Next() BroadcastMode { ... }  // Cycles: 0→1→2→0
```

### 3. Go Domain — QueueMessage Subscriber Field

**File:** `internal/core/domain/queue_message.go` (MODIFY)

Add `subscriber` field to `QueueMessage` entity:
- Add `subscriber string` field
- Add `Subscriber() string` getter
- Update `UnmarshalJSON` to extract `SUBSCRIBER` field from payload
- Add `IsBroadcast() bool` method — returns `true` when `subscriber == ""` (NULL in JSON)

```go
type QueueMessage struct {
    messageID     string
    processName   string
    logLevel      LogLevel
    payload       string
    timestamp     time.Time
    sendToWebhook bool
    subscriber    string  // NEW — empty string means broadcast
}

func (m *QueueMessage) Subscriber() string    { return m.subscriber }
func (m *QueueMessage) IsBroadcast() bool      { return m.subscriber == "" }
```

### 4. BoltDB — Persist Mode Per Client

**File:** `internal/adapter/storage/boltdb/` (existing adapter)

Extend existing config storage to persist `BroadcastMode`:
- Key: `broadcast_mode` (string value: `"global"`, `"subscriber"`, `"broadcast"`)
- Loaded on startup, saved on change

No new adapter files needed — extend existing `ConfigRepository` interface if present, or add to the subscriber config blob.

### 5. UI — Keybinding, Filtering, Status Indicator

**Files:** `internal/adapter/ui/messages.go`, `internal/adapter/ui/main_screen.go`

**Keybinding in messages.go:**
```go
case tea.KeyPresses:
    switch msg.Key {
    case 'b':
        return m, m.cycleBroadcastMode()  // cycles mode and persists
    }
```

**Filter in viewport rendering (main_screen.go):**
```go
func (m *Model) filterMessages(msgs []*domain.QueueMessage) []*domain.QueueMessage {
    if m.broadcastMode == domain.BroadcastModeGlobal {
        return msgs  // no filter
    }
    filtered := make([]*domain.QueueMessage, 0, len(msgs))
    for _, msg := range msgs {
        switch m.broadcastMode {
        case domain.BroadcastModeSubscriber:
            if msg.Subscriber() != "" {
                filtered = append(filtered, msg)
            }
        case domain.BroadcastModeBroadcast:
            if msg.IsBroadcast() {
                filtered = append(filtered, msg)
            }
        }
    }
    return filtered
}
```

**Status indicator in main_screen.go header area:**
```
┌─────────────────────────────────────────────────────────────────┐
│ [GLOBAL] │ OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg') │ 00:00 │
└─────────────────────────────────────────────────────────────────┘
  ^^^^^^^^ mode indicator
```

---

## Files to Modify

| File | Change |
|------|--------|
| `assets/sql/Omni_Tracer.sql` | Add `message_.PUT('SUBSCRIBER', subscriber_name_)` in `Enqueue_Event___()` |
| `internal/core/domain/queue_message.go` | Add `subscriber` field, `Subscriber()` getter, `IsBroadcast()` method, update JSON unmarshal |
| `internal/core/domain/` | NEW: `broadcast_mode.go` — value object for three modes |
| `internal/adapter/storage/boltdb/` | Persist `BroadcastMode` (extend existing config storage) |
| `internal/adapter/ui/messages.go` | Add `b` keybinding for mode cycling |
| `internal/adapter/ui/main_screen.go` | Add `filterMessages()`, add mode status indicator to header |

---

## Out of Scope

- Changes to Oracle AQ routing (correlation-based routing already works)
- Server-side filtering (this is client-side display only)
- Admin-forced mode controls (user-controlled only)
- Changes to dequeue C layer (no correlation extraction needed in Go)

---

## Verification

1. **PL/SQL:** `Trace_Message('test')` → JSON payload has `"SUBSCRIBER":null`
2. **PL/SQL:** `TRACE_MESSAGE_BARNACLE('test')` → JSON payload has `"SUBSCRIBER":"BARNACLE"`
3. **UI:** Press `b` → mode cycles and indicator updates
4. **UI:** Mode set to SubscriberOnly → global `Trace_Message()` calls hidden from view
5. **UI:** Mode set to BroadcastOnly → subscriber-specific calls hidden from view
6. **Persistence:** Set mode, restart app → mode restored

---

## Dependencies

- Story 1.5 (Correlation-Based Message Routing) — already complete ✅
- Story 2.1 (Display Procedure Name in Header) — can be done in parallel or before

---

## Effort Estimate

**Small** — single story, isolated UI changes + one-line PL/SQL addition + domain value object.