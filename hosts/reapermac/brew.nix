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
      "mise"
      "neovim"
      "oci-cli"
      "cmake"
      "ninja"
      "terminal-notifier"
      "anomalyco/tap/opencode"
    ];

    caskArgs.require_sha = true;

    casks = [
      "alacritty"
      "arc"
      "thebrowsercompany-dia"
      "bitwarden"
      "brave-browser"
      "claude-code"
      "codex"
      "goland"
      "discord"
      "ghostty"
      "intellij-idea"
      "keybase"
      "obsidian"
      "obs"
      "orbstack"
      "pycharm"
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
