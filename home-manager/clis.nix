{ pkgs, lib, ... }:
{
  home.packages = with pkgs;
    [
      act
      argocd
      boost
      capnproto
      cmake
      git-absorb
      kubernetes-helm
      kubeseal
      libevent
      llvm
      ninja
      oci-cli
      pkg-config
      qrencode
      qt6.full
      sqlfluff
      torsocks
      tree
      zeromq
    ]
    ++ lib.optionals stdenv.isDarwin [
      terraform
    ];
}
