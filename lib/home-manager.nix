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
  
  home-manager.users.head = { pkgs, ... }: {
    home.stateVersion = "25.05";
    
    programs.zed-editor = {
      enable = true;
      extensions = [
        "nix"
        "catppuccin"
        "catppuccin-icons"
        "git-firefly"
        "typst"
        "zig"
        "just"
        "typos"
      ];
      
      userSettings = {
        telemetry = {
          metrics = false;
          diagnostics = false;
        };
        
        terminal = {
          shell = {
            program = "zsh";
          };
        };
        
        theme = {
          mode = "system";
          light = "Catppuccin Latte";
          dark = "Catppuccin Mocha";
        };
        
        icon_theme = "Catppuccin Mocha";
        
        features = {
          edit_prediction_provider = "copilot";
        };
      };
    };
  };
}