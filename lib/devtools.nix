{ config, pkgs, ... }:
{
  # NVIDIA drivers
  hardware.graphics.enable = true;
  services.xserver.videoDrivers = [
    "nvidia"
    "modesetting"
  ];
  boot.blacklistedKernelModules = [
    "nouveau"
  ];
  hardware.nvidia = {
    modesetting.enable = true;
    powerManagement.enable = false;
    powerManagement.finegrained = false;
    open = false;
    nvidiaSettings = true;
    package = config.boot.kernelPackages.nvidiaPackages.stable;
    prime = {
      offload = {
        enable = true;
        enableOffloadCmd = true;
      };
      intelBusId = "PCI:0:2:0";
      nvidiaBusId = "PCI:1:0:0";
    };
  };
  environment.variables = {
    NVD_BACKEND = "direct";
    LIBVA_DRIVER_NAME = "nvidia";
  };

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

  # Create desktop entries for nvidia-offload applications
  environment.systemPackages = with pkgs; [
    (makeDesktopItem {
      name = "vivaldi-nvidia";
      desktopName = "Vivaldi (NVIDIA)";
      exec = "nvidia-offload vivaldi %U";
      icon = "vivaldi";
      categories = [ "Network" "WebBrowser" ];
      mimeTypes = [ "text/html" "text/xml" "application/xhtml+xml" ];
    })
    (makeDesktopItem {
      name = "goland-nvidia";
      desktopName = "GoLand (NVIDIA)";
      exec = "nvidia-offload goland %f";
      icon = "goland";
      categories = [ "Development" "IDE" ];
      mimeTypes = [ "text/plain" "application/x-go" ];
    })
    (makeDesktopItem {
      name = "zed-nvidia";
      desktopName = "Zed (NVIDIA)";
      exec = "nvidia-offload zeditor %F";
      icon = "zed-editor";
      categories = [ "Development" "TextEditor" ];
      mimeTypes = [ "text/plain" ];
    })
    (makeDesktopItem {
      name = "ghostty-nvidia";
      desktopName = "Ghostty (NVIDIA)";
      exec = "nvidia-offload ghostty";
      icon = "ghostty";
      categories = [ "System" "TerminalEmulator" ];
    })
  ] ++ [
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
    alacritty
  	neovim
  	git
  	cargo
  	vscode
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
    gnumake
    jetbrains.goland
  ];
}
