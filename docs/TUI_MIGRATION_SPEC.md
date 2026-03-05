# OmniView TUI Migration Specification

## Document Information

- **Project**: OmniView TUI Migration
- **Target Framework**: Bubble Tea v2 + Lipgloss v2 + Bubbles v2
- **Version**: 1.0.0
- **Status**: Specification

---

## 1. Executive Summary

This document specifies the migration of OmniView from a hybrid CLI/TUI application to a complete Terminal User Interface (TUI) using Bubble Tea v2.

### Current State
- Welcome screen: Bubble Tea (animation ~3 seconds)
- Main application: CLI (console output)
- Tracer output: Printed to stdout via `fmt.Println()`

### Target State
- Welcome screen: Bubble Tea (animated splash)
- Loading screen: Bubble Tea (DB connection, package deployment progress)
- Main screen: Bubble Tea + Lipgloss (real-time log viewer)
- The entire application runs within a single Bubble Tea program

---

## 2. Current Architecture Analysis

### 2.1 Current main() Flow

```
┌─────────────────────────────────────────────────────────────┐
│  main()                                                     │
├─────────────────────────────────────────────────────────────┤
│  1. app.New()                                              │
│  2. ui.StartWelcome() ← Bubble Tea splash screen           │
│     - Runs animation (~3 seconds)                          │
│     - Exits with tea.Quit                                  │
│  3. updater.CleanupOldBinary()                            │
│  4. updater.CheckAndUpdate()                              │
│  5. signalChan, done channel setup                         │
│  6. ctx, cancel := context.WithCancel()                   │
│  7. boltAdapter.Initialize() ← BoltDB                     │
│  8. LoadClientConfigurations() ← config loader            │
│  9. dbAdapter.Connect(ctx) ← Oracle DB                    │
│ 10. permissionService.DeployAndCheck()                    │
│ 11. tracerService.DeployAndCheck()                        │
│ 12. subscriberService.RegisterSubscriber()                 │
│ 13. tracerService.StartEventListener() ← BLOCKING         │
│     - Listens for Oracle queue messages                   │
│     - Prints to stdout via fmt.Println()                 │
│  14. omniApp.ShowStatus(done) ← waits for Enter           │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 Key Components to Migrate

| Component | Type | Current | Target |
|-----------|------|---------|--------|
| Welcome | UI | Bubble Tea | Bubble Tea |
| Loading | UI | Console | Bubble Tea |
| Main Logs | UI | stdout | Bubble Tea + Viewport |
| Database | Service | In main() | In Model |
| Event Listener | Service | Blocking goroutine | Channel-based |
| Signal Handling | System | In main() | Bubble Tea built-in |

---

## 3. Target Architecture

### 3.1 Single Program Architecture

```text
┌────────────────────────────────────────────────────────────────┐
│                    tea.Program (ONE program)                   │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                         Model                             │ │
│  │  ┌────────────────────────────────────────────────────┐   │ │
│  │  │ screen: string       // "welcome" | "loading" |    │   │ │
│  │  │                      // "main" | "settings"        │   │ │
│  │  │                                                    │   │ │
│  │  │ // Application Services (same as current)          │   │ │
│  │  │ dbAdapter: *OracleAdapter                          │   │ │
│  │  │ boltAdapter: *BoltAdapter                          │   │ │
│  │  │ permissionService: *PermissionService              │   │ │
│  │  │ tracerService: *TracerService                      │   │ │
│  │  │ subscriberService: *SubscriberService              │   │ │
│  │  │                                                    │   │ │
│  │  │ // Tracer Data                                     │   │ │
│  │  │ messages: []QueueMessage   // Log messages         │   │ │
│  │  │ eventCh: chan QueueMessage // Channel for msgs     │   │ │
│  │  │                                                    │   │ │
│  │  │ // UI State                                        │   │ │
│  │  │ viewport: viewport.Model  // For scrolling logs    │   │ │
│  │  │ loadingProgress: []string // Status messages       │   │ │
│  │  │ loadingError: error       // If loading fails      │   │ │
│  │  │ width: int, height: int   // Terminal size         │   │ │
│  │  └────────────────────────────────────────────────────┘   │ │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │ Update(msg tea.Msg) ──► Handles all events               │  │
│  │                                                          │  │
│  │ • tickMsg: Animation frames                              │  │
│  │ • tea.KeyPressMsg: Navigation (q=quit, etc.)             │  │
│  │ • tea.WindowSizeMsg: Resize handling                     │  │
│  │ • DBConnectedMsg: DB connection complete                │   │
│  │ • LoadingProgressMsg: Status update                      │  │
│  │ • QueueMessageMsg: New log message from tracer          │   │
│  │ • LoadingCompleteMsg: All setup done                     │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ View() string ──► Renders based on screen               │ │
│  │                                                            │ │
│  │ switch m.screen {                                        │ │
│  │ case "welcome": return renderWelcome()                  │ │
│  │ case "loading": return renderLoading()                  │ │
│  │ case "main":    return renderMain()                     │ │
│  │ case "settings": return renderSettings()                 │ │
│  │ }                                                        │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                                │
└────────────────────────────────────────────────────────────────┘

### 3.2 Screen Flow

┌──────────┐    animation    ┌─────────┐    DB ready    ┌────────┐
│ welcome  │────────────────►│ loading │───────────────►│  main  │
│ (3 secs) │                 │         │                │        │
└──────────┘                 └────┬────┘                └────┬───┘
    │                             │                          │
    │                      ┌──────┴──────┐          ┌────────┴──────┐
    │                      │             │          │               │
    │               DB Connection   Deployment      │ settings      │
    │               Progress Bar    Progress        └───────────────┘
    │                                                  │
    ▼                                                  │
  tea.Quit                                             ▼
  (if user presses q)                            tea.Quit (from any)
```

---

## 4. Detailed Specification

### 4.1 Message Types

Define custom messages for communication within the TUI:

```go
// Animation tick
type TickMsg time.Time

// Database operations
type DBConnectedMsg struct{ Err error }
type LoadingProgressMsg struct{ Message string }
type LoadingCompleteMsg struct{ Err error }

// Tracer events (from background to UI)
type QueueMessageMsg struct{ Message *domain.QueueMessage }

// Navigation
type NavigateMsg struct{ TargetScreen string }
```

### 4.2 Model Structure

```go
// Model is the single source of truth for the entire application
type Model struct {
    // Screen navigation
    screen string // "welcome", "loading", "main", "settings"

    // Terminal dimensions
    width  int
    height int

    // Database adapters and services (from current main.go)
    dbAdapter         *oracle.OracleAdapter
    boltAdapter       *boltdb.BoltAdapter
    permissionService *permissions.PermissionService
    tracerService     *tracer.TracerService
    subscriberService *subscribers.SubscriberService

    // Loading state
    loadingProgress []string  // Status messages
    loadingError    error

    // Main screen state
    messages  []QueueMessage  // Log messages
    viewport  viewport.Model   // Scrollable log view
    autoScroll bool           // Auto-scroll to bottom

    // App metadata
    appVersion string
    appName    string
}
```

### 4.3 Initialization (Init method)

The Init method starts the loading process:

```go
func (m *Model) Init() tea.Cmd {
    // Start with welcome screen animation
    return tea.Batch(
        // Start animation tick
        tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
            return TickMsg{t}
        }),
    )
}
```

After welcome completes, send a message to start loading:

```go
case TickMsg:
    if m.isWelcomeComplete {
        return m, StartLoadingCmd  // Command that runs DB initialization
    }
```

### 4.4 Background Task Integration Pattern

Use channels to communicate between background services and the UI (based on Bubble Tea realtime example):

```go
// Channel for tracer messages
eventCh := make(chan domain.QueueMessage, 100)

// Command that runs the event listener in background
func startEventListenerCmd(eventCh chan domain.QueueMessage) tea.Cmd {
    return func() tea.Msg {
        for {
            msg := <-eventCh  // Blocks until message arrives
            return QueueMessageMsg{Message: &msg}
        }
    }
}

// In Update:
case QueueMessageMsg:
    m.messages = append(m.messages, *msg.Message)
    m.viewport.SetContent(renderMessages(m.messages))
    if m.autoScroll {
        m.viewport.GotoBottom()
    }
    return m, nil
```

### 4.5 Loading Process

Instead of running all initialization in main(), run as commands:

```go
// Command to run DB initialization
func runLoadingCmd() tea.Cmd {
    return func() tea.Msg {
        // This runs in background, sends progress messages
        return LoadingProgressMsg{Message: "Initializing BoltDB..."}
    }
}

// In Update:
case LoadingProgressMsg:
    m.loadingProgress = append(m.loadingProgress, msg.Message)
    return m, nil
```

### 4.6 View Functions

Each screen has its own render function:

```go
func (m *Model) View() string {
    switch m.screen {
    case "welcome":
        return m.renderWelcome()
    case "loading":
        return m.renderLoading()
    case "main":
        return m.renderMain()
    case "settings":
        return m.renderSettings()
    }
    return ""
}

func (m *Model) renderMain() string {
    // Use Lipgloss for layout
    header := styles.TitleStyle.Render("OmniView - Real-time Traces")
    help := styles.MutedStyle.Render("↑/↓ scroll | a toggle auto-scroll | q quit")

    // Viewport for logs
    logContent := m.viewport.View()

    return lipgloss.Place(
        m.width, m.height,
        lipgloss.Center, lipgloss.Top,
        header+"\n"+help+"\n\n"+logContent,
    )
}
```

---

## 5. File Structure

### 5.1 New Directory Structure

```
internal/adapter/ui/
├── ui.go                    # Main entry point, tea.Program creation
├── model.go                 # Model struct, Init, Update, View
├── messages.go              # Custom message type definitions
├── screens/
│   ├── welcome/
│   │   └── welcome.go       # Welcome screen rendering
│   ├── loading/
│   │   └── loading.go       # Loading screen rendering
│   ├── main/
│   │   └── main.go          # Main log viewer rendering
│   └── settings/
│       └── settings.go      # Settings screen rendering
├── commands/
│   ├── loading.go           # Loading process commands
│   └── events.go            # Event listener commands
└── assets/
    └── logo.go              # ASCII logo (moved from app.go)
```

### 5.2 Files to Modify

| File | Action |
|------|--------|
| `cmd/omniview/main.go` | Replace with simple tea.Program.Run() |
| `internal/adapter/ui/ui.go` | Rewrite as main entry point |
| `internal/adapter/ui/model.go` | New - main Model |
| `internal/adapter/ui/messages.go` | New - message types |
| `internal/app/app.go` | Remove logo, keep app metadata |
| `internal/adapter/ui/assets/logo.go` | New - ASCII logo |

### 5.3 Files to Create

| File | Description |
|------|-------------|
| `model.go` | Main Model struct with all state |
| `messages.go` | Custom tea.Msg types |
| `commands/loading.go` | Background loading commands |
| `commands/events.go` | Event listener commands |
| `screens/welcome/welcome.go` | Welcome rendering |
| `screens/loading/loading.go` | Loading rendering |
| `screens/main/main.go` | Main log viewer rendering |
| `screens/settings/settings.go` | Settings rendering |

---

## 6. Implementation Steps

### Step 1: Prepare Dependencies

Add Bubbles to go.mod:

```bash
go get charm.land/bubbles/v2
```

### Step 2: Create Message Types

Create `internal/adapter/ui/messages.go`:

```go
package ui

import "OmniView/internal/core/domain"

// TickMsg - animation frame
type TickMsg struct{ Time time.Time }

// DBConnectedMsg - database connection result
type DBConnectedMsg struct{ Err error }

// LoadingProgressMsg - loading status update
type LoadingProgressMsg struct{ Message string }

// LoadingCompleteMsg - all loading done
type LoadingCompleteMsg struct{ Err error }

// QueueMessageMsg - new tracer message
type QueueMessageMsg struct{ Message *domain.QueueMessage }

// NavigateMsg - screen navigation
type NavigateMsg struct{ TargetScreen string }
```

### Step 3: Create the Main Model

Create `internal/adapter/ui/model.go` with the Model struct and Init, Update, View methods.

### Step 4: Extract Commands

Create `internal/adapter/ui/commands/` for:
- Database initialization commands
- Event listener commands

### Step 5: Create Screen Renderers

Create `internal/adapter/ui/screens/` for each screen's View function.

### Step 6: Modify main.go

Replace current main.go with:

```go
package main

import (
    "OmniView/internal/adapter/ui"
    "OmniView/internal/app"
    "os"
)

func main() {
    model := ui.NewModel(app.New())
    if _, err := ui.NewProgram(model).Run(); err != nil {
        os.Exit(1)
    }
}
```

---

## 7. Key Technical Decisions

### 7.1 Why Single Program?

- **No screen flashing** between program transitions
- **Shared state** between screens
- **Built-in signal handling** (Ctrl+C)
- **Simpler** mental model

### 7.2 Why Channel-Based Events?

Following Bubble Tea's realtime example:
- Background goroutines send messages via channel
- Update() receives messages and updates state
- View() re-renders automatically
- No direct state mutation from goroutines

### 7.3 Why Viewport?

From Bubbles documentation:
- Handles scrolling natively
- Supports keyboard and mouse
- Can append content dynamically
- Auto-scroll support for real-time logs

---

## 8. Differences from Current Implementation

| Aspect | Current | New TUI |
|--------|---------|---------|
| Welcome | Separate tea.Program | Same program, different screen |
| Loading | Console print statements | Progress bar in TUI |
| Logs | stdout via fmt.Println | Viewport in TUI |
| Exit | Ctrl+C + Enter | Ctrl+C or q |
| Scrolling | N/A | Arrow keys in viewport |
| Window resize | N/A | Handled via WindowSizeMsg |

---

## 9. Testing Strategy

### 9.1 Unit Test Each Screen

```go
func TestWelcomeView(t *testing.T) {
    m := NewModel()
    m.screen = "welcome"
    m.frame = 5

    view := m.renderWelcome()
    // Assert view contains expected content
}
```

### 9.2 Integration Test

- Test full flow: welcome → loading → main
- Test DB connection failure handling
- Test event listener message flow

---

## 10. Open Questions

1. **Settings screen**: What settings should be configurable in the TUI?
2. **Message buffer**: Should we limit the number of stored messages to prevent memory issues?
3. **Filtering**: Should we add log level filtering in the main view?
4. **Alternative entry**: Should there be a CLI mode fallback?

---

## 11. Reference Materials

- [Bubble Tea v2 Documentation](https://pkg.go.dev/charm.land/bubbletea/v2)
- [Lipgloss v2 Documentation](https://pkg.go.dev/charm.land/lipgloss/v2)
- [Bubbles Viewport](https://pkg.go.dev/charm.land/bubbles/v2/viewport)
- [Bubble Tea Examples](https://github.com/charmbracelet/bubbletea/tree/main/examples)

---

## Appendix A: Current Styles

From `internal/adapter/ui/styles/styles.go`:

```go
PrimaryColor    = lipgloss.Color("86")   // Teal green
SecondaryColor  = lipgloss.Color("99")   // Light purple
AccentColor     = lipgloss.Color("213")  // Pink/magenta
BackgroundColor = lipgloss.Color("0")   // Black
SurfaceColor    = lipgloss.Color("235")  // Dark gray
TextColor       = lipgloss.Color("255") // White
MutedColor      = lipgloss.Color("244") // Gray
```

---

## Appendix B: QueueMessage Structure

From `internal/core/domain/queue_message.go`:

```go
type QueueMessage struct {
    messageID   string
    processName string
    logLevel    LogLevel  // DEBUG, INFO, WARNING, ERROR, CRITICAL
    payload     string
    timestamp   time.Time
}

func (m *QueueMessage) Format() string {
    return fmt.Sprintf("[%s] [%s] %s: %s",
        m.timestamp.Format("2006-01-02 15:04:05"),
        m.logLevel,
        m.processName,
        m.payload,
    )
}
```
