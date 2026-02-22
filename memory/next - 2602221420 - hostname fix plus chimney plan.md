# Next — 2602221420 — hostname fix plus chimney plan

## 1 — Active Plan: [[plan - 2602211419 - chimney integration observability deploy status and cache control]]

Open actions (phases 2–5):

- 1.1 — Phase 2: Replace `log.Print*` with slog + OTEL log bridge (action 6)
- 1.2 — Phase 2: Add panic recovery middleware (action 7)
- 1.3 — Phase 2: Add request log middleware (action 8)
- 1.4 — Phase 3: Promote cache counters; add Dragonfly + rate-limit gauges (action 9)
- 1.5 — Phase 3: Add request metrics; wire panics + deploy-event counters (action 10)
- 1.6 — Phase 4: Add deploy event ring buffer + `POST /api/deploy/events` (action 11)
- 1.7 — Phase 4: Add `GET /api/deploy/status` (action 12)
- 1.8 — Phase 4: Add CI deploy hook to `chimney-deploy.yml` (action 13)
- 1.9 — Phase 5: Implement `POST /api/cache/invalidate` (action 14)
- 1.10 — Phase 5: Register `/api/cache/invalidate` route (action 15)

## 2 — Issue #181 fix (done this session)

PR #332 open — fix(rpc): propagate hostname through peers.list and peers.get
=> hostname now flows daemon peerstore → RPCPeerData → rpc.PeerData → PeerInfo JSON → CLI table
=> regression tests added in `pkg/rpc/integration_test.go`

## Pending PRs

- PR #330 — smoke tests unconditional (auto-merge pending)
- PR #331 — research PDFs in docs/research/ (auto-merge pending)
- PR #332 — issue #181 hostname fix (auto-merge pending)

Which items to work on?
