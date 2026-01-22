{
  lib,
  dockerTools,
  sgs,
  cacert,
  tini,
  busybox,
}:

dockerTools.buildLayeredImage {
  inherit (sgs) name;

  contents = [ cacert busybox ];
  config = {
    Entrypoint = [
      (lib.getExe tini)
      "--"
    ];
    Cmd = [ (lib.getExe sgs) ];
  };
}
