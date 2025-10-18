{
  namescale,
  lib,
  dockerTools,

  iSdebugBuild ? false,
  coreutils,
  dnsutils,
  gnused,
  gnugrep,
  findutils,
  vim,
}:
let
  port = 53;
in
dockerTools.buildLayeredImage {
  name = "sinanmohd/namescale";
  tag = "git";

  contents = [
    namescale
  ]
  ++ lib.optional iSdebugBuild [
    dockerTools.binSh
    coreutils
    dnsutils
    gnused
    gnugrep
    findutils
    vim
  ];

  config = {
    Cmd = [
      (lib.getExe namescale)
    ];
    ExposedPorts = {
      "${builtins.toString port}/tcp" = { };
      "${builtins.toString port}/udp" = { };
    };
  };
}
