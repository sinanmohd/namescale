inputs:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.namescale;
  inherit (pkgs.stdenv.hostPlatform) system;

  configFormat = pkgs.formats.toml { };
  configFile = configFormat.generate "namescale.toml" cfg.settings;

  defaultEnvs = {
    NAMESCALE_CONFIG = "${configFile}";
  };
in
{
  meta.maintainers = with lib.maintainers; [ sinanmohd ];

  options.services.namescale = {
    enable = lib.mkEnableOption "namescale";
    package = lib.mkOption {
      type = lib.types.package;
      description = "The namescale package to use.";
      default = inputs.self.packages.${system}.namescale;
    };

    settings = lib.mkOption {
      inherit (configFormat) type;
      default = { };
      description = ''
        Configuration options for namescale.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    systemd.services.namescale = rec {
      description = "Zeroconf Wildcard DNS for Tailnet.";

      wantedBy = [ "multi-user.target" ];
      after = lib.optional config.services.tailscale.enable "tailscaled.service";
      requires = after;
      environment = defaultEnvs;

      serviceConfig = {
        DynamicUser = true;
        AmbientCapabilities = [ "CAP_NET_BIND_SERVICE" ];
        CapabilityBoundingSet = [ "CAP_NET_BIND_SERVICE" ];

        Type = "simple";
        Restart = "on-failure";
        ExecStart = lib.getExe cfg.package;
      };
    };
  };
}
