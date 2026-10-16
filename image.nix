{
  lib,
  dockerTools,
  sgs,
  cacert,
  tini,
}:

dockerTools.buildLayeredImage {
  inherit (sgs) name;

  contents = [ cacert ];
  config = {
    Entrypoint = [
      (lib.getExe tini)
      "--"
    ];
    Cmd = [ (lib.getExe sgs) ];
  };
}
