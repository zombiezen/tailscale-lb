# Copyright 2022 Ross Light
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

{ pkgs ? import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/a0b7e70db7a55088d3de0cc370a59f9fbcc906c3.tar.gz") {}
}:

let
  tailscale-lb = pkgs: pkgs.callPackage ./tailscale-lb.nix {
    buildGoModule = pkgs.buildGo119Module;
  };

  dockerImageName = "ghcr.io/zombiezen/tailscale-lb";
  mkDocker =
    { pkgs
    , name ? dockerImageName
    , tag ? null
    }:
    pkgs.dockerTools.buildImage {
      inherit name;
      tag = if builtins.isNull tag then (tailscale-lb pkgs).version else tag;

      copyToRoot = pkgs.buildEnv {
        name = "tailscale-lb";
        paths = [
          (tailscale-lb pkgs)
          pkgs.cacert
        ];
      };

      config = {
        Entrypoint = [ "/bin/tailscale-lb" ];
      };
    };
  docker = {
    amd64 = args@{ name ? dockerImageName, tag ? null }:
      mkDocker (args // { pkgs = pkgs.pkgsCross.musl64; });
    arm64 = args@{ name ? dockerImageName, tag ? null }:
      mkDocker (args // { pkgs = pkgs.pkgsCross.aarch64-multiplatform-musl; });
  };
in

{
  inherit pkgs;
  go = pkgs.go_1_19;
  tailscale-lb = tailscale-lb pkgs;

  inherit mkDocker docker;

  ci = pkgs.linkFarm "tailscale-lb-ci" [
    { name = "tailscale-lb"; path = tailscale-lb pkgs; }
    { name = "docker-image-tailscale-lb-amd64.tar.gz"; path = docker.amd64 {}; }
    { name = "docker-image-tailscale-lb-arm64.tar.gz"; path = docker.arm64 {}; }
  ];
}
