# Command reference

## Global option

```text
--json   Return JSON output
```

Commands that produce structured pxon output use `--json` on standard output. Progress output is suppressed or redirected away from standard output where implemented. Command failures emit an `error` object and exit non-zero. Image and clone failures include their partial operation result instead; failed skill installation or verification emits the full per-destination report.

## `pxon skills install`

```text
pxon skills install [--target agents|claude]... [--force] [--json]
```

Opens a multi-select menu and installs the bundled PXON usage skill in one or both supported user locations:

- `~/.agents/skills/pxon`
- `~/.claude/skills/pxon`

Both destinations are selected by default. Use space to toggle the current option and enter to confirm. Pass repeatable `--target` flags to select destinations without a prompt. `--json` never opens the menu and defaults to both destinations. The skill covers configuration, images, creation, SSH access, deletion, skill verification, secret handling, and JSON automation.

The command is idempotent when an installed skill matches the bundled version. It automatically updates a previously installed skill when its checksum proves that it has not been modified. It refuses to overwrite unmanaged or locally changed files unless `--force` is supplied. With `--json`, it returns the skill name and an `installations` array containing each selected target, path, status, and any error:

- `installed`
- `up-to-date`
- `updated`
- `replaced`

## `pxon skills verify`

```sh
pxon skills verify
pxon skills verify --target claude
pxon skills verify --json
```

Compares installed skill contents with those bundled in the running binary without writing files. By default both destinations are checked; `--target` is repeatable. Statuses are `not-installed`, `up-to-date`, `outdated` (an unmodified copy from another version), and `modified` (local edits or unmanaged contents). Invalid destinations, including symlinks, are reported as errors independently for each destination.

The command exits non-zero if any installed copy is stale or cannot be inspected. An absent destination does not fail verification. JSON always includes the complete `{ "name": "pxon", "verifications": [...] }` report, including on failure. Use `skills install` to update unmodified copies; modified copies require explicit `--force`.

Human-readable commands print a stderr reminder for stale copies. `suppress_skills_notice: true` or `PXON_SUPPRESS_SKILLS_NOTICE=true` disables it. JSON, SSH, completion, help, and skill-management commands do not print reminders.

## `pxon images list`

```sh
pxon images list [--node <name>] [--json]
```

Lists images available on the selected node, using the default node when omitted:

- `archive`: downloaded LXC archives on active storage supporting `vztmpl`, identified by their full volume ID.
- `template`: LXC container templates carrying the exact `pxon` tag, identified as `lxc:<vmid>`.

Images are sorted by ID. JSON contains a top-level `data` array, including when the result is empty. Every image includes `id`, `kind`, `name`, and `node`, plus `storage` for archives or `vmid` for container templates. Copy `id` into `pxon create --image <id>` or `default_image`.

Discovery uses the storage availability already collected by Proxmox rather than probing every storage mount. Unavailable or unknown storage is skipped, and content requests are filtered to `vztmpl`. Availability therefore follows the periodic cluster status updates.

## `pxon images create`

```sh
pxon images create <source-name|vmid> <image-name> [--node <name>] [--vmid <new-id>] [--storage <id>] [--snapshot <name>] [--json]
```

Creates a full copy of a configured pxon-managed container, removes all inherited network interfaces, disables boot at host startup, and converts the copy into a Proxmox container template. The source is preserved. This creates a template with its own VMID, rather than an exported archive file. Container templates remain visible in `pxon list` with their `template` field set to `1`, and can be removed with the normal managed `delete` command.

The source must be stopped. To copy a running source, pass the name of an existing snapshot with `--snapshot`. PXON does not create snapshots or stop the source automatically. Snapshot and full-copy support depend on the source storage. `--storage` chooses storage for the full copy and otherwise preserves the source storage. `--node` restricts source lookup; the image always stays on its source node. The default destination VMID is the next available ID.

```sh
pxon images create app-01 app-image --vmid 900 --json
pxon images create app-01 app-image --snapshot ready --storage local-lvm --json
pxon create app-02 --image lxc:900 --json
```

JSON includes `image`, `source`, `stage`, `upid`, and the clone `task` when available. `stage` advances through `clone`, `configure`, `template`, and `completed`. Cloning is asynchronous and waits up to ten minutes; template conversion is synchronous and returns no task ID. On failure, the command exits non-zero and emits the same result with `error` and the last stage. A failed or timed-out operation may leave a copy at the reported VMID. Inspect it in Proxmox before retrying or explicitly deleting it; PXON does not automatically delete partial copies.

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
| `--image <id>` | configured | Archive volume ID or `lxc:<template-vmid>` from `images list`; alias of `--template`. |
| `--select-image` | `false` | Interactively choose an image; unavailable with `--json`, `--image`, or `--template`. |
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

`--image` and `--template` cannot be combined. For archive images, the existing rootfs, password, SSH key, and resource defaults apply. For container templates, PXON makes a full clone on the same node, replaces `net0` using the selected/default network, removes other inherited network interfaces, disables boot at host startup, and starts it only after configuration succeeds. Pass `--node` when the image is on a different node from the default.

Clones inherit disk size, root credentials, SSH keys, and privilege mode. Explicit `--rootfs`, `--disk-size`, `--password`, `--ssh-key`, or `--unprivileged` flags are rejected before cloning. Their configured defaults are ignored. RAM, cores, and swap are inherited unless explicitly overridden; `--storage` or its configured default selects the clone storage. Application data and system identities are copied as well: prepare the source for reuse before creating an image.

JSON preserves the final Proxmox task fields and adds `hostname`, `vmid`, `node`, `image`, `rootfs`, `net0`, and `stage`. Container-image creation also includes `clone_task` and `start_task` when available. Clone failures return the destination VMID, the last stage (`clone`, `configure`, or `start`), and `error`, with a non-zero exit status. Inspect that destination before retrying; partial clones are not automatically deleted. Clone and start tasks each wait up to ten minutes.

```sh
pxon create batch-01 --json
pxon create batch-02 --image lxc:900 --json
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
pxon ssh [name|vmid] [-- command...]
```

Without an argument, pxon opens an interactive managed-container selector, excluding templates. Templates are rejected as explicit SSH targets as well. With an argument, the match is:

- numeric argument: exact VMID;
- non-numeric argument: case-insensitive container name.

pxon then replaces its own process with:

```text
ssh root@<configured-ip>
```

To run a command instead of opening an interactive shell, put it after `--`:

```text
pxon ssh <name|vmid> -- <command> [args...]
```

pxon forwards each argument after `--` to the local OpenSSH client after the
destination; it does not invoke or interpolate a local shell. Quote shell
expressions as a single argument, just as you would when invoking `ssh` directly.

The command requires a static IPv4 address in the container `net0` configuration. pxon does not currently resolve DHCP leases or guest-agent addresses.

Examples:

```sh
pxon ssh web-01
pxon ssh 104
pxon ssh web-01 -- uname -a
pxon ssh 104 -- 'systemctl is-active nginx && uptime'
pxon ssh -- hostname
```

The last form opens the managed-container selector before running `hostname`.

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
