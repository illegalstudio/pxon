# Configuration

pxon reads YAML configuration through Viper and supports environment overrides with the `PXON_` prefix.

## Required connection values

The following values are required for every command:

| YAML key | Environment variable | Description |
|---|---|---|
| `endpoint` | `PXON_ENDPOINT` | Proxmox API base URL, normally ending in `/api2/json`. |
| `token_id` | `PXON_TOKEN_ID` | Proxmox API token identifier, for example `automation@pve!pxon`. |
| `token_secret` | `PXON_TOKEN_SECRET` | Secret associated with the API token. |

The default configuration path is `~/.config/pxon/config.yaml`. pxon also reads the legacy `~/.pxon/config.yaml` location.

## Interactive configuration

Start with the required connection values in the environment or an existing configuration file:

```sh
PXON_ENDPOINT='https://proxmox.example.com:8006/api2/json' \
PXON_TOKEN_ID='automation@pve!pxon' \
PXON_TOKEN_SECRET='replace-with-token-secret' \
pxon config
```

The wizard:

1. Chooses a Proxmox node.
2. Detects storage that supports LXC root filesystems.
3. Detects available LXC templates.
4. Requests a default disk size and root password.
5. Offers SSH public keys found under `~/.ssh/*.pub`.
6. Configures DHCP or a static IPv4 pool.
7. Saves the final YAML file.

When only one suitable storage, template, or bridge exists, pxon selects it automatically.

## YAML example

```yaml
endpoint: https://proxmox.example.com:8006/api2/json
token_id: automation@pve!pxon
token_secret: replace-with-token-secret
insecure: false

default_storage: local-lvm
default_image: local:vztmpl/debian-13-standard_13.1-2_amd64.tar.zst
default_disk_size: "8"
default_password: replace-with-initial-root-password
default_ssh_public_key_path: ~/.ssh/id_ed25519.pub
default_net0: ""

network:
  mode: pool
  bridge: vmbr0
  gateway: 192.0.2.1
  netmask: "24"
  cidr: 0
  range_start: 192.0.2.100
  range_end: 192.0.2.199
```

The values above are examples. Use storage IDs, templates, addresses, and credentials from your own Proxmox environment.

## All environment variables

| Environment variable | YAML key |
|---|---|
| `PXON_ENDPOINT` | `endpoint` |
| `PXON_TOKEN_ID` | `token_id` |
| `PXON_TOKEN_SECRET` | `token_secret` |
| `PXON_INSECURE` | `insecure` |
| `PXON_DEFAULT_STORAGE` | `default_storage` |
| `PXON_DEFAULT_IMAGE` | `default_image` |
| `PXON_DEFAULT_DISK_SIZE` | `default_disk_size` |
| `PXON_DEFAULT_PASSWORD` | `default_password` |
| `PXON_DEFAULT_SSH_PUBLIC_KEY_PATH` | `default_ssh_public_key_path` |
| `PXON_DEFAULT_NET0` | `default_net0` |
| `PXON_NETWORK_MODE` | `network.mode` |
| `PXON_NETWORK_BRIDGE` | `network.bridge` |
| `PXON_NETWORK_GATEWAY` | `network.gateway` |
| `PXON_NETWORK_NETMASK` | `network.netmask` |
| `PXON_NETWORK_CIDR` | `network.cidr` |
| `PXON_NETWORK_RANGE_START` | `network.range_start` |
| `PXON_NETWORK_RANGE_END` | `network.range_end` |

## Creation precedence

For settings that have command-line flags, `pxon create` resolves values in this order:

1. Explicit command-line flag.
2. Matching configured default.
3. Automatic discovery or generation when supported.

Important cases:

- `--rootfs` overrides `--storage`, `--disk-size`, and their defaults.
- `--net0` overrides `default_net0` and the structured `network` section.
- `--ssh-key` overrides `default_ssh_public_key_path`.
- `--node` overrides automatic node selection.
- `--vmid` overrides the next ID returned by Proxmox.

## TLS verification

`insecure` defaults to `false`. Set it to `true` only when you intentionally need to bypass TLS certificate verification.

See [Security and automation](security.md) before storing credentials or disabling verification.
