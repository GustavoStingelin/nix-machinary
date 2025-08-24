{ pkgs, ... }:

{
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
}