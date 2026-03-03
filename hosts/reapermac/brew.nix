{ ... }:

{
  homebrew = {
    enable = true;

    onActivation = {
      autoUpdate = true;
      upgrade = true;
      cleanup = "zap";
    };

    brews = [
      "anomalyco/tap/opencode"
      {
        name = "tor";
        restart_service = true;
      }
      "terminal-notifier"
    ];

    caskArgs.require_sha = true;

    casks = [
      "alacritty"
      "arc"
      "bitwarden"
      "brave-browser"
      "claude-code"
      "codex"
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
    ];
  };

  environment.variables.HOMEBREW_DOWNLOAD_CONCURRENCY = "auto";
}
