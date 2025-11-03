{ pkgs, home-manager, homeImports, ... }:

{
  imports = [
    ./brew.nix
    home-manager.darwinModules.home-manager
  ];

  system = {
    stateVersion = 6;
    primaryUser = "head";
   # defaults = {
   #   dock = {
   #     autohide = true;
   #     magnification = true;
   #     showRecentApps = false;        
   #   };  
   # };
  };

  networking.hostName = "reapermac";
  security.pam.services.sudo_local.touchIdAuth = true;
  nixpkgs.config.allowUnfree = true;
  nixpkgs.config.allowBroken = true;

  nix = {
    # When Nix is installed with `--determinate`
    enable = false;
    settings = {
      experimental-features = [
        "nix-command"
        "flakes"
      ];
      trusted-users = [
        "@admin"
        "head"
      ];
    };
  };

  #services.nix-daemon.enable = true;

  programs.zsh.enable = true;

  users.users.head.home = "/Users/head";

  home-manager.useGlobalPkgs = true;
  home-manager.useUserPackages = true;
  home-manager.users.head = {
    home.stateVersion = "25.05";
    home.enableNixpkgsReleaseCheck = false;
    imports = homeImports;
  };

  environment.systemPackages = with pkgs; [
    hello
  ];
}
