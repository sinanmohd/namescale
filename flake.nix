{
  inputs.nixpkgs.url = "github:NixOs/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
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
    in
    {

      packages = forAllSystems (
        { system, pkgs }:
        {
          namescale = pkgs.callPackage ./nix/package.nix { };
          default = self.packages.${system}.namescale;
        }
      );

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
