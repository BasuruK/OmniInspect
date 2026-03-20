package styles

import (
	"charm.land/lipgloss/v2"
)

// ==========================================
// Color palette based on OmniView branding
// ==========================================

var (
	// Primary colors
	PrimaryColor   = lipgloss.Color("86")  // Teal green
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
