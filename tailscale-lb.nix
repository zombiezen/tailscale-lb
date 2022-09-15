{ lib
, buildGoModule
, nix-gitignore
}:

let
  src =
    let
      root = ./.;
      patterns = nix-gitignore.withGitignoreFile extraIgnores root;
      extraIgnores = [
        ".envrc"
        "*.nix"
        "/.github/"
        ".vscode/"
        "result"
        "result-*"
      ];
    in builtins.path {
      name = "tailscale-lb-source";
      path = root;
    };
in

buildGoModule {
  pname = "tailscale-lb";
  version = "0.1.0";

  inherit src;

  vendorHash = "sha256-s63ZWaytdIE8iojdbhWp9NnTo4g/QaL/1opTnk4i97g=";

  ldflags = [ "-s" "-w" ];

  meta = {
    maintainers = [ lib.maintainers.zombiezen ];
  };
}
