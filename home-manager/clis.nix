{ pkgs, ... }:
{
  home.packages = with pkgs; [
    act
    git-absorb
  ];
}
