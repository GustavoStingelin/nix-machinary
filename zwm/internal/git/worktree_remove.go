package git

import "context"

// RemoveWorktree unregisters a worktree and deletes its directory. It is
// deliberately unforced: Git refuses a worktree holding uncommitted or untracked
// changes, and that refusal is the only thing standing between this and work
// that was never committed anywhere else.
func (client Client) RemoveWorktree(ctx context.Context, directory Directory, path WorktreePath) error {
	_, err := client.run(ctx, directory, "worktree", "remove", "--", string(path))
	return err
}

// DeleteBranch deletes a local branch even when it is unmerged. The unmerged
// case is the normal one here — a `zwm/pr-<n>-<hash>` branch never merges
// anywhere locally — so refusing it would make the option useless; the caller is
// responsible for taking an explicit opt-in first.
func (client Client) DeleteBranch(ctx context.Context, directory Directory, branch Branch) error {
	_, err := client.run(ctx, directory, "branch", "-D", "--", string(branch))
	return err
}
