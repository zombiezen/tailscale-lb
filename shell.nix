args@{...}:
let
  inherit (import ./. args) pkgs go;
in

pkgs.mkShell {
  packages = [ go ];
}
