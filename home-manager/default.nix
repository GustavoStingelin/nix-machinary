{ config, pkgs, lib, ... }:

let
  home-manager = builtins.fetchTarball "https://github.com/nix-community/home-manager/archive/master.tar.gz";
in
{
  imports = [
    (import "${home-manager}/nixos")
  ];

  home-manager.useGlobalPkgs = true;
  home-manager.useUserPackages = true;
  
  home-manager.users.head = { pkgs, ... }: {
    home.stateVersion = "25.05";
    home.enableNixpkgsReleaseCheck = false;
    
    imports = [
      ./zed.nix
    ];
  };
}