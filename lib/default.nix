{ config, lib, pkgs, ... }:

{
  imports = [
    ./gnome.nix
    ./devtools.nix
    ../home-manager
  ];
}
