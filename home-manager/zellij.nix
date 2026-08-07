{ config, pkgs, ... }:
let
  home = config.home.homeDirectory;
  zjstatus = pkgs.fetchurl {
    url = "https://github.com/dj95/zjstatus/releases/download/v0.23.0/zjstatus.wasm";
    hash = "sha256-4AaQEiNSQjnbYYAh5MxdF/gtxL+uVDKJW6QfA/E4Yf8=";
  };
  sessionizer = pkgs.fetchurl {
    url = "https://github.com/laperlej/zellij-sessionizer/releases/download/v0.5.0/zellij-sessionizer.wasm";
    hash = "sha256-xBhBwCPnToH5mg/Y2V4FBO0gLfLNuSYE31HJ5OoLoFs=";
  };
  # Vendored prebuilt WASM for our attention-indicator plugin. Source lives in
  # zellij-plugins/zwm-attn/; rebuild with `just build-zwm-attn` after changes.
  # (A Nix cross-build via pkgsCross.wasi32 requires compiling LLVM+rustc from
  # source, which is impractical here, so the artifact is vendored like the
  # upstream plugins above.)
  zwm-attn = ../zellij-plugins/zwm-attn/dist/zwm-attn.wasm;
  mkTabTemplate = { showPwd ? true }: ''
    default_tab_template {
      pane size=1 borderless=true {
        plugin location="file:${zjstatus}" {
          format_left "{mode}#[fg=#1E1E2E,bg=#CBA6F7,bold] {session} #[fg=#CBA6F7,bg=#1E1E2E]{tabs}"
          format_center ""
          format_right "${if showPwd then "{command_pwd}" else ""}{command_battery}#[fg=#1E1E2E,bg=#F9E2AF]{datetime} "
          format_space "#[bg=#1E1E2E]"
          hide_frame_for_single_pane "false"

          mode_normal "#[fg=#1E1E2E,bg=#89B4FA,bold] {name} #[fg=#89B4FA,bg=#1E1E2E]"
          mode_tmux "#[fg=#1E1E2E,bg=#A6E3A1,bold] {name} #[fg=#A6E3A1,bg=#1E1E2E]"
          mode_locked "#[fg=#1E1E2E,bg=#F9E2AF,bold] {name} #[fg=#F9E2AF,bg=#1E1E2E]"

          tab_normal "#[fg=#1E1E2E,bg=#313244] #[fg=#CDD6F4,bg=#313244] {index} {name} #[fg=#313244,bg=#1E1E2E]"
          tab_normal_fullscreen "#[fg=#1E1E2E,bg=#45475A] #[fg=#CDD6F4,bg=#45475A] {index} {name} #[fg=#45475A,bg=#1E1E2E]"
          tab_normal_sync "#[fg=#1E1E2E,bg=#6C7086] #[fg=#CDD6F4,bg=#6C7086] {index} {name} #[fg=#6C7086,bg=#1E1E2E]"
          tab_active "#[fg=#1E1E2E,bg=#89B4FA,bold,italic]{index} {name} #[fg=#89B4FA,bg=#1E1E2E]"
          tab_active_fullscreen "#[fg=#1E1E2E,bg=#F9E2AF,bold,italic]{index} {name} #[fg=#F9E2AF,bg=#1E1E2E]"
          tab_active_sync "#[fg=#1E1E2E,bg=#A6E3A1,bold,italic]{index} {name} #[fg=#A6E3A1,bg=#1E1E2E]"

          command_pwd_command "pwd"
          command_pwd_format "#[fg=#89B4FA] {stdout} "
          command_pwd_interval "10"

          command_battery_command "sh -c 'if command -v pmset >/dev/null 2>&1; then pmset -g batt | awk \"/%/ {sub(/^.*\\t/, \\\"\\\"); sub(/;.*$/, \\\"\\\"); print}\"; fi'"
          command_battery_format "#[fg=#FAB387] {stdout} "
          command_battery_interval "300"
          command_battery_rendermode "static"
          command_battery_hideonemptystdout "true"

          datetime_timezone "America/Sao_Paulo"
          datetime "#[fg=#F9E2AF] {format} "
          datetime_format "%a %d %b %H:%M"
        }
      }

      children
    }
  '';
  layoutText = ''
    layout {
      ${mkTabTemplate {}}

      swap_tiled_layout name="vertical" {
        tab split_direction="vertical" {
          pane
          pane
        }
      }

      swap_tiled_layout name="horizontal" {
        tab split_direction="horizontal" {
          pane
          pane
        }
      }
    }
  '';
in
{
  # The package is deliberately left at the default: overlay-zellij in flake.nix
  # points pkgs.zellij at unstable, so this and the zellij on zwm's wrapped PATH
  # are the same build. Setting it here would reintroduce the skew it prevents.
  programs.zellij.enable = true;

  xdg.configFile."zellij/config.kdl".text = ''
    pane_frames true

    plugins {
      zellij-sessionizer location="file:${sessionizer}" {
        cwd "/"
        root_dirs "${home}/code"
        session_layout "${home}/.config/zellij/layouts/sessionizer.kdl"
      }
      zwm-attn location="file:${zwm-attn}"
    }

    // Load one headless zwm-attn instance per session so it can mark any tab
    // with an attention glyph when an agent finishes / needs input.
    load_plugins {
      zwm-attn
    }

    keybinds {
      // Ctrl+y opens the zwm session dashboard in a floating pane. It closes on
      // exit, so after you jump to a tab (or quit) the overlay disappears. Bound
      // in shared_except "locked" so it works from normal mode without a prefix;
      // normal mode leaves j/k/enter/q for the TUI. Note: Ctrl+m is byte-identical
      // to Enter, which is why the chord is Ctrl+y.
      shared_except "locked" {
        bind "Ctrl y" {
          Run "zwm" "tui" {
            floating true
            close_on_exit true
            name "zwm-tui"
          }
        }
      }
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

  xdg.configFile."zellij/layouts/default.kdl".text = ''
    ${layoutText}
  '';

  xdg.configFile."zellij/layouts/sessionizer.kdl".text = ''
    ${layoutText}
  '';

  xdg.configFile."zellij/layouts/worktree.kdl".text = ''
    layout {
      ${mkTabTemplate { showPwd = false; }}
      tab split_direction="vertical" {
        pane
        pane split_direction="horizontal" {
          pane
          pane
        }
      }
    }
  '';
}
