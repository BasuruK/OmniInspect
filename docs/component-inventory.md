# OmniInspect - Component Inventory

**Date:** 2026-04-15

## Overview

This inventory focuses on the terminal UI components and related screen-level building blocks that shape the user experience of OmniInspect. The repo does not use a browser component model; instead, reusable behavior is expressed through Bubble Tea model helpers, forms, overlays, and shared Lip Gloss styles.

## Screen Components

### Welcome Screen

- **Files:** `internal/adapter/ui/welcome.go`, `internal/adapter/ui/animations/omniview_logo_anim.go`
- **Purpose:** Animated first impression, config readiness checks, loading transition.
- **Notes:** Works in tandem with welcome-specific state on the root model.

### Loading Screen

- **Files:** `internal/adapter/ui/loading.go`
- **Purpose:** Shows progress while connecting to Oracle, checking permissions, deploying assets, and preparing the subscriber/listener flow.
- **Notes:** Driven by typed Bubble Tea messages and spinner/progress state.

### Main Trace Console

- **Files:** `internal/adapter/ui/main_screen.go`
- **Purpose:** Core log viewer for live trace messages.
- **Key behaviors:** viewport rendering, wrapping, auto-scroll toggling, database settings access, trace formatting, sanitization.

### Onboarding Screen

- **Files:** `internal/adapter/ui/onboarding.go`
- **Purpose:** First-run collection of database configuration.
- **Notes:** Delegates form behavior to `AddDatabaseForm` and persists through a command.

### Database Settings Screen

- **Files:** `internal/adapter/ui/database_settings.go`, `internal/adapter/ui/database_list.go`
- **Purpose:** Manage saved database configurations after onboarding.
- **Notes:** Includes validation, switching, editing, and deletion flows.

## Reusable UI Building Blocks

### AddDatabaseForm

- **Files:** `internal/adapter/ui/add_database_form.go`
- **Purpose:** Shared form model for onboarding and database configuration editing.
- **Why it matters:** It centralizes field definitions, submission state, keyboard navigation, and validation-oriented behavior.

### Chrome and Layout Helpers

- **Files:** `internal/adapter/ui/chrome.go`
- **Purpose:** Shared layout wrappers for headers, footers, info bars, and screen framing.
- **Why it matters:** Keeps view composition consistent across screens.

### Typed Message Layer

- **Files:** `internal/adapter/ui/messages.go`
- **Purpose:** Async and screen-transition contract for the root Bubble Tea model.
- **Why it matters:** Prevents stringly typed UI signaling and makes flows testable.

### Shared Style Tokens

- **Files:** `internal/adapter/ui/styles/styles.go`
- **Purpose:** Centralized color palette, panel styles, form styles, footer/header styles, and severity rendering.
- **Why it matters:** This is the reusable design-token layer for the TUI.

## Non-UI Runtime Components With UX Impact

### Tracer Service

- **Files:** `internal/service/tracer/tracer_service.go`
- **UX impact:** Controls live message throughput, listener lifecycle, and webhook dispatch behavior that affects the main console experience.

### Updater Flow

- **Files:** `internal/service/updater`, `internal/updater`, update state inside `internal/adapter/ui/model.go`
- **UX impact:** Governs update prompts and update progress inside the TUI.

### Persistence and Onboarding State

- **Files:** `internal/adapter/storage/boltdb/*`, `internal/core/domain/webhook.go`, `internal/core/domain/database_settings.go`
- **UX impact:** Determines what the user sees on first run versus returning sessions.

## Suggested Maintenance Boundaries

- Add new screens under `internal/adapter/ui` and keep screen-specific state isolated on the root model.
- Extend shared styles before introducing new per-screen visual conventions.
- Reuse typed messages and form models where possible instead of creating parallel interaction patterns.
- Keep business logic out of `View()` methods and out of ad hoc UI-only helper state.

---

_Generated using BMAD Method `document-project` workflow_
