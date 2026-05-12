---
title: 'Move Procedure Call Next To Database Name'
type: 'feature'
created: '2026-05-12'
status: 'done'
route: 'one-shot'
---

# Move Procedure Call Next To Database Name

## Intent

**Problem:** The generated funny-name procedure call was visible in the main header, but it rendered as a separate header detail line instead of sitting next to the active database name.

**Approach:** Keep the procedure call in the header, compose it inline with the subtitle/database identity line, and style it with the existing purple API-caller color token.

## Suggested Review Order

**Header Placement**

- Entry point: inline detail beside the database subtitle instead of a third line.
  [`chrome.go:62`](../../internal/adapter/ui/chrome.go#L62)

**Purple Styling**

- Reuses the existing purple API-caller token for procedure-call emphasis.
  [`styles.go:154`](../../internal/adapter/ui/styles/styles.go#L154)

**Regression Coverage**

- Verifies database name and procedure call share a header line.
  [`main_screen_test.go:20`](../../internal/adapter/ui/main_screen_test.go#L20)

- Test helpers make the header-line assertion explicit and reusable.
  [`main_screen_test.go:540`](../../internal/adapter/ui/main_screen_test.go#L540)
