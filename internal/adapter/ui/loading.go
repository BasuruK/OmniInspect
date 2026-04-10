package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Loading Update
// ==========================================

// updateLoading handles messages when screen == "loading".
func (m *Model) updateLoading(msg tea.Msg) (*Model, tea.Cmd) {
	if m.dbSettings.visible {
		return m.updateDatabaseSettings(msg)
	}

	switch msg := msg.(type) {

	// Spinner animation frame
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.loading.spinner, cmd = m.loading.spinner.Update(msg)
		return m, cmd

	// Step 0: Update check result (before database connection)
	case updateCheckResultMsg:
		if msg.err != nil {
			// Update check failed — non-fatal, log warning and proceed
			log.Printf("[updater] Update check failed: %v\n", msg.err)
			m.update.checking = false
			m.loading.current = "Connecting to database..."
			return m, connectDBCmd(m, false)
		}
		m.update.checking = false
		if msg.info != nil {
			// Update available — prompt user
			m.update.info = msg.info
			m.update.prompting = true
			m.loading.current = ""
			return m, nil
		}
		// No update available — proceed to database connection
		m.loading.current = "Connecting to database..."
		return m, connectDBCmd(m, false)

	// Handle key input when prompting for update
	case tea.KeyPressMsg:
		if m.update.prompting {
			switch msg.String() {
			case "y", "Y", "enter":
				// User accepted update
				m.update.prompting = false
				m.update.applying = true
				m.update.stage = "Starting update..."
				return m, tea.Batch(applyUpdateCmd(m, m.update.info), waitForUpdateEventCmd(m.ctx, m.updateEventChannel))
			case "n", "N", "escape":
				// User declined update — proceed to database connection
				m.update.prompting = false
				m.loading.current = "Connecting to database..."
				return m, connectDBCmd(m, false)
			}
		}
		if m.update.err != nil {
			// Update error — allow user to continue or quit
			switch msg.String() {
			case "y", "Y", "enter":
				// Continue without update
				m.update.err = nil
				m.loading.current = "Connecting to database..."
				return m, connectDBCmd(m, false)
			case "n", "N", "q":
				// Quit
				m.cancel()
				return m, tea.Quit
			}
		}
		if m.loading.err != nil {
			switch msg.String() {
			case "r", "R":
				m.stopLoadingRetryTimer()

				waitSeconds := math.Min(math.Pow(2, float64(m.loading.retryCount)), 30)
				retryCtx, retryCancel := context.WithCancel(m.ctx)
				m.loading.retryCancel = retryCancel
				m.loading.current = fmt.Sprintf("Retrying connection in %.0f seconds...", waitSeconds)
				m.loading.retryTimer = time.NewTimer(time.Duration(waitSeconds) * time.Second)
				m.loading.retryCount++
				return m, waitForRetryTimerCmd(retryCtx, m.loading.retryTimer, m.loading.retryGeneration)

			case "s", "S":
				return m, func() tea.Msg {
					return SwitchDatabaseMsg{}
				}

			case "q", "Q":
				m.cancel()
				return m, tea.Quit
			}
		}

	case retryTimerExpiryMsg:
		if msg.generation != m.loading.retryGeneration || m.loading.retryTimer == nil {
			return m, nil
		}

		m.stopLoadingRetryTimer()
		m.loading.err = nil
		m.loading.current = "Connecting to database..."
		return m, connectDBCmd(m, false)

	case SwitchDatabaseMsg:
		m.stopLoadingRetryTimer()
		m.loading.current = ""

		activeID := ""
		if m.appConfig != nil {
			activeID = m.appConfig.ID()
		}

		databases, err := m.dbSettingsRepo.GetAll(m.ctx)
		if err != nil {
			log.Printf("[UI] Failed to load database settings: %v", err)
			databases = nil
		}

		m.initDatabaseSettings(databases, activeID)
		return m, nil

	// Update progress
	case updateProgressMsg:
		m.update.stage = msg.stage
		return m, waitForUpdateEventCmd(m.ctx, m.updateEventChannel)

	// Update complete — defensive fallback if updater doesn't exit
	case updateCompleteMsg:
		m.update.applying = false
		m.loading.current = "Update complete. Restarting..."
		return m, tea.Quit

	// Update error
	case updateErrorMsg:
		m.update.applying = false
		m.update.err = msg.err
		m.loading.current = ""
		return m, nil

	// Step 1: Oracle DB connection result
	case dbConnectedMsg:
		if msg.err != nil {
			m.loading.err = fmt.Errorf("database connection failed: %w", msg.err)
			m.loading.current = ""
			return m, nil
		}
		m.loading.retryCount = 0
		m.loading.retryTimer = nil
		m.loading.steps = append(m.loading.steps, "✓ Connected to Oracle database")

		// Initialize services before proceeding
		if err := m.initializeServices(); err != nil {
			m.loading.err = fmt.Errorf("service initialization failed: %w", err)
			return m, nil
		}

		if m.appConfig != nil && m.appConfig.PermissionsValidated() {
			m.loading.steps = append(m.loading.steps, "✓ Permissions verified (cached)")
			m.loading.current = "Deploying tracer package..."
			return m, deployTracerCmd(m)
		}

		m.loading.current = "Checking permissions..."
		return m, checkPermissionsCmd(m)

	// Step 2: Permission deploy/check result
	case permissionsCheckedMsg:
		if msg.err != nil {
			m.loading.err = fmt.Errorf("permission check failed: %w", msg.err)
			return m, nil
		}
		if err := m.markActiveConnectionPermissionsValidated(); err != nil {
			m.loading.err = fmt.Errorf("failed to persist permission validation: %w", err)
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
		if msg.subscriber == nil {
			m.loading.err = fmt.Errorf("subscriber registration returned nil subscriber")
			return m, nil
		}

		// Defensive nil check for tracerService
		if m.tracerService == nil {
			m.loading.err = fmt.Errorf("tracer service not initialized")
			return m, nil
		}

		// Start event listener before transitioning to main screen
		if err := m.tracerService.StartEventListener(m.eventStreamCtx, msg.subscriber, m.appConfig.Username()); err != nil {
			m.loading.err = fmt.Errorf("failed to start event listener: %w", err)
			return m, nil
		}

		m.subscriber = msg.subscriber
		m.loading.steps = append(m.loading.steps,
			"✓ Subscriber registered: "+msg.subscriber.Name())
		m.loading.current = ""

		// All loading complete — transition to main screen
		m.screen = screenMain
		m.initViewport()

		return m, waitForEventCmd(m.eventStreamCtx, m.eventChannel)
	}

	return m, nil
}

func (m *Model) stopLoadingRetryTimer() {
	m.loading.retryGeneration++

	if m.loading.retryCancel != nil {
		m.loading.retryCancel()
		m.loading.retryCancel = nil
	}

	if m.loading.retryTimer == nil {
		return
	}

	if !m.loading.retryTimer.Stop() {
		select {
		case <-m.loading.retryTimer.C:
		default:
		}
	}

	m.loading.retryTimer = nil
}

// ==========================================
// Loading View
// ==========================================

// viewLoading renders the loading screen with spinner and step progress.
func (m *Model) viewLoading() string {
	panelWidth := min(max(m.width-8, 20), 72)
	if m.width > 0 {
		panelWidth = min(panelWidth, m.width)
	}
	horizontalFrame, _ := styles.PrimaryPanelStyle.GetFrameSize()
	bodyWidth := max(panelWidth-horizontalFrame, 1)
	lines := []string{
		styles.LoadingTitleStyle.Render("Initializing OmniView"),
		styles.SubtitleStyle.Render("Preparing the Oracle trace session and live event pipeline."),
		"",
	}

	// Handle update states first (before regular loading steps)
	if m.update.prompting && m.update.info != nil {
		// Show update prompt
		updateInfo := fmt.Sprintf("Update %s available (current: %s)",
			m.update.info.NewVersion, m.update.info.CurrentVersion)
		lines = append(lines,
			styles.LoadingStepStyle.Render("✓ Update available"),
			"",
			styles.SubtitleStyle.Width(bodyWidth).Render(updateInfo),
			"",
			styles.LoadingCurrentStyle.Render("[Y] Update  [N] Skip"),
		)
		panel := renderPanel("Startup Status", panelWidth, lipgloss.JoinVertical(lipgloss.Left, lines...))
		return placeCentered(m.width, m.height, panel)
	}

	if m.update.applying {
		// Show update progress
		lines = append(lines,
			styles.LoadingStepStyle.Render("✓ Update available"),
			"",
			styles.LoadingCurrentStyle.Render(m.loading.spinner.View()+" "+m.update.stage),
		)
		panel := renderPanel("Startup Status", panelWidth, lipgloss.JoinVertical(lipgloss.Left, lines...))
		return placeCentered(m.width, m.height, panel)
	}

	if m.update.err != nil {
		// Show update error with option to continue
		lines = append(lines,
			styles.LoadingErrorStyle.Render("Update failed"),
			styles.SubtitleStyle.Width(bodyWidth).Render(m.update.err.Error()),
			"",
			styles.SubtitleStyle.Render("[Y] Continue without update  [N/Q] Quit"),
		)
		panel := renderPanel("Startup Status", panelWidth, lipgloss.JoinVertical(lipgloss.Left, lines...))
		return placeCentered(m.width, m.height, panel)
	}

	// Regular loading steps
	for _, step := range m.loading.steps {
		lines = append(lines, styles.LoadingStepStyle.Render(step))
	}

	if m.loading.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		infoStyle := styles.SubtitleStyle

		errorMsg := fmt.Sprintf("%s %s", errorStyle.Render("✗"), m.loading.err.Error())
		retryStatus := ""
		if m.loading.retryTimer != nil && m.loading.current != "" {
			retryStatus = infoStyle.Render(m.loading.current)
		}

		lines = append(
			lines,
			"",
			styles.LoadingErrorStyle.Render("Startup blocked"),
			styles.SubtitleStyle.Width(bodyWidth).Render(errorMsg),
			"",
			retryStatus,
			styles.SubtitleStyle.Width(bodyWidth).Render(infoStyle.Render("R Retry  •  S Switch  •  Q Quit")),
		)
	} else if m.loading.current != "" {
		lines = append(
			lines,
			"",
			styles.LoadingCurrentStyle.Render(m.loading.spinner.View()+" "+m.loading.current),
		)
	}

	panel := renderPanel("Startup Status", panelWidth, lipgloss.JoinVertical(lipgloss.Left, lines...))
	content := placeCentered(m.width, m.height, panel)
	if m.dbSettings.visible {
		content = renderCenteredOverlay(content, m.viewDatabaseSettings(), m.width, m.height)
		if m.dbSettings.showAddForm {
			content = renderCenteredOverlay(content, m.dbSettings.addForm.Modal(), m.width, m.height)
		}
	}

	return content
}
