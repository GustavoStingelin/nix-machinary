{
  description = "Multi-system Nix configuration";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.05";
    nixpkgs-unstable.url = "github:nixos/nixpkgs/nixpkgs-unstable";

    home-manager = {
      url = "github:nix-community/home-manager/release-25.05";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    disko = {
      url = "github:nix-community/disko";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    catppuccin = {
      url = "github:catppuccin/nix/release-25.05";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    flake-utils.url = "github:numtide/flake-utils";

    nix-darwin = {
      url = "github:nix-darwin/nix-darwin/nix-darwin-25.05";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, home-manager, disko, catppuccin, flake-utils, nix-darwin, ... }:
    let
      linuxSystem = "x86_64-linux";
      overlay-unstable = final: prev:
        let
          unstable = import nixpkgs-unstable {
          system = prev.system;
          config = prev.config;
          };
        in
        {
          inherit unstable;
          sparrow = unstable.sparrow;
        };
      overlay-zwm = final: prev: {
        zwm = final.callPackage ./packages/zwm.nix { };
      };
      overlays = [ overlay-unstable overlay-zwm ];
      linuxPkgs = import nixpkgs {
        system = linuxSystem;
        inherit overlays;
      };

      homeImports = [
        catppuccin.homeModules.catppuccin
        ./home-manager/catppuccin.nix
        ./home-manager/zed.nix
        ./home-manager/zsh.nix
        ./home-manager/git.nix
        ./home-manager/helix.nix
        ./home-manager/alacritty.nix
        ./home-manager/ghostty.nix
        ./home-manager/neovim.nix
        ./home-manager/lsp.nix
        ./home-manager/atuin.nix
        ./home-manager/gpg.nix
        ./home-manager/gh.nix
        ./home-manager/clis.nix
        ./home-manager/tuis.nix
        ./home-manager/zellij.nix
        ./home-manager/zwm.nix
        ./home-manager/agents.nix
        ./home-manager/ruby.nix
      ];

      # Common configuration shared between systems
      commonModules = [
        ./lib
        home-manager.nixosModules.home-manager
        {
          nixpkgs.overlays = overlays;
          home-manager.useGlobalPkgs = true;
          home-manager.useUserPackages = true;
          home-manager.backupFileExtension = "backup";

          # Enable zsh system-wide
          programs.zsh.enable = true;

          # Set zsh as default shell for user
          users.users.head.shell = linuxPkgs.zsh;

          home-manager.users.head = {
            home.stateVersion = "25.05";
            home.enableNixpkgsReleaseCheck = false;

            imports = homeImports;
          };
        }
      ];
    in
    {
      # NixOS configurations
      nixosConfigurations = {
        # Dell notebook - hostname: reapermobile
        reapermobile = nixpkgs.lib.nixosSystem {
          system = linuxSystem;
          modules = commonModules ++ [
            disko.nixosModules.disko
            ./hosts/reapermobile
          ];
        };

        # Future NixOS systems can be added here
        # Example: Desktop with NixOS - hostname: reaper
        # reaper = nixpkgs.lib.nixosSystem {
        #   inherit system;
        #   modules = commonModules ++ [
        #     disko.nixosModules.disko
        #     ./hosts/reaper
        #   ];
        # };
      };

      # Home Manager configurations for Ubuntu/non-NixOS systems
      homeConfigurations = {
        # Desktop with Ubuntu - hostname: reaper
        reaper = home-manager.lib.homeManagerConfiguration {
          pkgs = linuxPkgs;
          modules = [
            {
              home.username = "head";
              home.homeDirectory = "/home/head";
              home.stateVersion = "25.05";
              home.enableNixpkgsReleaseCheck = false;

              imports = homeImports;
            }
          ];
        };
      };

      # nix-darwin (macOS) configurations
      darwinConfigurations = {
        # Apple Silicon Mac (adjust if you use x86_64-darwin)
        reapermac = nix-darwin.lib.darwinSystem {
          system = "aarch64-darwin";
          specialArgs = { inherit home-manager homeImports; };
          modules = [
            { nixpkgs.overlays = overlays; }
            ./hosts/reapermac
          ];
        };
      };
    } // flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system overlays;
        };
      in
      {
      # Development shell
      devShells.default = pkgs.mkShell {
        buildInputs = [
          pkgs.go_1_24
          pkgs.git
          pkgs.nix
          home-manager.packages.${system}.home-manager
        ];

        shellHook = ''
          unset GOROOT
          unset GOBIN
          export GOCACHE="''${XDG_CACHE_HOME:-$HOME/.cache}/nix-machinary/go-build-1.24.10"
          echo "Nix development environment loaded"
          echo "Available commands:"
          echo "  # NixOS systems:"
          echo "  nixos-rebuild switch --flake .#reapermobile  # Dell notebook"
          echo "  # nixos-rebuild switch --flake .#reaper      # Desktop (when installed)"
          echo ""
          echo "  # Home Manager (Ubuntu/non-NixOS):"
          echo "  home-manager switch --flake .#reaper         # Desktop Ubuntu"
        '';
      };

      packages.zwm = pkgs.zwm;

      # Checks to validate evaluation/build of configurations
      checks = (nixpkgs.lib.optionalAttrs (system == "x86_64-linux") {
        # Ensure the NixOS host evaluates and can build the system closure
        nixos-reapermobile = self.nixosConfigurations.reapermobile.config.system.build.toplevel;

        # Ensure Home Manager configs evaluate and build activation packages
        hm-reaper = self.homeConfigurations.reaper.activationPackage;

        # Evaluate macOS config on Linux without building macOS derivations
        darwin-reapermac-eval = let
          dcfg = self.darwinConfigurations.reapermac;
        in builtins.seq dcfg.config.system.build.toplevel.drvPath (pkgs.writeText "darwin-reapermac-eval-ok" "ok");
      }) // {
        zwm-all = pkgs.runCommand "zwm-all" {
          nativeBuildInputs = [ pkgs.zwm ];
        } ''
          workdir="$(mktemp -d)"
          trap 'rm -rf "$workdir"' EXIT

          zwm --help >"$workdir/help"
          grep -Fqx '   wco  check out a branch in a worktree' "$workdir/help"
          grep -Fqx '   o    open a project' "$workdir/help"
          grep -Fqx '   wpr  check out a pull request in a worktree' "$workdir/help"
          grep -Fqx '   --project string, -C string  select a project before the subcommand' "$workdir/help"
          if grep -Eq '^   co([[:space:]]|$)' "$workdir/help"; then
            echo "zwm help advertises removed co command" >&2
            exit 1
          fi

          assert_usage() {
            expected="$1"
            shift
            if zwm "$@" >"$workdir/stdout" 2>"$workdir/stderr"; then
              echo "zwm $* unexpectedly succeeded" >&2
              exit 1
            else
              exit_code=$?
            fi
            test "$exit_code" -eq 64
            test ! -s "$workdir/stdout"
            printf 'zwm: usage: %s\n' "$expected" >"$workdir/expected"
            cmp "$workdir/expected" "$workdir/stderr"
          }

          assert_usage "unknown subcommand 'co'" co topic
          assert_usage "o requires a project name or path" o
          assert_usage "o accepts exactly one project name or path" o repo extra
          assert_usage "o does not accept -C/--project" -C selected o repo
          assert_usage "o does not accept -C/--project" o repo -C selected

          # Shell completion emits a sourceable script and advertises subcommands.
          zwm completion zsh >"$workdir/completion"
          grep -Fqx '#compdef zwm' "$workdir/completion"
          zwm --generate-shell-completion >"$workdir/root-complete"
          grep -Fqx 'wco:check out a branch in a worktree' "$workdir/root-complete"

          touch "$out"
        '';
      };
    });
}
