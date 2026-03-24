package styles

import (
	"charm.land/lipgloss/v2"
)

// ==========================================
// Color palette based on OmniView branding
// ==========================================

var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("127") // Purple/magenta
	SecondaryColor = lipgloss.Color("99")  // Light purple
	AccentColor    = lipgloss.Color("213") // Pink/magenta

	// Background colors
	BackgroundColor = lipgloss.Color("0")   // Black
	SurfaceColor    = lipgloss.Color("235") // Dark gray

	// Text colors
	TextColor  = lipgloss.Color("255") // White
	MutedColor = lipgloss.Color("244") // Gray

	// Log level colors
	DebugColor    = lipgloss.Color("244") // Gray
	InfoColor     = lipgloss.Color("86")  // Teal (matches primary)
	WarningColor  = lipgloss.Color("214") // Orange
	ErrorColor    = lipgloss.Color("196") // Red
	CriticalColor = lipgloss.Color("199") // Hot pink

	// Status colors
	SuccessColor = lipgloss.Color("82")  // Green
	FailureColor = lipgloss.Color("196") // Red
)

// ==========================================
// Brand Styles
// ==========================================

var (
	LogoStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	LogoSubtleStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	VersionStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	TitleStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Bold(true).
			Underline(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	LoadingStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor)

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(SurfaceColor)
)

// ==========================================
// Layout styles
// ==========================================

var (
	CenteredStyle = lipgloss.NewStyle().
			Width(80).
			Align(lipgloss.Center)

	ContainerStyle = lipgloss.NewStyle().
			Width(60).
			Align(lipgloss.Center).
			Padding(1, 2)
)

// ==========================================
// Loading styles
// ==========================================

var (
	LoadingTitleStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	LoadingStepStyle = lipgloss.NewStyle().
				Foreground(SuccessColor)

	LoadingCurrentStyle = lipgloss.NewStyle().
				Foreground(SecondaryColor)

	LoadingErrorStyle = lipgloss.NewStyle().
				Foreground(FailureColor).
				Bold(true)
)

// ==========================================
// Main Screen styles
// ==========================================

var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(0, 1)

	ViewportStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(SurfaceColor).
			Padding(0, 1)
)

// ==========================================
// Onboarding styles
// ==========================================

var (
	OnboardingBorderStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	OnboardingPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(SurfaceColor).
				Padding(1, 2)

	OnboardingTitleStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	OnboardingFieldLabelStyle = lipgloss.NewStyle().
					Foreground(TextColor).
					Bold(true)

	OnboardingActiveLabelStyle = lipgloss.NewStyle().
					Foreground(PrimaryColor).
					Bold(true)

	OnboardingFieldValueStyle = lipgloss.NewStyle().
					Foreground(MutedColor)

	OnboardingActiveValueStyle = lipgloss.NewStyle().
					Foreground(TextColor)

	OnboardingActiveIndicatorStyle = lipgloss.NewStyle().
					Foreground(PrimaryColor).
					Bold(true)

	OnboardingSeparatorStyle = lipgloss.NewStyle().
					Foreground(MutedColor)

	OnboardingFieldActiveStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(PrimaryColor).
					Padding(0, 1)

	OnboardingErrorStyle = lipgloss.NewStyle().
				Foreground(FailureColor)

	OnboardingHintStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	OnboardingSavedStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true)

	// Database Manager pane styles
	DBPaneBorderStyle         = lipgloss.NewStyle().Foreground(MutedColor)
	DBListActiveStyle         = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	DBListSelectedStyle       = lipgloss.NewStyle().Foreground(PrimaryColor)
	DBListNormalStyle         = lipgloss.NewStyle().Foreground(TextColor)
	DBFormFieldActiveStyle    = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor).
		Padding(0, 1)
	DBDialogStyle             = lipgloss.NewStyle().Foreground(FailureColor).Bold(true)
	DBPaneTitleStyle          = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	DBPaneHintStyle           = lipgloss.NewStyle().Foreground(MutedColor)
	DBPaneActiveIndicatorStyle = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
	DBPaneButtonStyle         = lipgloss.NewStyle().Foreground(TextColor)
	DBPaneButtonActiveStyle   = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true)
)
