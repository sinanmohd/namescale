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
    HOME = "%S/namescale";
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
    environmentFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      example = "/var/lib/alina/secrets";
      default = null;
      description = ''
        Secrets may be passed to the service without adding them to the world-readable Nix store using this option.
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
        AmbientCapabilities = [ "" ];
        CapabilityBoundingSet = [ "" ];
        StateDirectory = "namescale";

        Type = "simple";
        Restart = "on-failure";
        EnvironmentFile = lib.mkIf (cfg.environmentFile != null) cfg.environmentFile;
        ExecStart = lib.getExe cfg.package;
      };
    };
  };
}
