{
  lib,
  buildGoModule,
  fetchNpmDeps,
  makeWrapper,
  npmHooks,
  # build deps
  nodejs,
  # runtime deps
  bash,
  kubectl,
  kubernetes-helm,
}:

let
  src = ./.;
  npmDeps = fetchNpmDeps {
    name = "sgs-npm-deps";
    inherit src;
    hash = "sha256-6hKCf0h/LIuJQRfnsWHpRZiqo7JqgXMfJJ1VbVJYAdQ=";
    env.NODE_ENV = "production";
  };
in
buildGoModule rec {
  name = "sgs";

  inherit src;
  vendorHash = "sha256-fURrfskr97eB8dNPBDuEDFBF0wIA14YsBLQuHgGnQjk=";

  ldflags = [
    "-s"
    "-w"
  ];

  subPackages = [ "cmd/sgs" ];
  preCheck = ''
    unset subPackages
  '';

  # build tailwind CSS
  nativeBuildInputs = [
    makeWrapper
    nodejs
    npmHooks.npmConfigHook
  ];
  inherit npmDeps;
  preBuild = ''
    NODE_ENV=production make generate-tailwindcss
  '';

  # remove npm deps from the goModules derivation
  overrideModAttrs = prev: {
    nativeBuildInputs = lib.remove npmHooks.npmConfigHook prev.nativeBuildInputs;
    preBuild = null;
  };

  # not strictly required, but nice to have for mkShell
  buildInputs = [
    bash
    kubectl
    kubernetes-helm
  ];

  postInstall = ''
    wrapProgram $out/bin/sgs --prefix PATH : ${lib.makeBinPath buildInputs}
  '';

  meta.mainProgram = "sgs";
}
