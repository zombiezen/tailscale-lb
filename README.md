# Tailscale Load Balancer

This project is a basic load-balancer for forwarding [Tailscale][] TCP traffic.
This is useful for setting up [virtual IPs for services on Tailscale][].

[Tailscale]: https://tailscale.com/
[virtual IPs for services on Tailscale]: https://github.com/tailscale/tailscale/issues/465

## Status

**This project is largely a proof-of-concept/prototype.**
Having [virtual IPs for services on Tailscale][] has been discussed upstream,
but I had a need for this on my own Tailnet.

I'm sharing the code for this in the interest of
sharing the results of my experimentation,
but I don't have a ton of time to spare for this particular project.
Bugfixes welcome, but don't expect huge feature development or production-readiness.
If you find this program useful, consider [sponsoring me][]!

[sponsoring me]: https://github.com/sponsors/zombiezen

## Installation

**TODO:** Docker image

If you're using [Nix][], you can check out the repository and run the following:

```shell
nix-env --file . --install -A tailscale-lb
```

[Nix]: https://nixos.org/

## Usage

Create a configuration file:

```ini
# This is the hostname that will show up in the Tailscale console
# and be used by MagicDNS.
hostname = example
# Generate an authentication key from https://login.tailscale.com/admin/settings/keys
auth-key = tskey-foo

# For each port you want to listen on,
# add a section like this:
[tcp 80]
# ... and then add one or more backends.
# tailscale-lb will round-robin TCP connections
# among the various IP addresses it discovers.
# A backend can be one of:

# a) An IPv4 address. If the port is omitted, then the section's port is used.
backend = 127.0.0.1:80

# b) An IPv6 address. If the port is omitted, then the section's port is used.
backend = [2001:db8::1234]:80

# c) A DNS name. If the port is omitted, then the section's port is used.
backend = example.com:80

# d) SRV records. The port is obtained from the SRV record.
# Priority and weight are ignored.
backend = srv _http._tcp.example.com
```

Then run tailscale-lb with the configuration file as its argument:

```shell
tailscale-lb foo.ini
```

## License

[Apache 2.0](LICENSE)

