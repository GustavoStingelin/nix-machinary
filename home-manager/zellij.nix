{ config, pkgs, ... }:
let
  home = config.home.homeDirectory;
  sessionizer = pkgs.fetchurl {
    url = "https://github.com/laperlej/zellij-sessionizer/releases/download/v0.5.0/zellij-sessionizer.wasm";
    hash = "sha256-xBhBwCPnToH5mg/Y2V4FBO0gLfLNuSYE31HJ5OoLoFs=";
  };
in
{
  programs.zellij.enable = true;

  xdg.configFile."zellij/config.kdl".text = ''
    plugins {
      zellij-sessionizer location="file:${sessionizer}" {
        cwd "/"
        root_dirs "${home}/code"
        session_layout ":default"
      }
    }

    keybinds {
      tmux {
        bind "g" {
          LaunchOrFocusPlugin "zellij-sessionizer" {
            floating true
            move_to_focused_tab true
          }
          SwitchToMode "Locked"
        }
      }
    }
  '';
}
