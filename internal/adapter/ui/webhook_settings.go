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
	layout     webhookSettingsLayout
}

type webhookSettingsLayout struct {
	panelWidth        int
	innerWidth        int
	compact           bool
	showSubtitle      bool
	showCurrentStatus bool
	showHint          bool
}

// ==========================================
// Helpers
// ==========================================

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

	m.resizeWebhookSettings(m.width, m.height)
}

func (m *Model) resizeWebhookSettings(width, height int) {
	if !m.webhookSettings.visible {
		return
	}

	_, contentHeight := screenContentSize(width, height)
	panelWidth := settingsPanelWidth(width)
	innerWidth := max(panelWidth-4, 1)

	m.webhookSettings.layout = webhookSettingsLayout{
		panelWidth:        panelWidth,
		innerWidth:        innerWidth,
		compact:           contentHeight <= 16,
		showSubtitle:      contentHeight >= 10,
		showCurrentStatus: contentHeight >= 14,
		showHint:          contentHeight >= 12,
	}
}

func (m *Model) closeWebhookSettings() {
	m.webhookSettings = webhookSettingsState{}
}

func (m *Model) clearWebhookSettingsDialog() {
	m.webhookSettings.showDialog = false
	m.webhookSettings.dialogMsg = ""
}

// ==========================================
// Update
// ==========================================

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

func (m *Model) viewWebhookSettings() string {
	layout := m.webhookSettings.layout
	if layout.panelWidth == 0 {
		m.resizeWebhookSettings(m.width, m.height)
		layout = m.webhookSettings.layout
	}

	panelWidth := layout.panelWidth
	innerWidth := layout.innerWidth

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

	parts := []string{styles.OnboardingTitleStyle.Render("Settings")}
	appendSpacer := func() {
		if !layout.compact {
			parts = append(parts, "")
		}
	}

	if layout.showSubtitle {
		parts = append(parts, styles.SubtitleStyle.Width(innerWidth).Render("Configure the optional webhook endpoint used for trace delivery."))
	}
	if layout.showCurrentStatus {
		appendSpacer()
		parts = append(parts, renderEmbeddedField(embeddedFieldOptions{
			Label:       "Current Webhook",
			Value:       currentWebhook,
			Width:       innerWidth,
			BorderColor: "#F0C802",
		}))
	}

	appendSpacer()
	parts = append(parts, renderEmbeddedField(embeddedFieldOptions{
		Label:      "Webhook URL",
		Value:      editValue,
		Width:      innerWidth,
		Focused:    m.webhookSettings.cursor == webhookFieldURL,
		FooterText: "Leave empty to disable webhook delivery.",
	}))

	appendSpacer()
	parts = append(parts, renderCenteredActionButtons(
		innerWidth,
		"Save",
		m.webhookSettings.cursor == webhookBtnSave,
		"Cancel",
		m.webhookSettings.cursor == webhookBtnCancel,
	))

	if m.webhookSettings.showDialog && m.webhookSettings.dialogMsg != "" {
		parts = append(
			parts,
			styles.OnboardingErrorStyle.Render("Error: "+m.webhookSettings.dialogMsg),
		)
		if layout.showHint {
			parts = append(parts, styles.SubtitleStyle.Width(innerWidth).Render("Press Esc to dismiss the message and stay on this screen."))
		}
	}

	if layout.showHint {
		appendSpacer()
		parts = append(parts, styles.OnboardingHintStyle.Width(innerWidth).Render("↑/↓ Navigate  •  Enter Confirm  •  Ctrl+U Clear  •  Esc Back"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return renderFramedPanel("Settings", panelWidth, panelTypeInfo, content)
}

// ==========================================
// Async Commands
// ==========================================

func (m *Model) saveWebhookSettingsCmd() tea.Cmd {
	input := strings.TrimSpace(m.webhookSettings.input)
	boltAdapter := m.boltAdapter

	return func() tea.Msg {
		if input == "" {
			if err := boltAdapter.DeleteWebhookConfig(domain.DefaultWebhookID); err != nil {
				return webhookConfigSavedMsg{deleted: true, err: fmt.Errorf("clear webhook configuration: %w", err)}
			}
			return webhookConfigSavedMsg{deleted: true}
		}

		config, err := domain.NewWebhookConfig(domain.DefaultWebhookID, input, true)
		if err != nil {
			return webhookConfigSavedMsg{err: fmt.Errorf("invalid webhook URL: %w", err)}
		}

		if err := boltAdapter.SaveWebhookConfig(config); err != nil {
			return webhookConfigSavedMsg{err: fmt.Errorf("save webhook configuration: %w", err)}
		}

		return webhookConfigSavedMsg{config: config}
	}
}
