package git

import "context"

func (client Client) AddExistingWorktree(ctx context.Context, directory Directory, path WorktreePath, branch Branch) error {
	_, err := client.run(ctx, directory, "worktree", "add", string(path), string(branch))
	return err
}

func (client Client) AddNewWorktree(ctx context.Context, directory Directory, branch Branch, path WorktreePath, commit Commit) error {
	_, err := client.run(ctx, directory, "worktree", "add", "-b", string(branch), string(path), string(commit))
	return err
}
