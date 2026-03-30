package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ==========================================
// Shared Screen Layout
// ==========================================

// screenContentSize: calculates the usable content area by subtracting frame/padding
// dimensions from terminal size. The returned size must never exceed the actual
// terminal area, otherwise bordered sections can wrap into the next line.
func screenContentSize(termWidth, termHeight int) (int, int) {
	horizontalFrame, verticalFrame := styles.ScreenStyle.GetFrameSize()
	contentWidth := max(termWidth-horizontalFrame, 1)
	contentHeight := max(termHeight-verticalFrame, 1)
	return contentWidth, contentHeight
}

// renderScreen: assembles vertical sections into a screen with proper content dimensions.
func renderScreen(width, height int, sections ...string) string {
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return styles.ScreenStyle.
		Width(max(width, 1)).
		Height(max(height, 1)).
		Render(content)
}

// placeCentered: centers content within the given width and height using lipgloss positioning.
func placeCentered(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// renderCenteredOverlay: overlays centered content on top of base content, replacing the underlying lines.
func renderCenteredOverlay(base, overlay string, width, height int) string {
	x := max((width-lipgloss.Width(overlay))/2, 0)
	y := max((height-lipgloss.Height(overlay))/2, 0)
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)

	baseLines := strings.Split(lipgloss.Place(width, height, lipgloss.Left, lipgloss.Top, base), "\n")
	overlayLines := strings.Split(overlay, "\n")

	for row := 0; row < overlayHeight && y+row < len(baseLines); row++ {
		baseLine := lipgloss.NewStyle().Width(width).Render(baseLines[y+row])
		overlayLine := lipgloss.NewStyle().Width(overlayWidth).Render(overlayLines[row])
		left := ansi.Cut(baseLine, 0, x)
		right := ansi.Cut(baseLine, min(x+overlayWidth, width), width)
		baseLines[y+row] = left + overlayLine + right
	}

	return strings.Join(baseLines, "\n")
}

// renderScreenHeader: renders a header with title, subtitle on the left and meta info on the right.
func renderScreenHeader(width int, title, subtitle, meta string) string {
	left := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderTitleStyle.Render(title),
		styles.HeaderSubtitleStyle.Render(subtitle),
	)

	if strings.TrimSpace(meta) == "" {
		return lipgloss.NewStyle().Width(width).Render(left)
	}

	// Calculate available width based on actual content
	metaWidth := lipgloss.Width(meta)
	rightWidth := max(min(metaWidth+2, width/3), 1)
	leftWidth := max(width-rightWidth-1, 1)

	right := lipgloss.NewStyle().
		Foreground(styles.SecondaryColor).
		Bold(true).
		Width(rightWidth).
		Align(lipgloss.Right).
		Render(meta)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).Render(left),
		right,
	)
}

// renderInfoBar: renders an info bar with the given text at the specified width.
func renderInfoBar(width int, text string) string {
	return applyTotalWidth(styles.InfoBarStyle, width).Render(text)
}

// renderFooterBar: renders a footer bar with the given text at the specified width.
// Text is aligned to fill the width for a flush appearance.
func renderFooterBar(width int, text string) string {
	// Use AlignHorizontal to stretch content across full width
	style := applyTotalWidth(styles.FooterStyle, width).AlignHorizontal(lipgloss.Left)
	return style.Render(text)
}

// renderPanel: renders a titled panel with the given body content at the specified width.
func renderPanel(title string, width int, body string) string {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.SectionTitleStyle.Render(title),
		"",
		body,
	)

	return applyTotalWidth(styles.PrimaryPanelStyle, width).Render(content)
}

// renderFramedPanel: renders a panel with double-line border framing, title, and content blocks.
func renderFramedPanel(title string, width int, blocks ...string) string {
	titleText := " " + title + " "
	width = max(width, lipgloss.Width(titleText)+3)
	innerWidth := max(width-4, 1)
	topFill := max(width-lipgloss.Width(titleText)-3, 0)

	var lines []string
	for _, block := range blocks {
		for _, line := range strings.Split(block, "\n") {
			wrapped := lipgloss.NewStyle().Width(innerWidth).Render(line)
			lines = append(lines, strings.Split(wrapped, "\n")...)
		}
	}

	var b strings.Builder
	b.WriteString(styles.FieldBorderStyle.Render("╭─"))
	b.WriteString(styles.SectionTitleStyle.Render(titleText))
	b.WriteString(styles.FieldBorderStyle.Render(strings.Repeat("─", topFill) + "╮"))
	b.WriteString("\n")

	for _, line := range lines {
		padding := max(innerWidth-lipgloss.Width(line), 0)
		b.WriteString(styles.FieldBorderStyle.Render("│ "))
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(styles.FieldBorderStyle.Render(" │"))
		b.WriteString("\n")
	}

	b.WriteString(styles.FieldBorderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯"))
	return b.String()
}

// applyTotalWidth: applies the desired rendered width directly. In Lip Gloss v2,
// Width controls the total rendered box width, including border and padding.
func applyTotalWidth(style lipgloss.Style, totalWidth int) lipgloss.Style {
	return style.Width(max(totalWidth, 1))
}

// applyTotalSize: applies the desired rendered width and height directly. In
// Lip Gloss v2, Width and Height control the total rendered box size.
func applyTotalSize(style lipgloss.Style, totalWidth, totalHeight int) lipgloss.Style {
	return style.
		Width(max(totalWidth, 1)).
		Height(max(totalHeight, 1))
}

// ==========================================
// Embedded Label Fields
// ==========================================

type embeddedFieldOptions struct {
	Label       string
	Value       string
	Width       int
	Focused     bool
	Required    bool
	BorderColor string
	FooterText  string
}

// renderEmbeddedField creates a bordered field with a label and value, optionally styled for focus and required state.
func renderEmbeddedField(opts embeddedFieldOptions) string {
	labelWidth := lipgloss.Width(opts.Label)
	if opts.Required {
		labelWidth += lipgloss.Width(" (*)")
	}
	width := max(opts.Width, labelWidth+8)
	innerWidth := max(width-4, 1)

	borderStyle := styles.FieldBorderStyle
	labelStyle := styles.OnboardingFieldLabelStyle
	if opts.Required {
		borderStyle = styles.FieldRequiredBorderStyle
		labelStyle = styles.OnboardingRequiredLabelStyle
	}
	if opts.Focused {
		borderStyle = styles.FieldFocusedBorderStyle
		labelStyle = styles.OnboardingActiveLabelStyle
	}
	if opts.BorderColor != "" {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(opts.BorderColor))
	}

	label := labelStyle.Render(opts.Label)
	if opts.Required {
		label += " " + styles.OnboardingRequiredLabelStyle.Render("(*)")
	}

	var b strings.Builder
	b.WriteString(borderStyle.Render("╭─ "))
	b.WriteString(label)
	b.WriteString(borderStyle.Render(" "))
	b.WriteString(borderStyle.Render(strings.Repeat("─", max(width-lipgloss.Width(label)-5, 0))))
	b.WriteString(borderStyle.Render("╮"))
	b.WriteString("\n")
	valueBlock := lipgloss.NewStyle().Width(innerWidth).Render(opts.Value)
	for _, line := range strings.Split(valueBlock, "\n") {
		padding := max(innerWidth-lipgloss.Width(line), 0)
		b.WriteString(borderStyle.Render("│ "))
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(borderStyle.Render(" │"))
		b.WriteString("\n")
	}
	b.WriteString(borderStyle.Render("╰"))
	b.WriteString(borderStyle.Render(strings.Repeat("─", width-2)))
	b.WriteString(borderStyle.Render("╯"))

	if strings.TrimSpace(opts.FooterText) != "" {
		b.WriteString("\n")
		b.WriteString(styles.FieldFooterStyle.Width(width).Align(lipgloss.Right).Render(opts.FooterText))
	}

	return b.String()
}

// ==========================================
// Buttons
// ==========================================

type buttonVariant int

const (
	buttonVariantPrimary buttonVariant = iota
	buttonVariantSecondary
)

// actionButtonTotalWidth: calculates the total width needed for a button including its border frame.
func actionButtonTotalWidth(style lipgloss.Style, label string, minimumWidth int) int {
	horizontalFrame, _ := style.GetFrameSize()
	return max(minimumWidth, lipgloss.Width(label)+horizontalFrame+2)
}

// renderActionButton: renders a styled button with optional focus state and variant (primary/secondary).
func renderActionButton(label string, width int, focused bool, variant buttonVariant) string {
	style := styles.PrimaryButtonStyle
	if variant == buttonVariantSecondary {
		style = styles.DestructiveButtonStyle
	}

	if focused {
		focusBackground := styles.PrimaryButtonFocusColor
		if variant == buttonVariantSecondary {
			focusBackground = styles.SecondaryButtonFocusColor
		}
		style = style.
			Background(focusBackground).
			Inherit(styles.FocusedButtonStyle)
	}

	totalWidth := actionButtonTotalWidth(style, label, width)
	return applyTotalWidth(style, totalWidth).Render(label)
}

// renderCenteredActionButtons creates two action buttons centered within the given total width, with appropriate spacing.
func renderCenteredActionButtons(totalWidth int, primaryLabel string, primaryFocused bool, secondaryLabel string, secondaryFocused bool) string {
	primaryStyle := styles.PrimaryButtonStyle
	secondaryStyle := styles.DestructiveButtonStyle
	buttonWidth := max(
		actionButtonTotalWidth(primaryStyle, primaryLabel, 20),
		actionButtonTotalWidth(secondaryStyle, secondaryLabel, 20),
	)

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderActionButton(primaryLabel, buttonWidth, primaryFocused, buttonVariantPrimary),
		"  ",
		renderActionButton(secondaryLabel, buttonWidth, secondaryFocused, buttonVariantSecondary),
	)

	return lipgloss.PlaceHorizontal(totalWidth, lipgloss.Center, row)
}
