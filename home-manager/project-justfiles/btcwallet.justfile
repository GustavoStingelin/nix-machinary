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
test mode="local":
    #!/usr/bin/env sh
    set -eu
    root="$(git rev-parse --show-toplevel)"
    cd "$root"
    case "{{ mode }}" in
      local)
        fmt_lint='export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
    export GOFLAGS="-buildvcs=false"
    tool() { GOTOOLCHAIN=go1.25.11 go -C tools tool -n "$1"; }
    gosimports="$(tool gosimports)"
    golangci_lint="$(tool golangci-lint)"
    gofmt="$(GOTOOLCHAIN=go1.25.11 go env GOROOT)/bin/gofmt"
    gofiles="$(find . -type f -name '"'"'*.go'"'"' -not -name '"'"'*.pb.go'"'"')"
    export GOTOOLCHAIN=go1.25.11
    export GOOS=linux
    "$gosimports" -w $gofiles
    "$gofmt" -l -w -s $gofiles
    "$golangci_lint" config verify -v --config .golangci.yml
    "$golangci_lint" run -v --config .golangci.yml --concurrency=4 --fix
    "$golangci_lint" run -v --config .golangci.yml --concurrency=4 --fix --build-tags="itest dev nolog" ./itest
    "$golangci_lint" run -v --config .golangci.yml --concurrency=4 --fix --build-tags="itest dev debug stdlog" ./wallet/internal/db/sqlite ./wallet/internal/db/itest
    "$golangci_lint" run -v --config .golangci.yml --concurrency=4 --fix --build-tags="itest dev debug stdlog test_db_postgres" ./wallet/internal/db/pg ./wallet/internal/db/itest'
        ;;
      full)
        fmt_lint='make fmt && make lint'
        ;;
      *)
        echo 'usage: just test [local|full]' >&2
        exit 2
        ;;
    esac
    zellij run --cwd "$root" --name fmt-lint --block-until-exit-success -- sh -c "$fmt_lint && zellij action close-pane --pane-id \"\$ZELLIJ_PANE_ID\"" &
    zellij run --cwd "$root" --name unit --block-until-exit-success -- sh -c 'make unit && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"' &
    zellij run --cwd "$root" --name itest --block-until-exit-success -- sh -c 'make itest db=sqlite && make itest db=kvdb && make itest db=postgres chain=bitcoind && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"' &
    zellij run --cwd "$root" --name itest-db --block-until-exit-success -- sh -c 'make itest-db db=sqlite cover=1 && make itest-db db=postgres cover=1 && zellij action close-pane --pane-id "$ZELLIJ_PANE_ID"'
