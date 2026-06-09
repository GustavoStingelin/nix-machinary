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
      "claude-code"
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
