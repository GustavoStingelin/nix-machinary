package zellij

import (
	"context"
	"errors"
	"testing"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
)

func TestLaunch_characterizes_exact_create_and_focus_argv(t *testing.T) {
	tests := []struct {
		name     string
		tabNames string
		want     Command
	}{
		{
			name:     "create",
			tabNames: "another-tab\n",
			want: Command{Name: CommandZellij, Args: []string{
				"action", "new-tab",
				"--layout", "/home/tester/.config/zellij/layouts/worktree.kdl",
				"--name", "project:feature",
				"--cwd", "/worktrees/feature",
			}},
		},
		{
			name:     "focus",
			tabNames: "another-tab\nproject:feature\n",
			want:     Command{Name: CommandZellij, Args: []string{"action", "go-to-tab-name", "project:feature"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			log := &callLog{}
			runner := &fakeRunner{log: log, responses: []runResponse{{output: Output{Stdout: test.tabNames}}, {}}}
			environment := fakeEnvironment{log: log, values: map[EnvironmentVariable]string{
				EnvironmentZellij: "session-1",
				EnvironmentHome:   "/home/tester",
			}}

			// When
			_, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, Input{
				Title: TabTitle("project:feature"),
				Cwd:   Directory("/worktrees/feature"),
			})

			// Then
			if err != nil {
				t.Fatalf("launch: %v", err)
			}
			assertEqual(t, test.want, runner.commands[1])
			t.Logf("literal command array: %#v", test.want)
		})
	}
}

func TestLaunch_creates_tab_with_ordinary_project_cwd(t *testing.T) {
	// Given
	log := &callLog{}
	input := Input{Title: TabTitle("project"), Cwd: Directory("/projects/ordinary root")}
	runner := &fakeRunner{log: log, responses: []runResponse{{output: Output{}}, {output: Output{Stderr: "created\n"}}}}
	environment := fakeEnvironment{log: log, values: map[EnvironmentVariable]string{
		EnvironmentZellij: "session-1",
		EnvironmentHome:   "/home/tester",
	}}

	// When
	result, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, input)

	// Then
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	assertEqual(t, Result{Action: Created, Title: input.Title, Cwd: input.Cwd, Output: Output{Stderr: "created\n"}}, result)
	assertEqual(t, Command{Name: CommandZellij, Args: []string{
		"action", "new-tab",
		"--layout", "/home/tester/.config/zellij/layouts/worktree.kdl",
		"--name", "project",
		"--cwd", "/projects/ordinary root",
	}}, runner.commands[1])
}

func TestLaunch_focuses_existing_exact_tab_name_when_current_session_lists_title(t *testing.T) {
	// Given
	log := &callLog{}
	input := Input{Title: TabTitle("project:feature with space"), Cwd: Directory("/worktrees/feature with space")}
	runner := &fakeRunner{
		log: log,
		responses: []runResponse{
			{output: Output{Stdout: "other-tab\nproject:feature with space\n"}},
			{output: Output{Stdout: "focused\n"}},
		},
	}
	environment := fakeEnvironment{
		log:    log,
		values: map[EnvironmentVariable]string{EnvironmentZellij: "session-1"},
	}

	// When
	result, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, input)

	// Then
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	assertEqual(t, Result{Action: Focused, Title: input.Title, Cwd: input.Cwd, Output: Output{Stdout: "focused\n"}}, result)
	assertEqual(t, []Command{
		{Name: CommandZellij, Args: []string{"action", "query-tab-names"}},
		{Name: CommandZellij, Args: []string{"action", "go-to-tab-name", "project:feature with space"}},
	}, runner.commands)
	assertEqual(t, []string{
		"available:git",
		"available:gh",
		"available:zellij",
		"env:ZELLIJ",
		"run:zellij",
		"run:zellij",
	}, log.calls)
}

func TestLaunch_creates_tab_when_current_session_has_only_prefix_or_suffix_matches(t *testing.T) {
	// Given
	log := &callLog{}
	input := Input{Title: TabTitle("project"), Cwd: Directory("/projects/project")}
	runner := &fakeRunner{
		log: log,
		responses: []runResponse{
			{output: Output{Stdout: "project:feature\nother-project\nproject-suffix\n"}},
			{output: Output{Stderr: "created\n"}},
		},
	}
	environment := fakeEnvironment{
		log: log,
		values: map[EnvironmentVariable]string{
			EnvironmentZellij: "session-1",
			EnvironmentHome:   "/home/tester",
		},
	}

	// When
	result, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, input)

	// Then
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	assertEqual(t, Result{Action: Created, Title: input.Title, Cwd: input.Cwd, Output: Output{Stderr: "created\n"}}, result)
	assertEqual(t, []Command{
		{Name: CommandZellij, Args: []string{"action", "query-tab-names"}},
		{Name: CommandZellij, Args: []string{
			"action", "new-tab",
			"--layout", "/home/tester/.config/zellij/layouts/worktree.kdl",
			"--name", "project",
			"--cwd", "/projects/project",
		}},
	}, runner.commands)
}

func TestLaunch_returns_typed_preflight_error_before_create_when_home_is_unavailable(t *testing.T) {
	// Given
	log := &callLog{}
	runner := &fakeRunner{log: log, responses: []runResponse{{output: Output{Stdout: "another-tab\n"}}}}
	environment := fakeEnvironment{
		log:    log,
		values: map[EnvironmentVariable]string{EnvironmentZellij: "session-1"},
	}

	// When
	_, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, Input{
		Title: TabTitle("project"),
		Cwd:   Directory("/projects/project"),
	})

	// Then
	if !errors.Is(err, errs.ErrPreflight) {
		t.Fatalf("expected preflight class, got %v", err)
	}
	assertEqual(t, []Command{{Name: CommandZellij, Args: []string{"action", "query-tab-names"}}}, runner.commands)
}

func TestLaunch_retains_external_output_and_cause_when_zellij_action_fails(t *testing.T) {
	queryCause := errors.New("query unavailable")
	focusCause := errors.New("focus unavailable")
	createCause := errors.New("create unavailable")
	tests := []struct {
		name      string
		cause     error
		responses []runResponse
		want      Command
	}{
		{
			name:  "query",
			cause: queryCause,
			responses: []runResponse{
				{output: Output{Stderr: "query failed\n"}, err: queryCause},
			},
			want: Command{Name: CommandZellij, Args: []string{"action", "query-tab-names"}},
		},
		{
			name:  "focus",
			cause: focusCause,
			responses: []runResponse{
				{output: Output{Stdout: "project:feature\n"}},
				{output: Output{Stderr: "focus failed\n"}, err: focusCause},
			},
			want: Command{Name: CommandZellij, Args: []string{"action", "go-to-tab-name", "project:feature"}},
		},
		{
			name:  "create",
			cause: createCause,
			responses: []runResponse{
				{output: Output{Stdout: "another tab\n"}},
				{output: Output{Stderr: "create failed\n"}, err: createCause},
			},
			want: Command{Name: CommandZellij, Args: []string{
				"action", "new-tab",
				"--layout", "/home/tester/.config/zellij/layouts/worktree.kdl",
				"--name", "project:feature",
				"--cwd", "/worktrees/feature",
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			log := &callLog{}
			runner := &fakeRunner{log: log, responses: test.responses}
			environment := fakeEnvironment{
				log: log,
				values: map[EnvironmentVariable]string{
					EnvironmentZellij: "session-1",
					EnvironmentHome:   "/home/tester",
				},
			}

			// When
			_, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, Input{
				Title: TabTitle("project:feature"),
				Cwd:   Directory("/worktrees/feature"),
			})

			// Then
			if !errors.Is(err, errs.ErrExternal) {
				t.Fatalf("expected external class, got %v", err)
			}
			if !errors.Is(err, test.cause) {
				t.Fatalf("expected cause %v, got %v", test.cause, err)
			}
			var failure *CommandFailure
			if !errors.As(err, &failure) {
				t.Fatalf("expected CommandFailure, got %T", err)
			}
			assertEqual(t, test.want, failure.Command)
			assertEqual(t, test.want, runner.commands[len(runner.commands)-1])
			if failure.Output.Stderr == "" {
				t.Fatal("expected external stderr to be retained")
			}
		})
	}
}
