<p align="center">
  <img src="assets/logo-mark.png" alt="pxon logo" width="130">
</p>

<h1 align="center">pxon</h1>

<p align="center">
  <em>A focused CLI for everyday Proxmox LXC workflows.</em>
</p>

<p align="center">
  <strong>Interactive setup &middot; Safe managed-container scope &middot; Deterministic automation &middot; JSON output</strong>
</p>

<p align="center">
  pxon creates, lists, connects to, and deletes Proxmox VE LXC containers while keeping its scope limited to containers tagged with <code>pxon</code>.
</p>

---

## Features

- Creates unprivileged LXC containers through the Proxmox VE API.
- Discovers storage, templates, bridges, and network defaults interactively.
- Assigns the next free address from a configured static IPv4 pool.
- Lists only containers carrying the `pxon` tag.
- Opens an SSH session by container name or VMID.
- Deletes managed containers with explicit confirmation and optional SSH `known_hosts` cleanup.
- Supports JSON output for scripting and automation.

## Requirements

- A reachable Proxmox VE API endpoint.
- A Proxmox API token with access to the resources pxon reads or modifies.
- Go 1.26 or newer to build from source.
- `ssh` for `pxon ssh`.
- `ssh-keygen` for SSH `known_hosts` detection and cleanup during `pxon delete`.

## Quick start

Install pxon with Homebrew:

```sh
brew install illegalstudio/tap/pxon
```

Or build pxon from source:

```sh
git clone https://github.com/illegalstudio/pxon.git
cd pxon
mkdir -p ./bin
go build -o ./bin/pxon .
```

Bootstrap the required Proxmox credentials, then run the interactive configuration wizard:

```sh
export PXON_ENDPOINT='https://proxmox.example.com:8006/api2/json'
export PXON_TOKEN_ID='automation@pve!pxon'
export PXON_TOKEN_SECRET='replace-with-token-secret'

./bin/pxon config
```

Create and manage a container:

```sh
./bin/pxon create app-01
./bin/pxon list
./bin/pxon ssh app-01
./bin/pxon delete app-01
```

For a non-interactive deletion:

```sh
./bin/pxon delete app-01 --force --json
```

`--force` bypasses confirmation, force-deletes the container through Proxmox, and removes matching hostname or IP entries from `~/.ssh/known_hosts`.

## Commands

| Command | Purpose |
|---|---|
| `pxon config` | Discover and save default storage, template, SSH key, and network settings. |
| `pxon create <hostname>` | Create a tagged LXC container and wait for the Proxmox task. |
| `pxon list` | List pxon-managed containers. |
| `pxon ssh [name\|vmid]` | Open `root@<container-ip>` through the local SSH client. |
| `pxon delete [name\|vmid]` | Delete a managed container and optionally clean SSH host keys. |

Run `pxon <command> --help` for the complete flag reference.

## Managed-container boundary

Every container created by pxon receives the `pxon` tag. The `list`, `ssh`, and `delete` commands only operate on LXC containers with that exact tag, so unrelated Proxmox guests remain outside the normal pxon workflow.

## Documentation

- [Documentation index](docs/README.md)
- [Installation](docs/installation.md)
- [Configuration](docs/configuration.md)
- [Command reference](docs/commands.md)
- [Networking](docs/networking.md)
- [Security and automation](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Releasing](docs/releasing.md)

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

The module currently has no generated code. Format changed Go files with `gofmt`.
