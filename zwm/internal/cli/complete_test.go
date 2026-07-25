package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubCompleter struct {
	branchesProject ProjectNameOrPath
	prProject       ProjectNameOrPath
	branches        []string
	projects        []string
	pullRequests    []string
}

func (completer *stubCompleter) Branches(_ context.Context, project ProjectNameOrPath) []string {
	completer.branchesProject = project
	return completer.branches
}

func (completer *stubCompleter) Projects(_ context.Context) []string {
	return completer.projects
}

func (completer *stubCompleter) PullRequests(_ context.Context, project ProjectNameOrPath) []string {
	completer.prProject = project
	return completer.pullRequests
}

func TestShellCompletion_emits_candidates_for_interactive_positions(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "existing branch position",
			args:     []string{"wco", "--generate-shell-completion"},
			expected: "main\nfeature/topic\n",
		},
		{
			name:     "new branch name has no candidates",
			args:     []string{"wco", "-b", "--generate-shell-completion"},
			expected: "",
		},
		{
			name:     "new branch start point completes branches",
			args:     []string{"wco", "-b", "new/topic", "--generate-shell-completion"},
			expected: "main\nfeature/topic\n",
		},
		{
			name:     "open project completes projects",
			args:     []string{"o", "--generate-shell-completion"},
			expected: "alpha\nbeta\n",
		},
		{
			name:     "pull request completes open pull requests",
			args:     []string{"wpr", "--generate-shell-completion"},
			expected: "12:Fix bug\n34:Add feature\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			completer := &stubCompleter{
				branches:     []string{"main", "feature/topic"},
				projects:     []string{"alpha", "beta"},
				pullRequests: []string{"12:Fix bug", "34:Add feature"},
			}

			// When
			exitCode, stdout, stderr := runCLIWithCompleter(t, completer, test.args...)

			// Then
			require.Equal(t, 0, exitCode)
			require.Equal(t, test.expected, stdout)
			require.Empty(t, stderr)
		})
	}
}

func TestShellCompletion_passes_selected_project_to_branch_lookup(t *testing.T) {
	// Given
	completer := &stubCompleter{branches: []string{"main"}}

	// When
	exitCode, stdout, _ := runCLIWithCompleter(t, completer, "-C", "chosen", "wco", "--generate-shell-completion")

	// Then
	require.Equal(t, 0, exitCode)
	require.Equal(t, "main\n", stdout)
	require.Equal(t, ProjectNameOrPath("chosen"), completer.branchesProject)
}

func TestShellCompletion_is_inert_when_no_completer_is_configured(t *testing.T) {
	// When
	exitCode, stdout, stderr := runCLI(t, &recordingService{}, "wco", "--generate-shell-completion")

	// Then
	require.Equal(t, 0, exitCode)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
}
