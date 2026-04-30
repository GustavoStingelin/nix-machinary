{ config, lib, pkgs, ... }:

{
  imports = [
    ./hardware-configuration.nix
  ];

  networking.hostName = "reaperdesktop";

  boot = {
    kernelPackages = pkgs.linuxPackages_latest;
    kernelModules = [ "kvm-amd" ];

    loader = {
      systemd-boot.enable = lib.mkForce false;
      efi.canTouchEfiVariables = true;
    };

    lanzaboote = {
      enable = true;
      pkiBundle = "/var/lib/sbctl";
    };

    initrd.systemd.enable = true;

    supportedFilesystems = [
      "ntfs"
      "bcachefs"
      "btrfs"
    ];

    blacklistedKernelModules = [ "nouveau" ];
  };

  hardware.cpu.amd.updateMicrocode = lib.mkDefault config.hardware.enableRedistributableFirmware;
  hardware.graphics = {
    enable = true;
    enable32Bit = true;
  };

  services.xserver.videoDrivers = [ "nvidia" "modesetting" ];

  hardware.nvidia = {
    modesetting.enable = true;
    powerManagement.enable = false;
    powerManagement.finegrained = false;
    open = false;
    nvidiaSettings = true;
    package = config.boot.kernelPackages.nvidiaPackages.stable;
    prime.offload = {
      enable = lib.mkForce false;
      enableOffloadCmd = lib.mkForce false;
    };
  };

  environment.variables = {
    NVD_BACKEND = "direct";
    LIBVA_DRIVER_NAME = "nvidia";
    GI_TYPELIB_PATH = "/run/current-system/sw/lib/girepository-1.0";
  };

  programs = {
    nix-ld.enable = true;
    gamemode.enable = true;
    gamescope.enable = true;
    steam = {
      enable = true;
      protontricks.enable = true;
    };
  };

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

  environment.systemPackages = with pkgs; [
    goverlay
    mangohud
    protonup-qt
    gamescope
    vulkan-tools
    sbctl
  ];

  system.stateVersion = "25.05";
}
