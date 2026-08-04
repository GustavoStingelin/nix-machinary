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
  vendorHash = "sha256-486ChndLNCZnBhfyHss7/YA3SuV1uJUGaLLUxCkzxPI=";

  doCheck = true;
  nativeBuildInputs = [ makeWrapper ];
  nativeCheckInputs = [ git ];

  postFixup = ''
    wrapProgram "$out/bin/zwm" \
      --prefix PATH : "${lib.makeBinPath [ git gh zellij ]}"
  '';
}
