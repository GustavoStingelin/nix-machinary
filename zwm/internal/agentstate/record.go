// Package agentstate is the three-way source of truth for coding-agent attention.
//
// The zwm-attn Zellij plugin only knows a binary, self-clearing tab glyph, so it
// cannot answer "is this agent working, waiting for me, or done". Instead the
// attention hooks call `zwm attn <state>`, which writes a small JSON record per
// Zellij session+pane here; the `zwm tui` dashboard reads them back. Records are
// keyed by session and pane because a pane id is only unique within its session.
package agentstate

import (
	"fmt"
	"time"
)

// State is an agent's current attention state.
type State string

const (
	// Working means the agent is busy and needs nothing.
	Working State = "working"
	// Waiting means the agent is blocked on the user (input or a permission).
	Waiting State = "waiting"
	// Done means the agent finished its turn and the result is unreviewed.
	Done State = "done"
)

// ParseState normalizes a hook signal into a State. It accepts the canonical
// state names and the synonyms the Zellij attention hooks emit ("finished" for
// Done), so callers can pass either vocabulary.
func ParseState(raw string) (State, error) {
	switch raw {
	case "working":
		return Working, nil
	case "waiting":
		return Waiting, nil
	case "done", "finished":
		return Done, nil
	default:
		return "", fmt.Errorf("unknown attention state %q", raw)
	}
}

// Record is one agent's attention state at a point in time. TabTitle is the
// Zellij tab the agent runs in (zwm's "<key>" or "<key>:<branch>" naming),
// reconstructed at write time so the dashboard can place the agent under its
// tab; it is empty when the tab could not be determined (e.g. a detached PR
// worktree or a non-repo pane).
type Record struct {
	Session   string    `json:"session"`
	PaneID    string    `json:"pane_id"`
	Agent     string    `json:"agent,omitempty"`
	TabTitle  string    `json:"tab_title,omitempty"`
	State     State     `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}

// urgency orders states so a session's rollup surfaces the state that most wants
// the user's attention: something blocked on them outranks a finished turn,
// which outranks an agent that is still working.
func (state State) urgency() int {
	switch state {
	case Waiting:
		return 3
	case Done:
		return 2
	case Working:
		return 1
	default:
		return 0
	}
}

// MostUrgent returns the state that most wants the user's attention among the
// records, or the empty State when there are none. It is used to badge a session
// header from the states of the agents inside it.
func MostUrgent(records []Record) State {
	var best State
	for _, record := range records {
		if record.State.urgency() > best.urgency() {
			best = record.State
		}
	}
	return best
}
