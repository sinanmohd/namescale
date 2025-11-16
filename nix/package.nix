{
  lib,
  buildGoModule,
}:

buildGoModule (finalAttrs: {
  pname = "namescale";
  version = "git";

  src = lib.cleanSourceWith {
    filter =
      name: type:
      lib.cleanSourceFilter name type
      && !(builtins.elem (baseNameOf name) [
        "nix"
        "flake.nix"
      ]);
    src = ../.;
  };

  vendorHash = "sha256-Tq9wN+9gDVPMZdZfF2zIqgkmUFIaSHDg+HsqSZQkpu8=";

  meta = {
    platforms = lib.platforms.unix;
    license = lib.licenses.agpl3Plus;
    mainProgram = "namescale";
    maintainers = with lib.maintainers; [ sinanmohd ];
  };
})
