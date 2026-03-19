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

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Screen Constants
// ==========================================

const (
	screenWelcome = "welcome"
	screenLoading = "loading"
	screenMain    = "main"
)

// ==========================================
// Sub-State Structs
// ==========================================

type welcomeState struct {
	frame        int
	logoRevealed int
	showVersion  bool
	showSubtitle bool
	complete     bool
}

type loadingState struct {
	steps   []string      // Completed steps descriptions
	current string        // Step currently in progress
	err     error         // Error encountered during loading, if any
	spinner spinner.Model // Animated dots TODO: Make this into a loading progress bar
}

type mainState struct {
	messages   []*domain.QueueMessage // Log messages to display
	viewport   viewport.Model         // Scrollable viewport for messages
	autoScroll bool                   // Whether to auto-scroll to the latest message
	ready      bool                   // Whether the main screen is ready to display messages
}

// ==========================================
// Model
// ==========================================

// Model is the root Bubble Tea model for entire Omniview application
type Model struct {
	screen string // Current screen: welcome, loading, or main
	width  int    // Terminal width
	height int    // Terminal height

	welcome welcomeState
	loading loadingState
	main    mainState

	// Cancellable contexts for all backgroun operions
	ctx    context.Context
	cancel context.CancelFunc

	// Application Services (injected via NewModel)
	dbAdapter         *oracle.OracleAdapter
	permissionService *permissions.PermissionService
	tracerService     *tracer.TracerService
	subscriberService *subscribers.SubscriberService
	appConfig         *domain.DatabaseSettings
	subscriber        *domain.Subscriber

	// Channel: event listener -> TUI
	eventChannel chan *domain.QueueMessage

	// App reference for accessing global state and methods
	app *app.App
}

// ModelOpts holds the dependencies injected into the Model
type ModelOpts struct {
	App               *app.App
	DBAdapter         *oracle.OracleAdapter
	PermissionService *permissions.PermissionService
	TracerService     *tracer.TracerService
	SubscriberService *subscribers.SubscriberService
	AppConfig         *domain.DatabaseSettings
	EventChannel      chan *domain.QueueMessage
}

func NewModel(opts ModelOpts) *Model {
	ctx, cancel := context.WithCancel(context.Background())

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
	)

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
		eventChannel:      opts.EventChannel,
		loading: loadingState{
			spinner: s,
		},
		main: mainState{
			autoScroll: true,
		},
	}
}

// ==========================================
// init
// ==========================================

// Init starts the welcome screen animation
func (m *Model) Init() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg{time: t}
	})
}

// ==========================================
// Update
// ==========================================

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global handler (active on every screen)
	switch msg := msg.(type) {

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel() // Cancel all background operations
			return m, tea.Quit
		case "q":
			if m.screen == screenWelcome {
				m.cancel() // Cancel all background operations
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Resize viewport if on Main Screen
		if m.screen == screenMain && m.main.ready {
			viewportHeight := m.height - headerHeight
			if viewportHeight < 1 {
				viewportHeight = 1
			}
			m.main.viewport.SetWidth(m.width)
			m.main.viewport.SetHeight(viewportHeight)
		}

		// Initialize viewport on first size message if we are on the main screen
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

// ==========================================
// View
// ==========================================

// View renders the current screen based on m.screen
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
