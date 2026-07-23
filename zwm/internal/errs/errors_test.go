package errs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestError_retains_typed_class_when_wrapped(t *testing.T) {
	// Given
	cause := errors.New("invalid grammar")
	err := Wrap(Usage, "invalid command", cause)

	// When
	matched := errors.Is(err, ErrUsage)
	var classified *Error
	asTyped := errors.As(err, &classified)

	// Then
	require.True(t, matched)
	require.True(t, asTyped)
	require.Equal(t, Usage, classified.Class)
	require.ErrorIs(t, err, cause)
}

func TestError_returns_contract_exit_code_when_classified(t *testing.T) {
	tests := []struct {
		name  string
		class Class
		want  int
	}{
		{name: "usage", class: Usage, want: 64},
		{name: "project", class: Project, want: 65},
		{name: "preflight", class: Preflight, want: 69},
		{name: "external", class: External, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			err := New(test.class, "message")

			// When
			got := ExitCode(err)

			// Then
			require.Equal(t, test.want, got)
		})
	}
}
