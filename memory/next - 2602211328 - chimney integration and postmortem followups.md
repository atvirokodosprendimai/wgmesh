---
generated: 2602211328
---

# Next — 2602211328

## Sources checked

- Active plans: none (main pull plan complete)
- Todos: none
- Goodjobs: none
- Planned items ({[!]}): none
- Open inline comments: none

## Items

1 - Implement chimney integration (specs are done, no plan yet)
  - 1.1 — Plan implementation phases for chimney×table.beerpub.dev integration
    OTEL instrumentation → deploy status+hook → cache control → metrics
    => from brainstorm [[brainstorm - 2602211225 - chimney integration with table.beerpub.dev]]
    Specs ready: observability, deploy status, metrics, cache control

2 - Open postmortem action items [[postmortem - 2602211013 - chimney 503 incident]]
  - 2.1 — Add Caddyfile syntax validation to CI (`caddy validate` step in chimney-build.yml)
  - 2.2 — Tag all servers `service=chimney` or add DNS cleanup to teardown workflow
  - 2.3 — Add e2e smoke test step to `chimney-deploy.yml` (healthz + probe after bootstrap)
  - 2.4 — Document wgmesh iptables behaviour in deploy README

3 - Moot / defer
  - 3.1 — Decide SSE vs polling for table live data → moot now that Coroot handles live telemetry
