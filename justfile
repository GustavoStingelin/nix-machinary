# Multi-system Nix configuration

# Default recipe - show available commands
default:
    @just --list

# NixOS System Commands
# Apply NixOS configuration
nixos-switch system:
    sudo nixos-rebuild switch --flake .#{{system}}

# Test NixOS configuration without switching
nixos-test system:
    sudo nixos-rebuild test --flake .#{{system}}

# Build NixOS configuration
nixos-build system:
    sudo nixos-rebuild build --flake .#{{system}}


# Ubuntu/Non-NixOS System Commands
# Apply home-manager configuration
home-switch system:
    home-manager switch --flake .#{{system}}

# Build home-manager configuration
home-build system:
    home-manager build --flake .#{{system}}

## DarwinNix MacOS System Commands
# Apply darwin configuration
darwin-switch system:
    sudo darwin-rebuild switch --flake .#{{system}}

# System Info
# Show current system hostname and available configurations
hostname:
    @echo "Current hostname: $(hostname)"
    @echo ""
    @echo "Available configurations:"
    @echo "  reapermobile - Dell notebook (NixOS)"
    @echo "  reaper       - Desktop (Ubuntu/NixOS)"
    @echo "  reapermac    - MacBook (macOS)"
    @echo ""
    @echo "Usage examples:"
    @echo "  just nixos-switch reapermobile"
    @echo "  just home-switch reaper"


# Go verification
zwm-format:
    GOTOOLCHAIN=go1.24.10 go -C zwm fmt ./...

zwm-test:
    GOTOOLCHAIN=go1.24.10 go -C zwm test ./...

zwm-race:
    GOTOOLCHAIN=go1.24.10 go -C zwm test -race ./...

zwm-vet:
    GOTOOLCHAIN=go1.24.10 go -C zwm vet ./...

zwm-mock:
    GOTOOLCHAIN=go1.24.10 go -C zwm tool mockery --config .mockery.yml

zwm-check: zwm-format zwm-test zwm-race zwm-vet zwm-mock
    GIT_MASTER=1 git diff --exit-code -- zwm/internal/mocks

# Rebuild the vendored zwm-attn Zellij plugin WASM (needs the wasm32-wasip1
# rust target: `rustup target add wasm32-wasip1`). Run after editing the plugin
# source, then commit dist/zwm-attn.wasm.
build-zwm-attn:
    cd zellij-plugins/zwm-attn && cargo build --release --target wasm32-wasip1
    cp zellij-plugins/zwm-attn/target/wasm32-wasip1/release/zwm-attn.wasm zellij-plugins/zwm-attn/dist/zwm-attn.wasm
    @echo "Vendored zellij-plugins/zwm-attn/dist/zwm-attn.wasm"

# Rebuild the vendored zwm-bar status-bar WASM, same requirements as
# build-zwm-attn. Run after editing the plugin, then commit dist/zwm-bar.wasm.
# Note that the store path changes with the artifact, and Zellij keys plugin
# permissions by path — so a rebuilt bar asks for its permission again.
build-zwm-bar:
    cd zellij-plugins/zwm-bar && cargo build --release --target wasm32-wasip1
    cp zellij-plugins/zwm-bar/target/wasm32-wasip1/release/zwm-bar.wasm zellij-plugins/zwm-bar/dist/zwm-bar.wasm
    @echo "Vendored zellij-plugins/zwm-bar/dist/zwm-bar.wasm"

# Unit-test zwm-bar's rendering. The crate targets wasm32-wasip1, where a test
# binary cannot run, so the pure render module is tested against the host.
test-zwm-bar:
    cd zellij-plugins/zwm-bar && cargo test --lib --target "$(rustc -vV | awk '/^host:/{print $2}')"

# Check flake
check: zwm-check test-zwm-bar
    nix flake check

collect-garbage:
    nix-collect-garbage -d
