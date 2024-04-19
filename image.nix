{ lib, dockerTools, sgs, cacert, tini }:

let chart = ./deploy/chart;
in dockerTools.buildLayeredImage {
  inherit (sgs) name;

  contents = [ cacert ];
  config = {
    Entrypoint = [ (lib.getExe tini) "--" ];
    Cmd = [ (lib.getExe sgs) ];
    Env = [ "SGS_WORKER_CHART_PATH=${chart}" ];
  };
}
