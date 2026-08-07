# btcwallet recipes, kept out of the btcwallet repo.
#
# This file lives in nix-machinary (home-manager/project-justfiles/) and is
# linked into ~/code/btcwallet and ~/code/.wt/btcwallet by
# home-manager/project-justfiles.nix. Edit it there and re-switch — the linked
# copies are read-only nix store paths.

# [no-cd] keeps the invocation directory: the link that covers the worktrees
# sits at ~/code/.wt/btcwallet, which is not itself a checkout, so the recipe
# has to resolve the repo root from where it was called.

# Run every suite in its own zellij pane; each pane closes itself once green.
[no-cd]
test:
    #!/usr/bin/env sh
    set -eu
    root="$(git rev-parse --show-toplevel)"
    cd "$root"
    zellij run --cwd "$root" --name fmt-lint --block-until-exit-success -- sh -c 'make fmt && make lint && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"' &
    zellij run --cwd "$root" --name unit --block-until-exit-success -- sh -c 'make unit && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"' &
    zellij run --cwd "$root" --name itest --block-until-exit-success -- sh -c 'make itest db=sqlite && make itest db=kvdb && make itest db=postgres chain=bitcoind && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"' &
    zellij run --cwd "$root" --name itest-db --block-until-exit-success -- sh -c 'make itest-db db=sqlite cover=1 && make itest-db db=postgres cover=1 && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"'
