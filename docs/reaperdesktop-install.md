# `reaperdesktop` install procedure

## 1. Boot the installer

1. Boot the NixOS installer ISO in **UEFI mode**.
2. In the motherboard firmware, keep Secure Boot disabled until the first installed boot succeeds.
3. Confirm the target disk is the NVMe drive:

   ```bash
   lsblk
   ```

   This guide assumes the install target is `/dev/nvme0n1` and that destroying its contents is acceptable.

## 2. Clone the repository

```bash
nix-shell -p git
git clone https://github.com/GustavoStingelin/nix-machinary.git
cd nix-machinary
```

## 3. Partition and format the disk with disko

Run disko against the new host definition. This is destructive and will wipe `/dev/nvme0n1`, then recreate the GPT/LUKS/Btrfs layout declared in `disko.nix`.

```bash
sudo nix --experimental-features 'nix-command flakes' run github:nix-community/disko -- --mode disko --flake .#reaperdesktop
```

## 4. Generate and copy the real hardware configuration

The repository ships a placeholder `hosts/reaperdesktop/hardware-configuration.nix`. Replace it with a generated file before installation, but do **not** generate filesystem entries here: the filesystem and LUKS layout already comes from `disko.nix` in the flake.

```bash
sudo nixos-generate-config --no-filesystems --root /mnt
sudo cp /mnt/etc/nixos/hardware-configuration.nix hosts/reaperdesktop/hardware-configuration.nix
```

That generated file should contain detected kernel modules and platform details, but it should not reintroduce `fileSystems` or `boot.initrd.luks.devices` declarations that would conflict with the disko module already imported by `.#reaperdesktop`.

## 5. Create Secure Boot keys before installation

`reaperdesktop` already enables Lanzaboote with `boot.lanzaboote.pkiBundle = "/var/lib/sbctl"`, so the signing keys must exist inside the target root before `nixos-install` builds the bootloader.

Create the keys in the installer environment, copy them into the target system, and **do not** commit or back up these private keys into the repository. This uses `nix shell` because the live installer may not already have `sbctl` on `PATH`.

```bash
nix --experimental-features 'nix-command flakes' shell nixpkgs#sbctl -c sh -c 'sudo "$(command -v sbctl)" create-keys'
sudo mkdir -p /mnt/var/lib
sudo cp -a /var/lib/sbctl /mnt/var/lib/sbctl
```

Optional sanity check:

```bash
nix eval "path:$PWD#nixosConfigurations.reaperdesktop.config.networking.hostName"
nix eval "path:$PWD#nixosConfigurations.reaperdesktop.config.boot.lanzaboote.pkiBundle"
```

The second command should evaluate to `"/var/lib/sbctl"`.

## 6. Install `reaperdesktop`

```bash
sudo nixos-install --flake .#reaperdesktop
sudo nixos-enter --root /mnt -c 'passwd head'
```

If you also want a root password:

```bash
sudo nixos-enter --root /mnt -c passwd
```

## 7. First boot

1. Reboot into the installed system.
2. Log in as `head`.
3. Confirm the host name and NVIDIA driver are active:

   ```bash
   hostnamectl
   nvidia-smi
   ```

## 8. Enroll Secure Boot keys with `sbctl`

The keys were created before installation so Lanzaboote could sign boot artifacts. After the first successful boot, enroll those same keys into firmware.

```bash
sudo sbctl enroll-keys --microsoft
sudo nixos-rebuild switch --flake .#reaperdesktop
```

## 9. Firmware / BIOS notes

1. After enrolling keys, return to firmware setup.
2. Enable UEFI Secure Boot.
3. If the firmware offers standard/custom modes, use the mode that preserves the enrolled `sbctl` keys.
4. Keep CSM / legacy boot disabled.

## 10. Rebuild and verify

Run these checks after the Secure Boot rebuild:

```bash
sudo nixos-rebuild switch --flake .#reaperdesktop
sudo sbctl status
sudo sbctl verify
bootctl status
nvidia-smi
steam --version
```

Useful graphics/game tooling checks:

```bash
which mangohud
which protonup-qt
which goverlay
which gamescope
vulkaninfo | head
```
