# Copyright 2023 Ross Light
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

{
  description = "A basic load-balancer for forwarding Tailscale TCP traffic";

  inputs = {
    nixpkgs.url = "nixpkgs";
    flake-utils.url = "flake-utils";
    flake-compat = {
      url = "github:edolstra/flake-compat";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
  let
    supportedSystems = [
      flake-utils.lib.system.x86_64-linux
      flake-utils.lib.system.aarch64-linux
      flake-utils.lib.system.x86_64-darwin
      flake-utils.lib.system.aarch64-darwin
    ];
  in
    flake-utils.lib.eachSystem supportedSystems (system:
    let
      pkgs = self.lib.nixpkgs system;
    in
    {
      packages = {
        default = self.lib.mkTailscaleLB pkgs;

        ci = pkgs.linkFarm "tailscale-lb-ci" ([
          { name = "tailscale-lb"; path = self.packages.${system}.default; }
        ] ++ pkgs.lib.lists.optional (self.packages.${system} ? docker-amd64) {
          name = "docker-image-tailscale-lb-amd64.tar.gz";
          path = self.packages.${system}.docker-amd64;
        } ++ pkgs.lib.lists.optional (self.packages.${system} ? docker-arm64) {
          name = "docker-image-tailscale-lb-arm64.tar.gz";
          path = self.packages.${system}.docker-arm64;
        });
      } // pkgs.lib.optionalAttrs pkgs.hostPlatform.isLinux {
        docker-amd64 = self.lib.mkDocker {
          pkgs = if pkgs.targetPlatform.isx86_64 then pkgs else pkgs.pkgsCross.musl64;
        };
        docker-arm64 = self.lib.mkDocker {
          pkgs = if pkgs.targetPlatform.isAarch64 then pkgs else pkgs.pkgsCross.aarch64-multiplatform-musl;
        };
      };

      apps.default = {
        type = "app";
        program = "${self.packages.${system}.default}/bin/tailscale-lb";
      };

      devShells.default = pkgs.mkShell {
        inputsFrom = [
          self.packages.${system}.default
        ];
      };
    }) // {
      lib = {
        nixpkgs = system: import nixpkgs { inherit system; };

        mkTailscaleLB = pkgs: pkgs.callPackage ./tailscale-lb.nix {
          buildGoModule = pkgs.buildGo122Module;
        };

        mkDocker =
          { pkgs
          , name ? "ghcr.io/zombiezen/tailscale-lb"
          , tag ? null
          }:
          let
            tailscale-lb = self.lib.mkTailscaleLB pkgs;
          in
            pkgs.dockerTools.buildImage {
              inherit name;
              tag = if builtins.isNull tag then tailscale-lb.version else tag;

              copyToRoot = pkgs.buildEnv {
                name = "tailscale-lb";
                paths = [
                  tailscale-lb
                  pkgs.cacert
                ];
              };

              config = {
                Entrypoint = [ "/bin/tailscale-lb" ];

                Labels = {
                  "org.opencontainers.image.source" = "https://github.com/zombiezen/tailscale-lb";
                  "org.opencontainers.image.licenses" = "Apache-2.0";
                  "org.opencontainers.image.version" = tailscale-lb.version;
                };
              };
            };
      };
    };
}
