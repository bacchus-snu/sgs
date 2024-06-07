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
    hash = "sha256-zDMM603SjTGXhKSXRj4JfoTvCoZQnz8Wb6TX/2+QXDo=";
    env.NODE_ENV = "production";
  };
in
buildGoModule rec {
  name = "sgs";

  inherit src;
  vendorHash = "sha256-Wtk0GfNCpTALHVlOl1Al94E/WoS59BcBDmER6SRDcZQ=";

  ldflags = [
    "-s"
    "-w"
  ];

  subPackages = [
    "cmd/sgs"
    "cmd/sgs-register-harbor"
  ];
  # test all packages
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
    cp -r deploy/chart $out/chart
    install -m755 deploy/worker-sync.sh $out/bin/worker-sync.sh

    wrapProgram $out/bin/sgs \
      --prefix PATH : ${lib.makeBinPath buildInputs} \
      --set-default SGS_WORKER_COMMAND "$out/bin/worker-sync.sh" \
      --set-default SGS_DEPLOY_CHART_PATH "$out/chart" \
      --set-default SGS_DEPLOY_REGHARBOR_PATH "$out/bin/sgs-register-harbor"
  '';

  meta.mainProgram = "sgs";
}
