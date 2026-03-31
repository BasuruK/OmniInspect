package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Form Field
// ==========================================

type FormField struct {
	Label       string
	Placeholder string
	Value       string
	IsPassword  bool
}

// ==========================================
// Add Database Form Component
// ==========================================

// addFormField indices
const (
	formFieldDatabaseID = 0
	formFieldHost       = 1
	formFieldPort       = 2
	formFieldService    = 3
	formFieldUser       = 4
	formFieldPass       = 5
	formFieldCount      = 6
)

// Focusable positions beyond form fields: Save = formFieldCount, Cancel = formFieldCount+1
const (
	formBtnSave   = formFieldCount
	formBtnCancel = formFieldCount + 1
	formMaxCursor = formBtnCancel
)

type AddDatabaseForm struct {
	fields    []FormField
	cursor    int
	width     int
	height    int
	submitted bool
	cancelled bool
	errMsg    string
}

// NewAddDatabaseForm: creates a new AddDatabaseForm with the specified terminal dimensions and empty fields.
func NewAddDatabaseForm(width, height int) AddDatabaseForm {
	return AddDatabaseForm{
		fields: []FormField{
			{Label: "Database ID", Placeholder: "e.g., PROD-EU"},
			{Label: "Database Host", Placeholder: "e.g., localhost"},
			{Label: "Database Port", Placeholder: "e.g., 1521"},
			{Label: "Service Name / SID", Placeholder: "e.g., FREEPDB1"},
			{Label: "Username", Placeholder: "e.g., SYSTEM"},
			{Label: "Password", Placeholder: "password", IsPassword: true},
		},
		width:  width,
		height: height,
	}
}

func (f AddDatabaseForm) IsSubmitted() bool { return f.submitted }
func (f AddDatabaseForm) IsCancelled() bool { return f.cancelled }

// FieldValues returns (databaseID, host, port, service, username, password).
func (f AddDatabaseForm) FieldValues() (string, string, string, string, string, string) {
	return f.fields[formFieldDatabaseID].Value,
		f.fields[formFieldHost].Value,
		f.fields[formFieldPort].Value,
		f.fields[formFieldService].Value,
		f.fields[formFieldUser].Value,
		f.fields[formFieldPass].Value
}

// ─────────────────────────
// Validation
// ─────────────────────────

// sanitizePasteInput: filters pasted content to only include printable ASCII characters (0x20-0x7F),
// removing control characters, newlines, and other non-printable characters to match keyboard input validation.
func sanitizePasteInput(content string) string {
	var result strings.Builder
	for _, r := range content {
		if r >= 0x20 && r < 0x7F {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// validate: validates all form fields and returns an error message if any field is invalid, or empty string if valid.
func (f *AddDatabaseForm) validate() string {
	for i, field := range f.fields {
		v := strings.TrimSpace(field.Value)
		switch i {
		case formFieldDatabaseID:
			if v == "" {
				return "Database ID cannot be empty"
			}
		case formFieldHost:
			if v == "" {
				return "Database host cannot be empty"
			}
		case formFieldPort:
			if v == "" {
				return "Database port cannot be empty"
			}
			port, err := strconv.Atoi(v)
			if err != nil || port < 1 || port > 65535 {
				return "Port must be a number between 1 and 65535"
			}
		case formFieldService:
			if v == "" {
				return "Service name cannot be empty"
			}
		case formFieldUser:
			if v == "" {
				return "Username cannot be empty"
			}
		case formFieldPass:
			if v == "" {
				return "Password cannot be empty"
			}
		}
	}
	return ""
}

// ─────────────────────────
// Update
// ─────────────────────────

// Update: handles keyboard input for the add database form including navigation, character input, and form submission.
func (f AddDatabaseForm) Update(msg tea.Msg) (AddDatabaseForm, tea.Cmd) {
	if f.submitted || f.cancelled {
		return f, nil
	}

	switch msg := msg.(type) {
	case tea.PasteMsg:
		if f.cursor < formFieldCount {
			sanitized := sanitizePasteInput(msg.Content)
			f.fields[f.cursor].Value += sanitized
			f.errMsg = ""
		}
		return f, nil

	case tea.KeyPressMsg:
		key := msg.String()

		switch key {
		case "esc":
			f.cancelled = true
			return f, nil

		case "up", "shift+tab":
			if f.cursor > 0 {
				f.cursor--
			}
			f.errMsg = ""
			return f, nil

		case "down":
			if f.cursor < formMaxCursor {
				f.cursor++
			}
			f.errMsg = ""
			return f, nil

		case "tab":
			f.cursor = (f.cursor + 1) % (formMaxCursor + 1)
			f.errMsg = ""
			return f, nil

		case "enter":
			if f.cursor == formBtnCancel {
				f.cancelled = true
				return f, nil
			}
			if f.cursor == formBtnSave {
				if errMsg := f.validate(); errMsg != "" {
					f.errMsg = errMsg
					return f, nil
				}
				f.submitted = true
				return f, nil
			}
			// On a field — advance to next field
			if f.cursor < formFieldCount-1 {
				f.cursor++
			} else {
				f.cursor = formBtnSave
			}
			f.errMsg = ""
			return f, nil

		case "backspace":
			if f.cursor < formFieldCount {
				v := f.fields[f.cursor].Value
				if len(v) > 0 {
					f.fields[f.cursor].Value = v[:len(v)-1]
				}
				f.errMsg = ""
			}
			return f, nil

		case "ctrl+u":
			if f.cursor < formFieldCount {
				f.fields[f.cursor].Value = ""
				f.errMsg = ""
			}
			return f, nil
		}

		// Character input — only when a field is focused
		if f.cursor < formFieldCount && len(msg.Text) > 0 && !msg.Mod.Contains(tea.ModCtrl) {
			f.fields[f.cursor].Value += msg.Text
			f.errMsg = ""
		}
	}

	return f, nil
}

// ─────────────────────────
// Styles (scoped to component)
// ─────────────────────────

var (
	formTitleStyle     = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Bold(true)
	formLabelStyle     = lipgloss.NewStyle().Foreground(styles.MutedColor)
	formLabelFocused   = lipgloss.NewStyle().Foreground(styles.TextColor).Bold(true)
	formValueStyle     = lipgloss.NewStyle().Foreground(styles.TextColor)
	formPlaceholder    = lipgloss.NewStyle().Foreground(styles.MutedColor)
	formFieldFocused   = lipgloss.NewStyle().Foreground(styles.TextColor).Background(styles.SurfaceColor)
	formFieldUnfocused = lipgloss.NewStyle().Foreground(styles.MutedColor)
	formCursorStyle    = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Bold(true)
	formSepStyle       = lipgloss.NewStyle().Foreground(styles.SurfaceColor)
	formBtnStyle       = lipgloss.NewStyle().Foreground(styles.MutedColor)
	formBtnActiveStyle = lipgloss.NewStyle().Foreground(styles.PrimaryColor).Bold(true).Underline(true)
	formHintStyle      = lipgloss.NewStyle().Foreground(styles.MutedColor)
	formErrorStyle     = lipgloss.NewStyle().Foreground(styles.FailureColor)
	formBoxStyle       = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(styles.SurfaceColor).
				Padding(1, 2)
)

// ─────────────────────────
// Render
// ─────────────────────────

// Render: renders the form centered within the terminal using the Modal method.
func (f AddDatabaseForm) Render() string {
	return lipgloss.Place(f.width, f.height, lipgloss.Center, lipgloss.Center, f.Modal())
}

// Modal: renders the add database form as a modal dialog with all fields, validation errors, and action buttons.
func (f AddDatabaseForm) Modal() string {
	modalWidth := max(min(f.width-12, 72), 52)
	fieldWidth := max(modalWidth-4, 24)
	lines := []string{
		formTitleStyle.Render("Add Database Connection"),
		styles.OnboardingBannerStyle.Render("Important: fields marked with (*) are required."),
		"",
	}

	for i, field := range f.fields {
		isFocused := i == f.cursor

		displayValue := field.Value
		if displayValue == "" {
			displayValue = formPlaceholder.Render(field.Placeholder)
		} else if field.IsPassword {
			displayValue = formValueStyle.Render(strings.Repeat("•", min(len(field.Value), 30)))
		} else {
			displayValue = formValueStyle.Render(displayValue)
		}

		if isFocused {
			displayValue += formCursorStyle.Render("_")
		}

		lines = append(
			lines,
			renderEmbeddedField(embeddedFieldOptions{
				Label:    field.Label,
				Value:    displayValue,
				Width:    fieldWidth,
				Focused:  isFocused,
				Required: true,
			}),
			"",
		)
	}

	if f.errMsg != "" {
		lines = append(lines, formErrorStyle.Render(f.errMsg))
		lines = append(lines, "")
	}

	lines = append(
		lines,
		renderCenteredActionButtons(
			fieldWidth,
			"Save",
			f.cursor == formBtnSave,
			"Cancel",
			f.cursor == formBtnCancel,
		),
		"",
		formHintStyle.Render("↑/↓ Navigate  •  Enter Confirm  •  Esc Cancel"),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return renderFramedPanel("Stored Connections", modalWidth, content)
}