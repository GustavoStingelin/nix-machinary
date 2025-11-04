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
      "llvm"
      {
        name = "tor";
        restart_service = true;
      }
      "torsocks"
    ];

    caskArgs.require_sha = true;

    casks = [
      "alacritty"
      "arc"
      "bitwarden"
      "brave-browser"
      "claude-code"
      "codex"
      "discord"
      "ghostty"
      "obsidian"
      "obs"
      "orbstack"
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
