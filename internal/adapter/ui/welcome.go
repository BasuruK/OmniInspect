package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/adapter/ui/animations"
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"

	"context"
	"errors"
	"fmt"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m *Model) updateWelcome(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case welcomeResizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.welcome.loadingStarted {
			m.welcome.progressBar.SetWidth(computeProgressBarWidth(m.width))
		}
		return m, nil

	case dbReadyMsg:
		return m.handleDBReady(msg)
	}

	// Route loading-sequence messages while the animation is still playing.
	if m.welcome.loadingStarted {
		switch msg.(type) {
		case progress.FrameMsg, dbConnectedMsg, permissionsCheckedMsg, tracerDeployedMsg, subscriberRegisteredMsg:
			return m.handleWelcomeLoadingMsg(msg)
		}
	}

	return m.handleWelcomeTick(msg)
}

func (m *Model) handleWelcomeGlobal(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.welcome.loadingStarted {
			m.welcome.progressBar.SetWidth(computeProgressBarWidth(m.width))
		}
		return m, func() tea.Msg {
			return welcomeResizeMsg{Width: msg.Width, Height: msg.Height}
		}
	}
	return m, nil
}

func (m *Model) handleWelcomeTick(msg tea.Msg) (*Model, tea.Cmd) {
	animUpdated, cmd := m.welcome.animModel.Update(msg)
	m.welcome.animModel = animUpdated.(animations.Model)

	animDone := !m.welcome.animModel.IsPlaying()

	if animDone && m.welcome.dbReady {
		m.welcome.complete = true
		return m.handleWelcomeComplete()
	}

	if animDone && !m.welcome.dbReady {
		m.welcome.complete = true
	}

	return m, cmd
}

func (m *Model) handleDBReady(msg dbReadyMsg) (*Model, tea.Cmd) {
	m.welcome.dbReady = true
	m.welcome.dbErr = msg.err
	m.welcome.dbSettings = msg.settings

	if m.welcome.complete {
		return m.handleWelcomeComplete()
	}

	// If settings are available, start the loading sequence in parallel with the animation.
	// This way, when the animation ends, initialization may already be done or nearly done.
	if msg.err == nil && msg.settings != nil {
		m.appConfig = msg.settings
		if err := m.initializeServices(); err != nil {
			m.welcome.dbErr = err
			return m, nil
		}
		m.welcome.loadingStarted = true
		m.welcome.progressBar = progress.New(
			progress.WithColors(styles.ProgressBarStartColor, styles.ProgressBarEndColor),
			progress.WithoutPercentage(),
			progress.WithFillCharacters('━', '─'),
		)
		m.welcome.progressBar.SetWidth(computeProgressBarWidth(m.width))
		m.loading.started = true
		m.loading.current = "Connecting to database..."
		return m, connectDBCmd(m, false)
	}

	return m, nil
}

func (m *Model) handleWelcomeComplete() (*Model, tea.Cmd) {
	if m.welcome.dbErr != nil {
		m.loading.err = m.welcome.dbErr
		m.loading.current = ""
		m.screen = screenLoading
		return m, nil
	}

	// Fast path: parallel loading finished before animation ended.
	if m.welcome.loadingComplete {
		m.loading.complete = true
		return m, m.enterStartupLoadingScreen()
	}

	// Loading was started in parallel but is still in progress — hand off to the
	// loading screen which has all progress state already populated.
	if m.welcome.loadingStarted {
		m.loading.started = true
		return m, m.enterStartupLoadingScreen()
	}

	settings := m.welcome.dbSettings

	if settings != nil {
		m.appConfig = settings
		if err := m.initializeServices(); err != nil {
			m.loading.err = err
			m.loading.current = ""
			m.screen = screenLoading
			return m, nil
		}

		m.loading.err = nil
		m.loading.started = false
		m.loading.complete = false
		return m, m.enterStartupLoadingScreen()
	}

	m.screen = screenOnboarding
	m.onboarding.AddDatabaseForm = NewAddDatabaseForm(m.width, m.height)
	return m, nil
}

func (m *Model) checkDBConfig() (*domain.DatabaseSettings, error) {
	ctx := context.Background()
	settingsRepo := boltdb.NewDatabaseSettingsRepository(m.boltAdapter)
	settings, err := settingsRepo.GetDefault(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrDefaultSettingsNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return settings, nil
}

// progressBarWidth is the maximum character width of the progress bar.
const progressBarWidth = 80

// computeProgressBarWidth clamps the bar width to the terminal width with
// padding, capped at progressBarWidth.
func computeProgressBarWidth(termWidth int) int {
	if termWidth > 0 && termWidth-4 < progressBarWidth {
		return max(20, termWidth-4)
	}
	return progressBarWidth
}

func (m *Model) viewWelcome() string {
	content := m.welcome.animModel.View().Content
	animWidth := m.welcome.animModel.RenderWidth()

	if m.welcome.loadingStarted {
		labelContent := ""
		if !m.welcome.loadingComplete && m.loading.current != "" {
			labelContent = styles.LoadingCurrentStyle.Render(m.loading.current)
		}

		// Always render the label block at a fixed height so the bar position never shifts.
		labelBlock := lipgloss.NewStyle().
			Width(animWidth).
			Height(2).
			AlignHorizontal(lipgloss.Center).
			AlignVertical(lipgloss.Bottom).
			Render(labelContent)

		bar := lipgloss.NewStyle().
			Width(animWidth).
			AlignHorizontal(lipgloss.Center).
			Render(m.welcome.progressBar.View())

		parts := []string{content, labelBlock, bar}
		content = lipgloss.JoinVertical(lipgloss.Center, parts...)
	}

	if m.width > 0 && m.height > 0 {
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
	}
	return content
}

// initDBCmd runs database config check in background.
// Service initialization is deferred to the main update loop in handleDBReady.
func initDBCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		settings, err := m.checkDBConfig()
		if err != nil || settings == nil {
			return dbReadyMsg{settings: nil, err: err}
		}
		return dbReadyMsg{settings: settings, err: nil}
	}
}

// ==========================================
// Parallel loading during welcome animation
// ==========================================

// handleWelcomeLoadingMsg processes loading sequence messages that arrive while the
// welcome animation is still playing. On error, it switches to the loading screen
// immediately so the user sees feedback. On success it tracks progress and sets
// loadingComplete so handleWelcomeComplete can skip the loading screen entirely.
func (m *Model) handleWelcomeLoadingMsg(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {

	case progress.FrameMsg:
		pb, cmd := m.welcome.progressBar.Update(msg)
		m.welcome.progressBar = pb
		return m, cmd

	case dbConnectedMsg:
		if msg.err != nil {
			m.loading.err = fmt.Errorf("database connection failed: %w", msg.err)
			m.loading.current = ""
			// Switch to loading screen immediately so user sees the error.
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		m.loading.steps = append(m.loading.steps, "✓ Connected to Oracle database")

		if m.appConfig != nil && m.appConfig.PermissionsValidated() {
			m.loading.steps = append(m.loading.steps, "✓ Permissions verified (cached)")
			m.loading.current = "Deploying tracer package..."
			// Set to 50% — two of four conceptual steps done (connect + permissions).
			pbCmd := m.welcome.progressBar.SetPercent(0.50)
			return m, tea.Batch(pbCmd, deployTracerCmd(m))
		}
		m.loading.current = "Checking permissions..."
		pbCmd := m.welcome.progressBar.SetPercent(0.25)
		return m, tea.Batch(pbCmd, checkPermissionsCmd(m))

	case permissionsCheckedMsg:
		if msg.err != nil {
			m.loading.err = fmt.Errorf("permission check failed: %w", msg.err)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		if err := m.markActiveConnectionPermissionsValidated(); err != nil {
			m.loading.err = fmt.Errorf("failed to persist permission validation: %w", err)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		m.loading.steps = append(m.loading.steps, "✓ Permissions verified")
		m.loading.current = "Deploying tracer package..."
		pbCmd := m.welcome.progressBar.SetPercent(0.50)
		return m, tea.Batch(pbCmd, deployTracerCmd(m))

	case tracerDeployedMsg:
		if msg.err != nil {
			m.loading.err = fmt.Errorf("tracer deployment failed: %w", msg.err)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		m.loading.steps = append(m.loading.steps, "✓ Tracer package deployed")
		m.loading.current = "Registering subscriber..."
		pbCmd := m.welcome.progressBar.SetPercent(0.75)
		return m, tea.Batch(pbCmd, registerSubscriberCmd(m))

	case subscriberRegisteredMsg:
		if msg.err != nil {
			m.loading.err = fmt.Errorf("subscriber registration failed: %w", msg.err)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		if msg.subscriber == nil {
			m.loading.err = fmt.Errorf("subscriber registration returned nil subscriber: %w", domain.ErrNilSubscriber)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		if m.tracerService == nil {
			m.loading.err = fmt.Errorf("tracer service not initialized: %w", domain.ErrTracerNotInitialized)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		if err := m.tracerService.StartEventListener(m.eventStreamCtx, msg.subscriber, m.appConfig.Username()); err != nil {
			m.loading.err = fmt.Errorf("failed to start event listener: %w", err)
			m.welcome.complete = true
			m.screen = screenLoading
			return m, nil
		}
		m.subscriber = msg.subscriber
		m.loading.steps = append(m.loading.steps, "✓ Subscriber registered: "+msg.subscriber.Name())
		m.loading.current = ""
		pbCmd := m.welcome.progressBar.SetPercent(1.0)
		m.welcome.loadingComplete = true
		m.loading.complete = true

		// If animation already finished, hand off to the loading screen so the
		// update prompt remains a hard gate before main.
		if m.welcome.complete {
			return m, m.enterStartupLoadingScreen()
		}

		// Animation still playing — wait for it to end (handleWelcomeComplete handles transition).
		// Keep the progress bar animating to 100%.
		return m, pbCmd
	}

	return m, nil
}
