package ui

import "OmniView/internal/adapter/ui/styles"

// ==========================================
// Settings Panel Dialog
// ==========================================

// settingsDialog holds the shared inline error/info dialog state used by
// settings panels. Embed this in a panel-specific state struct to avoid
// repeating the same three fields in every panel.
type settingsDialog struct {
	msg     string
	isError bool
	visible bool
}

// clear resets the dialog to its zero (hidden) state.
func (d *settingsDialog) clear() {
	d.msg = ""
	d.isError = false
	d.visible = false
}

// set shows a message in the dialog.
func (d *settingsDialog) set(msg string, isError bool) {
	d.msg = msg
	d.isError = isError
	d.visible = true
}

// renderSettingsDialogLines returns the rendered lines for the inline dialog
// (blank spacer, styled message, dismiss hint) ready to be appended to a
// panel's parts slice. Returns nil when the dialog is not visible.
func renderSettingsDialogLines(d settingsDialog, innerWidth int) []string {
	if !d.visible || d.msg == "" {
		return nil
	}
	var line string
	if d.isError {
		line = styles.OnboardingErrorStyle.Render("Error: " + d.msg)
	} else {
		line = styles.OnboardingSavedStyle.Render(d.msg)
	}
	return []string{
		"",
		line,
		styles.SubtitleStyle.Width(innerWidth).Render("Press Esc to dismiss the message and stay on this screen."),
	}
}
