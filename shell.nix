{
  mkShell,
  # main package
  sgs,
  sgs-docs,
  # development tools
  air,
  docker-compose,
  go-migrate,
  golangci-lint,
  templ,
}:

mkShell {
  inputsFrom = [
    sgs
    sgs-docs
  ];
  packages = [
    air
    docker-compose
    go-migrate
    golangci-lint
    templ
  ];

  env = sgs.passthru.testEnv;
}
