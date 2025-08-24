{ config, pkgs, lib, ... }:
{
  #virtual box settings
   virtualisation.virtualbox.host.enable = true;
   users.extraGroups.docker.members = [ "head" ];
   virtualisation.virtualbox.host.enableExtensionPack = true;
   virtualisation.virtualbox.host.enableKvm = true;
   virtualisation.virtualbox.host.addNetworkInterface = false;
   virtualisation.docker.rootless = {
  	enable = true;
  	setSocketVariable = true;
  };
  virtualisation.docker.enable = true;
  environment.variables = {
    GI_TYPELIB_PATH = "/run/current-system/sw/lib/girepository-1.0";
  };
  #tailscale	
  services.tailscale.enable = true;
  # Allow unfree packages
  nixpkgs.config.allowUnfree = true;
  services.libinput.enable = true;
  # multi-touch gesture recognizer
  services.touchegg.enable = true;
  # List packages installed in system profile. To search, run:
  networking.firewall.checkReversePath = false;
  # $ nix search wget
  services.udev.packages = [pkgs.vial pkgs.via];

  environment.systemPackages = with pkgs; [
	  vim
	  vial
	  libgtop
  	gparted
  	minikube
  	pkg-config
  	openssl
  	libxml2
  	libxslt
  	appimage-run
  	wget
  	wavm
  	curl
  	btop-cuda
  	htop
    kitty
  	neovim
  	git
  	cargo
  	vscode
  	zed-editor
  	chromium
  	vivaldi
  	obsidian
  	libgccjit
  	go
  	rustc
  	docker
  	nodejs_24
  	unzip
  	uv
  	tailscale
  	localsend
  	sparrow
  	bitcoin	
  	protonvpn-gui
  	gnupg
  	dig
  	neofetch
  	discord
  	vlc
  	curl
    ripgrep
    fd
    sd
    rsync
    jq
    zsh
  ];
}
