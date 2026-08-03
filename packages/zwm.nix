{
  buildGoModule,
  lib,
  makeWrapper,
  git,
  gh,
  zellij,
}:

buildGoModule {
  pname = "zwm";
  version = "0.1.0";

  src = ../zwm;
  vendorHash = "sha256-l6Vj8wpyTSVwClWdiKZGoaJxx+IvR6AAJE58a5eRkTI=";

  doCheck = true;
  nativeBuildInputs = [ makeWrapper ];
  nativeCheckInputs = [ git ];

  postFixup = ''
    wrapProgram "$out/bin/zwm" \
      --prefix PATH : "${lib.makeBinPath [ git gh zellij ]}"
  '';
}
