---
tldr: CLI subcommand to register, list, and remove local services on mesh nodes — talks directly to Lighthouse API with credentials derived from the mesh secret
---

# Service CLI — register local services for managed ingress via Lighthouse

## Target

A mesh node running a local service (e.g. Ollama on `:11434`) has no way to expose it through the managed ingress.
The Lighthouse `POST /v1/sites` endpoint exists but there's no CLI to call it.
This spec closes that gap: `wgmesh service add ollama :11434` registers the service and makes it reachable at a managed URL.

This is the Stage 0 exit gate in [[spec - first-customer - roadmap to first paying customer]].

## Behaviour

### Commands

- `wgmesh service add <name> <local-addr>` — register a service with Lighthouse
- `wgmesh service list` — show registered services for this node
- `wgmesh service remove <name>` — deregister a service

### `service add <name> <local-addr>`

- `<name>` is a human-readable identifier (e.g. `ollama`, `api`, `grafana`). Lowercase alphanumeric + hyphens. Must be unique within the org.
- `<local-addr>` is `[host]:port` (e.g. `:11434`, `127.0.0.1:8080`). Host defaults to `0.0.0.0` if omitted.
- Does NOT validate whether the local port is actually listening — the service may start later. Lighthouse health checks catch dead origins.
- Derives the node's mesh IP from the secret (same derivation as `wgmesh join`).
- Calls `POST /v1/sites` on Lighthouse with:
  - `domain`: `<name>.<mesh-id>.wgmesh.dev` (auto-generated from name + mesh identity)
  - `origin.mesh_ip`: derived mesh IP
  - `origin.port`: from `<local-addr>`
  - `origin.protocol`: `http` (default; `--protocol https` flag for TLS origins)
- Prints the managed URL on success: `https://<name>.<mesh-id>.wgmesh.dev`
- Stores registration locally in `/var/lib/wgmesh/services.json` for `service list` to work offline.

### `service list`

- Calls `GET /v1/sites` on Lighthouse filtered to this node's mesh IP.
- Falls back to local `/var/lib/wgmesh/services.json` if Lighthouse is unreachable.
- Output format:
  ```
  NAME      URL                                    PORT    STATUS
  ollama    https://ollama.abc123.wgmesh.dev       11434   active
  api       https://api.abc123.wgmesh.dev          8080    pending_dns
  ```

### `service remove <name>`

- Looks up the service by name in local state or via `GET /v1/sites`.
- Calls `DELETE /v1/sites/{id}` on Lighthouse.
- Removes from local `/var/lib/wgmesh/services.json`.
- Prints confirmation.

### Lighthouse URL discovery

- The Lighthouse URL is derived from the mesh secret, not passed explicitly.
- The secret already encodes mesh identity via `crypto.DeriveKeys()`. Extend this to derive a well-known Lighthouse endpoint.
- Discovery chain:
  1. DNS SRV lookup: `_lighthouse._tcp.<mesh-id>.wgmesh.dev` → host + port
  2. Fallback: `https://lighthouse.<mesh-id>.wgmesh.dev`
- This allows the mesh operator to point the SRV record at any Lighthouse instance.

### Authentication

- The org API key (`cr_...`) is needed for Lighthouse API calls.
- Stored locally during account setup: `wgmesh join --secret <secret> --account <api-key>` writes the key to `/var/lib/wgmesh/account.json`.
- `service` subcommands read from `/var/lib/wgmesh/account.json`.
- If no account is configured, print: `No account configured. Run: wgmesh join --secret <secret> --account <api-key>`

### Local state file

`/var/lib/wgmesh/services.json`:
```json
{
  "services": {
    "ollama": {
      "site_id": "site_abc123def456",
      "name": "ollama",
      "domain": "ollama.abc123.wgmesh.dev",
      "local_addr": ":11434",
      "protocol": "http",
      "registered_at": "2026-03-03T19:00:00Z"
    }
  }
}
```

## Design

### Direct-to-Lighthouse (no daemon involvement)

The CLI calls the Lighthouse REST API directly.
No RPC to the running daemon needed — mesh IP is derived from the secret, same as `wgmesh status`.

This means `service add` works even when the daemon isn't running (e.g. during setup).
The daemon doesn't need to know about services — Lighthouse is the source of truth.

### Mesh ID derivation

The mesh secret already derives keys via `crypto.DeriveKeys()`.
The mesh ID (used in DNS names) is derived from the secret: first 6 bytes of the DHT info-hash, hex-encoded (12 chars).
This is deterministic — every node with the same secret computes the same mesh ID.

### Domain naming

`<service-name>.<mesh-id>.wgmesh.dev`

- `wgmesh.dev` is the managed domain (wildcard DNS pointed at edge nodes).
- `<mesh-id>` scopes to the specific mesh (prevents cross-mesh collisions).
- `<service-name>` is the user-chosen name.

The Lighthouse `DNSTarget` for managed services is `edge.wgmesh.dev` — edges are configured with wildcard certs for `*.*.wgmesh.dev`.

### Flags

```
wgmesh service add <name> <local-addr>
  --secret        Mesh secret (or WGMESH_SECRET env var)
  --protocol      Origin protocol: http (default) or https
  --health-path   Health check path (default: /)
  --health-interval  Health check interval (default: 30s)

wgmesh service list
  --secret        Mesh secret (or WGMESH_SECRET env var)
  --json          Output as JSON

wgmesh service remove <name>
  --secret        Mesh secret (or WGMESH_SECRET env var)
```

## Verification

- `wgmesh service add ollama :11434` succeeds and prints a managed URL
- `wgmesh service list` shows the registered service with status
- `wgmesh service remove ollama` deregisters and confirms
- A registered service becomes reachable at its managed URL when Lighthouse + edge nodes are running
- Works without a running daemon (direct Lighthouse call)
- Fails gracefully when Lighthouse is unreachable (clear error, no crash)
- Fails gracefully when no account is configured (clear message with instructions)

## Friction

- **Lighthouse must be reachable**: Service registration requires network access to the Lighthouse API. Offline-first is not possible for the MVP.
- **DNS propagation**: The `<name>.<mesh-id>.wgmesh.dev` domain requires wildcard DNS to be configured. This is an operator responsibility.
- **No port validation**: A typo in the port won't be caught until Lighthouse health checks fail. Accepted tradeoff — services may legitimately start after registration.
- **Account setup is a separate step**: The user must run `join --account` before `service add`. This is an extra step but keeps concerns separated.

## Interactions

- Depends on [[spec - lighthouse - cdn control plane with rest api dragonfly store xds and federated sync]] — the server-side API
- Depends on [[spec - crypto - secret-derived keys envelope encryption membership proofs and rotation]] — mesh IP and mesh ID derivation
- Extends [[spec - cli entry point - dual mode dispatch with daemon wiring and rpc server]] — new `service` subcommand
- Feeds into [[spec - first-customer - roadmap to first paying customer]] — Stage 0 exit criteria

## Mapping

> [[main.go]] — `service` subcommand dispatch
> [[service.go]] — service add/list/remove implementation
> [[pkg/lighthouse/client.go]] — Lighthouse HTTP client + SRV discovery
> [[pkg/mesh/services.go]] — local service state (load/save)
> [[pkg/mesh/account.go]] — account credential storage
> [[pkg/crypto/derive.go]] — `MeshID()` method on DerivedKeys

## Future

{[!] `wgmesh service update <name>` — change port or protocol without remove+add}
{[!] `--account` flag on `wgmesh join` to store API key during mesh join}
{[?] Service health visible in `wgmesh status` output}
{[?] Auto-register services from Docker labels or systemd units}
