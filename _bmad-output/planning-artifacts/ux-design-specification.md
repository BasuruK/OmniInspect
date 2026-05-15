---
stepsCompleted: [1]
inputDocuments:
  - DESIGN.md
  - _bmad-output/planning-artifacts/epics.md
  - internal/adapter/ui/database_settings.go
status: 'complete'
workflowType: 'ux-design'
project_name: OmniInspect
user_name: Basuruk
date: '2026-05-14'
---

# UX Design Specification: Epic 3 Story 1 — Delete Procedure UX Correction

**Author:** Sally (UX Designer) + Basuruk (Product Owner)
**Date:** 2026-05-14
**Type:** Small UX Correction

---

## Problem Statement

Two UX issues identified by the user in Story 3.1 (Delete Subscriber Procedure):

### Issue 1: Keyboard Shortcut Mismatch
**What:** Currently `Ctrl+D` is assigned to delete a procedure.
**Problem:** `Ctrl+D` is a commonly used shortcut (EOF/detach in terminals). Using it for delete is dangerous and unexpected.
**Fix:** Change to plain `P` (mnemonic: **P** for **P**rocedure delete).

### Issue 2: Silent Delay on Delete
**What:** User confirms deletion, then nothing appears until the success dialog shows (after the async operation completes).
**Problem:** The silence creates anxiety — "Did I break something? Is it still working?"
**Fix:** Show a spinner inside the Danger Zone section with a "Deleting procedure, please wait a moment..." message immediately after confirmation, replacing the hint text.

---

## Design Changes

### Change 1: Keyboard Shortcut

| Location | Before | After |
|----------|--------|-------|
| `database_settings.go:261` | `case "ctrl+d":` | `case "p":` |
| `database_settings.go:435` | `"Press Ctrl+D to delete..."` | `"Press P to delete..."` |
| Help bar hint | `D Delete` | (unchanged — doesn't mention procedure delete) |

### Change 2: Deletion Loading State

**State field to add:**
```go
dropProcedureDeleting bool
```

**Behavior:**

1. User presses `P` → confirmation modal appears
2. User presses `Y` → `dropProcedureDeleting = true` + async delete starts
3. During deletion:
   - Danger Zone section shows: **spinner + "Deleting procedure, please wait a moment..."**
   - Confirmation modal is dismissed
4. On success: `dropProcedureDeleting = false` → dialog: "Procedure deleted successfully. Restart OmniView to regenerate."
5. On failure: `dropProcedureDeleting = false` → error dialog

**Visual rendering during deletion:**
```text
╭─ Danger Zone ────────────────────────────────────╮
│                                                  │
│   ◐  Deleting procedure, please wait a moment... │
│                                                  │
╰──────────────────────────────────────────────────╯
```

---

## Component Inventory

### Danger Zone (during idle state)
- Border: `styles.ErrorColor`
- Content: hint text with shortcut reminder
- States: idle, confirming (modal overlay), deleting (spinner)

### Danger Zone (during deletion)
- Same border, same section
- Content: spinner (using existing `charm.land/bubbles/v2/spinner`) + message
- Spinner color: `styles.AccentColor` or `styles.WarningColor`

### Confirmation Modal
- Unchanged from current implementation
- Shown before deletion starts

### Result Dialog
- Unchanged behavior from current implementation
- Shown after deletion completes (success or error)

---

## Technical Notes

- Uses existing Bubble Tea spinner from `charm.land/bubbles/v2/spinner`
- Spinner state managed in root `Model` (like other loading states)
- `dropSubscriberProcedureMsg` triggers async deletion via `tea.Cmd` function
- `dropSubscriberProcedureResultMsg` handles completion and resets `dropProcedureDeleting`

---

## Acceptance Criteria

1. Pressing `P` (not `Ctrl+D`) opens the delete confirmation
2. Pressing `Ctrl+D` while on Database Settings does nothing
3. After confirming delete, the Danger Zone shows spinner + wait message
4. Success/error dialog appears after spinner completes
5. All changes pass `make build` and `make test`