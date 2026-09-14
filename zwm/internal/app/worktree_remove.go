package app

import (
	"context"
	"fmt"

	"github.com/GustavoStingelin/nix-machinary/zwm/internal/errs"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/git"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/project"
	"github.com/GustavoStingelin/nix-machinary/zwm/internal/worktree"
)

// RemoveWorktreeInput names one managed worktree to delete.
//
// DeleteBranch is a separate opt-in because the two losses are not the same
// kind of loss: the worktree is a checkout Git can recreate from the branch,
// while the branch may be the only place its commits still exist.
type RemoveWorktreeInput struct {
	Project      project.Resolution
	Worktree     worktree.Path
	DeleteBranch bool
}

// RemoveWorktreeResult reports what was deleted. BranchDeleted is false whenever
// the branch was kept, so the caller can say so rather than assume.
type RemoveWorktreeResult struct {
	Worktree      worktree.Path
	Branch        git.Branch
	BranchDeleted bool
}

// RemoveWorktree deletes one of a project's worktrees, and optionally the branch
// it was checked out on. The worktree need not be one zwm created: the ones that
// most need pruning are often the ones made by hand.
//
// The target is verified against `git worktree list` rather than trusted from
// the caller: it must be a live branch checkout Git has registered for this
// project, and it must not be the primary worktree — deleting that would take
// the repository with it.
func (service BranchService) RemoveWorktree(ctx context.Context, input RemoveWorktreeInput) (RemoveWorktreeResult, error) {
	branch, err := service.removableBranch(ctx, input.Project, input.Worktree)
	if err != nil {
		return RemoveWorktreeResult{}, err
	}
	if err := service.git.RemoveWorktree(ctx, git.Directory(input.Project.ProjectRoot), git.WorktreePath(input.Worktree)); err != nil {
		return RemoveWorktreeResult{}, errs.Wrap(errs.External, "remove managed worktree", err)
	}
	result := RemoveWorktreeResult{Worktree: input.Worktree, Branch: branch}
	if !input.DeleteBranch {
		return result, nil
	}
	if err := service.git.DeleteBranch(ctx, git.Directory(input.Project.ProjectRoot), branch); err != nil {
		// The worktree is already gone, so this is reported with the removal
		// treated as done: retrying would only fail on a worktree that no longer
		// exists.
		return result, errs.Wrap(errs.External, fmt.Sprintf("delete branch '%s'", branch), err)
	}
	result.BranchDeleted = true
	return result, nil
}

// removableBranch authorizes the target and returns the branch it has checked
// out.
func (service BranchService) removableBranch(ctx context.Context, resolution project.Resolution, path worktree.Path) (git.Branch, error) {
	raw, err := service.git.ListWorktrees(ctx, git.Directory(resolution.ProjectRoot))
	if err != nil {
		return "", errs.Wrap(errs.External, "list Git worktrees", err)
	}
	records, err := worktree.ParsePorcelainZ(raw)
	if err != nil {
		return "", errs.Wrap(errs.External, "parse Git worktrees", err)
	}
	for index, record := range records {
		if record.Path != path {
			continue
		}
		// Git lists the primary worktree first, and the porcelain output carries no
		// other marker for it.
		if index == 0 {
			return "", errs.New(errs.Usage, fmt.Sprintf("'%s' is the project's primary worktree", path))
		}
		if record.State != worktree.HeadBranch {
			return "", errs.New(errs.Usage, fmt.Sprintf("'%s' is not a branch checkout", path))
		}
		branch, ok := worktree.LocalBranch(record.Branch)
		if !ok {
			return "", errs.New(errs.Usage, fmt.Sprintf("'%s' has no local branch", path))
		}
		return git.Branch(branch), nil
	}
	return "", errs.New(errs.Usage, fmt.Sprintf("'%s' is not a registered worktree of this project", path))
}
