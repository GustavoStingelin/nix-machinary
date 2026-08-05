package command

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/agentstate"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/cli"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/zellij"
)

// glyphPipeTimeout bounds the cosmetic tab-glyph pipe so a wedged Zellij can
// never hang an editor hook, which fires on every turn.
const glyphPipeTimeout = 3 * time.Second

// closedSignal is the pseudo-state an editor's exit hook sends to forget its
// pane's attention record.
const closedSignal = "closed"

// tabTitler reconstructs, for the pane's working directory, the Zellij tab title
// zwm would have given the tab, so the dashboard can place the agent under it. It
// returns "" when the tab can't be determined (non-repo, detached PR worktree).
type tabTitler interface {
	TabTitle(ctx context.Context, cwd string) string
}

// attnRecorder implements cli.AttnRecorder: it persists a three-way attention
// record for the current pane and fires the (binary, cosmetic) zwm-attn tab
// glyph. The glyph write mirrors what the old shell wrapper did.
type attnRecorder struct {
	env    zellij.Environment
	runner zellij.Runner
	store  *agentstate.Store
	titles tabTitler
}

// NewSystemAttnRecorder wires the real environment, Zellij runner, on-disk state
// store, and the tab-title reconstructor (project resolver + git).
func NewSystemAttnRecorder() cli.AttnRecorder {
	gitClient := git.NewClient(git.Config{})
	home, _ := os.LookupEnv("HOME")
	return attnRecorder{
		env:    zellij.SystemEnvironment{},
		runner: zellij.SystemRunner{},
		store:  agentstate.NewStore(agentstate.Dir(os.LookupEnv)),
		titles: systemTabTitler{
			resolver: project.NewResolver(projectRepository{client: gitClient}),
			git:      gitClient,
			home:     home,
		},
	}
}

func (recorder attnRecorder) Record(ctx context.Context, signal, agent string) error {
	// No-op outside Zellij, exactly like the previous wrapper's `[ -n "$ZELLIJ" ]`
	// guard: hooks fire everywhere, but attention state only means something in a
	// session.
	if session, present := recorder.env.Lookup(zellij.EnvironmentZellij); !present || session == "" {
		return nil
	}

	sessionName, _ := recorder.env.Lookup(zellij.EnvironmentZellijSessionName)
	paneID, _ := recorder.env.Lookup(zellij.EnvironmentZellijPaneID)

	// "closed" means the agent process exited (its editor's exit hook), so forget
	// the record — the pane/tab may still be open, so tab reconciliation can't
	// catch this. No glyph pipe: piping would re-mark the tab, and the plugin
	// clears the glyph itself on focus.
	if signal == closedSignal {
		if sessionName != "" && paneID != "" {
			_ = recorder.store.Delete(sessionName, paneID)
		}
		return nil
	}

	state, err := agentstate.ParseState(signal)
	if err != nil {
		return errs.Wrap(errs.Usage, "record attention state", err)
	}

	// Persisting needs both keys; without them the record can't be addressed, but
	// the glyph pipe below still works, so fall through instead of erroring.
	if sessionName != "" && paneID != "" {
		if writeErr := recorder.store.Write(agentstate.Record{
			Session:  sessionName,
			PaneID:   paneID,
			Agent:    agent,
			TabTitle: recorder.tabTitle(ctx, sessionName, paneID),
			State:    state,
		}); writeErr != nil {
			return errs.Wrap(errs.External, "write attention state", writeErr)
		}
	}

	// Fire the tab glyph best-effort: it is cosmetic, and a hook must not fail
	// (or stall) because the plugin is missing or the pipe wedged.
	pipeCtx, cancel := context.WithTimeout(ctx, glyphPipeTimeout)
	defer cancel()
	_, _ = recorder.runner.Run(pipeCtx, zellij.Command{
		Name: zellij.CommandZellij,
		Args: []string{
			"pipe", "--plugin", "zwm-attn", "--name", "zwm-attn",
			"--args", "pane_id=" + paneID + ",event=" + string(state),
		},
	})
	return nil
}

// tabTitle best-effort reconstructs the current pane's tab title; failures yield
// "" so the agent simply falls back to a session-level row.
//
// Reconstruction runs git, and agents signal frequently (every tool call), so a
// title already recorded for this pane is reused. A pane's worktree is stable,
// and if its tab closes the reconciler deletes the record, so the next signal
// recomputes a fresh title.
func (recorder attnRecorder) tabTitle(ctx context.Context, session, paneID string) string {
	if existing, ok := recorder.store.Read(session, paneID); ok && existing.TabTitle != "" {
		return existing.TabTitle
	}
	if recorder.titles == nil {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return recorder.titles.TabTitle(ctx, cwd)
}

// systemTabTitler reconstructs a tab title the way zwm's commands name tabs:
// "<key>" for the primary worktree, "<key>:<branch>" for a linked branch
// worktree. It reuses the same project resolver, so the key matches byte-for-byte.
type systemTabTitler struct {
	resolver project.Resolver
	git      git.Client
	home     string
}

func (titler systemTabTitler) TabTitle(ctx context.Context, cwd string) string {
	resolution, err := titler.resolver.Resolve(ctx, project.Request{
		Home:             project.Directory(titler.home),
		Project:          project.Value(""),
		WorkingDirectory: project.Directory(cwd),
	})
	if err != nil {
		return ""
	}
	key := string(resolution.Key)

	currentRoot, err := titler.git.WorktreeRoot(ctx, git.Directory(cwd))
	if err != nil {
		return ""
	}
	// The primary worktree's tab is titled with just the key (zwm o); a linked
	// worktree's tab is "<key>:<branch>" (zwm wco). Compare canonical paths since
	// resolution.ProjectRoot is symlink-resolved.
	if canonical(string(currentRoot)) == string(resolution.ProjectRoot) {
		return key
	}

	branch, err := titler.git.CurrentBranch(ctx, git.Directory(cwd))
	if err != nil || branch == "" {
		return ""
	}
	// A pull-request worktree checks out a branch named "zwm/pr-<n>-<hash>", but
	// its tab is titled "<key>:pr-<n>" (zwm wpr). Recover the number from the
	// branch so PR agents land on their tab too.
	if suffix, ok := managedPRSuffix(string(branch)); ok {
		return key + ":" + suffix
	}
	return key + ":" + string(branch)
}

// managedPRBranch matches the branch name zwm gives a pull-request worktree,
// capturing the PR number (see app.pullRequestBranch).
var managedPRBranch = regexp.MustCompile(`^zwm/pr-(\d+)-`)

// managedPRSuffix maps a pull-request worktree branch to the "pr-<n>" tab-title
// suffix zwm wpr uses; the second result is false for ordinary branches.
func managedPRSuffix(branch string) (string, bool) {
	if match := managedPRBranch.FindStringSubmatch(branch); match != nil {
		return "pr-" + match[1], true
	}
	return "", false
}

func canonical(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
