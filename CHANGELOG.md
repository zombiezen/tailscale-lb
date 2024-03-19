# tailscale-lb Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[Unreleased]: https://github.com/zombiezen/tailscale-lb/compare/v0.4.0...main

## [0.4.0][] - 2024-03-19

Version 0.4 adds the ability to use a self-hosted coordination server.

[0.4.0]: https://github.com/zombiezen/tailscale-lb/releases/tag/v0.4.0

### Added

- New `control-url` configuration setting permits using a self-hosted coordination server
  ([#5](https://github.com/zombiezen/tailscale-lb/pull/5)).

### Changed

- Upgraded `tsnet` to 1.60.1.

## [0.3.0][] - 2023-03-18

Version 0.3 adds HTTP-aware reverse-proxying.

[0.3.0]: https://github.com/zombiezen/tailscale-lb/releases/tag/v0.3.0

### Added

- HTTP reverse-proxying with `X-Forwarded-For` and Tailscale-specific features
  like optional identity headers and automatic TLS certificates.
- Nix packaging has now been converted into a flake.

### Changed

- `default.nix` now just evaluates to the tailscale-lb derivation.
- Upgraded `tsnet` to 1.38.1.

## [0.2.1][] - 2022-09-21

[0.2.1]: https://github.com/zombiezen/tailscale-lb/releases/tag/v0.2.1

### Fixed

- No longer treating absolute paths in `state-directory` configuration as relative.

## [0.2.0][] - 2022-09-21

Version 0.2 adds the ability to persist the Tailscale IP address between runs.

[0.2.0]: https://github.com/zombiezen/tailscale-lb/releases/tag/v0.2.0

### Added

- Add option to allow storing Tailscale state between runs.
  This allows for non-ephemeral use cases.

### Changed

- Automatically log out from Tailscale on graceful exit
  if running without a state directory.
- Configuration order has been flipped so that
  the last configuration file passed on the command-line
  has the highest precedence.

## [0.1.1][] - 2022-09-19

[0.1.1]: https://github.com/zombiezen/tailscale-lb/releases/tag/v0.1.1

### Fixed

- Fixed IPv4 DNS lookups
  ([#1](https://github.com/zombiezen/tailscale-lb/issues/1)).

## [0.1.0][] - 2022-09-19

Initial release

[0.1.0]: https://github.com/zombiezen/tailscale-lb/releases/tag/v0.1.0
