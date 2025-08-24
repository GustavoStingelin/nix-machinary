{ pkgs, ... }:
{
  programs.gpg = {
    enable = true;
    settings = {
      default-key = "0x15CBADFE29F2017B";
    };
  };

  services.gpg-agent = {
    enable = true;
    pinentryPackage = pkgs.pinentry-tty;
    defaultCacheTtl = 28800; # 8 hours
    maxCacheTtl = 86400;     # 24 hours
  };
}