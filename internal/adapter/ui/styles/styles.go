package styles

import (
	"charm.land/lipgloss/v2"
)

// Color palette based on OmniView branding
var (
	// Primary colors
	PrimaryColor    = lipgloss.Color("86")   // Teal green
	SecondaryColor  = lipgloss.Color("99")   // Light purple
	AccentColor     = lipgloss.Color("213")  // Pink/magenta

	// Background colors
	BackgroundColor = lipgloss.Color("0")   // Black
	SurfaceColor     = lipgloss.Color("235") // Dark gray

	// Text colors
	TextColor       = lipgloss.Color("255") // White
	MutedColor      = lipgloss.Color("244") // Gray
)

// Brand styles
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

// Layout styles
var (
	CenteredStyle = lipgloss.NewStyle().
			Width(80).
			Align(lipgloss.Center)

	ContainerStyle = lipgloss.NewStyle().
			Width(60).
			Align(lipgloss.Center).
			Padding(1, 2)
)
