# OmniView TUI Migration Guide

## Document Information

- **Project**: OmniView TUI Migration
- **Target Framework**: Bubble Tea v2.0 + Lipgloss v2.0 + Bubbles v2.0
- **Module**: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/bubbles/v2`
- **Status**: Implementation Guide

---

## 1. Executive Summary

This document is a **step-by-step implementation guide** for migrating OmniView from a hybrid CLI/TUI application to a fully integrated Terminal User Interface using a **single Bubble Tea v2 program**.

### Current State

| Layer | Technology | Behavior |
|-------|-----------|----------|
| Welcome screen | Bubble Tea v2 | Animated splash (~3s), exits via `tea.Quit` |
| Initialization | Console (`fmt.Println`) | Sequential setup printed to stdout |
| Tracer output | Console (`fmt.Println`) | Log messages written directly to stdout |
| Exit | `os.Signal` + `bufio.Reader` | Ctrl+C or press Enter to quit |

### Target State

| Layer | Technology | Behavior |
|-------|-----------|----------|
| Welcome screen | Bubble Tea v2 (screen within program) | Same animation, no program restart |
| Loading screen | Bubble Tea v2 + Spinner | DB connection & deployment progress |
| Main screen | Bubble Tea v2 + Viewport | Scrollable real-time log viewer |
| Exit | Bubble Tea v2 built-in | `q` or `ctrl+c` from any screen |

**Key change**: Everything runs inside **one** `tea.Program`. No more separate program for the welcome screen, no more `fmt.Println` for output.

---

## 2. Prerequisites

### 2.1 Required Dependencies

The project already has Bubble Tea v2 and Lipgloss v2. Bubbles v2 needs to be promoted from indirect to direct:

```bash
go get charm.land/bubbletea/v2@latest
go get charm.land/lipgloss/v2@latest
go get charm.land/bubbles/v2@latest
```

After running these, `go.mod` should contain:

```go
// go.mod (relevant lines)
require (
    charm.land/bubbletea/v2 v2.0.1
    charm.land/lipgloss/v2  v2.0.0
    charm.land/bubbles/v2   v2.0.0
)
```

### 2.2 Import Paths

Bubble Tea v2 uses **vanity import paths** under `charm.land/`:

```go
import (
    tea      "charm.land/bubbletea/v2"
    lipgloss "charm.land/lipgloss/v2"

    "charm.land/bubbles/v2/spinner"
    "charm.land/bubbles/v2/viewport"
)
```

> **Warning**: The old `github.com/charmbracelet/bubbletea` paths are v1.
> Never mix v1 and v2 imports.

---

## 3. Current Architecture

### 3.1 Application Flow (from `cmd/omniview/main.go`)

```text
┌───────────────────────────────────────────────────────────────┐
│  main()                                                       │
├───────────────────────────────────────────────────────────────┤
│  1. app.New()                                                 │
│  2. ui.NewUIAdapter().StartWelcome()   ← Bubble Tea program   │
│     └─ Runs animation, calls tea.Quit  ← PROGRAM EXITS       │
│  3. updater.CleanupOldBinary()                                │
│  4. updater.CheckAndUpdate()                                  │
│  5. signal channel + context setup                            │
│  6. boltAdapter.Initialize()           ← BoltDB              │
│  7. cfgLoader.LoadClientConfigurations()                      │
│     └─ May prompt user via stdin       ← USES os.Stdin        │
│  8. dbAdapter.Connect(ctx)             ← Oracle DB            │
│  9. permissionService.DeployAndCheck()                         │
│ 10. tracerService.DeployAndCheck()                             │
│ 11. subscriberService.RegisterSubscriber()                     │
│ 12. tracerService.StartEventListener() ← Background goroutine │
│     └─ Prints to stdout via fmt.Println ← CONSOLE OUTPUT      │
│ 13. omniApp.ShowStatus(done)           ← Blocks on Enter key  │
│ 14. select { <-done | <-signalChan }                          │
└───────────────────────────────────────────────────────────────┘
```

### 3.2 Problems with Current Architecture

1. **Welcome screen is a separate `tea.Program`** — causes a visual flash when it exits and console output begins.
2. **`fmt.Println()` for tracer output** — cannot be scrolled, searched, or styled.
3. **`ConfigLoader` reads from `os.Stdin`** — Bubble Tea controls stdin; this will break inside a TUI.
4. **`ShowStatus()` blocks on Enter** — incompatible with the Elm Architecture event loop.
5. **Signal handling is manual** — Bubble Tea v2 handles `ctrl+c` natively via `tea.KeyPressMsg`.

### 3.3 What Already Works

The welcome screen (`internal/adapter/ui/welcome/welcome.go`) already uses correct Bubble Tea v2 patterns:
- Returns `tea.View` from `View()` via `tea.NewView()`
- Handles `tea.WindowSizeMsg` correctly
- Uses `tea.Tick` for animation
- Uses pointer receiver `*Model`

This code will be **refactored into the main model**, not rewritten.

### 3.4 Component Migration Map

| Component | Current Location | Action |
|-----------|-----------------|--------|
| Welcome animation | `internal/adapter/ui/welcome/welcome.go` | Merge into main model as a screen |
| Welcome styles | `internal/adapter/ui/styles/styles.go` | Keep, extend with new styles |
| ASCII logo | `internal/app/app.go` (`GetLogoASCII`) | Move to `ui` package |
| BoltDB init | `cmd/omniview/main.go` | Move to pre-TUI phase |
| Config loading | `internal/adapter/config/settings_loader.go` | Move to pre-TUI phase (uses stdin) |
| Updater | `cmd/omniview/main.go` | Move to pre-TUI phase |
| DB connection | `cmd/omniview/main.go` | TUI loading command |
| Permission check | `cmd/omniview/main.go` | TUI loading command |
| Tracer deploy | `cmd/omniview/main.go` | TUI loading command |
| Subscriber register | `cmd/omniview/main.go` | TUI loading command |
| Event listener | `internal/service/tracer/tracer_service.go` | Channel-based, TUI command |
| Tracer output | `TracerService.handleTracerMessage()` | Channel → viewport |

---

## 4. Bubble Tea v2 Key Concepts

Understanding these patterns is essential before implementing. All code in this guide uses the **v2 API**.

### 4.1 The Elm Architecture

Bubble Tea follows [The Elm Architecture](https://guide.elm-lang.org/architecture/): a loop of **Model → Update → View**.

```text
              ┌─────────┐
    Msg ─────►│ Update() │──────► Model (new state)
              └─────────┘              │
                   ▲                   │
                   │              ┌────▼────┐
                   │              │  View() │──────► Terminal Output
                   │              └─────────┘
                   │
              ┌────┴────┐
              │  Init() │──────► Initial Cmd (optional)
              └─────────┘
```

- **Model**: A struct holding all application state.
- **Init()**: Returns a `tea.Cmd` for initial I/O (start animation, request data).
- **Update(msg tea.Msg)**: Receives events (key presses, timer ticks, custom messages), returns updated model + optional command.
- **View()**: Pure function. Reads current model state, returns what to render. **Never mutates state.**

### 4.2 Declarative Views (`tea.View`)

In Bubble Tea v2, `View()` returns a `tea.View` struct — **not a plain string**. Terminal features like alternate screen mode and window titles are declared as fields on this struct:

```go
func (m *Model) View() tea.View {
    v := tea.NewView("Hello, world!")

    // Declarative terminal features (replaces program options from v1)
    v.AltScreen = true
    v.WindowTitle = "OmniView"

    return v
}
```

| View Field | Purpose |
|-----------|---------|
| `v.AltScreen` | Enter the alternate screen buffer (full-screen mode) |
| `v.WindowTitle` | Set the terminal window/tab title |
| `v.MouseMode` | Enable mouse tracking (`tea.MouseModeCellMotion`) |
| `v.ReportFocus` | Receive focus/blur events |
| `v.Cursor` | Control cursor position and shape |

> **v2 change**: Options like `tea.WithAltScreen()` and commands like `tea.EnterAltScreen` no longer exist. Set view fields instead.

### 4.3 Commands and Messages

A `tea.Cmd` is a function that performs I/O and returns a `tea.Msg`:

```go
// tea.Cmd = func() tea.Msg

// Example: a command that connects to a database
func connectDBCmd(ctx context.Context, adapter *oracle.OracleAdapter) tea.Cmd {
    return func() tea.Msg {
        err := adapter.Connect(ctx)       // runs in a goroutine
        return DBConnectedMsg{Err: err}   // result sent back to Update()
    }
}
```

Key rules:
- Commands run **concurrently** in goroutines managed by Bubble Tea.
- Commands **must not** mutate the model. They return a `tea.Msg` instead.
- `Update()` processes the message and updates the model.
- `tea.Batch(cmd1, cmd2)` runs multiple commands concurrently.
- `tea.Sequence(cmd1, cmd2)` runs commands one after another.

### 4.4 Key Messages in v2

```go
// Keyboard events (v2 uses tea.KeyPressMsg, NOT tea.KeyMsg)
case tea.KeyPressMsg:
    switch msg.String() {
    case "q", "ctrl+c":
        return m, tea.Quit
    case "up", "k":
        // scroll up
    case "space":              // NOTE: v2 uses "space", not " "
        // toggle something
    }

// Terminal resize
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
```

### 4.5 Channel-Based Realtime Pattern

To integrate background goroutines (like the event listener) with Bubble Tea, use a **channel + re-subscribing command** pattern:

```text
┌──────────────────┐     channel      ┌─────────────────┐
│ Background       │ ───────────────► │ waitForEvent()   │
│ goroutine        │  *QueueMessage   │ tea.Cmd          │
│ (event listener) │                  │ blocks on <-ch   │
└──────────────────┘                  └────────┬─────────┘
                                               │
                                      QueueMessageMsg
                                               │
                                      ┌────────▼─────────┐
                                      │ Update()          │
                                      │ appends message   │
                                      │ re-issues cmd ◄───── KEY: must re-subscribe!
                                      └──────────────────┘
```

The command reads **one** message from the channel and returns it. After `Update()` processes it, it re-issues the same command to wait for the next message:

```go
// Command: wait for one message from the channel
func waitForEventCmd(ch <-chan *domain.QueueMessage) tea.Cmd {
    return func() tea.Msg {
        msg := <-ch                          // blocks until a message arrives
        return QueueMessageMsg{Message: msg} // return it to Update()
    }
}

// In Update():
case QueueMessageMsg:
    m.messages = append(m.messages, msg.Message)
    // ... update viewport ...
    return m, waitForEventCmd(m.eventCh) // ← re-subscribe for next message
```

> **Common mistake**: Wrapping the channel read in a `for` loop inside the command.
> This doesn't work — `return` inside the loop exits immediately, making the loop pointless.
> The correct pattern is: read one message, return it, re-subscribe in `Update()`.

---

## 5. Target Architecture

### 5.1 Two-Phase Design

The application has two phases:

**Phase 1: Pre-TUI (before `tea.Program` starts)**
Handles operations that require direct stdin access or may restart the binary:

```text
1. app.New()
2. updater.CleanupOldBinary()
3. updater.CheckAndUpdate()        ← may replace binary and restart
4. boltAdapter.Initialize()        ← fast, no UI needed
5. cfgLoader.LoadClientConfigurations()  ← may prompt user via stdin
```

**Phase 2: TUI (inside `tea.Program`)**
Everything else runs as screens with async commands:

```text
6. Welcome animation              ← screen: "welcome"
7. Connect to Oracle DB            ← screen: "loading", step 1
8. Deploy/check permissions        ← screen: "loading", step 2
9. Deploy/check tracer             ← screen: "loading", step 3
10. Register subscriber            ← screen: "loading", step 4
11. Start event listener           ← transition to screen: "main"
12. Display real-time logs         ← screen: "main"
```

**Why two phases?** The `ConfigLoader` reads from `os.Stdin` via `bufio.Reader` when no saved config exists. Bubble Tea takes exclusive control of stdin, so interactive prompts must happen before the TUI starts. A future enhancement could replace this with a TUI-based settings form using `textinput` bubbles.

### 5.2 Screen State Machine

```text
┌──────────┐   animation done   ┌──────────┐   all steps OK   ┌──────────┐
│ welcome  │ ──────────────────► │ loading  │ ────────────────► │   main   │
│          │                     │          │                   │          │
│ ~3 sec   │                     │ spinner  │                   │ viewport │
│ logo     │                     │ progress │                   │ logs     │
│ version  │                     │ steps    │                   │ scroll   │
└──────────┘                     └─────┬────┘                   └──────────┘
     │                                 │                             │
     │ q/ctrl+c                        │ error                       │ q/ctrl+c
     ▼                                 ▼                             ▼
  tea.Quit                     show error msg                     tea.Quit
                               (retry or quit)
```

### 5.3 Model Structure

```go
// internal/adapter/ui/model.go

type Model struct {
    // ── Screen Navigation ─────────────────────────
    screen string // "welcome" | "loading" | "main"

    // ── Terminal Dimensions ───────────────────────
    width  int
    height int

    // ── Welcome Screen State ──────────────────────
    welcome welcomeState

    // ── Loading Screen State ──────────────────────
    loading loadingState

    // ── Main Screen State ─────────────────────────
    main mainState

    // ── Application Services ──────────────────────
    ctx               context.Context
    cancel            context.CancelFunc
    dbAdapter         *oracle.OracleAdapter
    permissionService *permissions.PermissionService
    tracerService     *tracer.TracerService
    subscriberService *subscribers.SubscriberService
    appConfig         *domain.DatabaseSettings

    // ── Event Channel ─────────────────────────────
    eventCh chan *domain.QueueMessage

    // ── App Metadata ──────────────────────────────
    app *app.App
}

type welcomeState struct {
    frame        int
    logoRevealed int
    showVersion  bool
    showSubtitle bool
    complete     bool
}

type loadingState struct {
    steps   []string       // completed step messages
    current string         // current step description
    error   error          // non-nil if a step failed
    spinner spinner.Model  // animated spinner
}

type mainState struct {
    messages   []*domain.QueueMessage
    viewport   viewport.Model
    autoScroll bool
    ready      bool // viewport initialized
}
```

### 5.4 Message Flow

```text
┌─────────────────────────────────────────────────────────────┐
│                     Message Types                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Built-in:                                                  │
│   tea.KeyPressMsg      ← keyboard input                     │
│   tea.WindowSizeMsg    ← terminal resize                    │
│   spinner.TickMsg      ← spinner animation frame            │
│                                                             │
│  Welcome:                                                   │
│   tickMsg              ← animation frame (80ms interval)    │
│                                                             │
│  Loading:                                                   │
│   dbConnectedMsg       ← Oracle DB connection result        │
│   permissionsCheckedMsg ← permission deploy/check result    │
│   tracerDeployedMsg    ← tracer deploy/check result         │
│   subscriberRegisteredMsg ← subscriber registration result  │
│                                                             │
│  Main:                                                      │
│   queueMessageMsg      ← new log from event listener        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. Implementation

### Step 1: Modify TracerService for Channel Output

The `TracerService` currently prints to stdout. Change it to send messages through an injected channel.

**File: `internal/service/tracer/tracer_service.go`**

```go
package tracer

import (
    "OmniView/assets"
    "OmniView/internal/core/domain"
    "OmniView/internal/core/ports"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "sync"
    "time"
)

type TracerService struct {
    db        ports.DatabaseRepository
    bolt      ports.ConfigRepository
    processMu sync.Mutex
    eventCh   chan<- *domain.QueueMessage  // ← NEW: injected channel
}

// Constructor now accepts an optional event channel.
// When eventCh is non-nil, messages are sent to the channel instead of stdout.
func NewTracerService(
    db ports.DatabaseRepository,
    bolt ports.ConfigRepository,
    eventCh chan<- *domain.QueueMessage,   // ← NEW parameter
) *TracerService {
    return &TracerService{
        db:      db,
        bolt:    bolt,
        eventCh: eventCh,
    }
}

// handleTracerMessage routes a message to the channel or stdout.
func (ts *TracerService) handleTracerMessage(msg *domain.QueueMessage) {
    if ts.eventCh != nil {
        ts.eventCh <- msg   // send to TUI via channel
    } else {
        fmt.Println(msg.Format())  // fallback: direct print
    }
}

// StartEventListener, blockingConsumerLoop, processBatch, DeployAndCheck
// remain unchanged — they already call handleTracerMessage internally.
```

**What changed**: Only the constructor signature and `handleTracerMessage`. All other methods (`StartEventListener`, `blockingConsumerLoop`, `processBatch`, `DeployAndCheck`) remain identical since they call `handleTracerMessage` internally.

---

### Step 2: Define Message Types

All custom messages live in one file. These are the events that flow through `Update()`.

**File: `internal/adapter/ui/messages.go`**

```go
package ui

import (
    "OmniView/internal/core/domain"
    "time"
)

// ── Welcome Screen Messages ──────────────────────

// tickMsg drives the welcome animation at 80ms intervals.
type tickMsg struct {
    time time.Time
}

// welcomeCompleteMsg signals the welcome animation is done.
type welcomeCompleteMsg struct{}

// ── Loading Screen Messages ──────────────────────

// startLoadingMsg tells Update() to begin the loading sequence.
type startLoadingMsg struct{}

// dbConnectedMsg is returned after Oracle DB connection attempt.
type dbConnectedMsg struct {
    err error
}

// permissionsCheckedMsg is returned after permission deploy/check.
type permissionsCheckedMsg struct {
    err error
}

// tracerDeployedMsg is returned after tracer deploy/check.
type tracerDeployedMsg struct {
    err error
}

// subscriberRegisteredMsg is returned after subscriber registration.
type subscriberRegisteredMsg struct {
    subscriber *domain.Subscriber
    err        error
}

// loadingCompleteMsg signals all loading steps succeeded.
type loadingCompleteMsg struct{}

// ── Main Screen Messages ─────────────────────────

// queueMessageMsg wraps a single log message from the event listener.
type queueMessageMsg struct {
    message *domain.QueueMessage
}
```

> **Convention**: Message types are unexported (lowercase) because they are internal to the `ui` package. Only export them if other packages need to send these messages.

---

### Step 3: Update Styles

Extend the existing styles with log-level colors and loading screen styles.

**File: `internal/adapter/ui/styles/styles.go`**

```go
package styles

import "charm.land/lipgloss/v2"

// ── Color Palette ────────────────────────────────

var (
    // Primary colors
    PrimaryColor   = lipgloss.Color("86")  // Teal green
    SecondaryColor = lipgloss.Color("99")  // Light purple
    AccentColor    = lipgloss.Color("213") // Pink/magenta

    // Background colors
    BackgroundColor = lipgloss.Color("0")   // Black
    SurfaceColor    = lipgloss.Color("235") // Dark gray

    // Text colors
    TextColor  = lipgloss.Color("255") // White
    MutedColor = lipgloss.Color("244") // Gray

    // Log level colors
    DebugColor    = lipgloss.Color("244") // Gray
    InfoColor     = lipgloss.Color("86")  // Teal (matches primary)
    WarningColor  = lipgloss.Color("214") // Orange
    ErrorColor    = lipgloss.Color("196") // Red
    CriticalColor = lipgloss.Color("199") // Hot pink

    // Status colors
    SuccessColor = lipgloss.Color("82")  // Green
    FailureColor = lipgloss.Color("196") // Red
)

// ── Brand Styles ─────────────────────────────────

var (
    LogoStyle = lipgloss.NewStyle().
        Foreground(PrimaryColor).
        Bold(true)

    LogoSubtleStyle = lipgloss.NewStyle().
        Foreground(SecondaryColor)

    VersionStyle = lipgloss.NewStyle().
        Foreground(MutedColor).
        Italic(true)

    TitleStyle = lipgloss.NewStyle().
        Foreground(TextColor).
        Bold(true).
        Underline(true)

    SubtitleStyle = lipgloss.NewStyle().
        Foreground(MutedColor)
)

// ── Loading Screen Styles ────────────────────────

var (
    LoadingTitleStyle = lipgloss.NewStyle().
        Foreground(PrimaryColor).
        Bold(true)

    LoadingStepStyle = lipgloss.NewStyle().
        Foreground(SuccessColor)

    LoadingCurrentStyle = lipgloss.NewStyle().
        Foreground(SecondaryColor)

    LoadingErrorStyle = lipgloss.NewStyle().
        Foreground(FailureColor).
        Bold(true)
)

// ── Main Screen Styles ──────────────────────────

var (
    HeaderStyle = lipgloss.NewStyle().
        Foreground(PrimaryColor).
        Bold(true).
        Padding(0, 1)

    HelpStyle = lipgloss.NewStyle().
        Foreground(MutedColor).
        Padding(0, 1)

    ViewportStyle = lipgloss.NewStyle().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(SurfaceColor).
        Padding(0, 1)
)

// ── Layout Styles ───────────────────────────────

var (
    CenteredStyle = lipgloss.NewStyle().
        Width(80).
        Align(lipgloss.Center)

    ContainerStyle = lipgloss.NewStyle().
        Width(60).
        Align(lipgloss.Center).
        Padding(1, 2)
)
```

---

### Step 4: Create the Main Model

This is the heart of the application. The Model holds all state and delegates rendering/updating to screen-specific methods.

**File: `internal/adapter/ui/model.go`**

```go
package ui

import (
    "OmniView/internal/adapter/storage/oracle"
    "OmniView/internal/app"
    "OmniView/internal/core/domain"
    "OmniView/internal/service/permissions"
    "OmniView/internal/service/subscribers"
    "OmniView/internal/service/tracer"
    "context"
    "time"

    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/spinner"
    "charm.land/bubbles/v2/viewport"
)

// ── Screen Constants ─────────────────────────────

const (
    screenWelcome = "welcome"
    screenLoading = "loading"
    screenMain    = "main"
)

// ── Sub-State Structs ────────────────────────────

type welcomeState struct {
    frame        int
    logoRevealed int
    showVersion  bool
    showSubtitle bool
    complete     bool
}

type loadingState struct {
    steps   []string      // completed step descriptions
    current string        // step currently in progress
    err     error         // non-nil if a step failed
    spinner spinner.Model // animated dots
}

type mainState struct {
    messages   []*domain.QueueMessage
    viewport   viewport.Model
    autoScroll bool
    ready      bool // true after first WindowSizeMsg sets dimensions
}

// ── Model ────────────────────────────────────────

// Model is the root Bubble Tea model for the entire OmniView application.
type Model struct {
    screen string
    width  int
    height int

    welcome welcomeState
    loading loadingState
    main    mainState

    // Cancellable context for all background operations
    ctx    context.Context
    cancel context.CancelFunc

    // Application services (injected via NewModel)
    dbAdapter         *oracle.OracleAdapter
    permissionService *permissions.PermissionService
    tracerService     *tracer.TracerService
    subscriberService *subscribers.SubscriberService
    appConfig         *domain.DatabaseSettings
    subscriber        *domain.Subscriber

    // Channel: event listener → TUI
    eventCh chan *domain.QueueMessage

    // App metadata
    app *app.App
}

// ModelOpts holds the dependencies injected into the Model.
type ModelOpts struct {
    App               *app.App
    DBAdapter         *oracle.OracleAdapter
    PermissionService *permissions.PermissionService
    TracerService     *tracer.TracerService
    SubscriberService *subscribers.SubscriberService
    AppConfig         *domain.DatabaseSettings
    EventCh           chan *domain.QueueMessage
}

// NewModel creates the root model with all dependencies.
func NewModel(opts ModelOpts) *Model {
    ctx, cancel := context.WithCancel(context.Background())

    // Initialize spinner for loading screen
    s := spinner.New()
    s.Spinner = spinner.Dots

    return &Model{
        screen:            screenWelcome,
        width:             80,
        height:            24,
        ctx:               ctx,
        cancel:            cancel,
        app:               opts.App,
        dbAdapter:         opts.DBAdapter,
        permissionService: opts.PermissionService,
        tracerService:     opts.TracerService,
        subscriberService: opts.SubscriberService,
        appConfig:         opts.AppConfig,
        eventCh:           opts.EventCh,
        loading: loadingState{
            spinner: s,
        },
        main: mainState{
            autoScroll: true,
        },
    }
}

// ── Init ─────────────────────────────────────────

// Init starts the welcome animation tick.
func (m *Model) Init() tea.Cmd {
    return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
        return tickMsg{time: t}
    })
}

// ── Update ───────────────────────────────────────

// Update routes messages to the active screen's handler.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Global handlers (active on every screen)
    switch msg := msg.(type) {
    case tea.KeyPressMsg:
        switch msg.String() {
        case "ctrl+c":
            m.cancel() // cancel all background operations
            return m, tea.Quit
        case "q":
            if m.screen != screenWelcome {
                m.cancel()
                return m, tea.Quit
            }
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height

        // Resize viewport if on main screen
        if m.screen == screenMain && m.main.ready {
            vpHeight := m.height - headerHeight
            if vpHeight < 1 {
                vpHeight = 1
            }
            m.main.viewport.SetWidth(m.width)
            m.main.viewport.SetHeight(vpHeight)
        }

        // Initialize viewport on first size message if we're on main screen
        if m.screen == screenMain && !m.main.ready {
            m.initViewport()
        }

        return m, nil
    }

    // Screen-specific handlers
    switch m.screen {
    case screenWelcome:
        return m.updateWelcome(msg)
    case screenLoading:
        return m.updateLoading(msg)
    case screenMain:
        return m.updateMain(msg)
    }

    return m, nil
}

// ── View ─────────────────────────────────────────

// View renders the current screen and sets terminal features.
func (m *Model) View() tea.View {
    var content string

    switch m.screen {
    case screenWelcome:
        content = m.viewWelcome()
    case screenLoading:
        content = m.viewLoading()
    case screenMain:
        if !m.main.ready {
            content = "Initializing..."
        } else {
            content = m.viewMain()
        }
    }

    // Create the view with declarative terminal features
    v := tea.NewView(content)
    v.AltScreen = true         // full-screen mode
    v.WindowTitle = "OmniView" // terminal tab title

    return v
}
```

---

### Step 5: Implement Welcome Screen Logic

The welcome screen is a direct adaptation of the existing `welcome.go` animation, now running as a screen within the main model.

**File: `internal/adapter/ui/welcome.go`**

```go
package ui

import (
    "fmt"
    "strings"
    "time"

    "OmniView/internal/adapter/ui/styles"

    tea "charm.land/bubbletea/v2"
    "charm.land/lipgloss/v2"
)

// ── Welcome Constants ────────────────────────────

const (
    tickInterval  = 80 * time.Millisecond
    versionDelay  = 4  // frames after logo before showing version
    subtitleDelay = 6  // frames after version before showing subtitle
    completeDelay = 10 // frames after subtitle before transitioning
)

var logoLines = []string{
    `  __  __ __ __  _ _  _   _  _ ___  _   _ `,
    ` /__\|  V  |  \| | || \ / || | __|| | | |`,
    "| \\/ | \\_/ | | ' | |`\\ V /'| | _| | 'V' |",
    ` \__/|_| |_|_|\__|_|  \_/  |_|___|!_/ \_!`,
}

// ── Welcome Update ───────────────────────────────

// updateWelcome handles messages when screen == "welcome".
func (m *Model) updateWelcome(msg tea.Msg) (*Model, tea.Cmd) {
    switch msg.(type) {
    case tickMsg:
        if m.welcome.complete {
            // Transition to loading screen
            m.screen = screenLoading
            return m, tea.Batch(
                m.loading.spinner.Tick(), // start spinner
                connectDBCmd(m),          // begin first loading step
            )
        }

        m.welcome.frame++

        // Reveal logo line by line (one line every 2 frames)
        if m.welcome.logoRevealed < len(logoLines) && m.welcome.frame%2 == 0 {
            m.welcome.logoRevealed++
        }

        // Show version after logo is fully revealed
        if m.welcome.logoRevealed >= len(logoLines) && m.welcome.frame >= versionDelay {
            m.welcome.showVersion = true
        }

        // Show subtitle after version
        if m.welcome.showVersion && m.welcome.frame >= subtitleDelay {
            m.welcome.showSubtitle = true
        }

        // Mark animation as complete
        if m.welcome.showSubtitle && m.welcome.frame >= completeDelay {
            m.welcome.complete = true
        }

        return m, tea.Tick(tickInterval, func(t time.Time) tea.Msg {
            return tickMsg{time: t}
        })
    }

    return m, nil
}

// ── Welcome View ─────────────────────────────────

// viewWelcome renders the welcome animation screen.
func (m *Model) viewWelcome() string {
    var b strings.Builder

    // Render revealed logo lines
    for i := 0; i < m.welcome.logoRevealed && i < len(logoLines); i++ {
        if i > 0 {
            b.WriteString("\n")
        }
        b.WriteString(styles.LogoStyle.Render(logoLines[i]))
    }

    // Version text
    if m.welcome.showVersion {
        b.WriteString("\n\n")
        versionText := fmt.Sprintf("Version: %s", m.app.GetVersion())
        b.WriteString(styles.VersionStyle.Render(versionText))
    }

    // Subtitle
    if m.welcome.showSubtitle {
        b.WriteString("\n")
        subtitleText := fmt.Sprintf("Created with ❤️ by %s", m.app.GetAuthor())
        b.WriteString(styles.LogoSubtleStyle.Render("\n" + subtitleText))
    }

    // Center in terminal
    content := b.String()
    return lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        content,
    )
}
```

---

### Step 6: Implement Loading Screen Logic

The loading screen shows a spinner with a list of completed and in-progress steps. Each step triggers the next via the message returned from its command.

**File: `internal/adapter/ui/loading.go`**

```go
package ui

import (
    "fmt"
    "strings"

    "OmniView/internal/adapter/ui/styles"

    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/spinner"
    "charm.land/lipgloss/v2"
)

// ── Loading Update ───────────────────────────────

// updateLoading handles messages when screen == "loading".
func (m *Model) updateLoading(msg tea.Msg) (*Model, tea.Cmd) {
    switch msg := msg.(type) {

    // Spinner animation frame
    case spinner.TickMsg:
        var cmd tea.Cmd
        m.loading.spinner, cmd = m.loading.spinner.Update(msg)
        return m, cmd

    // Step 1: Oracle DB connection result
    case dbConnectedMsg:
        if msg.err != nil {
            m.loading.err = fmt.Errorf("database connection failed: %w", msg.err)
            return m, nil
        }
        m.loading.steps = append(m.loading.steps, "✓ Connected to Oracle database")
        m.loading.current = "Checking permissions..."
        return m, checkPermissionsCmd(m)

    // Step 2: Permission deploy/check result
    case permissionsCheckedMsg:
        if msg.err != nil {
            m.loading.err = fmt.Errorf("permission check failed: %w", msg.err)
            return m, nil
        }
        m.loading.steps = append(m.loading.steps, "✓ Permissions verified")
        m.loading.current = "Deploying tracer package..."
        return m, deployTracerCmd(m)

    // Step 3: Tracer deploy/check result
    case tracerDeployedMsg:
        if msg.err != nil {
            m.loading.err = fmt.Errorf("tracer deployment failed: %w", msg.err)
            return m, nil
        }
        m.loading.steps = append(m.loading.steps, "✓ Tracer package deployed")
        m.loading.current = "Registering subscriber..."
        return m, registerSubscriberCmd(m)

    // Step 4: Subscriber registration result
    case subscriberRegisteredMsg:
        if msg.err != nil {
            m.loading.err = fmt.Errorf("subscriber registration failed: %w", msg.err)
            return m, nil
        }
        m.subscriber = msg.subscriber
        m.loading.steps = append(m.loading.steps,
            "✓ Subscriber registered: "+msg.subscriber.Name())
        m.loading.current = ""

        // All loading complete — transition to main screen
        m.screen = screenMain
        m.initViewport()

        // Start event listener and wait for first message
        m.tracerService.StartEventListener(m.ctx, m.subscriber, m.appConfig.Username())
        return m, waitForEventCmd(m.eventCh)
    }

    return m, nil
}

// ── Loading View ─────────────────────────────────

// viewLoading renders the loading screen with spinner and step progress.
func (m *Model) viewLoading() string {
    var b strings.Builder

    // Title
    b.WriteString(styles.LoadingTitleStyle.Render("⚙ Initializing OmniView"))
    b.WriteString("\n\n")

    // Completed steps (green checkmarks)
    for _, step := range m.loading.steps {
        b.WriteString(styles.LoadingStepStyle.Render(step))
        b.WriteString("\n")
    }

    // Error state
    if m.loading.err != nil {
        b.WriteString("\n")
        b.WriteString(styles.LoadingErrorStyle.Render("✗ " + m.loading.err.Error()))
        b.WriteString("\n\n")
        b.WriteString(styles.SubtitleStyle.Render("Press q to exit"))
        return lipgloss.Place(
            m.width, m.height,
            lipgloss.Center, lipgloss.Center,
            b.String(),
        )
    }

    // Current step with spinner
    if m.loading.current != "" {
        spinnerView := m.loading.spinner.View()
        b.WriteString(styles.LoadingCurrentStyle.Render(
            spinnerView + " " + m.loading.current,
        ))
        b.WriteString("\n")
    }

    return lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        b.String(),
    )
}
```

---

### Step 7: Implement Main Screen Logic

The main screen is a scrollable log viewer using the `viewport` bubble. New messages are appended at the bottom with auto-scroll.

**File: `internal/adapter/ui/main_screen.go`**

```go
package ui

import (
    "fmt"
    "strings"

    "OmniView/internal/adapter/ui/styles"
    "OmniView/internal/core/domain"

    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/viewport"
    "charm.land/lipgloss/v2"
)

// headerHeight is the number of terminal lines reserved for header + help.
const headerHeight = 4

// ── Main Update ──────────────────────────────────

// updateMain handles messages when screen == "main".
func (m *Model) updateMain(msg tea.Msg) (*Model, tea.Cmd) {
    switch msg := msg.(type) {

    // New log message from event listener
    case queueMessageMsg:
        m.main.messages = append(m.main.messages, msg.message)
        m.main.viewport.SetContent(m.renderLogContent())
        if m.main.autoScroll {
            m.main.viewport.GotoBottom()
        }
        // Re-subscribe to wait for next message
        return m, waitForEventCmd(m.eventCh)

    // Keyboard input
    case tea.KeyPressMsg:
        switch msg.String() {
        case "a":
            // Toggle auto-scroll
            m.main.autoScroll = !m.main.autoScroll
            if m.main.autoScroll {
                m.main.viewport.GotoBottom()
            }
            return m, nil
        }
    }

    // Forward all other messages to viewport (handles scrolling keys + mouse)
    var cmd tea.Cmd
    m.main.viewport, cmd = m.main.viewport.Update(msg)
    return m, cmd
}

// ── Main View ────────────────────────────────────

// viewMain renders the main log viewer screen.
func (m *Model) viewMain() string {
    // Header
    header := styles.HeaderStyle.Render("OmniView — Real-time Traces")

    // Help bar
    autoScrollIndicator := "off"
    if m.main.autoScroll {
        autoScrollIndicator = "on"
    }
    help := styles.HelpStyle.Render(
        fmt.Sprintf("↑/↓ scroll • a auto-scroll [%s] • q quit", autoScrollIndicator),
    )

    // Viewport
    viewportView := m.main.viewport.View()

    // Assemble layout
    return lipgloss.JoinVertical(
        lipgloss.Left,
        header,
        help,
        viewportView,
    )
}

// ── Log Rendering ────────────────────────────────

// renderLogContent formats all stored messages as a single string for the viewport.
func (m *Model) renderLogContent() string {
    if len(m.main.messages) == 0 {
        return styles.SubtitleStyle.Render("  Waiting for trace events...")
    }

    var b strings.Builder
    for _, msg := range m.main.messages {
        b.WriteString(formatLogLine(msg))
        b.WriteString("\n")
    }
    return b.String()
}

// formatLogLine applies color styling based on log level.
func formatLogLine(msg *domain.QueueMessage) string {
    timestamp := msg.Timestamp().Format("2006-01-02 15:04:05")

    // Choose color based on log level
    var levelStyle lipgloss.Style
    switch msg.LogLevel() {
    case domain.LogLevelDebug:
        levelStyle = lipgloss.NewStyle().Foreground(styles.DebugColor)
    case domain.LogLevelInfo:
        levelStyle = lipgloss.NewStyle().Foreground(styles.InfoColor)
    case domain.LogLevelWarning:
        levelStyle = lipgloss.NewStyle().Foreground(styles.WarningColor)
    case domain.LogLevelError:
        levelStyle = lipgloss.NewStyle().Foreground(styles.ErrorColor)
    case domain.LogLevelCritical:
        levelStyle = lipgloss.NewStyle().Foreground(styles.CriticalColor).Bold(true)
    default:
        levelStyle = lipgloss.NewStyle().Foreground(styles.MutedColor)
    }

    return fmt.Sprintf(
        "%s %s %s %s",
        lipgloss.NewStyle().Foreground(styles.MutedColor).Render(timestamp),
        levelStyle.Render(fmt.Sprintf("[%-8s]", msg.LogLevel())),
        lipgloss.NewStyle().Foreground(styles.SecondaryColor).Render(msg.ProcessName()),
        msg.Payload(),
    )
}

// initViewport creates and configures the viewport for the main screen.
// Called when we first receive terminal dimensions or transition to main screen.
func (m *Model) initViewport() {
    vpHeight := m.height - headerHeight
    if vpHeight < 1 {
        vpHeight = 1
    }

    m.main.viewport = viewport.New(
        viewport.WithWidth(m.width),
        viewport.WithHeight(vpHeight),
    )
    m.main.viewport.SetContent(m.renderLogContent())
    m.main.ready = true
}
```

---

### Step 8: Implement Async Commands

Commands are functions that perform I/O in background goroutines. Each returns a message that `Update()` processes.

**File: `internal/adapter/ui/commands.go`**

```go
package ui

import (
    "OmniView/internal/core/domain"

    tea "charm.land/bubbletea/v2"
)

// connectDBCmd connects to the Oracle database.
func connectDBCmd(m *Model) tea.Cmd {
    return func() tea.Msg {
        err := m.dbAdapter.Connect(m.ctx)
        return dbConnectedMsg{err: err}
    }
}

// checkPermissionsCmd deploys and verifies database permissions.
func checkPermissionsCmd(m *Model) tea.Cmd {
    return func() tea.Msg {
        _, err := m.permissionService.DeployAndCheck(m.ctx, m.appConfig.Username())
        return permissionsCheckedMsg{err: err}
    }
}

// deployTracerCmd deploys and verifies the tracer package.
func deployTracerCmd(m *Model) tea.Cmd {
    return func() tea.Msg {
        err := m.tracerService.DeployAndCheck(m.ctx)
        return tracerDeployedMsg{err: err}
    }
}

// registerSubscriberCmd registers a queue subscriber.
func registerSubscriberCmd(m *Model) tea.Cmd {
    return func() tea.Msg {
        subscriber, err := m.subscriberService.RegisterSubscriber(m.ctx)
        return subscriberRegisteredMsg{subscriber: subscriber, err: err}
    }
}

// waitForEventCmd waits for one message from the event channel.
// After Update() processes the message, it must re-issue this command
// to receive the next message. See Section 4.5 for the pattern.
func waitForEventCmd(ch <-chan *domain.QueueMessage) tea.Cmd {
    return func() tea.Msg {
        msg := <-ch // blocks until a message arrives
        return queueMessageMsg{message: msg}
    }
}
```

---

### Step 9: Create the UI Entry Point

This file exports `NewProgram()` — the only function `main.go` calls to start the TUI.

**File: `internal/adapter/ui/ui.go`**

```go
package ui

import (
    tea "charm.land/bubbletea/v2"
)

// NewProgram creates a configured tea.Program ready to Run().
func NewProgram(model *Model) *tea.Program {
    return tea.NewProgram(model)
}
```

---

### Step 10: Update main.go

The new `main.go` handles the pre-TUI phase, then delegates everything to the TUI.

**File: `cmd/omniview/main.go`**

```go
package main

import (
    "OmniView/internal/adapter/config"
    "OmniView/internal/adapter/storage/boltdb"
    "OmniView/internal/adapter/storage/oracle"
    "OmniView/internal/adapter/ui"
    "OmniView/internal/app"
    "OmniView/internal/core/domain"
    "OmniView/internal/service/permissions"
    "OmniView/internal/service/subscribers"
    "OmniView/internal/service/tracer"
    "OmniView/internal/updater"
    "fmt"
    "log"
    "os"
)

func main() {
    // ── Phase 1: Pre-TUI Setup ───────────────────
    // Operations that need stdin or may restart the binary.

    omniApp := app.New()

    // Self-update check (may replace binary and restart)
    updater.CleanupOldBinary()
    if err := updater.CheckAndUpdate(app.Version); err != nil {
        log.Printf("[updater] Update failed: %v\n", err)
    }

    // Initialize BoltDB (fast, no UI needed)
    boltAdapter := boltdb.NewBoltAdapter("omniview.bolt")
    if err := boltAdapter.Initialize(); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to initialize BoltDB: %v\n", err)
        os.Exit(1)
    }
    defer boltAdapter.Close()

    // Load configuration (may prompt user via stdin if first run)
    dbSettingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)
    cfgLoader := config.NewConfigLoader(dbSettingsRepo)
    appConfig, err := cfgLoader.LoadClientConfigurations()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
        os.Exit(1)
    }

    // ── Phase 2: Create Services ─────────────────

    // Event channel: tracer service → TUI
    eventCh := make(chan *domain.QueueMessage, 100)

    // Create adapters and services
    dbAdapter := oracle.NewOracleAdapter(appConfig)
    subscriberRepo := boltdb.NewSubscriberRepository(boltAdapter)
    permissionsRepo := boltdb.NewPermissionsRepository(boltAdapter)

    permissionService := permissions.NewPermissionService(dbAdapter, permissionsRepo, boltAdapter)
    tracerService := tracer.NewTracerService(dbAdapter, boltAdapter, eventCh)
    subscriberService := subscribers.NewSubscriberService(dbAdapter, subscriberRepo)

    // ── Phase 3: Start TUI ───────────────────────

    model := ui.NewModel(ui.ModelOpts{
        App:               omniApp,
        DBAdapter:         dbAdapter,
        PermissionService: permissionService,
        TracerService:     tracerService,
        SubscriberService: subscriberService,
        AppConfig:         appConfig,
        EventCh:           eventCh,
    })

    p := ui.NewProgram(model)
    if _, err := p.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

---

## 7. File Structure Summary

### 7.1 Target Directory Structure

```text
internal/adapter/ui/
├── ui.go              ← Entry point: NewProgram()
├── model.go           ← Model struct, Init(), Update(), View()
├── messages.go        ← Custom message type definitions
├── commands.go        ← tea.Cmd functions for async I/O
├── welcome.go         ← Welcome screen update + view logic
├── loading.go         ← Loading screen update + view logic
├── main_screen.go     ← Main log viewer update + view logic
└── styles/
    └── styles.go      ← Color palette and style definitions
```

> **Design note**: All screen files are in the same `ui` package. This avoids circular dependencies that would arise from separating screens into sub-packages (each sub-package would need to import the `Model` type from the parent package).

### 7.2 Files Changed

| File | Action | Why |
|------|--------|-----|
| `cmd/omniview/main.go` | **Rewrite** | Pre-TUI setup + `tea.Program.Run()` |
| `internal/adapter/ui/ui.go` | **Rewrite** | Slim entry: just `NewProgram()` |
| `internal/service/tracer/tracer_service.go` | **Modify** | Inject event channel, route to channel or stdout |
| `internal/adapter/ui/styles/styles.go` | **Extend** | Add log-level colors, loading/main screen styles |

### 7.3 Files Created

| File | Purpose |
|------|---------|
| `internal/adapter/ui/model.go` | Main Model struct, Init, Update, View |
| `internal/adapter/ui/messages.go` | All custom `tea.Msg` types |
| `internal/adapter/ui/commands.go` | All `tea.Cmd` functions |
| `internal/adapter/ui/welcome.go` | Welcome animation screen |
| `internal/adapter/ui/loading.go` | Loading progress screen |
| `internal/adapter/ui/main_screen.go` | Real-time log viewer screen |

### 7.4 Files Removed

| File | Why |
|------|-----|
| `internal/adapter/ui/welcome/welcome.go` | Logic moved to `internal/adapter/ui/welcome.go` |

---

## 8. Viewport Integration Details

The `viewport` bubble from Bubbles v2 handles scrollable content. Key differences from v1:

### 8.1 Constructor (v2)

```go
// v1 (WRONG — will not compile with Bubbles v2):
vp := viewport.New(80, 24)

// v2 (CORRECT):
vp := viewport.New(
    viewport.WithWidth(80),
    viewport.WithHeight(24),
)
```

### 8.2 Width/Height (v2)

```go
// v1 (WRONG — fields no longer exported):
vp.Width = 80
vp.Height = 24

// v2 (CORRECT — use setter/getter methods):
vp.SetWidth(80)
vp.SetHeight(24)
fmt.Println(vp.Width(), vp.Height())
```

### 8.3 Content Management

```go
// Set content as a single string (newlines separate lines)
vp.SetContent("line 1\nline 2\nline 3")

// Or set content as a slice of lines
vp.SetContentLines([]string{"line 1", "line 2", "line 3"})

// Retrieve content
content := vp.GetContent()
```

### 8.4 Scrolling

```go
// Programmatic scrolling
vp.GotoBottom()
vp.GotoTop()

// Keyboard scrolling is handled automatically when you pass
// tea.KeyPressMsg to viewport's Update() method.
```

### 8.5 New v2 Features

```go
// Soft wrapping (wraps long lines instead of clipping)
vp.SoftWrap = true

// Line numbers via left gutter
vp.LeftGutterFunc = func(info viewport.GutterContext) string {
    if info.Soft { return "     │ " }
    return fmt.Sprintf("%4d │ ", info.Index+1)
}

// Search highlighting
vp.SetHighlights(regexp.MustCompile("ERROR").FindAllStringIndex(content, -1))
vp.HighlightNext()
vp.HighlightPrevious()

// Per-line styling
vp.StyleLineFunc = func(lineIndex int) lipgloss.Style {
    // Apply different styles to different lines
    return lipgloss.NewStyle()
}
```

### 8.6 Removed in v2

- `HighPerformanceRendering` — removed entirely, the new Bubble Tea v2 renderer handles optimization automatically.

---

## 9. Testing Strategy

### 9.1 Unit Test Screen Rendering

Each screen's view function is a pure function of model state. Test by setting state and asserting the output contains expected strings.

```go
// File: internal/adapter/ui/welcome_test.go

package ui

import (
    "OmniView/internal/app"
    "strings"
    "testing"
)

func TestViewWelcome_ShowsLogo(t *testing.T) {
    m := NewModel(ModelOpts{
        App: app.New(),
    })
    m.welcome.logoRevealed = len(logoLines)

    view := m.viewWelcome()

    if !strings.Contains(view, `__  __`) {
        t.Error("expected logo content in welcome view")
    }
}

func TestViewWelcome_ShowsVersion(t *testing.T) {
    m := NewModel(ModelOpts{
        App: app.New(), // Version defaults to "dev"
    })
    m.welcome.logoRevealed = len(logoLines)
    m.welcome.showVersion = true

    view := m.viewWelcome()

    if !strings.Contains(view, "Version: dev") {
        t.Error("expected version string in welcome view")
    }
}
```

### 9.2 Unit Test Message Handling

Test that `Update()` returns the correct next state and command for each message type.

```go
// File: internal/adapter/ui/loading_test.go

package ui

import (
    "OmniView/internal/app"
    "fmt"
    "testing"
)

func TestUpdateLoading_DBConnected_Success(t *testing.T) {
    m := NewModel(ModelOpts{App: app.New()})
    m.screen = screenLoading

    newModel, cmd := m.Update(dbConnectedMsg{err: nil})
    updated := newModel.(*Model)

    if len(updated.loading.steps) != 1 {
        t.Errorf("expected 1 loading step, got %d", len(updated.loading.steps))
    }
    if cmd == nil {
        t.Error("expected a command to trigger next step")
    }
}

func TestUpdateLoading_DBConnected_Error(t *testing.T) {
    m := NewModel(ModelOpts{App: app.New()})
    m.screen = screenLoading

    newModel, cmd := m.Update(dbConnectedMsg{err: fmt.Errorf("connection refused")})
    updated := newModel.(*Model)

    if updated.loading.err == nil {
        t.Error("expected loading error to be set")
    }
    if cmd != nil {
        t.Error("expected no command after error")
    }
}
```

### 9.3 Integration Test

Test the full flow by simulating the message sequence:

```go
// File: internal/adapter/ui/flow_test.go

package ui

import (
    "OmniView/internal/app"
    "testing"
)

func TestFullFlow_WelcomeToLoading(t *testing.T) {
    m := NewModel(ModelOpts{App: app.New()})

    for i := 0; i < 20; i++ {
        newModel, _ := m.Update(tickMsg{})
        m = newModel.(*Model)
    }

    if m.screen != screenLoading {
        t.Errorf("expected screen %q, got %q", screenLoading, m.screen)
    }
}
```

---

## 10. Design Decisions

### 10.1 Why a Single `tea.Program`?

| Consideration | Separate Programs | Single Program |
|--------------|------------------|----------------|
| Screen transitions | Visual flash, state lost | Seamless, state preserved |
| Signal handling | Manual per program | Built-in via `tea.KeyPressMsg` |
| Shared state | Must serialize/pass between programs | Direct access in Model |
| Complexity | Simpler per-screen, complex orchestration | One model, clear flow |

### 10.2 Why Channel-Based Events?

The `TracerService.blockingConsumerLoop()` runs a blocking Oracle dequeue. This cannot run in the Bubble Tea event loop. The channel pattern:

1. The blocking loop runs in its own goroutine (unchanged).
2. It sends parsed messages through a buffered channel.
3. A `tea.Cmd` reads one message from the channel and returns it.
4. `Update()` appends the message to state and re-subscribes.

This keeps the service layer unchanged and the TUI responsive.

### 10.3 Why Pre-TUI Phase?

The `ConfigLoader.LoadClientConfigurations()` uses `bufio.NewReader(os.Stdin)` to prompt the user for database credentials when no saved config exists. Since Bubble Tea takes exclusive control of stdin to read terminal events, using `os.Stdin` directly inside the TUI would conflict.

**Future enhancement**: Replace the stdin-based config prompt with a TUI settings screen using `textinput` bubbles from Bubbles v2. This would move BoltDB init and config loading into the TUI loading sequence.

### 10.4 Why Same Package for All Screen Files?

Placing screen logic (`welcome.go`, `loading.go`, `main_screen.go`) in the same `ui` package avoids circular imports. If screens were in sub-packages (`screens/welcome/`), they would need to import the `Model` type from `ui`, while `ui` would need to call their render functions — creating a dependency cycle.

The trade-off is a larger package, but with clear file boundaries and a small number of screens (3), this is manageable.

### 10.5 Message Buffer Limit

For long-running sessions, unbounded `messages` growth could exhaust memory. Consider adding a maximum:

```go
const maxMessages = 10000

// In updateMain, after appending:
if len(m.main.messages) > maxMessages {
    // Keep the most recent half
    m.main.messages = m.main.messages[maxMessages/2:]
    m.main.viewport.SetContent(m.renderLogContent())
}
```

---

## 11. Migration Checklist

Use this checklist to track implementation progress.

- [ ] **Step 1**: Modify `TracerService` — add `eventCh` parameter to constructor, route messages to channel
- [ ] **Step 2**: Create `internal/adapter/ui/messages.go` — all message types
- [ ] **Step 3**: Update `internal/adapter/ui/styles/styles.go` — log-level colors, new screen styles
- [ ] **Step 4**: Create `internal/adapter/ui/model.go` — Model struct, sub-states, `NewModel()`, `Init()`
- [ ] **Step 5**: Create `internal/adapter/ui/welcome.go` — `updateWelcome()`, `viewWelcome()`
- [ ] **Step 6**: Create `internal/adapter/ui/loading.go` — `updateLoading()`, `viewLoading()`
- [ ] **Step 7**: Create `internal/adapter/ui/main_screen.go` — `updateMain()`, `viewMain()`, viewport init
- [ ] **Step 8**: Create `internal/adapter/ui/commands.go` — all `tea.Cmd` functions
- [ ] **Step 9**: Append `Update()` and `View()` to `model.go` — wire screen handlers
- [ ] **Step 10**: Rewrite `internal/adapter/ui/ui.go` — slim `NewProgram()` entry point
- [ ] **Step 11**: Rewrite `cmd/omniview/main.go` — pre-TUI setup + `tea.Program.Run()`
- [ ] **Step 12**: Remove `internal/adapter/ui/welcome/` directory (old welcome package)
- [ ] **Step 13**: Run `go mod tidy` to clean up dependencies
- [ ] **Step 14**: Test: verify `go build ./...` compiles
- [ ] **Step 15**: Test: run application, verify welcome → loading → main flow
- [ ] **Step 16**: Test: verify Ctrl+C and q quit cleanly from all screens
- [ ] **Step 17**: Test: verify terminal resize updates layout correctly

---

## 12. Reference Materials

| Resource | URL |
|----------|-----|
| Bubble Tea v2 Docs | https://pkg.go.dev/charm.land/bubbletea/v2 |
| Bubble Tea v2 Upgrade Guide | https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md |
| Lipgloss v2 Docs | https://pkg.go.dev/charm.land/lipgloss/v2 |
| Lipgloss v2 Upgrade Guide | https://github.com/charmbracelet/lipgloss/blob/main/UPGRADE_GUIDE_V2.md |
| Bubbles v2 (Viewport, Spinner) | https://pkg.go.dev/charm.land/bubbles/v2 |
| Bubbles v2 Upgrade Guide | https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md |
| Bubble Tea Realtime Example | https://github.com/charmbracelet/bubbletea/tree/main/examples/realtime |
| Bubble Tea Pager Example | https://github.com/charmbracelet/bubbletea/tree/main/examples/pager |

---

## Appendix A: QueueMessage Domain Entity

From `internal/core/domain/queue_message.go` — this is the data that flows from the event listener through the channel to the viewport.

```go
// Entity: Represents a message in the tracer queue
type QueueMessage struct {
    messageID   string      // unique identifier
    processName string      // Oracle process name
    logLevel    LogLevel    // DEBUG | INFO | WARNING | ERROR | CRITICAL
    payload     string      // log message content
    timestamp   time.Time   // when the event occurred
}

// Getters (unexported fields, read-only access)
func (m *QueueMessage) MessageID() string    { return m.messageID }
func (m *QueueMessage) ProcessName() string  { return m.processName }
func (m *QueueMessage) LogLevel() LogLevel   { return m.logLevel }
func (m *QueueMessage) Payload() string      { return m.payload }
func (m *QueueMessage) Timestamp() time.Time { return m.timestamp }

// Format returns a plain-text representation for display
func (m *QueueMessage) Format() string {
    return fmt.Sprintf("[%s] [%s] %s: %s",
        m.timestamp.Format("2006-01-02 15:04:05"),
        m.logLevel,
        m.processName,
        m.payload,
    )
}
```

## Appendix B: Color Palette Reference

```text
PrimaryColor    = "86"   → Teal green    ████
SecondaryColor  = "99"   → Light purple  ████
AccentColor     = "213"  → Pink/magenta  ████
BackgroundColor = "0"    → Black         ████
SurfaceColor    = "235"  → Dark gray     ████
TextColor       = "255"  → White         ████
MutedColor      = "244"  → Gray          ████
DebugColor      = "244"  → Gray          ████
InfoColor       = "86"   → Teal          ████
WarningColor    = "214"  → Orange        ████
ErrorColor      = "196"  → Red           ████
CriticalColor   = "199"  → Hot pink      ████
SuccessColor    = "82"   → Green         ████
FailureColor    = "196"  → Red           ████
```
