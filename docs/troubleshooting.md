# Troubleshooting

## Configuration not found or incomplete

Example:

```text
configuration not found or incomplete
```

At minimum, provide:

```sh
export PXON_ENDPOINT='https://proxmox.example.com:8006/api2/json'
export PXON_TOKEN_ID='automation@pve!pxon'
export PXON_TOKEN_SECRET='replace-with-token-secret'
```

Then run `pxon config`.

## Proxmox API returned an HTTP error

Check:

- the endpoint includes the correct scheme, host, port, and `/api2/json`;
- the API token identifier and secret are correct;
- the token has permission for the requested node, storage, or container;
- TLS trust is configured correctly;
- the Proxmox host is reachable from the local machine.

pxon includes the Proxmox status and response body in API errors when available.

## No pxon-managed containers appear

`pxon list`, `ssh`, and `delete` only include LXC guests whose tag list contains the exact `pxon` tag.

Confirm in Proxmox that:

- the guest type is LXC, not QEMU;
- the guest has the `pxon` tag;
- the API token can read the guest.

## Container has no configured IP address

`pxon ssh` reads a static IPv4 value from the Proxmox `net0` configuration.

It cannot currently discover:

- DHCP lease addresses;
- addresses reported only inside the guest;
- guest-agent addresses.

Use a static pool or pass an explicit static `--net0` during creation.

## No template is available

`pxon config` and `pxon create` require an LXC template volume visible on the selected node.

Download a container template in Proxmox, verify that its storage is active and supports `vztmpl`, then retry.

## No suitable rootfs storage is available

The configuration wizard filters storage to active, enabled entries supporting `rootdir`.

Verify the storage configuration and token permissions on the selected node.

## `ssh` or `ssh-keygen` is missing

`pxon ssh` requires the local `ssh` executable.

`pxon delete` requires `ssh-keygen` when `~/.ssh/known_hosts` exists and must be inspected.

Install an OpenSSH client package appropriate for the operating system and retry.

## Delete succeeded but known_hosts cleanup failed

The Proxmox task and local SSH cleanup are separate operations. If pxon reports:

```text
container deleted, but known_hosts cleanup failed
```

the container is already gone. Remove the stale entries manually:

```sh
ssh-keygen -R container-name
ssh-keygen -R 192.0.2.100
```

Do not rerun `pxon delete` expecting it to find the deleted container.

## A Proxmox task timed out

pxon waits up to two minutes for create and delete tasks. A timeout means the CLI stopped polling; it does not prove that Proxmox cancelled the task.

Inspect the task in the Proxmox UI or task log before retrying a destructive operation.
