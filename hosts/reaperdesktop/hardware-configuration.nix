# Replace this placeholder with a generated hardware configuration during install.
# After running disko, use `nixos-generate-config --no-filesystems --root /mnt` and
# copy `/mnt/etc/nixos/hardware-configuration.nix` over this file before
# `nixos-install --flake .#reaperdesktop`.
#
# This host already gets its filesystem and LUKS layout from `disko.nix`, so the
# generated replacement should not contain `fileSystems` or `boot.initrd.luks.devices` entries.
{ lib, modulesPath, ... }:

{
  imports = [
    (modulesPath + "/installer/scan/not-detected.nix")
  ];

  boot.initrd.availableKernelModules = [ "nvme" "xhci_pci" "usb_storage" "usbhid" "sd_mod" ];
  boot.initrd.kernelModules = [ ];
  boot.kernelModules = [ ];
  boot.extraModulePackages = [ ];

  swapDevices = [ ];

  networking.useDHCP = lib.mkDefault true;

  nixpkgs.hostPlatform = lib.mkDefault "x86_64-linux";
  hardware.cpu.amd.updateMicrocode = lib.mkDefault true;
}
