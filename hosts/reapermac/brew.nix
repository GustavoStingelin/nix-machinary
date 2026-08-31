{ ... }:

{
  homebrew = {
    enable = true;

    onActivation = {
      autoUpdate = true;
      upgrade = true;
    };

    brews = [
      "anomalyco/tap/opencode"
      {
        name = "tor";
        restart_service = true;
      }
      "terminal-notifier"
      "rtk"
      "libyaml"
    ];

    caskArgs.require_sha = true;

    casks = [
      "alacritty"
      "arc"
      "bitwarden"
      "brave-browser"
      # Upstream publishes two release channels, and the plain "claude-code"
      # cask follows the slower one: its livecheck reads .../releases/stable,
      # which trails .../releases/latest by a couple of weeks' worth of
      # versions. The @latest cask is the same binary from the same CDN, built
      # off the latest channel instead. The two conflict with each other, so
      # this is a swap, not an addition.
      "claude-code@latest"
      "codex"
      "dbeaver-community"
      "flameshot"
      "goland"
      "discord"
      "ghostty"
      "intellij-idea"
      "keybase"
      "obsidian"
      "obs"
      "orbstack"
      "pycharm"
      "rustrover"
      "secretive"
      "signal"
      "sparrow"
      "tailscale-app"
      "tor-browser"
      "transmission"
      "spotify"
      "visual-studio-code"
      "mullvad-vpn"
    ];
  };

  environment.variables.HOMEBREW_DOWNLOAD_CONCURRENCY = "auto";
}
