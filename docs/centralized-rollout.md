# Centralized deploy: rollout

Decision record: [ADR-0006](adr/ADR-0006-central-deploy/ADR-0006-central-deploy.md). This page is for the operator who runs `wgmesh -deploy`. The mode itself is described in [centralized-mode.md](centralized-mode.md).

## What changes and in which release

| Release | What `-deploy` does | What it refuses |
|---|---|---|
| N (first release with this change) | quotes every value it sends to a node; checks the state before the first connection; configures every reachable node and lists the failed ones; gives an all-private mesh an endpoint per node; restores the connected route after removing an own-network route | class A findings; class B findings only print `Warning: state:` lines |
| N+1 | the same | class A and class B findings: `mesh.StrictState` is set |
| N+2 | the same | a node whose config file cannot be written fails with phase `persist`: `mesh.PersistFailureFatal` is set |

No state field, flag or wire byte is added. A state file that deployed before keeps deploying at N unless it carries a class A value. Downgrading to an older release restores the old behaviour and loses nothing.

## Release note for N

- **State checks.** `-deploy` validates the state file before any connection and prints nothing to a node when it refuses.
  - Class A is refused at once: a character outside `[A-Za-z0-9._:/,@%+=\[\]-]` in the interface name, a public key, a public endpoint, the network, a routable network or an SSH host; a control character or a space; a value that starts with `-`; an empty interface name; a missing or unparsable `mesh_ip`; a `null` node. The error names the field, the node and the rule.
  - Class B is a warning at N: an interface name outside the Linux grammar (a letter first, then letters, digits, `_` or `-`, at most 15 characters), a CIDR that `net.ParseCIDR` rejects, a public endpoint that is not `host:port` with a port from 1 to 65535, a public key that is not 44 base64 characters decoding to 32 bytes, a listen or SSH port outside 1 to 65535, a mesh IP outside the network, a duplicate mesh IP or public key.
  - `-list`, `-remove` and `mesh list` do not validate, so a refused file can still be listed and a node removed.
- **Quoting.** Every value sent to a node is quoted for the shell (`ssh.Quote`, `ssh.Cmd`). For a valid state the bytes sent are the same as before, so a node's config does not change because of quoting. A value that cannot be quoted on a line of the wg-quick file (a newline, carriage return or NUL) is refused when the file is rendered.
- **A failed node does not stop the run.** Nodes are deployed in sorted hostname order. A node that fails in phase `detect`, `connect`, `install`, `apply`, `routes` or `persist` is recorded, the others are configured, and the run ends with one error that lists each failed node with its phase and the nodes that were configured. The exit code is 1, as before. The failed node's peers keep it in their configs, because the state file is the truth, not reachability. Run `wgmesh -deploy` again after fixing the node: each node is diffed against its live config, so a second run converges. A node left behind by a failed run is one generation behind the others until then.
- **A config file that cannot be written is a warning.** `Warnings:` after the last node lists it, and the node counts as configured. This changes at N+2.
- **Endpoints.** The order is: a `public_endpoint` written in the state wins; when every node's SSH host is a private IP address each node's endpoint is `<ssh host>:<listen port>`, the local node included; when the detected public IP equals the SSH host the endpoint is that host and the listen port; otherwise the node is behind NAT and has no endpoint. The deploy prints the reason: `Detected X has endpoint E (reason)`. A node with a private SSH host is not asked for its public IP, so an offline LAN mesh no longer waits for curl. A DNS name is never classified private: a LAN mesh that you reach by name still needs `public_endpoint` on every node that should be dialled, and so does a node with a private SSH host in a mixed mesh. An IPv6 host is bracketed. The detected endpoint is used for that run and is not written to the state file.
- **Connected route after an HA upgrade.** A mesh deployed by a release before the shared-subnet fix (PR #879) can leave `<own network> via <peer> dev wg0` on a node. When this release removes such a route, whatever its gateway, it adds the node's connected route back and prints `Restored connected route: <net> dev <iface>` (issue #894 item 1). It needs a local address in that network; without one it prints a warning and you restore the route by hand. A node whose route is removed by an older binary is not restored: upgrade the control node before the first deploy that removes the route, and look for the line.

## Going from N to N+1: class B

N+1 sets `mesh.StrictState`. Read the warnings first: run `wgmesh -deploy` at N and collect every `Warning: state:` line. Go when none is left. No-go while any is left, because N+1 refuses that state before it reaches a node. Fix the field the warning names, in the state file:

- a placeholder or truncated key: set a well-formed 44-character base64 key;
- a CIDR or port that does not parse: correct it;
- a duplicate mesh IP or key: one node has the other's value, set the right one.

Four warnings fire on states that may work today, so check them before N+1:

| Warning rule | Example | What to do |
|---|---|---|
| `port-out-of-range` for `listen_port` | `0` | set the port (the default is 51820) |
| `not-a-cidr` for `routable_networks` | `192.168.10.1` (a bare IP) | write the network, `192.168.10.1/32` or `192.168.10.0/24`; today it is a class B warning (a later change in the daemon may normalise it, do not rely on that) |
| `interface-name-grammar` | `wg.0` (a dot) | use a name from letters, digits, `_` and `-` |
| `not-host-port` for `public_endpoint` | `fd00::1:51820` (an unbracketed IPv6 address) | write `[fd00::1]:51820` |

Changing an interface name on a node that already runs under the old name is a larger change than a typo: plan it on its own.

## Going from N+1 to N+2: the persist warning

N+2 sets `mesh.PersistFailureFatal`. Read the `Warnings:` block: a line `failed to update config file /etc/wireguard/<iface>.conf` means the node runs with a live config that would not survive a reboot. Go when no `Warnings:` block has appeared for a release. A node that fails this way at N+2 is listed with phase `persist`, and its live peers are already applied.

## Reading a failed run

```
Failed to deploy: deploy failed on 1 of 3 nodes
  node2: detect: <error>
configured: node1, node3
```

- `detect` or `connect`: the node could not be reached (or its host key is refused: add it to `known_hosts`). It was dialled once and not again.
- `install` or `apply`: WireGuard could not be installed or the config could not be applied; the node may not be fully configured, and a second run retries it.
- `routes`: peers were applied, the routes were not.
- `persist`: the wg-quick text was refused before the node was touched, or (at N+2) the config file could not be written.

## Rollback

Reverting N restores the unquoted commands, the stop at the first failed node and the old endpoint rule. A node configured by N keeps a valid config, and the files N writes are the files the older release writes. Setting `StrictState` or `PersistFailureFatal` back to `false` returns a release to the N behaviour without a data change.
