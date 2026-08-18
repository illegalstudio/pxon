---
name: pxon
description: Operate the pxon CLI to configure Proxmox VE access and safely create, list, connect to, or delete pxon-managed LXC containers. Use when an AI agent is asked to install, configure, inspect, automate, troubleshoot, or run pxon, including requests involving Proxmox containers carrying the pxon tag.
---

# Operate PXON

Use the installed `pxon` executable as the source of truth for the available commands and flags.

## Preflight

1. Run `command -v pxon` and `pxon --version` before operating it.
2. Run `pxon <command> --help` before using flags whose behavior is unclear.
3. If configuration is missing, explain that PXON needs `PXON_ENDPOINT`, `PXON_TOKEN_ID`, and `PXON_TOKEN_SECRET` for the initial `pxon config` run.
4. Never print, persist in transcripts, or expose token secrets, passwords, private keys, or the contents of `~/.config/pxon/config.yaml`.
5. Keep TLS verification enabled. Do not set `insecure: true` unless the user explicitly accepts the risk for a controlled environment.

## Choose the operation

- Configure defaults with `pxon config`. Treat the wizard as interactive even when `--json` is present.
- Refresh this skill after upgrading PXON with `pxon skills install`, selecting each agent destination that should receive the update. If PXON reports locally modified skill files, do not add `--force` without the user's approval.
- Inspect managed containers with `pxon list --json`. PXON only returns LXC containers carrying the exact `pxon` tag.
- Create a container with `pxon create <hostname> --json` plus only the flags required by the request. Prefer an SSH public key over an initial password. Never place a password directly in a command unless the user explicitly requests that exposure.
- Connect with `pxon ssh <name-or-vmid>`. Run a remote command with `pxon ssh <name-or-vmid> -- <command> [args...]`; omit the target to select it interactively. PXON requires a configured static IPv4 address and replaces its process with the local SSH client. It forwards command arguments without invoking a local shell; quote a pipeline or compound command as one argument, just as with direct `ssh` usage.
- Delete with `pxon delete <name-or-vmid>`. First inspect `pxon list --json` and verify the resolved name and VMID with the user. Allow the interactive confirmation by default.

## Apply mutation safeguards

Treat `config`, `create`, and `delete` as state-changing operations against Proxmox or the local machine.

- State the intended target and important options before creating or deleting a container.
- Do not use `pxon delete --force` unless the user explicitly requests unattended deletion or explicitly approves bypassing confirmation.
- Before `--force`, explain that it permanently deletes the container, enables Proxmox force deletion, and automatically removes matching hostname and IP entries from `~/.ssh/known_hosts` after the Proxmox task succeeds.
- Do not treat the `pxon` tag as an authorization boundary. Proxmox permissions remain authoritative.
- If a Proxmox task fails or times out, report the task result without assuming the requested state change completed.

## Produce automation-friendly output

Use `--json` whenever another process or the agent needs to inspect structured output. Parse standard output only; do not parse human-readable tables, prompts, or progress messages.

Report remote Proxmox changes separately from local effects such as SSH `known_hosts` cleanup. Summarize the container name, VMID, node, task outcome, and cleanup result when those fields are available.
