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

    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, home-manager, disko, flake-utils, ... }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};

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

            imports = [
              ./home-manager/zed.nix
              ./home-manager/zsh.nix
              ./home-manager/git.nix
              ./home-manager/helix.nix
              ./home-manager/ghostty.nix
              ./home-manager/lsp.nix
              ./home-manager/atuin.nix
              ./home-manager/gpg.nix
            ];
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
            ./disko.nix
            {
              networking.hostName = "reapermobile";
              system.stateVersion = "25.05";
            }
          ];
        };

        # Future NixOS systems can be added here
        # Example: Desktop with NixOS - hostname: reaper
        # reaper = nixpkgs.lib.nixosSystem {
        #   inherit system;
        #   modules = commonModules ++ [
        #     disko.nixosModules.disko
        #     ./disko.nix
        #     {
        #       networking.hostName = "reaper";
        #       system.stateVersion = "25.05";
        #     }
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

              imports = [
                ./home-manager/zed.nix
                ./home-manager/zsh.nix
                ./home-manager/git.nix
                ./home-manager/helix.nix
                ./home-manager/ghostty.nix
                ./home-manager/lsp.nix
                ./home-manager/atuin.nix
                ./home-manager/gpg.nix
                ./home-manager/tuis.nix
                ./home-manager/ruby.nix
              ];
            }
          ];
        };

        # Generic home-manager config for other systems
        head = home-manager.lib.homeManagerConfiguration {
          inherit pkgs;
          modules = [
            {
              home.username = "head";
              home.homeDirectory = "/home/head";
              home.stateVersion = "25.05";
              home.enableNixpkgsReleaseCheck = false;

              imports = [
                ./home-manager/zed.nix
                ./home-manager/zsh.nix
                ./home-manager/git.nix
                ./home-manager/helix.nix
                ./home-manager/ghostty.nix
                ./home-manager/lsp.nix
                ./home-manager/atuin.nix
                ./home-manager/gpg.nix
                ./home-manager/tuis.nix
                ./home-manager/ruby.nix
              ];
            }
          ];
        };
      };
    } // flake-utils.lib.eachDefaultSystem (system: {
      # Development shell
      devShells.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          nixos-rebuild
          home-manager
          git
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
          echo "  home-manager switch --flake .#head           # Generic config"
        '';
      };
    });
}
