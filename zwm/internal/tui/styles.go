package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	cursorStyle  = lipgloss.NewStyle().Reverse(true)
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	waitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // red: blocked on you
	workingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))  // blue: busy
	doneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))  // green: finished
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
