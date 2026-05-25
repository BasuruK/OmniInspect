# Story 4.2: Help Overlay

## Status: ready-for-dev

---

## User Story

As a user of OmniView,
I want to press `H` to open an in-app help panel,
So that I can quickly reference the PL/SQL API methods, database management keys, webhook setup, and message filtering — without leaving the application.

---

## Acceptance Criteria

**Given** the user is on the Main Screen
**When** they press `H`
**Then** a help overlay panel appears centered over the main screen
**And** the overlay shows all five sections described below
**And** the footer shows the `H Help` keyboard hint alongside other shortcuts

**Given** the help overlay is open
**When** the user presses `H` or `Escape`
**Then** the overlay closes and the main screen is visible again

**Given** a subscriber with funny name BARNACLE is registered
**When** the help overlay is open
**Then** Section 1 shows the subscriber-specific method using the live name: `OMNI_TRACER_API.TRACE_MESSAGE_BARNACLE('msg', log_level_)`

**Given** no subscriber is active (e.g., onboarding state)
**When** the help overlay is open
**Then** Section 1 shows a placeholder: `OMNI_TRACER_API.TRACE_MESSAGE_<YOUR_NAME>('msg', log_level_)`

---

## Help Panel Content

### Section 1: Subscriber-Specific Method
```
OMNI_TRACER_API.TRACE_MESSAGE_<NAME>('msg', log_level_)
```
Routes the message ONLY to your OmniView subscriber instance.
Your instance's live method name is shown dynamically.

### Section 2: Global Broadcast Method
```
OMNI_TRACER_API.Trace_Message('msg', log_level_)
```
Sends the message to ALL connected OmniView subscribers simultaneously.

### Section 3: Database Management
- `D` — Open Database Settings (add, switch, or edit a database connection)
- In Database Settings: `N` New connection, `E` Edit selected, `Enter` Switch to selected

### Section 4: Webhook Configuration
- `S` — Open Settings → navigate to Webhook section
- Configure the endpoint URL and toggle webhook delivery on/off

### Section 5: Message Filtering (B key)
- `B` — Cycle display filter: `Global` → `Subscriber Only` → `Broadcast Only` → `Global`
  - **Global**: Shows all messages (broadcast + subscriber-specific)
  - **Subscriber Only**: Shows only messages routed to your subscriber
  - **Broadcast Only**: Shows only global broadcast messages (NULL correlation)

---

## Technical Design

### Files Changed

| File | Change |
|------|--------|
| `internal/adapter/ui/help_overlay.go` | **New** — `renderHelpOverlay()` + `updateHelpOverlay()` helpers |
| `internal/adapter/ui/help_overlay_test.go` | **New** — render tests for open/closed state and dynamic subscriber name |
| `internal/adapter/ui/model.go` | Add `showHelp bool` field to `Model` |
| `internal/adapter/ui/main_screen.go` | Wire `H` key in `updateMainScreen()`, inject overlay in `viewMainScreen()`, update `mainFooterText()` |
| `internal/adapter/ui/messages.go` | Add `ToggleHelpMsg` (optional typed msg, may inline directly) |

### State

```go
// On Model struct
showHelp bool
```

No persistence needed — help is a transient UI state.

### Key Routing

In `updateMainScreen()` key handler, add alongside `S`, `D`, `B`:

```go
case "h", "H":
    m.showHelp = !m.showHelp
    return m, nil
```

Escape key already handled globally — extend to also close help when `m.showHelp`:

```go
case "esc":
    if m.showHelp {
        m.showHelp = false
        return m, nil
    }
    // ... existing esc handling
```

### Rendering

`viewMainScreen()` renders the overlay on top when `m.showHelp`:

```go
if m.showHelp {
    overlay := m.renderHelpOverlay(contentWidth, contentHeight)
    // Place centered using lipgloss.Place
    return lipgloss.Place(contentWidth, contentHeight,
        lipgloss.Center, lipgloss.Center, overlay)
}
```

`renderHelpOverlay()` in `help_overlay.go`:
- Uses `lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1,2)` for framing
- Max width capped at ~80 chars for readability
- Calls `m.subscriberFunnyName()` or equivalent to inject the live procedure name into Section 1
- Closes with `[ H or Esc — Close ]` footer hint inside the panel

### Footer Update

```go
func (m *Model) mainFooterText() string {
    return "↑/↓ Scroll  •  A Auto Scroll [on/off]  •  B Mode [Global/Subscriber/Broadcast]  •  C Clear  •  D Database Settings  •  H Help  •  S Settings  •  Q Quit"
}
```

### Styling

Use existing style tokens from `styles/styles.go`. If a dedicated panel title style is missing, define it there as `HelpTitleStyle` — do not add ad hoc color literals in `help_overlay.go`.

---

## Out of Scope

- No scrolling within the overlay (content fits within typical terminal heights)
- No persistence of help-open state
- No help text on other screens (Settings, Database List, etc.) — Main Screen only

---

## Test Cases

| Case | Expected |
|------|----------|
| `renderHelpOverlay` with active subscriber BARNACLE | Output contains `TRACE_MESSAGE_BARNACLE` |
| `renderHelpOverlay` with no subscriber | Output contains `TRACE_MESSAGE_<YOUR_NAME>` placeholder |
| `updateMainScreen` with `H` keypress when `showHelp=false` | `showHelp` becomes `true` |
| `updateMainScreen` with `H` keypress when `showHelp=true` | `showHelp` becomes `false` |
| `mainFooterText()` | Contains `H Help` |
| Escape key when `showHelp=true` | `showHelp` becomes `false`, normal esc NOT triggered |
