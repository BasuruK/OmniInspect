package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/app"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	updaterSvc "OmniView/internal/service/updater"
	"OmniView/internal/updater"
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// ==========================================
// Screen Constants
// ==========================================

const (
	screenWelcome    = "welcome"
	screenLoading    = "loading"
	screenMain       = "main"
	screenOnboarding = "onboarding"

	// Panel height calculation: subtract border/padding and spacing from content height
	panelHeightCompensation = 3
	// Minimum usable panel height to ensure viewport remains functional
	minPanelHeight = 7
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
	messages        []*domain.QueueMessage // Log messages to display (bounded ring buffer, max 1000)
	renderedContent strings.Builder        // Incrementally built rendered content
	viewport        viewport.Model         // Scrollable viewport for messages
	autoScroll      bool                   // Whether to auto-scroll to the latest message
	ready           bool                   // Whether the main screen is ready to display messages

	// Cached column widths for trace layout optimization
	// Avoids O(n) full scan of messages on each new message
	cachedLevelWidth int // Cached maximum level column width
	cachedAPIWidth   int // Cached maximum API/process name column width
	cachedWidthKey   int // Last viewport width used to compute cached values (0 if invalid)
}

// onboardingState holds the state for the database configuration onboarding form.
// It embeds AddDatabaseForm to reuse the well-tested form component.
type onboardingState struct {
	AddDatabaseForm AddDatabaseForm
	errMsg          string
	submitted       bool
}

// updateState holds the state for update checking and application.
type updateState struct {
	checking  bool                // Whether an update check is in progress
	info      *updater.UpdateInfo // Available update info, if any
	prompting bool                // Whether user is being prompted to update
	applying  bool                // Whether the update is being applied
	stage     string              // Current progress stage description
	err       error               // Error encountered, if any
}

// ==========================================
// Model
// ==========================================

// Model is the root Bubble Tea model for entire Omniview application
type Model struct {
	screen string // Current screen: welcome, loading, main, or onboarding
	width  int    // Terminal width
	height int    // Terminal height

	welcome    welcomeState
	loading    loadingState
	main       mainState
	onboarding onboardingState
	dbSettings databaseSettingsState
	update     updateState

	// Cancellable contexts for all background operations
	ctx               context.Context
	cancel            context.CancelFunc
	eventStreamCtx    context.Context
	eventStreamCancel context.CancelFunc

	// BoltDB adapter for config read/write (used by onboarding)
	boltAdapter *boltdb.BoltAdapter
	dbFactory   DatabaseAdapterFactory

	// Application Services (injected via NewModel)
	dbSettingsRepo    ports.DatabaseSettingsRepository
	dbAdapter         ports.DatabaseRepository
	permissionService *permissions.PermissionService
	tracerService     *tracer.TracerService
	subscriberService *subscribers.SubscriberService
	updaterService    *updaterSvc.UpdaterService
	appConfig         *domain.DatabaseSettings
	subscriber        *domain.Subscriber

	// Channel: event listener -> TUI
	eventChannel chan *domain.QueueMessage

	// App reference for accessing global state and methods
	app *app.App

	// Internal message channel for update-related events
	updateEventChannel chan tea.Msg
}

// ModelOpts holds the dependencies injected into the Model
type ModelOpts struct {
	App                *app.App
	BoltAdapter        *boltdb.BoltAdapter
	DBFactory          DatabaseAdapterFactory
	DBSettingsRepo     ports.DatabaseSettingsRepository
	DBAdapter          ports.DatabaseRepository
	PermissionService  *permissions.PermissionService
	TracerService      *tracer.TracerService
	SubscriberService  *subscribers.SubscriberService
	UpdaterService     *updaterSvc.UpdaterService
	AppConfig          *domain.DatabaseSettings // Optional — onboarding screen populates this
	EventChannel       chan *domain.QueueMessage
	UpdateEventChannel chan tea.Msg // Optional - can be created by Model if not provided
}

func NewModel(opts ModelOpts) (*Model, error) {
	var errs []string

	if opts.App == nil {
		errs = append(errs, "App is required")
	}
	if opts.BoltAdapter == nil {
		errs = append(errs, "BoltAdapter is required")
	}
	if opts.DBSettingsRepo == nil {
		errs = append(errs, "DBSettingsRepo is required")
	}
	if opts.DBFactory == nil {
		errs = append(errs, "DBFactory is required")
	}
	if opts.UpdaterService == nil {
		errs = append(errs, "UpdaterService is required")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("new model: %s", strings.Join(errs, "; "))
	}

	// Determine channel values — use injected channels if provided, otherwise create default buffered channels
	eventChannel := opts.EventChannel
	if eventChannel == nil {
		eventChannel = make(chan *domain.QueueMessage, 16)
	}

	updateEventChannel := opts.UpdateEventChannel
	if updateEventChannel == nil {
		updateEventChannel = make(chan tea.Msg, 16)
	}

	ctx, cancel := context.WithCancel(context.Background())
	eventStreamCtx, eventStreamCancel := context.WithCancel(ctx)

	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
	)

	return &Model{
		screen:             screenWelcome,
		width:              80,
		height:             24,
		ctx:                ctx,
		cancel:             cancel,
		eventStreamCtx:     eventStreamCtx,
		eventStreamCancel:  eventStreamCancel,
		boltAdapter:        opts.BoltAdapter,
		dbFactory:          opts.DBFactory,
		dbSettingsRepo:     opts.DBSettingsRepo,
		app:                opts.App,
		dbAdapter:          opts.DBAdapter,
		permissionService:  opts.PermissionService,
		tracerService:      opts.TracerService,
		subscriberService:  opts.SubscriberService,
		updaterService:     opts.UpdaterService,
		appConfig:          opts.AppConfig,
		eventChannel:       eventChannel,
		updateEventChannel: updateEventChannel,
		loading: loadingState{
			spinner: s,
		},
		main: mainState{
			autoScroll: true,
		},
	}, nil
}

func (m *Model) resetConnectionEventStream() {
	if m.eventStreamCancel != nil {
		m.eventStreamCancel()
	}

	bufferSize := 16
	if m.eventChannel != nil && cap(m.eventChannel) > 0 {
		bufferSize = cap(m.eventChannel)
	}

	m.eventChannel = make(chan *domain.QueueMessage, bufferSize)
	m.eventStreamCtx, m.eventStreamCancel = context.WithCancel(m.ctx)
}

// initializeServices: initializes or reinitializes database adapter and dependent services if they are not already set.
func (m *Model) initializeServices() error {
	if m.appConfig == nil {
		return fmt.Errorf("initializeServices: database configuration is required")
	}

	if m.dbAdapter == nil {
		var err error
		m.dbAdapter, err = m.dbFactory(m.appConfig)
		if err != nil {
			return fmt.Errorf("initializeServices: failed to create db adapter: %w", err)
		}
		if m.dbAdapter == nil {
			return fmt.Errorf("initializeServices: db factory returned nil adapter")
		}
	}

	if m.permissionService == nil {
		permissionsRepo := boltdb.NewPermissionsRepository(m.boltAdapter)
		m.permissionService = permissions.NewPermissionService(m.dbAdapter, permissionsRepo, m.boltAdapter)
	}
	if m.tracerService == nil {
		var err error
		m.tracerService, err = tracer.NewTracerService(m.dbAdapter, m.boltAdapter, m.eventChannel)
		if err != nil {
			return fmt.Errorf("initializeServices: failed to create tracer service: %w", err)
		}
	}
	if m.subscriberService == nil {
		subscriberRepo := boltdb.NewSubscriberRepository(m.boltAdapter)
		m.subscriberService = subscribers.NewSubscriberService(m.dbAdapter, subscriberRepo)
	}

	return nil
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

// Update: Bubble Tea update function that handles global events (quit, resize) and delegates to screen-specific handlers.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global handler (active on every screen)
	switch msg := msg.(type) {

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel() // Cancel all background operations
			return m, tea.Quit
		case "q":
			// Only quit from screens that don't need 'q' for navigation
			if (m.screen == screenMain && !m.dbSettings.visible) || m.screen == screenWelcome || m.screen == screenLoading {
				m.cancel()
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Resize viewport if on Main Screen
		if m.screen == screenMain && m.main.ready {
			m.resizeMainViewport()
		}

		m.resizeDatabaseSettings(msg.Width, msg.Height)

		// Resize AddDatabaseForm if on Onboarding Screen
		if m.screen == screenOnboarding {
			m.onboarding.AddDatabaseForm = m.onboarding.AddDatabaseForm.WithDimensions(m.width, m.height)
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
	case screenOnboarding:
		return m.updateOnboarding(msg)
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
			if m.dbSettings.visible {
				content = renderCenteredOverlay(content, m.viewDatabaseSettings(), m.width, m.height)
				if m.dbSettings.showAddForm {
					content = renderCenteredOverlay(content, m.dbSettings.addForm.Modal(), m.width, m.height)
				}
			}
		}
	case screenOnboarding:
		content = m.viewOnboarding()
	}

	if m.width > 0 && m.height > 0 {
		content = styles.AppStyle.Width(m.width).Height(m.height).Render(content)
	}

	// Create the view with declarative terminal features
	v := tea.NewView(content)
	v.AltScreen = true         // full-screen mode
	v.WindowTitle = "OmniView" // terminal tab title

	return v
}
