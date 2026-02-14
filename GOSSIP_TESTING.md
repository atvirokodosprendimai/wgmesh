# How to Test Gossip

This repository includes an in-mesh gossip implementation in `pkg/discovery/gossip.go`.
Gossip is enabled by passing the `--gossip` flag to `wgmesh join`, which starts
the `MeshGossip` component alongside DHT-based discovery.

## Unit Tests

Unit tests live in `pkg/discovery/gossip_test.go` and cover announcement
handling, nil safety, own-key filtering, and exchange-mode construction.

Run them with:

```bash
go test ./pkg/discovery -run TestHandle -v -count=1
```

## Manual Integration Test (Requires WireGuard)

This verifies gossip inside an actual mesh using the `--gossip` flag.

1. Build the binary: `go build -o wgmesh`.
2. On three nodes (A, B, C) run:
   - `./wgmesh init --secret` on one node and copy the secret.
   - `./wgmesh join --secret "<SECRET>" --gossip --log-level debug` on all nodes.
3. Validate behavior:
   - On A and B, confirm they connect (via DHT exchange).
   - After B is connected to A, start C. C should learn about B via gossip
     through A (watch for `[Gossip]` log entries and `DiscoveredVia` entries
     tagged with `gossip` or `gossip-transitive`).

Notes:
- The `--gossip` flag enables in-mesh peer propagation via UDP gossip messages.
- Gossip uses the same UDP socket as peer exchange, routing ANNOUNCE messages
  to the gossip handler.

## Troubleshooting

- Ensure all nodes use the same secret (gossip key and port are derived from it).
- Check UDP access on the derived gossip port:
  - `wgmesh status --secret "<SECRET>"` prints `Gossip Port`.
- If you only see `dht` discoveries and no `gossip`, confirm the `--gossip` flag
  is passed and that peers have mesh IPs assigned.
