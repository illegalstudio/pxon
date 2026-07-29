# pxon documentation

pxon is a Go CLI for a focused set of Proxmox VE LXC operations. This documentation describes the behavior implemented by the current source tree.

## Guides

- [Installation](installation.md) — prerequisites, source builds, and PATH setup.
- [Configuration](configuration.md) — credentials, YAML, environment variables, and the interactive wizard.
- [Command reference](commands.md) — command behavior, flags, output, and automation examples.
- [Networking](networking.md) — DHCP, static pools, address selection, and SSH implications.
- [Security and automation](security.md) — API tokens, TLS, local secrets, deletion, and JSON output.
- [Troubleshooting](troubleshooting.md) — common errors and their likely causes.

## Core model

pxon considers a guest managed only when all of the following are true:

1. The Proxmox cluster resource is an LXC container.
2. Its semicolon-separated tag list contains the exact `pxon` tag.
3. The configured API token can read the resource.

Creation always adds the `pxon` tag. Listing, SSH selection, and deletion are restricted to that managed set.

## Configuration locations

The primary configuration file is:

```text
~/.config/pxon/config.yaml
```

The legacy location is also read:

```text
~/.pxon/config.yaml
```

Environment variables use the `PXON_` prefix and can override file values.
