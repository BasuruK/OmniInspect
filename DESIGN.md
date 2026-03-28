# OmniView TUI Design Specification

## Document Purpose

This document is the target-state design contract for AI agents and developers working on the OmniView terminal user interface.

It replaces generic TUI advice with repo-specific guidance that matches:

- the current OmniView architecture
- Bubble Tea v2 usage in this codebase
- Lip Gloss v2 styling and layout patterns

This document should be treated as the source of truth for UI structure, screen responsibilities, layout rules, and implementation constraints.

## Design Authority And Precedence

When making UI decisions, use this priority order:

1. `DESIGN.md` target-state product and UX rules
2. Bubble Tea v2 and Lip Gloss v2 documented API constraints
3. existing repository implementation details

This means:

- the current UI is not the design authority
- the current repo structure is useful migration context, not a visual contract
- if existing code conflicts with this document, update the code to match this document unless a library constraint makes that impossible
- if a rule in this document is too vague, resolve it using Bubble Tea v2 and Lip Gloss v2 documentation before copying the current implementation

## Audience

- AI coding agents making UI changes
- developers extending the Bubble Tea application
- reviewers validating whether a UI change fits the existing design system

## Technology Baseline

- Framework: `charm.land/bubbletea/v2`
- Styling: `charm.land/lipgloss/v2`
- Components: `charm.land/bubbles/v2`
- Current versions from `go.mod`:
  - `bubbletea/v2 v2.0.1`
  - `lipgloss/v2 v2.0.0`
  - `bubbles/v2 v2.0.0`

## Design Goals

The OmniView UI should be:

1. Readable in a terminal first.
2. Deterministic and state-driven.
3. Responsive to terminal size changes.
4. Keyboard-centric.
5. Consistent with this design specification even when the current implementation differs.
6. Simple to extend screen-by-screen without hidden coupling.
7. Clear enough that an AI agent can use it as an implementation spec, not just a descriptive note.

## Source of Truth in the Codebase

The current UI is organized around a single root Bubble Tea model and screen-specific render/update helpers.

- Composition root: `cmd/omniview/main.go`
- Root UI model: `internal/adapter/ui/model.go`
- Message definitions: `internal/adapter/ui/messages.go`
- Screen rendering:
  - `internal/adapter/ui/welcome.go`
  - `internal/adapter/ui/loading.go`
  - `internal/adapter/ui/main_screen.go`
  - `internal/adapter/ui/onboarding.go`
  - `internal/adapter/ui/database_settings.go`
  - `internal/adapter/ui/saved.go`
- Shared style tokens: `internal/adapter/ui/styles/styles.go`

These code locations describe where UI work currently lives. They do not override the target design in this document.

Agents may refactor or redesign the current UI implementation to satisfy this document as long as they preserve application correctness and Bubble Tea v2 / Lip Gloss v2 compatibility.

## Bubble Tea v2 Architecture Rules

These rules are validated against Bubble Tea v2 documentation and match the current repo structure.

### Required model structure

Bubble Tea applications should continue to follow the Elm-style model lifecycle:

- `Init() tea.Cmd`
- `Update(msg tea.Msg) (tea.Model, tea.Cmd)`
- `View() tea.View`

OmniView already follows this pattern in `internal/adapter/ui/model.go`. Future changes should preserve it.

### State ownership

- The root `Model` owns global application UI state.
- Screen-specific state should live in dedicated sub-state structs on `Model`.
- Background work must flow back into the UI through typed `tea.Msg` messages.
- Do not mutate business state from `View()`.

### Update loop rules

- Handle global events in the root `Update`.
- Delegate screen-specific behavior to screen-specific update helpers.
- Use typed messages for async completion, not raw strings.
- Use `tea.KeyPressMsg` for key handling.
- Handle `tea.WindowSizeMsg` centrally and propagate size-derived layout state as needed.

### View rules

- `View()` must be a pure rendering function derived from model state.
- Screen-specific rendering should live in `viewX()` helpers.
- Compose screen sections as strings, then assemble them with Lip Gloss layout helpers.
- Full-screen behavior, window title, and terminal-wide wrappers belong at the top-level `View()`.

### Commands and async work

- Use `tea.Cmd` for database connection attempts, loading flows, save actions, and event subscriptions.
- Long-running or blocking work must not happen inline in `View()`.
- UI transitions should happen by returning messages from commands and handling them in `Update()`.

## Lip Gloss v2 Styling Rules

These rules are validated against Lip Gloss v2 documentation and should be used as the default styling approach.

### Style construction

- Create styles with `lipgloss.NewStyle()`.
- Reuse and derive variants with `.Copy()` when appropriate.
- Keep design tokens centralized in `internal/adapter/ui/styles/styles.go`.
- Agents may replace existing token values when migrating the UI to the target design system in this document.

### Borders and framing

- Use `lipgloss.RoundedBorder()` for primary framed panels unless a screen has a strong reason to use a different border.
- Apply border color with `.BorderForeground(...)`.
- Apply internal spacing with `.Padding(...)`.

### Layout

- Use `.Width(...)`, `.Height(...)`, `.Align(...)`, and `.AlignVertical(...)` for box layout.
- Use `lipgloss.JoinVertical(...)` and `lipgloss.JoinHorizontal(...)` to compose sections.
- Use `lipgloss.Place(...)` to center or position panels within the available terminal area.
- Use `lipgloss.Width(...)`, `lipgloss.Height(...)`, `lipgloss.Size(...)`, and `style.GetFrameSize()` when layout calculations need exact rendered dimensions.

### Color handling

- Prefer centralized color tokens over inline color literals in screen code.
- Do not preserve the current palette merely because it already exists in the repo.
- If adaptive color support is introduced, use the Lip Gloss v2 recommended helpers such as `LightDark` or `Complete` rather than old v1-only patterns.

## Target Visual Language

This section defines the intended redesign direction. It takes precedence over the palette and styling currently present in the repository.

### Palette

Use a dark terminal-friendly palette centered around cool blue and cyan accents:

- Base text: `#E6EDF3`
- Muted text: `#9FB3C8`
- Primary accent: `#00BFFF`
- Secondary accent: `#4FD1C5`
- Surface border: `#2A3A4A`
- Surface background: `#0F1720`
- Focus/active highlight: `#38BDF8`
- Required/error: `#FF5D5D`
- Warning: `#F59E0B`
- Success: `#22C55E`

### Typography in terminal terms

- Titles: bold, high-contrast, one line where possible
- Labels: clear and compact
- Help text: muted but readable
- Dense content such as logs: low decoration, high scan speed
- Status text: color-coded with consistent severity mapping

### Visual tone

- data-rich
- high contrast
- disciplined and structured
- modern terminal dashboard rather than playful splash UI

## Target Layout Patterns

These are the preferred layouts for the redesign.

### Main screens

Use a header plus content plus help/status structure:

- top header for title and high-value context
- primary content region occupying most of the terminal
- visible bottom help/status line for keybindings and state

Use a split-pane layout only when the screen truly has two simultaneous information densities, such as a list and a detail view.

### Forms and setup flows

Use centered framed panels or modal-style layouts:

- constrained width
- vertical field flow by default
- visible action hints
- nearby validation messaging

### Overlays

If a modal or overlay is introduced, implement it as an explicit rendering composition. Do not assume framework-native modal behavior.

## Visual Grammar And Component Rendering Spec

This section is authoritative for the target UI rendering language.

These visual instructions are intentionally specific and should be treated as required design outcomes, even when Bubble Tea v2 or Lip Gloss v2 require custom composition to achieve them.

Where Lip Gloss does not provide a native primitive for a visual pattern, agents should implement the pattern manually while preserving the exact visual result described below.

### Global Layout And Structural Paradigms

Applications should primarily utilize one of two layout patterns based on context:

#### A. The Master-Detail View (Dashboard / Main Screen)

- **Header / Filter Bar:** Top section spanning the full width. Contains search inputs, dropdowns, and date filters side-by-side.
- **Split Pane (Horizontal):**
  - **Left Pane (List/Table):** Takes up ~40-50% of the screen width. Displays lists of items with clear column headers.
  - **Right Pane (Details):** Takes up the remaining width. Displays granular information about the selected item from the Left Pane. Features a tabbed navigation header.
- **Footer (Status Bar):** Sticky at the bottom. Displays pagination (e.g., `Page 1 of 1 (total: 21)`) and global keybindings.

#### B. The Modal / Form View (Data Entry)

- **Container:** Centered vertically and horizontally within the terminal, or spanning a fixed central width (e.g., 80-100 columns).
- **Warning/Info Banner:** Placed immediately below the modal title (e.g., *"Important: Fields marked with (*) are required."* in yellow/italic).
- **Form Flow:** Stacked vertically. Related fields can be placed side-by-side if horizontal space permits (e.g., Priority and Reporter).
- **Action Bar:** Anchored at the bottom of the modal, containing "Save" (Left/Center) and "Cancel" (Right) buttons.

### Component Design Specifications

All components must be styled using `lipgloss` to ensure consistent border drawing and padding.

#### A. Window Frames & Input Boxes (The "Embedded Label" Pattern)

The primary stylistic signature of this application is the **embedded border label**. Standard `lipgloss` borders must be used with rounded corners.

- **Curvature:** Use rounded corners.
  - Top-left: `╭` | Top-right: `╮` | Bottom-left: `╰` | Bottom-right: `╯`
  - Horizontal: `─` | Vertical: `│`
- **Labels:** Labels are embedded directly into the top border of the box, indented by 1 space.
  - *Correct visual output:* `╭─ Label ────────────────╮`
- **Padding:** Inputs should have `Padding(0, 1)` (0 top/bottom, 1 left/right inside the box).

#### B. Standard vs. Required Fields

Color and symbology differentiate field states:

- **Standard Fields:**
  - Border color: Muted Blue / Dark Cyan.
  - Label color: Light Blue.
- **Required Fields:**
  - Border color: Red.
  - Label color: Red.
  - Indicator: Must include `(*)` in red text. Place this directly under the bottom right corner of the field border, or appended to the label.

#### C. Selectors / Dropdowns

- Must visually mimic input boxes but include a downward-pointing triangle `▼` aligned to the far right of the input area.
- Example text: `Select a user                       ▼`
- When active/focused, the border should highlight, and a standard `bubbles/list` overlay should appear.

#### D. Text Areas (Multi-line)

- Must use the same embedded label border style.
- **Scrollbars:** If content exceeds the fixed height, a scrollbar must be rendered on the rightmost internal edge.
  - Track: Dark Blue/Cyan block (`█` or `│` with specific background).
  - Thumb: Bright Blue/Cyan block (`█`), indicating current scroll position.

#### E. Tabs

- Displayed as a horizontal list of words (e.g., `Info Details Comments`).
- **Inactive Tab:** Muted grey/white text, no styling.
- **Active Tab:** Cyan/Bright Blue text with a thick underline matching the width of the word.

#### F. Buttons

- Rendered as solid color blocks with centered text. No standard borders; the background color defines the boundary.
- **Primary Action (Save):** Background `Green`, Text `Black` or `Dark Grey`.
- **Secondary/Destructive Action (Cancel):** Background `Red/Pink`, Text `White`.
- **Dimensions:** Minimum width of ~15-20 characters. Height of 3 rows (padding top/bottom 1).

### Typography And Color Palette

Maintain high contrast and strictly use ANSI/TrueColor codes tailored to dark terminal themes.

#### Color Assignments (Lipgloss definitions)

- **Base Text:** `#E0E0E0` (Light Grey/White) for standard reading text.
- **Primary Accent (Borders/Tabs/Highlights):** `#00BFFF` (Deep Cyan / Light Blue).
- **Warning / Required:** `#FF4500` or `#FF0000` (Vibrant Red).
- **Success (Save Button):** `#2E8B57` or similar muted terminal green.
- **Destructive (Cancel Button):** `#C71585` (Medium Violet Red / Pinkish Red).

#### Status & Type Tags (Contextual Colors)

When displaying items in tables or tags, adhere to standard agile/Jira color coding:

- **Statuses:**
  - `To Do`: Orange / Gold
  - `In Progress`: Bright Purple
  - `In Review`: Bright Blue
  - `Done`: Green
- **Types:**
  - `Task`: Light Purple
  - `Bug`: Red / Pink
  - `Epic`: Orange
  - `Subtask`: Cyan

#### Selection Highlights (Lists/Tables)

- When a row in the Left Pane is selected, invert the contrast or apply a solid background:
  - Background: Deep Cyan/Blue.
  - Text: Bright White.

### Interaction & UX Rules

When implementing the `Update(msg tea.Msg)` loop, follow these behavioral guidelines:

1. **Hotkey Visibility:**
   - Never implement a hotkey without displaying it to the user.
   - **Global Actions:** Displayed in the bottom sticky footer (e.g., `^f Filter`, `^n New Issue`, `f1 Help`).
   - **Contextual Actions:** Displayed next to or directly under the target field (e.g., `(p)` under the Project dropdown, `(x)` under Assignee).
2. **Focus Indicators:**
   - The actively focused input, pane, or button must have its border color changed to a bright, active color (e.g., Bright Orange or White), distinguishing it from inactive cyan borders.
3. **Keyboard Navigation:**
   - `Tab` / `Shift+Tab`: Cycle through input fields in Forms.
   - `Up` / `Down` arrows: Navigate lists and menus.
   - `Enter`: Confirm selection or submit modal.
   - `Esc`: Cancel action, close modal, or unfocus input.

### Reference Lip Gloss Spec Snippet

This snippet remains part of the design reference and should be used as a rendering target:

```go
var (
    // Base standard rounded border
    RoundedBorder = lipgloss.Border{
        Top:         "─",
        Bottom:      "─",
        Left:        "│",
        Right:       "│",
        TopLeft:     "╭",
        TopRight:    "╮",
        BottomLeft:  "╰",
        BottomRight: "╯",
    }

    // Default Input Box Style
    InputBoxStyle = lipgloss.NewStyle().
        Border(RoundedBorder).
        BorderForeground(lipgloss.Color("#4169E1")). // Royal Blue / Cyan
        Padding(0, 1)

    // Required Input Box Style
    RequiredBoxStyle = InputBoxStyle.Copy().
        BorderForeground(lipgloss.Color("#FF0000")) // Red

    // Use lipgloss.NewStyle().SetString() or standard layout features
    // to inject labels into the top border string.
)
```

## Screen Inventory

All new design work should map to a specific screen responsibility.

### 1. Welcome Screen

File: `internal/adapter/ui/welcome.go`

Purpose:

- brand introduction
- version display
- author/subtitle display
- transition into loading or onboarding

Layout rules:

- centered in terminal using `lipgloss.Place(...)`
- minimal chrome
- animation is acceptable if it remains short and deterministic

### 2. Loading Screen

File: `internal/adapter/ui/loading.go`

Purpose:

- communicate startup progress
- show current step and completed steps
- surface blocking startup errors

Layout rules:

- centered or near-centered status panel
- loading indicator should be visually distinct
- errors must be readable and not hidden by animation

### 3. Main Trace Screen

File: `internal/adapter/ui/main_screen.go`

Purpose:

- display real-time trace messages
- provide log scrolling
- surface auto-scroll status
- expose database/settings navigation

Layout rules:

- top header for app identity
- help/status line directly beneath header
- scrollable content area below
- viewport owns scrolling behavior
- use a single-column live log layout by default unless a real detail pane adds clear value

Behavior rules:

- maintain bounded message history
- sanitize user-controlled log content before rendering
- allow manual scrolling without losing clarity about auto-scroll state

### 4. Onboarding Screen

File: `internal/adapter/ui/onboarding.go`

Purpose:

- collect initial database configuration
- validate fields progressively
- persist configuration

Layout rules:

- centered framed panel
- vertical form flow
- active field clearly indicated
- validation errors shown near the form, not hidden elsewhere

Behavior rules:

- keyboard-only completion must be smooth
- field order must match the user task flow
- password fields must mask display content

### 5. Database Settings Screen

File: `internal/adapter/ui/database_settings.go`

Purpose:

- display stored connections
- indicate active connection
- switch active database
- open add-database flow

Layout rules:

- centered framed panel
- active connection summary at the top
- list of database entries below
- help text at bottom

Behavior rules:

- selected row must be obvious
- active connection must be obvious
- errors should appear inline in the panel or overlay

### 6. Saved / Confirmation Screen

File: `internal/adapter/ui/saved.go`

Purpose:

- confirm configuration save success
- direct the user into the next application state

Layout rules:

- brief, centered, high-signal
- do not require excessive reading

## Component Standards

These are the default component behaviors agents should follow when adding or updating UI pieces.

### Panels

- Use rounded borders for primary panels.
- Use padding to create breathing room inside panels.
- Avoid deeply nested borders unless there is a clear information hierarchy benefit.
- If a bordered section needs a label, implement the label explicitly above the border or via a custom-composed header row. Do not assume Lip Gloss provides native border labels.

### Headers

- One strong title line per screen.
- Optional secondary line for help or status.
- Header copy should be short and operational.

### Help bars

- Every interactive screen must expose its most important keybindings in visible text.
- Only document keys that actually work.
- Keep help copy concise and in scan order of likely usage.

### Lists

- Support keyboard navigation.
- Highlight the selected row clearly.
- Do not overload rows with decorative styling that harms scan speed.
- Use consistent row rhythm and spacing so large tables remain readable.

### Forms

- Use vertical flow by default.
- Show field labels clearly.
- Keep placeholder text examples realistic.
- Surface validation errors near the form.
- Preserve typed values when validation fails.
- Required fields should be visually distinct using label text and focus/error color, not only color alone.

### Buttons and actions

- Primary actions should use the success or accent palette with strong contrast.
- Secondary or destructive actions should be visually distinct from primary actions.
- Do not hide the primary action among neutral text links.

### Feedback states

- Loading, success, warning, and error states must use distinct color and copy.
- Error states should explain what failed in user language, not raw internal jargon unless that detail is useful.

## Interaction Rules

### Keyboard visibility

- Never add a keybinding without making it visible in the relevant screen.
- Global quit behavior should remain consistent.
- Context-specific actions should be shown where the user needs them.

### Focus and selection

- The active field, active row, or active panel must be visually distinguishable.
- Focus indication should use at least one of:
  - stronger foreground color
  - stronger border color
  - explicit cursor/marker
  - background contrast change

### Resize behavior

- Screens must reflow on `tea.WindowSizeMsg`.
- Avoid hardcoded assumptions that only work at 80x24.
- When fixed widths are used, clamp them responsively.

### Error handling

- Errors should be visible in the current screen context.
- Do not silently swallow errors that affect user flow.
- Preserve recoverability where possible.

## Implementation Guidance for AI Agents

### Preferred workflow for UI changes

1. Identify the screen and state owner first.
2. Update or add typed messages if async behavior changes.
3. Keep business logic in services/adapters, not in view rendering.
4. Align the implementation to this document even if existing UI code differs.
5. Recompute layout from terminal dimensions instead of hardcoding assumptions.
6. Add or reuse shared style tokens before introducing one-off styles.

### When adding a new screen

- Add a new screen constant in `internal/adapter/ui/model.go`.
- Add a dedicated sub-state struct if the screen needs persistent UI state.
- Add `updateX()` and `viewX()` helpers.
- Expose keybindings in the rendered output.
- Reuse shared style tokens from `styles.go`.

### When adding a new reusable UI piece

- Prefer a small renderer/helper over copying string-building code across screens.
- Keep styling decisions near shared style definitions if reused in multiple places.
- If the component is interactive, document its state ownership clearly.
- If current code uses a conflicting visual pattern, migrate the older pattern rather than preserving inconsistency.

## Invalid or Unsupported Assumptions to Avoid

The previous version of this document contained several generic assumptions that should not be treated as hard requirements.

### Do not assume these are native features

- embedded labels inside borders are not a built-in Lip Gloss border feature
- sticky footers are a layout pattern, not a Bubble Tea primitive
- modal overlays are a rendering strategy, not a framework widget
- dropdowns are not automatic; they must be modeled and rendered explicitly

### Do not use outdated import guidance

Use:

- `charm.land/bubbletea/v2`
- `charm.land/lipgloss/v2`
- `charm.land/bubbles/v2`

Do not introduce older `github.com/charmbracelet/...` import paths into this repo.

### Do not put logic in the wrong layer

- no database calls in `View()`
- no business-rule branching hidden in style helpers
- no adapter construction inside render helpers

## Practical Design Decisions for OmniView

For OmniView, agents should follow these defaults during the redesign:

- Use the target palette and layout rules in this document, even if that requires changing the existing palette in `styles.go`.
- Favor simple framed panels over noisy decorative layouts.
- Use a single primary content region per screen.
- Prefer viewport-based scrolling for dense logs and long content.
- Keep onboarding and configuration flows centered and focused.
- Keep the main trace screen optimized for throughput and readability, not ornament.
- Treat existing UI code as migration material, not as design truth.

## Acceptance Criteria for UI Changes

A UI change is aligned with this design document when:

1. It fits the Bubble Tea model/update/view architecture.
2. It uses Lip Gloss layout and styling primitives in a maintainable way.
3. It implements the target visual language defined here, even if that replaces older repo styling.
4. It exposes the relevant keybindings to the user.
5. It responds correctly to terminal resizing.
6. It keeps screen responsibilities clear and localized.
7. It does not introduce undocumented one-off UI behavior.

## Validation Notes

This document was validated against current Bubble Tea v2 and Lip Gloss v2 documentation using Context7.

Validated guidance includes:

- Bubble Tea's model lifecycle based on `Init`, `Update`, and `View`
- use of `tea.KeyPressMsg` and typed message handling in `Update`
- command-driven async workflows via `tea.Cmd`
- Lip Gloss layout composition with `NewStyle`, borders, padding, width/height, alignment, `JoinVertical`, `JoinHorizontal`, and `Place`
- Lip Gloss measurement and frame-size helpers for safe terminal layout calculations
- Lip Gloss v2 color guidance favoring `LightDark` and `Complete` style helpers for adaptive color work

If Bubble Tea v2 or Lip Gloss v2 APIs change materially in the future, this document should be revalidated before introducing new framework-level guidance.
