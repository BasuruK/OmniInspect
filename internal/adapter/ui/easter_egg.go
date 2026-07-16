package ui

import (
	"OmniView/internal/adapter/ui/styles"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ==========================================
// Easter Egg Overlay
// ==========================================

// renderEasterEggOverlay renders the hidden Easter Egg message box.
// It is displayed centered over the main screen when m.showEasterEgg is true,
// triggered by pressing Alt+M.
func (m *Model) renderEasterEggOverlay() string {
	modalWidth := max(min(m.width-20, 60), 44)
	innerWidth := max(modalWidth-4, 24)

	centerStyle := lipgloss.NewStyle().Width(innerWidth).Align(lipgloss.Center)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		centerStyle.Render(styles.BodyTextStyle.Render("This software is build by Basuru Balasuriya")),
		"",
		centerStyle.Render(m.renderEasterEggAnimation(innerWidth)),
		centerStyle.Render(lipgloss.NewStyle().Foreground(styles.MutedColor).Render("chaotic double pendulum — outer bob traces the arcs")),
		"",
		centerStyle.Render(lipgloss.NewStyle().Foreground(styles.MutedColor).Render("[ Alt+M or Esc — Close ]")),
	)

	return renderFramedPanel("Easter Egg", modalWidth, panelTypeInfo, content)
}

// ==========================================
// Easter Egg Double-Pendulum Physics Animation
// ==========================================
//
// A chaotic double pendulum: two rigid arms swinging under gravity. The
// outer bob's chaotic path is traced and fades in (muted) behind the
// bright, live rig.

const (
	pendulumG         = 9.8  // gravity
	pendulumL1        = 1.0  // inner arm length
	pendulumL2        = 1.0  // outer arm length
	pendulumM1        = 1.0  // inner bob mass
	pendulumM2        = 1.0  // outer bob mass
	pendulumDT       = 0.02 // physics integration step (seconds)
	pendulumSubsteps = 3    // integration steps per animation tick

	easterEggTickInterval = 60 * time.Millisecond
	easterEggTraceSeconds = 10 // how long the outer-bob trace stays on screen
	easterEggTraceMax     = int(easterEggTraceSeconds * time.Second / easterEggTickInterval)
	easterEggGridRows     = 11 // rendered grid height, rows below the pivot
)

// easterEggTickMsg drives the pendulum animation frame-by-frame. It carries
// the session ID it was scheduled under so stale ticks from a previous
// open-then-close-then-reopen cycle can be discarded instead of running a
// second, concurrent animation loop.
type easterEggTickMsg struct {
	t       time.Time
	session int
}

// easterEggTickCmd schedules the next animation frame for the given session.
func easterEggTickCmd(session int) tea.Cmd {
	return tea.Tick(easterEggTickInterval, func(t time.Time) tea.Msg {
		return easterEggTickMsg{t: t, session: session}
	})
}

// eggPoint is an outer-bob trace sample in pendulum units (unscaled), so
// rendering can rescale it to whatever grid width the terminal allows.
type eggPoint struct{ x, y float64 }

// resetEasterEggPhysics (re)initializes the pendulum near-horizontal with a
// small offset between the arms — a classic recipe for chaotic motion.
func (m *Model) resetEasterEggPhysics() {
	m.easterEggTheta1 = math.Pi / 2
	m.easterEggTheta2 = math.Pi/2 + 0.15
	m.easterEggOmega1, m.easterEggOmega2 = 0, 0
	m.easterEggTrace = nil
}

// doublePendulumAccel returns the angular accelerations for a standard
// equal-mass, equal-length double pendulum (Lagrangian equations of motion).
func doublePendulumAccel(theta1, theta2, w1, w2 float64) (a1, a2 float64) {
	delta := theta1 - theta2
	den := pendulumL1 * (2*pendulumM1 + pendulumM2 - pendulumM2*math.Cos(2*delta))
	a1 = (-pendulumG*(2*pendulumM1+pendulumM2)*math.Sin(theta1) -
		pendulumM2*pendulumG*math.Sin(theta1-2*theta2) -
		2*math.Sin(delta)*pendulumM2*(w2*w2*pendulumL2+w1*w1*pendulumL1*math.Cos(delta))) / den

	den2 := pendulumL2 * (2*pendulumM1 + pendulumM2 - pendulumM2*math.Cos(2*delta))
	a2 = (2 * math.Sin(delta) * (w1*w1*pendulumL1*(pendulumM1+pendulumM2) +
		pendulumG*(pendulumM1+pendulumM2)*math.Cos(theta1) +
		w2*w2*pendulumL2*pendulumM2*math.Cos(delta))) / den2
	return a1, a2
}

// stepEasterEggPhysics advances the simulation (semi-implicit Euler, a few
// substeps per tick for stability) and records the outer bob's new position.
func (m *Model) stepEasterEggPhysics() {
	for range pendulumSubsteps {
		a1, a2 := doublePendulumAccel(m.easterEggTheta1, m.easterEggTheta2, m.easterEggOmega1, m.easterEggOmega2)
		m.easterEggOmega1 += a1 * pendulumDT
		m.easterEggOmega2 += a2 * pendulumDT
		m.easterEggTheta1 += m.easterEggOmega1 * pendulumDT
		m.easterEggTheta2 += m.easterEggOmega2 * pendulumDT
	}

	x1 := pendulumL1 * math.Sin(m.easterEggTheta1)
	y1 := pendulumL1 * math.Cos(m.easterEggTheta1)
	x2 := x1 + pendulumL2*math.Sin(m.easterEggTheta2)
	y2 := y1 + pendulumL2*math.Cos(m.easterEggTheta2)
	m.easterEggTrace = append(m.easterEggTrace, eggPoint{x2, y2})
	if len(m.easterEggTrace) > easterEggTraceMax {
		m.easterEggTrace = m.easterEggTrace[len(m.easterEggTrace)-easterEggTraceMax:]
	}
}

// renderEasterEggAnimation renders the pendulum rig plus the outer bob's
// fading trace as a colored ASCII grid.
func (m *Model) renderEasterEggAnimation(width int) string {
	width = max(width, 20)
	pivotCol, pivotRow := width/2, 0
	maxReach := max(math.Min(float64(easterEggGridRows-2), float64(width)/2-2), 1)
	scale := maxReach / (pendulumL1 + pendulumL2)

	toGrid := func(x, y float64) (int, int) {
		col := max(0, min(width-1, pivotCol+int(math.Round(x*scale))))
		row := max(0, min(easterEggGridRows-1, pivotRow+int(math.Round(y*scale))))
		return col, row
	}

	grid := make([][]rune, easterEggGridRows)
	bright := make([][]bool, easterEggGridRows)
	for r := range grid {
		grid[r] = []rune(strings.Repeat(" ", width))
		bright[r] = make([]bool, width)
	}
	set := func(col, row int, ch rune, isBright bool) {
		grid[row][col] = ch
		bright[row][col] = isBright
	}

	// Trace first so the live rig always draws on top of it.
	for _, p := range m.easterEggTrace {
		c, r := toGrid(p.x, p.y)
		set(c, r, '·', false)
	}

	x1 := pendulumL1 * math.Sin(m.easterEggTheta1)
	y1 := pendulumL1 * math.Cos(m.easterEggTheta1)
	x2 := x1 + pendulumL2*math.Sin(m.easterEggTheta2)
	y2 := y1 + pendulumL2*math.Cos(m.easterEggTheta2)
	c1, r1 := toGrid(x1, y1)
	c2, r2 := toGrid(x2, y2)

	// Coarse rod interpolation — plenty at this character-grid resolution.
	const rodSteps = 6
	drawRod := func(fromC, fromR, toC, toR int) {
		for i := 1; i < rodSteps; i++ {
			t := float64(i) / rodSteps
			set(fromC+int(math.Round(float64(toC-fromC)*t)), fromR+int(math.Round(float64(toR-fromR)*t)), '.', true)
		}
	}
	drawRod(pivotCol, pivotRow, c1, r1)
	drawRod(c1, r1, c2, r2)

	set(pivotCol, pivotRow, '+', true)
	set(c1, r1, 'o', true)
	set(c2, r2, 'O', true)

	brightStyle := lipgloss.NewStyle().Foreground(styles.WarningColor)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	lines := make([]string, easterEggGridRows)
	for r := range grid {
		var b strings.Builder
		for c, ch := range grid[r] {
			switch {
			case ch == ' ':
				b.WriteRune(' ')
			case bright[r][c]:
				b.WriteString(brightStyle.Render(string(ch)))
			default:
				b.WriteString(mutedStyle.Render(string(ch)))
			}
		}
		lines[r] = b.String()
	}
	return strings.Join(lines, "\n")
}
