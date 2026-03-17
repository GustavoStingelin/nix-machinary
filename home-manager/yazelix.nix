{ lib, ... }:
{
  # yazelix sets startupWMClass under xdg.desktopEntries, but this option is
  # unavailable in home-manager/release-25.05. macOS does not need desktop entries.
  xdg.desktopEntries = lib.mkForce {};

  programs.yazelix = {
    enable = true;

    # Use zsh as the default shell inside Zellij sessions
    default_shell = "zsh";

    # Ghostty is already installed via homebrew
    manage_terminals = true;
    terminals = [ "ghostty" ];
    terminal_config_mode = "user";

    # Use Helix as the editor - consistent with EDITOR=hx in zsh.nix
    editor_command = "hx";

    # Dependency management
    recommended_deps = true;
    yazi_extensions = true;
    yazi_media = false;

    # Theme
    zellij_theme = "catppuccin-macchiato";
    yazi_theme = "catppuccin-mocha";

    # Yazi plugins
    yazi_plugins = [ "git" ];

    # UX
    disable_zellij_tips = true;
    zellij_rounded_corners = true;
    enable_sidebar = true;

    # Welcome screen
    skip_welcome_screen = false;
    ascii_art_mode = "animated";
    show_macchina_on_welcome = true;
  };
}
