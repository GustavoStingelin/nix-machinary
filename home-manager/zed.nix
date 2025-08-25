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
      "rust"
      "go"
      "python"
      "typescript"
      "html"
      "css"
      "json"
      "yaml"
      "toml"
      "lua"
      "dockerfile"
      "terraform"
      "markdown"
      "bash"
      "sql"
      "golangci-lint"
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

      lsp = {
        typos = {
          binary = {
            path = "${pkgs.typos-lsp}/bin/typos-lsp";
          };
        };
      };
    };
  };
}
