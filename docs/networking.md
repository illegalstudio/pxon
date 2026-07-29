# Networking

pxon supports an explicit Proxmox `net0` value, a configured default `net0`, or generated DHCP/static-pool networking.

## Resolution order

During `pxon create`, networking is resolved in this order:

1. `--net0`
2. `default_net0`
3. the structured `network` configuration

If neither an explicit `net0` nor a usable structured configuration is available, creation fails before the Proxmox request is submitted.

## DHCP mode

Configuration:

```yaml
network:
  mode: dhcp
  bridge: vmbr0
```

Generated value:

```text
name=eth0,bridge=vmbr0,ip=dhcp
```

### SSH limitation

pxon derives a container address from a static `ip=` value in the Proxmox LXC `net0` configuration. It does not query DHCP leases or a guest agent.

As a result, `pxon ssh` cannot connect to a container whose `net0` only contains `ip=dhcp`. Use a static pool or an explicit static `--net0` value when pxon-managed SSH access is required.

## Static pool mode

Configuration:

```yaml
network:
  mode: pool
  bridge: vmbr0
  gateway: 192.0.2.1
  netmask: "24"
  range_start: 192.0.2.100
  range_end: 192.0.2.199
```

For each creation, pxon:

1. Reads all LXC containers on the selected node.
2. Extracts static IPv4 addresses from their `net0` configurations.
3. Walks the configured range from start to end.
4. Selects the first address not already present.
5. Builds a value such as:

```text
name=eth0,bridge=vmbr0,ip=192.0.2.100/24,gw=192.0.2.1
```

The range is inclusive.

## Netmask formats

`network.netmask` accepts:

- CIDR length: `24`
- slash-prefixed CIDR length: `/24`
- dotted IPv4 mask: `255.255.255.0`

The legacy numeric `network.cidr` setting is also supported when `network.netmask` is empty.

## Explicit `net0`

Use `--net0` when a container needs a one-off configuration:

```sh
pxon create database-01 \
  --net0 'name=eth0,bridge=vmbr1,ip=198.51.100.20/24,gw=198.51.100.1'
```

Set `default_net0` in YAML or `PXON_DEFAULT_NET0` when every created container should receive the same complete value.

## Concurrency

Pool selection reads current Proxmox configuration and chooses the first free address, but it does not reserve the address locally. Avoid simultaneous creates against the same small pool, because concurrent processes can observe the same address as available before either task completes.
