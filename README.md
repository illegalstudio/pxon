<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-mark-dark.png">
    <source media="(prefers-color-scheme: light)" srcset="assets/logo-mark.png">
    <img src="assets/logo-mark.png" alt="pxon logo" width="130">
  </picture>
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
- Lists available images and creates reusable templates from configured containers.
- Creates containers from an explicit image or an interactive image selector.
- Assigns the next free address from a configured static IPv4 pool.
- Lists only containers carrying the `pxon` tag.
- Opens an SSH session by container name or VMID.
- Deletes managed containers with explicit confirmation and optional SSH `known_hosts` cleanup.
- Supports JSON output for scripting and automation.
- Installs a bundled AI agent skill that teaches agents how to operate pxon safely.
- Verifies installed agent skills and reports outdated or locally modified copies.

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
pxon skills install
```

Or install the latest release with [mise](https://mise.jdx.dev/dev-tools/backends/github.html):

```sh
mise use -g github:illegalstudio/pxon@latest
pxon skills install
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
./bin/pxon ssh app-01 -- uname -a
./bin/pxon delete app-01
```

List images, save a stopped, configured container named `configured-app` as a reusable image, and create a new container from it:

```sh
pxon images list
pxon images create configured-app app-image
pxon create app-02 --image lxc:101
pxon create app-03 --select-image
```

Use the image ID returned by `images create` in place of `lxc:101`. The source must be stopped, or you can pass `--snapshot <existing-snapshot>` to copy a running source. The image is a full copy with its own VMID; the original is preserved. Containers cloned from it inherit disk size, credentials, and resource settings, and receive a fresh pxon network configuration.

Check the installed agent skills after upgrading pxon:

```sh
pxon skills verify
pxon skills install --target agents --target claude
```

`--target` selects destinations without a prompt. `skills install --json` also runs without prompting. Verification reports `not-installed`, `up-to-date`, `outdated`, or `modified`, and fails when an installed copy is stale. pxon prints stale-skill reminders after human-readable commands; set `suppress_skills_notice: true` in the configuration to silence them.

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
| `pxon images list` | List LXC archives and managed container templates available on a node. |
| `pxon images create <source> <name>` | Save a full copy of a configured managed container as a template. |
| `pxon list` | List pxon-managed containers. |
| `pxon ssh [name\|vmid] [-- command...]` | Open an SSH session or run a remote command as `root`. |
| `pxon delete [name\|vmid]` | Delete a managed container and optionally clean SSH host keys. |
| `pxon skills install` | Install the bundled PXON usage skill for AI agents. |
| `pxon skills verify` | Compare installed skills with the version bundled in pxon. |

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

## License

pxon is available under the [MIT License](LICENSE).
