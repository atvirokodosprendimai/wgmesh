# Postmortem: chimney.beerpub.dev 503 after blue/green deploy rewrite

| Field | Value |
|---|---|
| Title | chimney.beerpub.dev 503 after blue/green deploy rewrite |
| Incident # | 001 |
| Date | 2026-02-20 → 2026-02-21 |
| Authors | (session) |
| Status | Resolved |
| Duration | ~10h 52m (23:21 → 10:13 CET) |

---

## Summary

After merging PR #321 (blue/green deploy rewrite), the automatic deploy fired and failed:
`bluegreen.sh` was copied to origins but `compose.origin.yml` was not.
Six cascading issues were discovered and fixed during the remediation.
All fixes landed by 10:13 CET; `healthz` returned `{"status":"ok"}` confirming resolution.

---

## Impact

- 100% of `chimney.beerpub.dev` requests returned 503 for ~11 hours
- Dragonfly cache in-memory fallback kept chimney app running internally

---

## Root Causes

1. **Deploy incomplete** — `chimney-deploy.yml` SCPed only `bluegreen.sh`, not
   `compose.origin.yml` / `Caddyfile.origin` that it depends on
2. **SSH key injection broken** — cloud-init `users:` stanza didn't write
   `authorized_keys`; SSH to origins was impossible
3. **Stale DNS** — `chimney.beerpub.dev` pointed to an unlabeled server from a
   prior provision cycle (not cleaned by teardown)
4. **Caddyfile syntax invalid** — global `{ admin ... }` block placed *after*
   the site block; Caddy requires it first → exit code 1 on startup
5. **wgmesh iptables conflict** — wgmesh's `NET_ADMIN` rewrote iptables,
   blocking Docker's DNAT for bridge-mode Caddy on ports 80/443
6. **Origin Caddy redirected HTTP→HTTPS** — `chimney.beerpub.dev { tls { ... } }`
   caused 308 redirects; edge health checks expected 200 → all backends
   unhealthy → 503

---

## Trigger

Merge of PR #321 (`3143e23`) automatically triggered `chimney-build.yml` → `chimney-deploy.yml`.

---

## Detection

Automated health poll task (polling `/healthz` every 60s) detected 503s and notified via task notification.

---

## Resolution

| Time (CET) | Commit | Fix |
|---|---|---|
| 00:05 | `5cd44de` | SCP compose+caddy alongside bluegreen.sh in deploy |
| 07:20 | `292b306` | SSH key via `write_files` (cloud-init) |
| 07:28 | `0241084` | `--ssh-key` flag on `hcloud server create` |
| 07:55 | `a6fc566` | Remove dragonfly health-check dependency for chimney-blue |
| 08:51 | `8daf778` | Caddy `network_mode: host` (avoids wgmesh iptables) |
| 09:08 | `4e6aca8` | Global block before site block in Caddyfiles |
| 10:11 | `02735ee` | Origin Caddy `auto_https off` + `:80` site block |
| 10:13 | — | Bootstrap completed, healthz 200 |

---

## Action Items

| # | Item | Status |
|---|---|---|
| 1 | SCP compose+caddy alongside bluegreen.sh in deploy workflow | ✅ done |
| 2 | SSH key via `write_files` in cloud-init | ✅ done |
| 3 | Fix Caddyfile global block ordering | ✅ done |
| 4 | Origin Caddy `auto_https off` (HTTP-only internal) | ✅ done |
| 5 | Caddy `network_mode: host` for wgmesh compat | ✅ done |
| 6 | Add Caddyfile syntax validation to CI | ✅ done |
| 7 | Tag all servers with `service=chimney` or add DNS cleanup to teardown | ✅ done |
| 8 | Add e2e smoke test step to `chimney-deploy.yml` | ✅ done |
| 9 | Document wgmesh iptables behaviour in deploy README | ✅ done |

---

## Lessons Learned

### What went well

- DragonflyDB in-memory fallback kept chimney functional internally throughout
- Caddy TLS cert acquisition was near-instant once port 80 was reachable
- Health poll task gave early, clear signal of 503
- Blue/green design itself worked correctly once bootstrap was right
- wgmesh peer discovery converged quickly on nbg1

### What went wrong

- Deploy workflow was incomplete after the infra rewrite (no deploy-time file sync)
- Cloud-init SSH approach was fragile and untested
- Caddyfile syntax errors (block ordering) had no pre-deploy validation
- Origin served TLS when it should be HTTP-only (architecture mismatch in config)
- Stale DNS from unlabeled prior server wasn't caught by teardown

### Where we got lucky

- Dragonfly cache degraded gracefully (no data loss, in-memory fallback)
- No customer-facing service depended solely on chimney during the outage
- Let's Encrypt issued cert in seconds once ACME challenge was reachable

---

## Timeline

See Resolution table above.
Full commit log on branch `task/eidos-init`.
