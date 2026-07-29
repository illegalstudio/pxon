# Command reference

## Global option

```text
--json   Return JSON output
```

Commands that produce structured pxon output use `--json` on standard output. Progress output is suppressed or redirected away from standard output where implemented.

## `pxon config`

```text
pxon config
```

Runs the interactive configuration wizard and writes `~/.config/pxon/config.yaml`.

The required endpoint and token values must already be available through environment variables or an existing configuration file because the wizard queries Proxmox during discovery.

With `--json`, the final configuration summary is written as JSON. The wizard itself remains interactive.

## `pxon create`

```text
pxon create <hostname> [flags]
```

Creates an LXC container, applies the `pxon` tag, and waits up to two minutes for the Proxmox task.

### Flags

| Flag | Default | Description |
|---|---:|---|
| `--node <name>` | automatic | Proxmox node on which to create the container. |
| `--vmid <id>` | next ID | Explicit VMID. |
| `--template <volume>` | configured | Proxmox LXC template volume. |
| `--rootfs <value>` | composed | Complete rootfs value such as `local-lvm:8`. |
| `--storage <id>` | configured | Storage used when composing rootfs. |
| `--disk-size <size>` | configured | Disk size used when composing rootfs. |
| `--password <value>` | configured | Initial root password. |
| `--ssh-key <path>` | configured | SSH public key file to install. |
| `--memory <mb>` | `512` | RAM in MB. |
| `--cores <count>` | `1` | CPU core count. |
| `--swap <mb>` | `512` | Swap in MB. |
| `--net0 <value>` | configured/generated | Complete Proxmox `net0` value. |
| `--start` | `true` | Start the container after creation. |
| `--unprivileged` | `true` | Create an unprivileged container. |
| `--tag <tag>` | none | Additional tag; may be repeated or comma-separated. |

Examples:

```sh
pxon create web-01

pxon create worker-01 \
  --node pve-02 \
  --memory 2048 \
  --cores 2 \
  --disk-size 16 \
  --tag production \
  --tag workers

pxon create test-01 \
  --net0 'name=eth0,bridge=vmbr0,ip=192.0.2.120/24,gw=192.0.2.1' \
  --start=false
```

JSON mode returns the final Proxmox task status:

```sh
pxon create batch-01 --json
```

## `pxon list`

```text
pxon list
```

Lists managed LXC containers with:

- VMID
- name
- status
- configured static IP, when available
- node
- uptime
- current memory usage
- current disk usage

Only containers with the exact `pxon` tag appear.

JSON example:

```sh
pxon list --json
```

The JSON document contains a top-level `data` array with the Proxmox resource fields enriched by `managed` and, when available, `ip`.

## `pxon ssh`

```text
pxon ssh [name|vmid]
```

Without an argument, pxon opens an interactive managed-container selector. With an argument, the match is:

- numeric argument: exact VMID;
- non-numeric argument: case-insensitive container name.

pxon then replaces its own process with:

```text
ssh root@<configured-ip>
```

The command requires a static IPv4 address in the container `net0` configuration. pxon does not currently resolve DHCP leases or guest-agent addresses.

Examples:

```sh
pxon ssh web-01
pxon ssh 104
```

## `pxon delete`

```text
pxon delete [name|vmid] [--force]
```

Without a target, pxon opens the same managed-container selector used by `ssh`.

The interactive flow:

1. Resolves the target from the pxon-managed container set.
2. Searches `~/.ssh/known_hosts` for the container name and configured IP.
3. Requests explicit confirmation before permanent deletion.
4. If SSH matches exist, separately asks whether to remove them after deletion.
5. Starts the Proxmox delete task and waits up to two minutes.
6. Removes approved SSH entries only after the Proxmox task succeeds.

Running containers are force-deleted through the Proxmox API after the interactive confirmation.

### Automation

```sh
pxon delete web-01 --force --json
```

`--force`:

- bypasses both interactive confirmations;
- enables Proxmox force deletion;
- automatically removes matching hostname and IP entries from `~/.ssh/known_hosts`.

The JSON result includes the target, Proxmox task, number of SSH entries found, and whether they were removed.

Use `--force` carefully: it combines non-interactive container destruction with local SSH host-key cleanup.
