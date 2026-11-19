---
name: pxon
description: Operate pxon for Proxmox VE LXC workflows, reusable container images, and agent skill installation or verification. Use when asked to configure, inspect, automate, troubleshoot, or manage pxon and containers carrying its tag.
---

# Operate PXON

Use the installed `pxon` executable as the source of truth for the available commands and flags.

## Preflight

1. Run `command -v pxon` and `pxon --version` before operating it.
2. Run `pxon <command> --help` before using flags whose behavior is unclear.
3. If configuration is missing, explain that PXON needs `PXON_ENDPOINT`, `PXON_TOKEN_ID`, and `PXON_TOKEN_SECRET` for the initial `pxon config` run.
4. Never print, persist in transcripts, or expose token secrets, passwords, private keys, or the contents of `~/.config/pxon/config.yaml`.
5. Keep TLS verification enabled. Do not set `insecure: true` unless the user explicitly accepts the risk for a controlled environment.
6. Run `pxon skills verify --json` after an upgrade or when the installed instructions may be stale. It is read-only and checks both destinations by default. Stale or unreadable copies cause a non-zero exit status while still returning the full `verifications` array. `not-installed` alone does not fail verification.

## Choose the operation

- Configure defaults with `pxon config`. Treat the wizard as interactive even when `--json` is present.
- Refresh this skill with `pxon skills install --target agents` for Codex or `--target claude` for Claude Code. Repeat `--target` for multiple destinations; it is not comma-separated. Bare `skills install` opens a menu, while `--json` installs both destinations without prompting unless narrowed by `--target`. Choose the destinations relevant to the user's request. Local edits require explicit `--force`; preserve them unless replacing them is authorized. Human-readable commands remind users about stale skills; `suppress_skills_notice: true` silences the reminder.
- Inspect managed containers with `pxon list --json`. PXON only returns LXC containers carrying the exact `pxon` tag.
- Read live state with `pxon status <name-or-vmid> --json`. Use `pxon start <target> --json` or `pxon stop <target> --json` for power changes. These require an explicit managed target, reject templates, and do nothing if the desired state is already reached. They do not require SSH or a static IP. `stop` is clean shutdown only, preserving disk and never forcing termination.
- Power and status commands accept `--timeout <duration>` (default `2m`) for the overall deadline, including lookup and HTTP requests. `stop` additionally accepts `--shutdown-timeout <duration>` (default `1m`) for the server shutdown grace period. A local timeout does not cancel an accepted Proxmox task: inspect the returned UPID and state before retrying.
- Create a container with `pxon create <hostname> --json` plus only the flags required by the request. Prefer an SSH public key over an initial password. Never place a password directly in a command unless the user explicitly requests that exposure.
- Connect with `pxon ssh <name-or-vmid>`. Run a remote command with `pxon ssh <name-or-vmid> -- <command> [args...]`; omit the target to select it interactively. PXON requires a configured static IPv4 address and replaces its process with the local SSH client. It forwards command arguments without invoking a local shell; quote a pipeline or compound command as one argument, just as with direct `ssh` usage.
- Delete with `pxon delete <name-or-vmid>`. First inspect `pxon list --json` and verify the resolved name and VMID with the user. Allow the interactive confirmation by default.

## Reuse configured containers as images

List images with `pxon images list --json`, optionally adding `--node <name>`. The default node is used otherwise. Each `data` item has an `id`, `kind`, `name`, and `node`. Archives have IDs such as `local:vztmpl/ubuntu.tar.zst`; managed container templates use `lxc:<vmid>`.

Create from an explicit ID with `pxon create <hostname> --image <id> --json`. `--template` is an alias and cannot be combined with `--image`. `--select-image` opens an interactive picker and cannot be combined with `--json` or either explicit image flag. `default_image` can contain either kind of ID.

To save a configured machine:

1. Inspect `pxon list --json` and resolve the managed source name, VMID, node, and status. A source already marked `template: 1` is reusable directly.
2. Run `pxon images create <source-name-or-vmid> <image-name> --json`. Optional flags are `--vmid <new-id>`, `--storage <id>`, `--node <source-node>`, and `--snapshot <existing-snapshot>`.
3. The source must be stopped, or an existing snapshot must be named when it is running. Do not stop the source or create snapshots implicitly. The command makes a full copy with a separate VMID, removes its network interfaces, disables automatic boot, and converts the copy into a template. It preserves the source and does not export an archive file.
4. Read the returned `image.id` and use it with `create --image`. The clone is created on the template's node; pass `--node` when it differs from the default.

Container images preserve application files, credentials, SSH host keys, machine identifiers, disk size, and privilege mode. Prepare the source for reuse deliberately. Explicit `--rootfs`, `--disk-size`, `--password`, `--ssh-key`, and `--unprivileged` flags are rejected for these images, and their configured defaults are ignored. Memory, cores, and swap are inherited unless explicitly overridden. Storage and network defaults still apply. PXON assigns a new `net0`, removes other inherited interfaces, and starts the clone only after configuration succeeds. It does not sanitize the filesystem or regenerate service identities.

Managed templates also appear in `pxon list` with `template: 1`. They can be deleted through the normal managed `delete` command; select a regular container for SSH access.

## Apply mutation safeguards

Treat `config`, `create`, `start`, `stop`, `images create`, `skills install`, and `delete` as state-changing operations against Proxmox or the local machine.

- State the intended target and important options before creating or deleting a container.
- Do not use `pxon delete --force` unless the user explicitly requests unattended deletion or explicitly approves bypassing confirmation.
- Before `--force`, explain that it permanently deletes the container, enables Proxmox force deletion, and automatically removes matching hostname and IP entries from `~/.ssh/known_hosts` after the Proxmox task succeeds.
- Do not treat the `pxon` tag as an authorization boundary. Proxmox permissions remain authoritative.
- If a Proxmox task fails or times out, report the task result without assuming the requested state change completed.

## Produce automation-friendly output

Use `--json` whenever another process or the agent needs to inspect structured output. Parse standard output only; do not parse human-readable tables, prompts, or progress messages.

Creation JSON includes the image, hostname, VMID, node, network, stage, and final Proxmox task fields. Clones also include `clone_task` and `start_task` when available. Image creation includes `image`, `source`, `stage`, the clone `upid`, and `task` when available. Image and clone failures return a non-zero exit status with the partial result, `error`, and the last stage reached. Inspect the reported destination VMID and task before retrying or deleting anything; failures or timeouts can leave partial copies. There is no automatic rollback. Other command-level JSON failures return `{ "error": "..." }`.

Lifecycle JSON includes target identity, operation, last observed status and uptime, and `changed`. The latter means a mutating task was accepted, not necessarily completed. It is false for status reads and already-satisfied power operations. After task submission, partial errors preserve `upid`, the last `task` when available, and `error` in a single JSON report. Confirm final task outcome and guest state before assuming a power change succeeded.

Skill operations use `{ "name": "pxon", "installations": [...] }` or `{ "name": "pxon", "verifications": [...] }`. These reports are emitted even when individual destinations fail. Read each target's `status` and `error` rather than expecting a top-level error. Verification statuses are `not-installed`, `up-to-date`, `outdated`, and `modified`.

Report remote Proxmox changes separately from local effects such as SSH `known_hosts` cleanup. Summarize the container name, VMID, node, task outcome, and cleanup result when those fields are available.
