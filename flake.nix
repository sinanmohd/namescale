{
  inputs = {
    nixpkgs.url = "github:NixOs/nixpkgs/nixos-unstable";
    pre-commit-hooks.url = "github:cachix/git-hooks.nix";
  };

  outputs =
    inputs@{
      self,
      nixpkgs,
      pre-commit-hooks,
    }:
    let
      lib = nixpkgs.lib;

      forSystem =
        f: system:
        f {
          inherit system;
          pkgs = import nixpkgs { inherit system; };
        };
      supportedSystems = lib.platforms.unix;
      forAllSystems = f: lib.genAttrs supportedSystems (forSystem f);
      forLinuxSystems = f: lib.genAttrs lib.platforms.linux (forSystem f);
    in
    {
      checks = forAllSystems (
        { system, pkgs }:
        {
          pre-commit-check = pre-commit-hooks.lib.${system}.run {
            src = ./.;
            hooks = {
              check-case-conflicts.enable = true;
              nixfmt-rfc-style.enable = true;
              check-added-large-files.enable = true;
              check-executables-have-shebangs.enable = true;
              check-merge-conflicts.enable = true;
              check-symlinks.enable = true;
              check-toml.enable = true;
              detect-private-keys.enable = true;
              trim-trailing-whitespace.enable = true;
              end-of-file-fixer.enable = true;
              shellcheck.enable = true;
              golines.enable = true;
            };
          };
        }
      );

      packages =
        lib.recursiveUpdate
          (forAllSystems (
            { system, pkgs }:
            {
              namescale = pkgs.callPackage ./nix/package.nix { };
              default = self.packages.${system}.namescale;
            }
          ))
          (
            forLinuxSystems (
              { system, pkgs }:
              {
                container = pkgs.callPackage ./nix/container.nix {
                  namescale = self.packages.${system}.namescale;
                };
              }
            )
          );

      nixosModules = {
        namescale = import ./nix/module.nix inputs;
        default = self.nixosModules.namescale;
      };

      devShells = forAllSystems (
        { system, pkgs }:
        {
          namescale = pkgs.callPackage ./nix/shell.nix {
            namescale = self.packages.${system}.namescale;
          };
          default = self.devShells.${system}.namescale;
        }
      );
    };
}
