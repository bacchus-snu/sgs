{ stdenvNoCC, mdbook }:

stdenvNoCC.mkDerivation (finalAttrs: {
  name = "";
  src = ./.;

  nativeBuildInputs = [ mdbook ];

  buildPhase = ''
    runHook preBuild
    mdbook build -d "$out"
    runHook postBuild
  '';
})
