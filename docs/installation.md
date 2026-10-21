# Installation

## Prerequisites

Before building or running pxon, prepare:

- Go 1.26 or newer.
- Network access to the Proxmox VE API.
- A Proxmox API token.
- OpenSSH client tools if you will use `ssh` or SSH host-key cleanup.

Check the local Go version:

```sh
go version
```

Check the SSH tools:

```sh
command -v ssh
command -v ssh-keygen
```

## Homebrew

Install the latest release from the Illegal Studio tap:

```sh
brew install illegalstudio/tap/pxon
```

Upgrade an existing installation:

```sh
brew update
brew upgrade pxon
```

The formula supports Apple Silicon and Intel macOS, plus ARM64 and AMD64 Linux.

## mise

Install the latest release globally using the [GitHub backend](https://mise.jdx.dev/dev-tools/backends/github.html):

```sh
mise use -g github:illegalstudio/pxon@latest
```

## AI agent skill

PXON includes an Agent Skills-compatible skill that teaches AI agents how to configure and operate the CLI safely. Install it for the current user after installing or upgrading PXON:

```sh
pxon skills install
```

The command opens a multi-select menu with both destinations selected by default:

- `~/.agents/skills/pxon` for Codex and compatible Agent Skills hosts;
- `~/.claude/skills/pxon` for Claude Code.

Use space to toggle a destination and enter to confirm. Select destinations without opening a menu with repeatable `--target` flags:

```sh
pxon skills install --target agents --target claude
pxon skills verify --json
```

`skills install --json` installs both destinations without prompting unless `--target` narrows the selection. `skills verify` is read-only and reports every supported destination by default. Missing installations are reported as `not-installed`; outdated or modified installations cause a non-zero exit status while preserving the full JSON verification report.

Running the install command again is safe: PXON automatically updates a previously installed, unmodified skill while protecting local changes independently in each destination. If a selected directory contains a different or locally modified skill, PXON leaves it untouched and reports an error. Replace selected conflicting copies explicitly with:

```sh
pxon skills install --force
```

After human-readable commands, PXON reminds you when an installed skill differs from the running binary. Set `suppress_skills_notice: true` in `~/.config/pxon/config.yaml` or `PXON_SUPPRESS_SKILLS_NOTICE=true` to disable this reminder. JSON, SSH, completion, and skill-management commands do not print it.

Restart an AI agent if it does not detect the newly installed skill automatically.

## Build from source

Clone the repository and compile the binary:

```sh
git clone https://github.com/illegalstudio/pxon.git
cd pxon
mkdir -p ./bin
go build -o ./bin/pxon .
```

Verify the build:

```sh
./bin/pxon --help
```

## Add pxon to PATH

Install the compiled binary into a user-owned executable directory:

```sh
mkdir -p "$HOME/.local/bin"
install -m 0755 ./bin/pxon "$HOME/.local/bin/pxon"
```

Make sure the directory is present in `PATH`. For zsh:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Persist that line in the appropriate shell configuration file if needed.

## Update an existing source checkout

```sh
git pull --ff-only
mkdir -p ./bin
go build -o ./bin/pxon .
install -m 0755 ./bin/pxon "$HOME/.local/bin/pxon"
```

Print the installed version:

```sh
pxon --version
```

## First-run bootstrap

`pxon config` queries the Proxmox API to discover nodes, storage, templates, and bridges. It therefore needs the three required connection values before the wizard can start.

Provide them temporarily through the environment:

```sh
export PXON_ENDPOINT='https://proxmox.example.com:8006/api2/json'
export PXON_TOKEN_ID='automation@pve!pxon'
export PXON_TOKEN_SECRET='replace-with-token-secret'

pxon config
```

The wizard writes the resulting configuration to `~/.config/pxon/config.yaml`.

Continue with the [configuration guide](configuration.md) for all available settings.
