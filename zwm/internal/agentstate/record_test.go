package agentstate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseState_normalizes_signals_including_synonyms(t *testing.T) {
	tests := []struct {
		raw  string
		want State
	}{
		{"working", Working},
		{"waiting", Waiting},
		{"done", Done},
		{"finished", Done},
	}
	for _, test := range tests {
		got, err := ParseState(test.raw)
		require.NoError(t, err)
		require.Equal(t, test.want, got)
	}
}

func TestParseState_rejects_unknown_signals(t *testing.T) {
	_, err := ParseState("napping")
	require.Error(t, err)
}

func TestMostUrgent_orders_waiting_over_done_over_working(t *testing.T) {
	require.Equal(t, Waiting, MostUrgent([]Record{
		{State: Working}, {State: Done}, {State: Waiting},
	}))
	require.Equal(t, Done, MostUrgent([]Record{
		{State: Working}, {State: Done},
	}))
	require.Equal(t, Working, MostUrgent([]Record{{State: Working}}))
	require.Equal(t, State(""), MostUrgent(nil))
}
