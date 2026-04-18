package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"OmniView/internal/core/domain"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Webhook Settings Sub-State
// ==========================================

const (
	webhookFieldURL = iota
	webhookBtnSave
	webhookBtnCancel
	webhookMaxCursor = webhookBtnCancel
)

type webhookSettingsState struct {
	config     *domain.WebhookConfig
	visible    bool
	cursor     int
	input      string
	dialogMsg  string
	showDialog bool
}

// ==========================================
// Helpers
// ==========================================

// initWebhookSettings initializes the webhook settings sub-state with the given config.
func (m *Model) initWebhookSettings(config *domain.WebhookConfig) {
	input := ""
	if config != nil {
		input = config.URL
	}

	m.webhookSettings = webhookSettingsState{
		config:  config,
		visible: true,
		input:   input,
	}
}

// resizeWebhookSettings resizes the webhook settings panel to the given dimensions.
func (m *Model) resizeWebhookSettings(width, height int) {
	_ = width
	_ = height
}

// closeWebhookSettings closes the webhook settings overlay and resets the sub-state.
func (m *Model) closeWebhookSettings() {
	m.webhookSettings = webhookSettingsState{}
}

// clearWebhookSettingsDialog dismisses the error/info dialog in the webhook settings panel.
func (m *Model) clearWebhookSettingsDialog() {
	m.webhookSettings.showDialog = false
	m.webhookSettings.dialogMsg = ""
}

// ==========================================
// Update
// ==========================================

// updateWebhookSettings handles keyboard and paste input for the webhook settings panel.
func (m *Model) updateWebhookSettings(msg tea.Msg) (*Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if m.webhookSettings.cursor == webhookFieldURL {
			m.webhookSettings.input += sanitizePasteInput(msg.Content)
			m.clearWebhookSettingsDialog()
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "q", "esc":
			// Allow 'q' to be typed in the URL field
			if msg.String() == "q" && m.webhookSettings.cursor == webhookFieldURL {
				m.webhookSettings.input += "q"
				m.clearWebhookSettingsDialog()
				return m, nil
			}
			if m.webhookSettings.showDialog {
				m.clearWebhookSettingsDialog()
				return m, nil
			}
			m.closeWebhookSettings()
			return m, nil
		case "up", "shift+tab":
			if m.webhookSettings.cursor > 0 {
				m.webhookSettings.cursor--
			}
			m.clearWebhookSettingsDialog()
			return m, nil
		case "down":
			if m.webhookSettings.cursor < webhookMaxCursor {
				m.webhookSettings.cursor++
			}
			m.clearWebhookSettingsDialog()
			return m, nil
		case "tab":
			m.webhookSettings.cursor = (m.webhookSettings.cursor + 1) % (webhookMaxCursor + 1)
			m.clearWebhookSettingsDialog()
			return m, nil
		case "enter":
			switch m.webhookSettings.cursor {
			case webhookBtnCancel:
				m.closeWebhookSettings()
				return m, nil
			case webhookBtnSave:
				return m, m.saveWebhookSettingsCmd()
			default:
				m.webhookSettings.cursor = webhookBtnSave
				m.clearWebhookSettingsDialog()
				return m, nil
			}
		case "backspace":
			if m.webhookSettings.cursor == webhookFieldURL {
				value := m.webhookSettings.input
				if len(value) > 0 {
					_, size := utf8.DecodeLastRuneInString(value)
					if size > 0 {
						m.webhookSettings.input = value[:len(value)-size]
					}
				}
				m.clearWebhookSettingsDialog()
			}
			return m, nil
		case "ctrl+u":
			if m.webhookSettings.cursor == webhookFieldURL {
				m.webhookSettings.input = ""
				m.clearWebhookSettingsDialog()
			}
			return m, nil
		}

		if m.webhookSettings.cursor == webhookFieldURL && len(msg.Text) > 0 && !msg.Mod.Contains(tea.ModCtrl) {
			m.webhookSettings.input += msg.Text
			m.clearWebhookSettingsDialog()
		}
		return m, nil

	case webhookConfigSavedMsg:
		if msg.err != nil {
			m.webhookSettings.dialogMsg = msg.err.Error()
			m.webhookSettings.showDialog = true
			return m, nil
		}

		if msg.deleted {
			m.webhookSettings.config = nil
			m.closeWebhookSettings()
			return m, nil
		}

		m.webhookSettings.config = msg.config
		m.closeWebhookSettings()
		return m, nil

	case tea.WindowSizeMsg:
		m.resizeWebhookSettings(msg.Width, msg.Height)
		return m, nil
	}

	return m, nil
}

// ==========================================
// View
// ==========================================

// viewWebhookSettings renders the webhook settings panel as a string.
func (m *Model) viewWebhookSettings() string {
	panelWidth := settingsPanelWidth(m.width)
	innerWidth := max(panelWidth-4, 24)

	currentWebhook := styles.EmptyStateStyle.Render("No webhook configured.")
	if m.webhookSettings.config != nil && strings.TrimSpace(m.webhookSettings.config.URL) != "" {
		currentWebhook = styles.BodyTextStyle.Render(m.webhookSettings.config.URL)
	}

	inputValue := strings.TrimSpace(m.webhookSettings.input)
	editValue := formPlaceholder.Render("https://example.com/webhook")
	if inputValue != "" {
		editValue = formValueStyle.Render(m.webhookSettings.input)
	}
	if m.webhookSettings.cursor == webhookFieldURL {
		if inputValue == "" {
			editValue = formPlaceholder.Render("https://example.com/webhook") + formCursorStyle.Render("_")
		} else {
			editValue = formValueStyle.Render(m.webhookSettings.input) + formCursorStyle.Render("_")
		}
	}

	parts := []string{
		styles.SubtitleStyle.Render("Configure the optional webhook endpoint used for trace delivery."),
		"",
		renderEmbeddedField(embeddedFieldOptions{
			Label:       "Current Webhook",
			Value:       currentWebhook,
			Width:       innerWidth,
			BorderColor: "#F0C802",
		}),
		"",
		renderEmbeddedField(embeddedFieldOptions{
			Label:      "Webhook URL",
			Value:      editValue,
			Width:      innerWidth,
			Focused:    m.webhookSettings.cursor == webhookFieldURL,
			FooterText: "Leave empty to disable webhook delivery.",
		}),
		"",
		renderCenteredActionButtons(
			innerWidth,
			"Save",
			m.webhookSettings.cursor == webhookBtnSave,
			"Cancel",
			m.webhookSettings.cursor == webhookBtnCancel,
		),
	}

	if m.webhookSettings.showDialog && m.webhookSettings.dialogMsg != "" {
		parts = append(
			parts,
			"",
			styles.OnboardingErrorStyle.Render("Error: "+m.webhookSettings.dialogMsg),
			styles.SubtitleStyle.Width(innerWidth).Render("Press Esc to dismiss the message and stay on this screen."),
		)
	}

	if !m.webhookSettings.showDialog {
		parts = append(
			parts,
			"",
			styles.OnboardingHintStyle.Width(innerWidth).Render("↑/↓ Navigate  •  Enter Confirm  •  Ctrl+U Clear  •  Esc Back"),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return renderFramedPanel("Settings", panelWidth, panelTypeInfo, content)
}

// ==========================================
// Async Commands
// ==========================================

// saveWebhookSettingsCmd returns an async command that saves or deletes the webhook configuration.
// It derives the target ID from the existing config when present, falling back to the default ID.
func (m *Model) saveWebhookSettingsCmd() tea.Cmd {
	input := strings.TrimSpace(m.webhookSettings.input)
	boltAdapter := m.boltAdapter

	// Derive target ID from existing config, falling back to default
	targetID := domain.DefaultWebhookID
	if m.webhookSettings.config != nil && m.webhookSettings.config.ID != "" {
		targetID = m.webhookSettings.config.ID
	}

	return func() tea.Msg {
		if input == "" {
			if err := boltAdapter.DeleteWebhookConfig(targetID); err != nil {
				return webhookConfigSavedMsg{deleted: true, err: fmt.Errorf("clear webhook configuration: %w", err)}
			}
			return webhookConfigSavedMsg{deleted: true}
		}

		config, err := domain.NewWebhookConfig(targetID, input, true)
		if err != nil {
			return webhookConfigSavedMsg{err: fmt.Errorf("invalid webhook URL: %w", err)}
		}

		if err := boltAdapter.SaveWebhookConfig(config); err != nil {
			return webhookConfigSavedMsg{err: fmt.Errorf("save webhook configuration: %w", err)}
		}

		return webhookConfigSavedMsg{config: config}
	}
}
