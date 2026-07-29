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
