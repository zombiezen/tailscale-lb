{ pkgs ? import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/a0b7e70db7a55088d3de0cc370a59f9fbcc906c3.tar.gz") {}
}:

{
  inherit pkgs;
  go = pkgs.go_1_19;
  tailscale-lb = pkgs.callPackage ./tailscale-lb.nix {
    buildGoModule = pkgs.buildGo119Module;
  };
}
