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
  nativeBuildInputs = [
    air
    docker-compose
    go-migrate
    golangci-lint
    templ
  ];
}
