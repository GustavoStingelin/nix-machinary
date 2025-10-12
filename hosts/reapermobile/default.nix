{ config, pkgs, ... }:

{
  imports = [
    ./hardware-configuration.nix
  ];

  # Hostname
  networking.hostName = "reapermobile";

  # Boot configuration
  boot = {
    kernelPackages = pkgs.linuxPackages_latest;
    kernelModules = [ "kvm-intel" ];

    loader = {
      systemd-boot.enable = true;
    };

    initrd = {
      systemd.enable = true;
    };

    supportedFilesystems = [
      "ntfs"
      "bcachefs"
      "btrfs"
    ];

    # Blacklist nouveau for NVIDIA
    blacklistedKernelModules = [ "nouveau" ];
  };

  # NVIDIA drivers configuration
  hardware.graphics.enable = true;
  services.xserver.videoDrivers = [ "nvidia" "modesetting" ];

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
    GI_TYPELIB_PATH = "/run/current-system/sw/lib/girepository-1.0";
  };

  # User configuration
  users.users.head = {
    isNormalUser = true;
    description = "Head";
    extraGroups = [
      "wheel"
      "networkmanager"
      "docker"
      "gamemode"
      "libvirtd"
      "video"
      "audio"
    ];
  };

  # State version
  system.stateVersion = "25.05";

  # Non nix binaries
  programs.nix-ld.enable = true;

}
