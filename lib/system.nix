{ config, pkgs, ... }:

{
  # Enable Nix Flakes and experimental features
  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  # Networking
  networking.networkmanager.enable = true;

  # Time zone
  time.timeZone = "America/Sao_Paulo";

  # Internationalization
  i18n.defaultLocale = "en_US.UTF-8";

  # Audio with Pipewire
  services.pipewire = {
    enable = true;
    pulse.enable = true;
  };

  # Enable OpenSSH daemon
  services.openssh.enable = true;

  # Firefox
  programs.firefox.enable = true;

  # Basic system packages
  environment.systemPackages = with pkgs; [
    vim
    wget
    helix
    neovim
    git
    just
  ];
}
