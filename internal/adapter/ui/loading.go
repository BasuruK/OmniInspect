package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Loading Update
// ==========================================

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
		if msg.subscriber == nil {
			m.loading.err = fmt.Errorf("subscriber registration returned nil subscriber")
			return m, nil
		}

		// Start event listener before transitioning to main screen
		if err := m.tracerService.StartEventListener(m.ctx, msg.subscriber, m.appConfig.Username()); err != nil {
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

		return m, waitForEventCmd(m.ctx, m.eventChannel)
	}

	return m, nil
}

// ==========================================
// Loading View
// ==========================================

// viewLoading renders the loading screen with spinner and step progress.
func (m *Model) viewLoading() string {
	panelWidth := max(min(m.width-8, 72), 44)
	lines := []string{
		styles.LoadingTitleStyle.Render("Initializing OmniView"),
		styles.SubtitleStyle.Render("Preparing the Oracle trace session and live event pipeline."),
		"",
	}

	for _, step := range m.loading.steps {
		lines = append(lines, styles.LoadingStepStyle.Render(step))
	}

	if m.loading.err != nil {
		lines = append(
			lines,
			"",
			styles.LoadingErrorStyle.Render("Startup blocked"),
			styles.SubtitleStyle.Width(panelWidth-4).Render(m.loading.err.Error()),
			"",
			styles.SubtitleStyle.Render("Press q to exit."),
		)
	} else if m.loading.current != "" {
		lines = append(
			lines,
			"",
			styles.LoadingCurrentStyle.Render(m.loading.spinner.View()+" "+m.loading.current),
		)
	}

	panel := renderPanel("Startup Status", panelWidth, lipgloss.JoinVertical(lipgloss.Left, lines...))
	return placeCentered(m.width, m.height, panel)
}
