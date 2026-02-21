---
tldr: A structured JSON logging middleware captures every chimney request; output feeds an in-memory ring buffer (1000 lines); a /api/logs/stream SSE endpoint tails the buffer in real time so table.beerpub.dev can show live chimney activity without SSH access.
category: feature
---

# Chimney observability — structured request logging with log ring buffer and SSE stream

## Target

Live, in-process observability for the running chimney server.
Three components: a request logging middleware, a ring buffer that retains recent output, and an SSE endpoint that streams the buffer to table.beerpub.dev.
No external logging infrastructure required.

## Behaviour

### Request logging middleware

Every inbound HTTP request is wrapped and logged as a single JSON line on completion:

```json
{
  "time": "2026-02-21T12:28:00Z",
  "method": "GET",
  "path": "/api/github/pulls",
  "status": 200,
  "latency_ms": 4,
  "cache_hit": true,
  "request_id": "cr_a3f2b1",
  "bytes": 18432
}
```

Fields:
- `time` — RFC3339 UTC.
- `method`, `path` — from request.
- `status` — response status code.
- `latency_ms` — time from first byte in to last byte out.
- `cache_hit` — true if served from cache (Dragonfly or in-memory) without upstream fetch.
- `request_id` — unique per-request ID; set as `X-Request-ID` response header.
  Format: random hex string (e.g. 8 bytes). If client sends `X-Request-ID`, echo it back.
- `bytes` — response body size in bytes.

All routes are wrapped, including `/healthz`, `/api/*`, and static file serving.
Errors (5xx) are logged at the same level — no special treatment needed; the status field carries the signal.

### Log ring buffer

A fixed-size in-memory ring buffer retains the last **1000 log lines** (JSON strings).
Both the request middleware and all `log.Print*` calls in chimney write to the ring.
The ring is a goroutine-safe circular buffer: when full, the oldest entry is overwritten.

The ring serves two purposes:
1. Provides the tail for the SSE stream (new subscribers receive recent history).
2. Retains context across brief disconnects without requiring a persistent external log store.

### SSE endpoint (`GET /api/logs/stream`)

Returns a `text/event-stream` response.

On connect:
1. Send the last N lines from the ring buffer as individual `data:` events (N = configurable, default 100).
2. Subscribe to new log entries; emit each as a `data:` event as it arrives.
3. Send a comment (`: heartbeat`) every 30s to keep the connection alive through proxies.

Each event is one JSON log line.
Client disconnects (context cancellation) stop the goroutine; no leak.

No authentication on the stream by default — restrict at the reverse proxy layer (Caddy allow-list or IP restriction). This keeps chimney's implementation simple while the caller enforces access control.

### Panic recovery middleware

Wraps all handlers.
On panic: recovers, logs a structured JSON entry with `"level": "panic"`, `stack` (truncated to 4KB), and the original `path`.
Responds with 500 to the client.
Increments a global panic counter (surfaced via `/api/metrics`).

## Design

- **Ring buffer over external log sink**: no Loki, no ELK, no sidecar. The buffer fits in memory (<1MB for 1000 × ~300 bytes); restarts naturally truncate it. This matches chimney's lightweight deployment model.
- **Structured JSON over text logs**: machine-readable from day one. table.beerpub.dev can filter/highlight on `status >= 500` or `cache_hit == false` without parsing.
- **SSE over WebSocket**: SSE is unidirectional (server → client), works through HTTP/1.1 proxies without upgrade, and is native to `EventSource` in browsers. Sufficient for a log tail; no bidirectional channel needed.
- **Last 100 lines on connect**: gives table immediate context when it opens the stream, without replaying the entire buffer.
- **`X-Request-ID` echo**: if the incoming request already carries a request ID (e.g. from Caddy's upstream), it's echoed back; otherwise one is generated. This enables end-to-end correlation between the Caddy access log and chimney's request log.
- **Heartbeat comment**: SSE connections through reverse proxies (Caddy, nginx) time out if idle. A 30s comment prevents this without sending fake data events.

## Interactions

- `cmd/chimney/main.go` — wraps all `mux.HandleFunc` registrations with the logging middleware.
- `/api/logs/stream` — new route registered alongside existing routes.
- `/api/metrics` — consumes panic counter and request counters from this component.
- table.beerpub.dev — connects to SSE stream via `EventSource`; renders live log tail.
- Caddy reverse proxy — may enforce IP allow-list on `/api/logs/stream`.

## Mapping

> [[cmd/chimney/main.go]]
