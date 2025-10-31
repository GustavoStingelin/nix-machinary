{
  description = "Multi-system Nix configuration";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.05";

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
      url = "github:nix-darwin/nix-darwin";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, home-manager, disko, catppuccin, flake-utils, nix-darwin, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};

      homeImports = [
        catppuccin.homeModules.catppuccin
        ./home-manager/catppuccin.nix
        ./home-manager/zed.nix
        ./home-manager/zsh.nix
        ./home-manager/git.nix
        ./home-manager/helix.nix
        ./home-manager/ghostty.nix
        ./home-manager/lsp.nix
        ./home-manager/atuin.nix
        ./home-manager/gpg.nix
        ./home-manager/gh.nix
        ./home-manager/clis.nix
        ./home-manager/tuis.nix
        ./home-manager/zellij.nix
        ./home-manager/ruby.nix
      ];

      # Common configuration shared between systems
      commonModules = [
        ./lib
        home-manager.nixosModules.home-manager
        {
          home-manager.useGlobalPkgs = true;
          home-manager.useUserPackages = true;
          home-manager.backupFileExtension = "backup";

          # Enable zsh system-wide
          programs.zsh.enable = true;

          # Set zsh as default shell for user
          users.users.head.shell = pkgs.zsh;

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
          inherit system;
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
          inherit pkgs;
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
            ./hosts/reapermac
          ];
        };
      };
    } // flake-utils.lib.eachDefaultSystem (system: {
      # Development shell
      devShells.default = pkgs.mkShell {
        buildInputs = [
          pkgs.git
          pkgs.nix
          home-manager.packages.${system}.home-manager
        ];

        shellHook = ''
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

      # Checks to validate evaluation/build of configurations
      checks = {
        # Ensure the NixOS host evaluates and can build the system closure
        nixos-reapermobile = self.nixosConfigurations.reapermobile.config.system.build.toplevel;

        # Ensure Home Manager configs evaluate and build activation packages
        hm-reaper = self.homeConfigurations.reaper.activationPackage;

        # Evaluate macOS config on Linux without building macOS derivations
        darwin-reapermac-eval = let
          dcfg = self.darwinConfigurations.reapermac;
          _ = builtins.deepSeq dcfg.config.system.build.toplevel true;
        in pkgs.writeText "darwin-reapermac-eval-ok" "ok";
      };
    });
}
