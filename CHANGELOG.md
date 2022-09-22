# tailscale-lb Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

[Unreleased]: https://github.com/zombiezen/tailscale-lb/compare/v0.2.1...main

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
