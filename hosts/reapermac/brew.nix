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
    ];

    caskArgs.require_sha = true;

    casks = [
      "alacritty"
      "arc"
      "bitwarden"
      "brave-browser"
      "claude-code"
      "codex"
      "goland"
      "discord"
      "ghostty"
      "keybase"
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
