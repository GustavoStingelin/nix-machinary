# Bash Oracle Coverage Map

| Former Bash group | Real-binary testscript coverage | Real-Git package coverage |
| --- | --- | --- |
| parser | `grammar-preflight.txt` checks help, grammar, errors, all preflight modes, and no external calls | `internal/cli` parser tests |
| project-resolution | `project-newline.txt`, `project-rejection.txt`, and `project-nonworktree.txt` drive newline and rejected paths through the binary | `internal/project` resolution/rejection tests |
| validation | `invalid-branch.txt`, `existing-focus.txt`, `new-worktree.txt`, `add-failure.txt`, and `pull-request.txt` inspect exact structured process records | `internal/git` and `internal/worktree` parsing/validation tests |
| wco-existing | `existing-focus.txt`, `real-git.txt` | `internal/app` existing-branch real-Git tests |
| wco-new | `new-worktree.txt`, `new-collision.txt` | `internal/app` new-branch real-Git tests |
| o | `open-project.txt`, `open-project-output.txt`, and `open-project-rejection.txt` cover canonical root create/focus, path forms, newline argv, non-mutation snapshots, and strict rejection | `internal/command` and `internal/project` open-project tests |
| wpr | `pull-request.txt` covers create, reuse, malformed metadata, and recovery | `internal/app` and `internal/github` PR real-Git tests |
| zellij | all successful scripts assert create/focus via exact JSON command records | `internal/zellij` argv, preflight, and launcher tests |

`assert-log` decodes JSON Lines and compares sequence, executable, exact argv array, and cwd. It also rejects every prohibited Git and Zellij operation structurally; no substring command assertions are used.
