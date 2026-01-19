{ stdenvNoCC, mdbook }:

stdenvNoCC.mkDerivation (finalAttrs: {
  name = "sgs-docs";
  src = ./.;

  nativeBuildInputs = [ mdbook ];

  buildPhase = ''
    runHook preBuild
    # Build English documentation
    mdbook build en -d "$out"
    # Build Korean documentation
    mdbook build ko -d "$out/ko"
    runHook postBuild
  '';
})
