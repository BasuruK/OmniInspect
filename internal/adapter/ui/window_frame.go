package ui

import (
	"strings"

	"OmniView/internal/adapter/ui/styles"
	"charm.land/lipgloss/v2"
)

// ==========================================
// WindowFrame — Main frame with hazard stripe
// ==========================================

// WindowFrame renders a terminal window frame with:
//   - Hazard stripe (top and bottom): diagonal ╱ slashes in coral red
//   - Centered title bar
//   - Thick horizontal borders
//   - Dark content area for child components
type WindowFrame struct {
	title   string
	width   int
	height  int
	content string
}

// NewWindowFrame creates a new WindowFrame with the given title and dimensions.
func NewWindowFrame(title string, width, height int) WindowFrame {
	return WindowFrame{
		title:  title,
		width:  width,
		height: height,
	}
}

// SetContent sets the content to render inside the frame.
func (w WindowFrame) SetContent(content string) WindowFrame {
	w.content = content
	return w
}

// Render returns the full rendered frame as a string.
func (w WindowFrame) Render() string {
	// Hazard stripe character: ╱ (U+2571)
	hazardChar := "╱"
	hazardLine := strings.Repeat(hazardChar, w.width)

	// Title bar
	titleText := "-----[ " + w.title + " ]-----"
	titleBar := lipgloss.Place(w.width, 1, lipgloss.Center, lipgloss.Center, styles.FrameTitleStyle.Render(titleText))

	// Thick horizontal border using box-drawing character
	borderLine := strings.Repeat("─", w.width)

	// Top section: hazard stripe + title bar + border
	topSection := strings.Join([]string{
		styles.HazardStripeStyle.Render(hazardLine),
		titleBar,
		styles.FrameBorderStyle.Render("┌" + borderLine + "┐"),
	}, "\n")

	// Content area padding rows (above and below content)
	// Frame layout: hazard(1) + title(1) + border(1) + content(h-4) + border(1) + hazard(1) = h rows
	middleRowCount := w.height - 6
	if middleRowCount < 0 {
		middleRowCount = 0
	}

	// Content lines split from the content string
	contentLines := strings.Split(w.content, "\n")

	// Build middle section
	var sb strings.Builder
	sb.WriteString(styles.FrameBorderStyle.Render("│ "))
	sb.WriteString(styles.FrameContentStyle.Render(strings.Repeat(" ", w.width-2)))
	sb.WriteString(styles.FrameBorderStyle.Render(" │"))
	sb.WriteString("\n")

	// Render content lines with frame borders
	for i, line := range contentLines {
		if i >= middleRowCount-2 {
			break
		}
		lineWidth := w.width - 4
		if lineWidth < 0 {
			lineWidth = 0
		}
		if len(line) > lineWidth {
			line = line[:lineWidth]
		}
		padding := w.width - 4 - len(line)
		if padding < 0 {
			padding = 0
		}

		sb.WriteString(styles.FrameBorderStyle.Render("│ "))
		sb.WriteString(styles.FrameContentStyle.Render(line))
		sb.WriteString(strings.Repeat(" ", padding))
		sb.WriteString(styles.FrameBorderStyle.Render(" │"))
		sb.WriteString("\n")
	}

	// Bottom border
	bottomBorder := styles.FrameBorderStyle.Render("└" + borderLine + "┘")

	// Bottom hazard stripe
	bottomHazard := styles.HazardStripeStyle.Render(hazardLine)

	// Full frame
	return strings.Join([]string{
		topSection,
		sb.String(),
		bottomBorder,
		bottomHazard,
	}, "\n")
}
