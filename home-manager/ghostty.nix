{ ... }:

{
  programs.ghostty = {
    enable = true;
    package = null; # not packaged for aarch64-darwin in nixpkgs; brew cask provides the binary
    settings = {
      # Font
      font-size = 14;

      # Window
      background-opacity = 0.95;
      background-blur-radius = 20;
      macos-titlebar-style = "tabs"; # integrates tab bar into titlebar, matches terminal bg
      maximize = true;
      macos-option-as-alt = true;

      # Theme
      theme = "catppuccin-mocha";

      # Shell
      shell-integration = "zsh";
      quit-after-last-window-closed = true;

      # Yazelix: launch zellij+yazi on startup
      command = "sh -c 'exec $HOME/.config/yazelix/shells/posix/start_yazelix.sh'";
    };
  };
}
