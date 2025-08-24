{ config, lib, pkgs, ... }:

{
  imports = [
    ./gnome.nix
    ./devtools.nix
    ./zsh.nix
    ./git.nix
  ];
}