package styles

import (
	"charm.land/lipgloss/v2"
)

// ==========================================
// Core Color Tokens
// ==========================================

var (
	TextColor                 = lipgloss.Color("#E6EDF3")
	MutedColor                = lipgloss.Color("#5b636d")
	PrimaryColor              = lipgloss.Color("#00BFFF")
	SecondaryColor            = lipgloss.Color("#4FD1C5")
	AccentColor               = lipgloss.Color("#38BDF8")
	SelectionColor            = lipgloss.Color("#95C798")
	ConnectionBorderColor     = lipgloss.Color("#F0C802")
	BackgroundColor           = lipgloss.Color("#0B1118")
	SurfaceBackgroundColor    = lipgloss.Color("#0F1720")
	SurfaceColor              = lipgloss.Color("#2A3A4A")
	FocusColor                = lipgloss.Color("#38BDF8")
	SuccessColor              = lipgloss.Color("#006E05")
	WarningColor              = lipgloss.Color("#F59E0B")
	ErrorColor                = lipgloss.Color("#B50000")
	FailureColor              = ErrorColor
	DebugColor                = lipgloss.Color("#7D93AA")
	InfoColor                 = SecondaryColor
	CriticalColor             = lipgloss.Color("#FF8A8A")
	PrimaryButtonFocusColor   = lipgloss.Color("#0A8A11")
	SecondaryButtonFocusColor = lipgloss.Color("#CC1414")
	ApiCallerColor            = lipgloss.Color("#da5de6")
	ProgressBarFg             = lipgloss.Color("#FFD580") // light orange (start of gradient)
	ProgressBarBg             = lipgloss.Color("#CC5500") // dark orange  (end of gradient)
)

// ==========================================
// Screen Chrome
// ==========================================

var (
	AppStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	ScreenStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Padding(0, 2)

	PrimaryPanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(SurfaceColor).
				Padding(1, 2)

	FocusedPanelStyle = PrimaryPanelStyle.
				BorderForeground(FocusColor)

	HeaderTitleStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Bold(true)

	HeaderMetaStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	HeaderSubtitleStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	InfoBarStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(SurfaceColor).
			Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(SurfaceColor).
			Padding(0, 0) // No horizontal padding to fill full width

	SectionTitleStyle = lipgloss.NewStyle().
				Foreground(SecondaryColor).
				Bold(true)

	BodyTextStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	EmptyStateStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)
)

// ==========================================
// Welcome And Loading Styles
// ==========================================

var (
	LogoStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	LogoSubtleStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	VersionStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	LoadingStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	LoadingTitleStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Bold(true)

	LoadingStepStyle = lipgloss.NewStyle().
				Foreground(SuccessColor)

	LoadingCurrentStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	LoadingErrorStyle = lipgloss.NewStyle().
				Foreground(FailureColor).
				Bold(true)
)

// ==========================================
// Main Screen Styles
// ==========================================

var (
	HeaderStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	ViewportStyle = lipgloss.NewStyle().
			Foreground(TextColor)

	LogTimestampStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	LogProcessStyle = lipgloss.NewStyle().
			Foreground(ApiCallerColor)

	LogLevelStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Bold(true)
)

// ==========================================
// Form Styles
// ==========================================

var (
	OnboardingBorderStyle = lipgloss.NewStyle().
				Foreground(SurfaceColor)

	OnboardingPanelStyle = lipgloss.NewStyle().
				Inherit(PrimaryPanelStyle)

	OnboardingTitleStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Bold(true)

	OnboardingBannerStyle = lipgloss.NewStyle().
				Foreground(WarningColor).
				Italic(true)

	OnboardingFieldLabelStyle = lipgloss.NewStyle().
					Foreground(SecondaryColor).
					Bold(true)

	OnboardingActiveLabelStyle = lipgloss.NewStyle().
					Foreground(TextColor).
					Bold(true)

	OnboardingRequiredLabelStyle = lipgloss.NewStyle().
					Foreground(ErrorColor).
					Bold(true)

	OnboardingFieldValueStyle = lipgloss.NewStyle().
					Foreground(TextColor)

	OnboardingActiveValueStyle = lipgloss.NewStyle().
					Foreground(TextColor)

	OnboardingActiveIndicatorStyle = lipgloss.NewStyle().
					Foreground(FocusColor).
					Bold(true)

	OnboardingSeparatorStyle = lipgloss.NewStyle().
					Foreground(SurfaceColor)

	OnboardingFieldActiveStyle = lipgloss.NewStyle().
					BorderStyle(lipgloss.RoundedBorder()).
					BorderForeground(FocusColor).
					Padding(0, 1)

	OnboardingErrorStyle = lipgloss.NewStyle().
				Foreground(FailureColor).
				Bold(true)

	OnboardingHintStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	OnboardingSavedStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				Bold(true)

	FieldBorderStyle = lipgloss.NewStyle().
				Foreground(SurfaceColor)

	FieldFocusedBorderStyle = lipgloss.NewStyle().
				Foreground(FocusColor)

	FieldRequiredBorderStyle = lipgloss.NewStyle().
					Foreground(ErrorColor)

	FieldPlaceholderStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	FieldCursorStyle = lipgloss.NewStyle().
				Foreground(FocusColor).
				Bold(true)

	FieldFooterStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	PrimaryButtonStyle = lipgloss.NewStyle().
				Background(SuccessColor).
				Foreground(TextColor).
				Bold(true).
				Padding(1, 3).
				Align(lipgloss.Center)

	DestructiveButtonStyle = lipgloss.NewStyle().
				Background(ErrorColor).
				Foreground(TextColor).
				Bold(true).
				Padding(1, 3).
				Align(lipgloss.Center)

	FocusedButtonStyle = lipgloss.NewStyle().
				Foreground(TextColor).
				Underline(true)
)
