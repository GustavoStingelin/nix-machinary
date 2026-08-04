package tui

import (
	catppuccin "github.com/catppuccin/go"
	"github.com/charmbracelet/lipgloss"
)

// Colors come from the Catppuccin Mocha palette (matching the Zellij theme). The
// "waiting for you" state uses the softer Maroon rather than the vivid Red, which
// read too strong for a state you see often.
var mocha = catppuccin.Mocha

func color(c catppuccin.Color) lipgloss.Color { return lipgloss.Color(c.Hex) }

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(color(mocha.Overlay1()))
	cursorStyle  = lipgloss.NewStyle().Reverse(true)
	footerStyle  = lipgloss.NewStyle().Foreground(color(mocha.Overlay1()))
	errorStyle   = lipgloss.NewStyle().Foreground(color(mocha.Red()))
	waitingStyle = lipgloss.NewStyle().Foreground(color(mocha.Maroon())) // blocked on you
	workingStyle = lipgloss.NewStyle().Foreground(color(mocha.Blue()))   // busy
	doneStyle    = lipgloss.NewStyle().Foreground(color(mocha.Green()))  // finished
)

// stateStyle returns the color and human phrase for an attention state.
func stateStyle(state string) (lipgloss.Style, string) {
	switch state {
	case StateWaiting:
		return waitingStyle, "waiting for you"
	case StateWorking:
		return workingStyle, "working"
	case StateDone:
		return doneStyle, "done"
	default:
		return dimStyle, state
	}
}

// stateGlyph returns a compact icon for an attention state.
func stateGlyph(state string) string {
	switch state {
	case StateWaiting:
		return "●"
	case StateWorking:
		return "◐"
	case StateDone:
		return "✓"
	default:
		return " "
	}
}

// urgency ranks states so a session header can badge itself with the state that
// most wants the user's attention.
func urgency(state string) int {
	switch state {
	case StateWaiting:
		return 3
	case StateDone:
		return 2
	case StateWorking:
		return 1
	default:
		return 0
	}
}
