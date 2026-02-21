---
tldr: chimney is instrumented with the OpenTelemetry Go SDK — otelhttp wraps all handlers to emit per-request spans, OTEL metrics replace the Prometheus endpoint as primary output, structured slog output carries trace context for log-trace correlation; all telemetry is exported via OTLP to the Coroot agent running on table.beerpub.dev.
category: feature
---

# Chimney observability — OpenTelemetry instrumentation for Coroot traces, metrics, and logs

## Target

Full OpenTelemetry instrumentation for chimney, with the Coroot agent on `table.beerpub.dev`
as the collection and visualization backend.
Coroot already understands OTEL signals natively — SDK instrumentation adds the application-layer
detail (cache hit/miss, GitHub API calls, upstream latency) that eBPF auto-instrumentation cannot see.

## Behaviour

### OTEL SDK setup

Three providers initialised at startup, all exporting via OTLP to the Coroot agent:

- **TracerProvider** — `otlptracehttp` (HTTP/protobuf to `http://localhost:4318/v1/traces`).
- **MeterProvider** — `otlpmetrichttp` (HTTP/protobuf to `http://localhost:4318/v1/metrics`), 15s push interval.
- **LoggerProvider** — `otlploghttp` (HTTP/protobuf to `http://localhost:4318/v1/logs`); used as slog backend.

Configuration via standard OTEL env vars:
- `OTEL_EXPORTER_OTLP_ENDPOINT` — override collector address (default `http://localhost:4318`).
- `OTEL_SERVICE_NAME=chimney` — set at deployment time (or defaulted in code).
- `OTEL_RESOURCE_ATTRIBUTES` — optional extra resource labels (e.g. `deployment.slot=blue`).

On `SIGTERM` / `SIGINT`: `TracerProvider.Shutdown()`, `MeterProvider.Shutdown()`, `LoggerProvider.Shutdown()` with a 5s grace context to flush in-flight telemetry before exit.

### HTTP tracing middleware (`otelhttp`)

All routes wrapped with `otelhttp.NewHandler(mux, "chimney")`.

Automatically captures per-request spans with:
- `http.method`, `http.route`, `http.status_code`, `http.response_content_length`
- Span status set to `Error` for 5xx responses.

**Additional custom attributes set by chimney** (added via `labeler` pattern or response wrapper):
- `chimney.cache_hit` (`bool`) — whether the response was served from cache.
- `chimney.cache_tier` (`"dragonfly"` | `"memory"` | `"none"`) — which tier was hit.
- `chimney.github_path` (`string`) — the underlying GitHub API path for `/api/github/*` requests.

**`X-Trace-ID` response header**: the OTEL trace ID for the request is set as a response header.
Coroot's UI can look up the full trace by this ID; it also correlates with the Caddy access log if Caddy propagates W3C `traceparent`.

### GitHub upstream span

When a cache miss triggers a GitHub API fetch, a **child span** is created:

```
chimney.github_fetch
  github.api.path = "/repos/atvirokodosprendimai/wgmesh/pulls"
  github.api.status_code = 200
  github.conditional = true    # If-None-Match was sent
  github.not_modified = false  # 304 not returned
```

This span makes the upstream call visible in Coroot's service map and latency breakdown.
On GitHub error: `span.RecordError(err)` + `span.SetStatus(codes.Error, reason)`.

### Metrics

All counters promoted from `int64` globals to OTEL instruments (registered on the global MeterProvider).
The Prometheus text-format `/api/metrics` endpoint is replaced by OTEL metrics as the primary output.
(A Prometheus-compatible scrape endpoint can still be served via the OTEL `prometheus` exporter if needed as a secondary output — see metrics spec.)

| Instrument | Kind | Attributes | Description |
|---|---|---|---|
| `chimney.requests` | counter | `route`, `status_class` | Requests per route; `status_class` = 2xx/3xx/4xx/5xx |
| `chimney.request.duration` | histogram | `route` | Latency (ms); boundaries: 5, 25, 100, 500, 2000 |
| `chimney.cache.hits` | counter | `tier` | Cache hits by tier (dragonfly / memory) |
| `chimney.cache.misses` | counter | — | Cache misses (upstream required) |
| `chimney.cache.entries` | gauge | — | Current in-memory entry count (observed at collection) |
| `chimney.github.rate_limit.remaining` | gauge | — | Last observed `X-RateLimit-Remaining` |
| `chimney.github.rate_limit.reset` | gauge | — | Last observed `X-RateLimit-Reset` (Unix epoch) |
| `chimney.dragonfly.connected` | gauge | — | 1 = connected, 0 = not |
| `chimney.panics` | counter | — | Recovered panics |
| `chimney.deploy_events` | counter | `outcome` | Deploy events received (success / failure) |

### Structured logging with trace context

Replace `log.Print*` calls with `log/slog` using the OTEL log bridge backend (`go.opentelemetry.io/contrib/bridges/otelslog`).

Each log record emits with trace context (`trace_id`, `span_id`) automatically when called within a span context.
This enables **log-trace correlation** in Coroot: clicking a trace shows associated log lines.

Log record for each completed request (written by the middleware):

```json
{
  "level": "INFO",
  "msg": "request",
  "method": "GET",
  "route": "/api/github/pulls",
  "status": 200,
  "latency_ms": 4,
  "cache_hit": true,
  "cache_tier": "dragonfly",
  "bytes": 18432,
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7"
}
```

### Panic recovery middleware

Wraps all handlers before the OTEL middleware.

On panic:
1. `span.RecordError(panicErr)` on the current span.
2. `span.SetStatus(codes.Error, "panic")`.
3. Logs the panic via slog at `ERROR` level with `stack` attribute (truncated to 4KB).
4. Increments `chimney.panics` counter.
5. Responds with 500.

## Design

- **OTEL over custom ring buffer + SSE**: Coroot provides trace/log/metric exploration, alerting, and service maps out of the box. Building a custom stream and UI in table.beerpub.dev would duplicate Coroot's capabilities. The SDK cost (a handful of deps, ~20 lines of setup) is far lower than maintaining a custom observability stack.
- **OTLP HTTP over gRPC**: `otlptracehttp` avoids the gRPC dependency bloat. HTTP/protobuf works through standard reverse proxies without h2c configuration.
- **`otelhttp` as the primary span source**: automatically handles W3C `traceparent` propagation — if a request carries a traceparent (e.g. from Caddy or a test), the span becomes a child, connecting edge-to-origin in Coroot's service map.
- **`X-Trace-ID` response header**: exposes the OTEL trace ID so that, when debugging a 503, the operator can paste the trace ID directly into Coroot's search without log grepping.
- **Child span for GitHub fetch**: the upstream HTTP call is the dominant latency source in chimney. Making it a named child span means Coroot's waterfall view shows exactly how long GitHub took vs. chimney's processing.
- **Gauge for `rate_limit.remaining`**: Coroot can alert when this approaches zero, preventing surprise GitHub rate limit errors.
- **slog + OTEL bridge over `log.Print*`**: the bridge routes structured log records through the OTEL LoggerProvider, preserving trace context without changing the logging call sites significantly.

## Interactions

- Coroot agent on `table.beerpub.dev` — receives OTLP spans, metrics, logs on port 4318.
  Coroot's eBPF layer already captures network-level data; SDK adds application-layer context.
- `OTEL_EXPORTER_OTLP_ENDPOINT` env var — configured in `compose.origin.yml` or systemd unit.
- `OTEL_SERVICE_NAME=chimney` — set at deploy time (e.g. via `--env` in compose).
- `otelhttp.NewHandler` — wraps chimney's `http.ServeMux`.
- `cmd/chimney/main.go` — SDK init on startup, graceful shutdown on signal.
- [[spec - chimney metrics - prometheus text format endpoint for cache and request counters]] — the Prometheus endpoint is superseded as the primary metrics path; it may be kept as an optional secondary output via the OTEL Prometheus exporter.
- [[spec - chimney deploy status - deploy event ingestion and last-deploy endpoint]] — deploy events contribute to `chimney.deploy_events` counter.
- [[spec - chimney cache control - runtime cache invalidation api]] — invalidation events contribute a log record and a span attribute.

## Mapping

> [[cmd/chimney/main.go]]
