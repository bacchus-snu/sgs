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
  # test runtime deps
  etcd,
  kubernetes,
}:

let
  src = ./.;
  npmDeps = fetchNpmDeps {
    name = "sgs-npm-deps";
    inherit src;
    hash = "sha256-rrLnKyko1OwqkMRPVshyKJ8FJFKEVWUy2JN3/2QNwZw=";
    env.NODE_ENV = "production";
  };
in
buildGoModule rec {
  name = "sgs";

  inherit src;
  vendorHash = "sha256-OGmd54ODOsRSlI4/wKtyHLmrJWlMVaMrS7J0GC5d5z0=";

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

  # controller runtime testing dependencies
  env = {
    TEST_ASSET_ETCD = lib.getExe' etcd "etcd";
    TEST_ASSET_KUBECTL = lib.getExe' kubectl "kubectl";
    TEST_ASSET_KUBE_APISERVER = lib.getExe' kubernetes "kube-apiserver";
  };
  passthru.testEnv = env;

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
