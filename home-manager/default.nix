{ config, pkgs, lib, ... }:

let
  home-manager = builtins.fetchTarball "https://github.com/nix-community/home-manager/archive/master.tar.gz";
in
{
  imports = [
    (import "${home-manager}/nixos")
  ];

  home-manager.useGlobalPkgs = true;
  home-manager.useUserPackages = true;
  home-manager.backupFileExtension = "backup";

  # Enable zsh system-wide (required when setting as user shell)
  programs.zsh.enable = true;

  # Set zsh as default shell for user
  users.users.head.shell = pkgs.zsh;

  home-manager.users.head = { pkgs, ... }: {
    home.stateVersion = "25.05";
    home.enableNixpkgsReleaseCheck = false;

    imports = [
      ./zed.nix
      ./zsh.nix
      ./git.nix
      ./helix.nix
      ./alacritty.nix
      ./lsp.nix
    ];
  };
}
