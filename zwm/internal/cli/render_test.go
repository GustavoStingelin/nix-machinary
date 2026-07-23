package cli

import (
	"strings"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/stretchr/testify/require"
)

func TestResultRendering_renders_each_result_variant_with_exact_ordered_keys(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected string
	}{
		{
			name:   "worktree result",
			result: testResult,
			expected: "worktree_path=/tmp/worktree\n" +
				"display_identity=feature/topic\n" +
				"tab_action=created\n" +
				"tab_title=project:feature/topic\n" +
				"tab_worktree=/tmp/worktree\n",
		},
		{
			name: "open project result",
			result: OpenProjectResult{
				ProjectRoot: "/tmp/project",
				TabAction:   "focused",
				TabTitle:    "project",
				TabCwd:      "/tmp/project",
			},
			expected: "project_root=/tmp/project\n" +
				"tab_action=focused\n" +
				"tab_title=project\n" +
				"tab_cwd=/tmp/project\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var output strings.Builder

			// When
			err := writeResult(&output, test.result)

			// Then
			require.NoError(t, err)
			require.Equal(t, test.expected, output.String())
		})
	}
}

func TestResultRendering_returns_external_error_for_unknown_result_variant(t *testing.T) {
	// Given
	var output strings.Builder

	// When
	err := writeResult(&output, unsupportedResult{})

	// Then
	require.EqualError(t, err, "unsupported CLI result")
	require.Equal(t, errs.External, errs.ClassOf(err))
	require.Empty(t, output.String())
}

type unsupportedResult struct{}

func (unsupportedResult) result() {}
