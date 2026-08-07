package zellij

import (
	"context"
	"testing"
)

func TestListSessions_parses_names_and_exited_status_with_exact_argv(t *testing.T) {
	// Given the unformatted list with a running, current, and exited session
	log := &callLog{}
	runner := &fakeRunner{log: log, responses: []runResponse{
		{output: Output{Stdout: "" +
			"bitcoin [Created 7m ago] \n" +
			"nix-machinary [Created 1h ago] (current)\n" +
			"stale [Created 2days ago] (EXITED - attach to resurrect)\n"}},
	}}

	// When
	sessions, err := ListSessions(context.Background(), Config{Runner: runner})

	// Then
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	assertEqual(t, Command{Name: CommandZellij, Args: []string{"list-sessions", "--no-formatting"}}, runner.commands[0])
	assertEqual(t, []Session{
		{Name: "bitcoin"},
		{Name: "nix-machinary"},
		{Name: "stale", Exited: true},
	}, sessions)
}

func TestQueryTabNames_routes_to_the_named_session_and_strips_the_glyph(t *testing.T) {
	// Given a session whose second tab carries the attention glyph
	log := &callLog{}
	runner := &fakeRunner{log: log, responses: []runResponse{
		{output: Output{Stdout: "editor\n● agent\nlogs\n  seen\n"}},
	}}

	// When
	tabs, err := QueryTabNames(context.Background(), Config{Runner: runner}, "bitcoin")

	// Then the argv targets the named session...
	if err != nil {
		t.Fatalf("query tab names: %v", err)
	}
	assertEqual(t, Command{Name: CommandZellij, Args: []string{"--session", "bitcoin", "action", "query-tab-names"}}, runner.commands[0])

	// ...and only the glyphed tab is flagged, with the glyph stripped from its
	// title. A tab whose mark was cleared keeps the blank marker in Zellij, so it
	// reports the bare title and no attention.
	assertEqual(t, []Tab{
		{Title: "editor"},
		{Title: "agent", NeedsAttention: true},
		{Title: "logs"},
		{Title: "seen"},
	}, tabs)
}

func TestGoToTab_uses_exact_argv(t *testing.T) {
	// Given
	log := &callLog{}
	runner := &fakeRunner{log: log, responses: []runResponse{{}}}

	// When
	if _, err := GoToTab(context.Background(), Config{Runner: runner}, "agent"); err != nil {
		t.Fatalf("go to tab: %v", err)
	}

	// Then
	assertEqual(t, Command{Name: CommandZellij, Args: []string{"action", "go-to-tab-name", "agent"}}, runner.commands[0])
}

func TestCurrentSession_reads_the_session_name_env(t *testing.T) {
	log := &callLog{}

	present := fakeEnvironment{log: log, values: map[EnvironmentVariable]string{
		EnvironmentZellijSessionName: "bitcoin",
	}}
	name, ok := CurrentSession(Config{Environment: present})
	if !ok || name != "bitcoin" {
		t.Fatalf("want bitcoin/true, got %q/%v", name, ok)
	}

	absent := fakeEnvironment{log: log, values: map[EnvironmentVariable]string{}}
	if _, ok := CurrentSession(Config{Environment: absent}); ok {
		t.Fatal("expected no current session when the env var is unset")
	}
}
