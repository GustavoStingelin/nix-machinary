{ config, pkgs, ... }:
let
  home = config.home.homeDirectory;
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
  zwm-bar = ../zellij-plugins/zwm-bar/dist/zwm-bar.wasm;
  # The status bar is our own plugin (zellij-plugins/zwm-bar/) rather than
  # zjstatus, which subscribes to SessionUpdate — an event Zellij 0.44 emits every
  # second carrying every live session's full tab and pane manifest. One bar
  # instance per tab meant nine instances decoding that payload and repainting
  # every second whether or not anything had changed: a sampled nine-tab session
  # with no user input spent 94s of wasm CPU per 150s of wall clock, all under
  # apply_event_to_plugin, and spiked past 500% when a burst of events reached
  # every instance at once. zwm-bar takes ModeUpdate (which carries the session
  # name too) and TabUpdate, and nothing else, so an idle session is idle.
  #
  # It also needs no configuration: the colours, separators and spacing that used
  # to live in the format strings here now live in the plugin, ported from them.
  tabTemplate = ''
    default_tab_template {
      pane size=1 borderless=true {
        plugin location="file:${zwm-bar}"
      }

      children
    }
  '';
  layoutText = ''
    layout {
      ${tabTemplate}

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
      ${tabTemplate}
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
