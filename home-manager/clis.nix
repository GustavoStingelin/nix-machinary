{ pkgs, lib, ... }:
{
  home.packages = with pkgs;
    [
      act
      argocd
      git-absorb
      kubeseal
      sqlfluff
      tree
    ]
    ++ lib.optionals stdenv.isDarwin [
      terraform
    ];
}
