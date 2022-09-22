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
        "LICENSE"
        "README.md"
        "CHANGELOG.md"
        ".envrc"
        "*.nix"
        "/.github/"
        ".vscode/"
        "/infra/"
        "result"
        "result-*"
      ];
    in builtins.path {
      name = "tailscale-lb-source";
      path = root;
      filter = nix-gitignore.gitignoreFilterPure (_: _: true) patterns root;
    };
in

buildGoModule {
  pname = "tailscale-lb";
  version = "0.2.1";

  inherit src;

  vendorHash = "sha256-+fdwPzDEUHfI5m5SIqOvYVKZ0g+jkAf7uDpwjjpmyq4=";

  ldflags = [ "-s" "-w" ];

  subPackages = [ "." ];

  meta = {
    description = "Basic load-balancer for forwarding Tailscale TCP traffic";
    homepage = "https://github.com/zombiezen/tailscale-lb";
    license = lib.licenses.asl20;
    maintainers = [ lib.maintainers.zombiezen ];
  };
}
