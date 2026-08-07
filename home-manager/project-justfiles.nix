{ ... }:
# Personal `just` recipes for projects that don't use just themselves.
#
# `.justfile` is in the global gitignore (see git.nix), so these never show up
# in the project's `git status` — and because the source of truth is this repo,
# they survive a reinstall or a wiped checkout.
let
  btcwallet = ./project-justfiles/btcwallet.justfile;
in
{
  home.file = {
    # The main checkout.
    "code/btcwallet/.justfile".source = btcwallet;

    # Every btcwallet worktree at once: zwm parks them under
    # ~/code/.wt/btcwallet/<branch>, and `just` walks up from the invocation
    # directory until it finds a justfile, so one link at the parent covers
    # worktrees that don't exist yet.
    "code/.wt/btcwallet/.justfile".source = btcwallet;
  };
}
