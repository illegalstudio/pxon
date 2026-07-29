# Security and automation

## API token scope

Use a dedicated Proxmox API token rather than personal credentials. Grant only the privileges needed for the operations you intend to use.

Depending on the command, pxon needs to:

- read cluster LXC resources and container configuration;
- read nodes, storage, storage content, and network bridges;
- request the next VMID;
- create or delete LXC containers;
- read Proxmox task status and logs.

Keep read-only and destructive automation separate when your Proxmox permission model allows it.

## Local secret storage

The YAML configuration can contain:

- `token_secret`
- `default_password`

Treat the file as sensitive:

```sh
chmod 600 "$HOME/.config/pxon/config.yaml"
```

Do not commit real credentials, passwords, private keys, or environment files containing them.

The `default_ssh_public_key_path` setting points to a public key. pxon reads and submits the public key content during container creation; it does not read the corresponding private key.

## TLS

Keep:

```yaml
insecure: false
```

With the default setting, pxon verifies the Proxmox HTTPS certificate. `insecure: true` disables certificate verification and should be limited to controlled environments where the risk is understood.

## Password handling

Passing `--password` can expose a secret through shell history or process inspection. Prefer a protected configuration file or another controlled execution environment, and avoid sharing command transcripts that contain credentials.

## Managed scope

`list`, `ssh`, and `delete` only operate on LXC containers carrying the exact `pxon` tag. This boundary reduces accidental interaction with unrelated guests, but it is not an authorization boundary: Proxmox API permissions remain the source of access control.

## Deletion

Interactive deletion requires an explicit `Yes` selection. If the target is running, pxon asks for the same permanent-deletion confirmation and then sends a forced delete request to Proxmox.

`--force` is designed for automation and performs three actions:

1. Skips the container confirmation.
2. Skips the SSH cleanup confirmation.
3. Removes matching hostname/IP entries from `~/.ssh/known_hosts` after successful deletion.

Review the resolved target and credentials carefully before using it in unattended jobs.

## SSH known hosts

pxon delegates matching and removal to the local `ssh-keygen` command:

- `ssh-keygen -F` finds plain or hashed entries.
- `ssh-keygen -R` removes approved matches.

Both the container name and its configured static IP are checked. Cleanup occurs only after the Proxmox delete task reports `OK`.

## JSON and stream separation

Use `--json` when another program consumes pxon output:

```sh
pxon list --json
pxon create batch-01 --json
pxon delete batch-01 --force --json
```

Treat standard output as the structured result. Do not parse the human-readable table or interactive wizard output.
