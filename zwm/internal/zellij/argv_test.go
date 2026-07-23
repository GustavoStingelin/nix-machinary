package zellij

import (
	"context"
	"testing"
)

func TestLaunch_preserves_spaces_and_newlines_in_title_layout_and_cwd_arguments(t *testing.T) {
	// Given
	log := &callLog{}
	input := Input{
		Title: TabTitle("project:feature with space\nand newline"),
		Cwd:   Directory("/worktrees/feature with space\nand newline"),
	}
	runner := &fakeRunner{
		log: log,
		responses: []runResponse{
			{output: Output{Stdout: "\nproject:feature\n"}},
			{output: Output{}},
		},
	}
	environment := fakeEnvironment{
		log: log,
		values: map[EnvironmentVariable]string{
			EnvironmentZellij: "session-1",
			EnvironmentHome:   "/home/tester with space\nand newline",
		},
	}

	// When
	result, err := Launch(context.Background(), Config{Runner: runner, Environment: environment}, input)

	// Then
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	assertEqual(t, Created, result.Action)
	assertEqual(t, Command{Name: CommandZellij, Args: []string{
		"action", "new-tab",
		"--layout", "/home/tester with space\nand newline/.config/zellij/layouts/worktree.kdl",
		"--name", "project:feature with space\nand newline",
		"--cwd", "/worktrees/feature with space\nand newline",
	}}, runner.commands[1])
}
