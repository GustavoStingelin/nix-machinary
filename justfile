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

# Check flake
check: zwm-check
    nix flake check

collect-garbage:
    nix-collect-garbage -d
