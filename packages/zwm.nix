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
  vendorHash = "sha256-wzu82FZ4L3fAq8AuB9VAhTwNN7dLXpaXWr04Z4CCUo8=";

  doCheck = true;
  nativeBuildInputs = [ makeWrapper ];
  nativeCheckInputs = [ git ];

  postFixup = ''
    wrapProgram "$out/bin/zwm" \
      --prefix PATH : "${lib.makeBinPath [ git gh zellij ]}"
  '';
}
