{ ... }:

{
  programs.ghostty = {
    enable = true;
    package = null; # not packaged for aarch64-darwin in nixpkgs; brew cask provides the binary
    # Home Manager 26.05 added a user service that defaults on; it asserts
    # against a null package, and there is no binary here for it to launch.
    systemd.enable = false;
    settings = {
      # Font
      font-size = 14;

      # Window
      background-opacity = 0.95;
      background-blur-radius = 20;
      macos-titlebar-style = "hidden"; # removes title bar on macOS
      maximize = true;
      macos-option-as-alt = true; # Option key behaves as Alt for terminal keybindings

      # Theme
      theme = "catppuccin-mocha";

      # Shell
      shell-integration = "zsh";
      quit-after-last-window-closed = true;

      # Keybindings: Override Alt+arrow defaults to allow Zellij pane navigation
      # Ghostty defaults map alt+arrow_left to esc:b and alt+arrow_right to esc:f (word movement)
      # We override to send the raw terminal sequences so Zellij receives Alt+Left/Right for pane focus
      keybind = [
        "alt+arrow_left=csi:1;3D"
        "alt+arrow_right=csi:1;3C"
      ];
    };
  };
}
