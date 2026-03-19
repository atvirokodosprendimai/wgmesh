# Specification: Issue #395

## Classification
documentation

## Deliverables
documentation

## Problem Analysis

The current Lighthouse service is designed as a multi-tenant CDN control plane (specified in
`eidos/spec - lighthouse - cdn control plane with rest api dragonfly store xds and federated sync.md`).
It handles generic HTTP/HTTPS origin proxying with simple health checks, TLS termination via
Let's Encrypt, and federation between lighthouse nodes via UDP LWW sync over the WireGuard mesh.

The service registration CLI (`wgmesh service add/list/remove`, implemented in `service.go`) already
lets mesh nodes publish local services to the Lighthouse control plane, resulting in a
`<name>.<mesh-id>.wgmesh.dev` domain backed by reverse-proxy edge nodes.

However, AI workloads (inference endpoints such as Ollama, vLLM, LM Studio, llama.cpp) have
fundamentally different traffic characteristics compared to ordinary HTTP services:

- **Long-lived requests**: LLM completions typically take 5–120 seconds; the current
  `HealthCheck.timeout` (5 s default) and edge proxy defaults would prematurely abort them.
- **Streaming responses**: `/api/generate` (Ollama), `/v1/completions` with `stream: true` (OpenAI
  protocol) use chunked transfer encoding or server-sent events (SSE); intermediate proxies must not
  buffer the full response.
- **GPU resource scarcity**: A single backend can only serve one or a few concurrent requests; naive
  round-robin load balancing sends requests to busy GPUs, causing queuing inside the backend rather
  than at the gateway where back-pressure can be signalled.
- **Model-aware routing**: A mesh may expose multiple models (`ollama`, `mistral`, `llava`) on
  different ports or nodes; clients need URL-based routing by model name rather than pure domain
  routing.
- **Token-based rate limiting**: Standard RPS bucket rate limiting is inappropriate; limits should be
  expressed in tokens-per-minute or requests-per-model to match API provider conventions.
- **OpenAPI self-description for LLM agents**: The current `/v1/openapi.json` endpoint is noted as
  "designed for LLM agents" in the eidos spec but does not yet describe AI-specific fields.

The evolution plan must bridge the gap from the current generic ingress design to a purpose-built
AI service gateway without breaking existing generic service registrations.

## Implementation Tasks

### Task 1: Add an `eidos/` architecture decision document for the AI gateway evolution

**File to create:** `eidos/spec - lighthouse - ai service gateway evolution.md`

The document must use the same YAML front-matter format as other eidos specs:

```
---
tldr: <one-sentence summary>
category: core
---
```

The body must cover the following sections in order, using the exact headings below:

#### Section: AI Service Traffic Characteristics

Describe why AI inference workloads differ from generic HTTP services. Include concrete numbers:

- Typical LLM completion latency range: 5–120 seconds (varies with model size and hardware).
- Streaming protocols: chunked transfer encoding (HTTP/1.1), SSE (`text/event-stream`).
- Concurrency constraint: GPU backends typically handle 1–4 concurrent requests before queuing
  increases latency super-linearly.
- Payload sizes: prompt + context can reach 128 KB; streamed output tokens arrive 20–80 ms apart.

#### Section: Current Lighthouse Capabilities (Baseline)

Enumerate what Lighthouse already provides that is reusable without modification:

1. **Site registration** — `POST /v1/sites` stores `mesh_ip`, `port`, `protocol`, `HealthCheck`.
2. **Domain mapping** — `<name>.<mesh-id>.wgmesh.dev` via managed DNS.
3. **TLS termination** — Let's Encrypt `auto` mode via edge Caddy nodes.
4. **xDS / Caddyfile config** — `GET /v1/xds/config` and `GET /v1/xds/caddyfile` for edge node
   auto-configuration.
5. **Origin health checking** — dual-source health (lighthouse-originated probes + edge-reported
   results); 2-failure / 2-pass threshold.
6. **Federated sync** — UDP LWW replication between lighthouse nodes for HA.
7. **Per-org token-bucket rate limiting** — `pkg/ratelimit` with `X-RateLimit-*` headers.
8. **Service registration CLI** — `wgmesh service add/list/remove` in `service.go`.

#### Section: Required Changes for AI Service Ingress

List each change, the component it touches, and the exact field or behaviour modification:

**A. Extended `Site` type — AI metadata fields**

Add the following optional fields to the `Site` struct (in the extracted lighthouse repo's
`pkg/lighthouse/types.go`). Go field names use PascalCase; JSON tags map to `snake_case` for the
wire format:

```go
Backend             string `json:"backend,omitempty"`              // "ai" | "" (empty = generic HTTP)
ModelName           string `json:"model_name,omitempty"`           // e.g. "llama3" — model-based routing
MaxConcurrent       int    `json:"max_concurrent,omitempty"`       // 0 = unlimited; >0 = cap concurrent per origin
StreamingMode       string `json:"streaming_mode,omitempty"`       // "sse" | "chunked" | "off"
InferenceTimeoutSecs int   `json:"inference_timeout_secs,omitempty"` // per-request timeout; 0 → default 300
```

These fields are stored in Dragonfly under the existing `lh:site:<id>` key (JSON marshalling is
additive — existing sites that omit the fields behave identically to today).

**B. `POST /v1/sites` request body — AI fields**

Extend `CreateSiteRequest` in `pkg/lighthouse/api.go` (lighthouse repo) to accept the new fields.
Validation rules:
- `backend` must be `"ai"` or `""`.
- If `backend == "ai"` and `streaming_mode` is unset, default to `"chunked"`.
- If `backend == "ai"` and `inference_timeout_secs` is 0, default to `300`.
- `max_concurrent` must be >= 0.

**C. Caddyfile generation — streaming and timeout overrides**

Update `GET /v1/xds/caddyfile` handler (`pkg/lighthouse/xds.go`, lighthouse repo) to emit
AI-aware Caddyfile directives when `site.Backend == "ai"`:

```
# AI-backend Caddyfile block generated for an AI service (Backend="ai")
ollama.<mesh-id>.wgmesh.dev {
    reverse_proxy 10.42.0.5:11434 {
        transport http {
            response_header_timeout 300s
            dial_timeout 10s
        }
        flush_interval -1          # disable response buffering (enables streaming)
        header_up X-Forwarded-For {remote_host}
    }
    tls { ... }
    header X-Served-By "wgmesh-edge"
    header X-CDN "wgmesh"
}
```

Key directives:
- `flush_interval -1` disables Caddy's response buffering, required for SSE and chunked streaming.
- `response_header_timeout` set to `site.InferenceTimeoutSecs` seconds (default 300).
- `max_concurrent` concurrency limiting: Caddy v2.7+ supports `limits { max_header_size ... }` and
  the `caddy-ratelimit` module for request-level caps. The generated Caddyfile must require at
  minimum **Caddy v2.7**. When `max_concurrent > 0`, emit:
  ```
  # Requires caddy-ratelimit module (github.com/mholt/caddy-ratelimit)
  rate_limit {
      zone ai_concurrency_{$SITE_ID} {
          key  {http.request.remote_ip}
          events {max_concurrent}
          window 1s
      }
  }
  ```
  If the `caddy-ratelimit` module is unavailable, set `max_concurrent = 0` in the generated block
  and log a warning at Lighthouse startup.

**D. xDS config — streaming cluster attributes**

Update `GET /v1/xds/config` handler to include AI cluster metadata:

```json
{
  "clusters": [
    {
      "name": "ollama.<mesh-id>.wgmesh.dev",
      "endpoints": [{"mesh_ip": "10.42.0.5", "port": 11434}],
      "ai": {
        "backend": "ai",
        "model_name": "llama3",
        "streaming_mode": "chunked",
        "inference_timeout_secs": 300,
        "max_concurrent": 2
      }
    }
  ]
}
```

Envoy-based edges translate `streaming_mode: chunked` to `stream_idle_timeout` and
`max_connection_duration` in the cluster config. Document that this is Envoy-version-dependent
and must be verified at deployment.

**E. Service CLI — AI flags**

Add the following flags to `wgmesh service add` (in `service.go` in this repo):

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--backend` | string | `""` | Set to `ai` for AI inference endpoints |
| `--model` | string | `""` | Model name (e.g. `llama3`) — sets `ModelName` |
| `--max-concurrent` | int | `0` | Max concurrent requests per origin |
| `--streaming` | string | `chunked` | Streaming mode: `sse`, `chunked`, or `off` |
| `--inference-timeout` | duration | `5m` | Per-request timeout for inference endpoints |

The `--inference-timeout` flag accepts Go duration strings (e.g., `5m`, `300s`, `2m30s`).
It is converted to integer seconds when constructing `CreateSiteRequest.InferenceTimeoutSecs`
via `int(timeout.Seconds())`. Sub-second values (e.g., `500ms`) are rounded down to 0, which
triggers the server-side default of 300 seconds; the CLI must warn when this occurs:
```
Warning: --inference-timeout 500ms rounds to 0 seconds; using server default of 300 seconds.
```

When `--backend ai` is passed without `--model`, print a warning:
```
Warning: --model not set; this service will be accessible by domain only, not by model-specific URL paths.
```

The new flags are passed in the `CreateSiteRequest` body sent to `POST /v1/sites` via the
`lighthouse-go` client SDK. The `lighthouse-go` SDK's `CreateSiteRequest` struct must be extended
to include the AI fields (this is a change to the `lighthouse-go` repo, not this repo).

**F. OpenAPI spec update**

Update the `/v1/openapi.json` handler (lighthouse repo, `pkg/lighthouse/api.go`) to document all
new AI fields in the `CreateSiteRequest` and `Site` schemas. The description for `backend: "ai"`
must note that streaming, extended timeouts, and concurrency limits are automatically applied to
the generated edge configuration.

#### Section: Migration Plan — Discovery to Managed Ingress

This section documents the staged migration path for a node running `wgmesh join` that wants to
expose an AI service through the managed ingress.

**Stage 0 (current state): Manual registration**

```
mesh node                  Lighthouse API             Edge node
   |                           |                          |
   |-- wgmesh service add ---->|                          |
   |   (mesh_ip, port)         |-- xDS pull (30s) ------->|
   |                           |                          |
external client ----------- DNS ------> edge node -> mesh node
```

Discovery and ingress are fully decoupled. The mesh node participates in peer discovery (DHT,
LAN, gossip, GitHub registry) independently of its service registration. The managed ingress
layer only needs the `mesh_ip` and `port`; it does not interact with the discovery system at all.

This stage requires no code changes to the discovery layer.

**Stage 1 (target): Daemon-assisted auto-registration**

When a node runs `wgmesh join --service ollama:11434 --backend ai`, the daemon calls
`wgmesh service add` internally after the WireGuard interface is up and a mesh IP is assigned.
This removes the need to run a separate `wgmesh service add` command.

Implementation path (outside the scope of this spec; captured here for sequencing):
1. Add `--service <name>:<port>` and `--backend` flags to `wgmesh join` (in `main.go` / daemon config).
2. After `daemon.assignMeshIP()` succeeds, call `serviceAddCmd()` programmatically or as a
   subprocess.
3. On `wgmesh leave` / daemon shutdown, call `wgmesh service remove <name>` to deregister.

**Stage 2 (future): Automatic discovery-to-ingress bridge**

A future `wgmesh service discover` command scans peers from the RPC socket
(`pkg/rpc` — `peers.list` method) and proposes services for registration:

```
$ wgmesh service discover
Found 3 peers with open ports:
  10.42.0.5:11434  (ollama API, detected via HTTP fingerprint)
  10.42.0.6:8080   (unknown HTTP service)
  10.42.0.7:7860   (Gradio UI detected)

Register 10.42.0.5:11434 as service 'ollama'? [y/N]
```

This requires HTTP fingerprinting of common AI frameworks (Ollama API, OpenAI-compatible, Gradio,
ComfyUI) — out of scope here but noted as the natural Stage 2 successor.

#### Section: Integration with Service Registration CLI

The `wgmesh service` subcommand in `service.go` is the primary user-facing integration point.
The AI-specific changes (`--backend`, `--model`, `--streaming`, `--max-concurrent`,
`--inference-timeout`) extend the existing command surface without breaking backwards
compatibility.

The `lighthouse-go` client SDK (external repo `github.com/atvirokodosprendimai/lighthouse-go`)
must expose `CreateSiteRequest.Backend`, `CreateSiteRequest.ModelName`, etc. before the CLI
changes can be completed. The wgmesh repo has a `go.mod` dependency on this SDK; once the SDK is
updated and a new version tagged, run `go get github.com/atvirokodosprendimai/lighthouse-go@<new-version>`
and `go mod tidy`.

Affected files in this repo:
- `service.go` — add AI flags to `serviceAddCmd()`; pass new fields in `lighthouse.CreateSiteRequest{}`
- `go.mod` / `go.sum` — updated SDK version after lighthouse-go publishes AI fields

#### Section: Performance and Scaling Considerations

**Connection pooling**

Caddy edge nodes maintain persistent connections to origin mesh IPs. For AI backends, set
`keepalive 16` in the `reverse_proxy` transport block to avoid connection setup latency on each
inference call. The number 16 is a suggested starting point for single-GPU nodes; multi-GPU or
batched-inference backends may need higher values.

**Request queuing**

When `max_concurrent > 0`, requests that exceed the limit should receive `HTTP 503` immediately,
rather than being queued at the proxy. The `Retry-After` header value should reflect estimated
wait time:
- A conservative fixed value of `5` seconds is appropriate when the estimated queue drain time
  is unknown (e.g. first request denied).
- If the edge tracks average inference duration (`avg_duration_secs`), emit
  `Retry-After: <avg_duration_secs>` to give clients a realistic retry window.
- Clients are expected to apply their own exponential backoff on top of this hint; `Retry-After`
  is a minimum interval, not a guarantee.

The alternative (queuing at the proxy) risks silent request accumulation and makes latency
unpredictable; immediate 503 with `Retry-After` is preferred.

**Streaming buffer sizes**

Caddy's default write buffer is 4 KB. AI streaming tokens are typically 4–20 bytes each but
arrive in bursts. Set `response_buffer_size 0` (no buffering) in the generated Caddyfile to
flush each token immediately. This reduces time-to-first-token (TTFT) as perceived by the client.

**Lighthouse node scaling**

The Lighthouse control plane is lightweight (Dragonfly + HTTP API). It does not sit in the data
path for proxied traffic — only for config pull (xDS/Caddyfile) and health aggregation. A single
Lighthouse node handles hundreds of registered services with sub-millisecond API response times.
Horizontal scaling (multiple lighthouse nodes) is already handled by the existing UDP LWW
federation described in the eidos spec.

**Edge node scaling**

Edge nodes are the data-path bottleneck. Each edge node runs Caddy and proxies requests to mesh
IPs. For AI workloads:
- Deploy edge nodes with ≥1 Gbps uplink (token streams are low-bandwidth but connection
  duration is high).
- Set Caddy's `max_connections` to match expected concurrent AI sessions.
- Monitor `X-Served-By` response header to identify overloaded edge nodes.

**Health check timeout alignment**

The existing `HealthCheck.timeout` field defaults to 5 s. For AI backends, the health check
endpoint must respond quickly regardless of inference load — the health endpoint tests server
liveness, not inference capacity. Recommended health check paths by framework:

| AI Framework | Recommended health path | Notes |
|---|---|---|
| Ollama | `/api/tags` | Returns model list; fast even under load |
| OpenAI-compatible (vLLM, llama.cpp) | `/health` | Standard OpenAI health path |
| LM Studio | `/v1/models` | Lists loaded models |
| Gradio | `/` with 2xx check | No dedicated health endpoint |
| ComfyUI | `/system_stats` | Returns GPU/memory stats |
| Generic | `/health` | Fall back to `/` if `/health` returns 404 |

Use a 2-second timeout for all AI health checks (not the `inference_timeout_secs` value).
The `InferenceTimeoutSecs` field
applies only to the per-request proxy timeout, not to health probes. When `wgmesh service add`
is called with `--backend ai` and no explicit `--health-path`, default to `/health` (not `/`)
to avoid accidental health checks against chatty root endpoints.

**DNS TTL**

The `wgmesh.dev` managed domain uses a short TTL (60 s) to allow rapid failover when a mesh node
changes its IP or goes offline. For AI services behind a stable lighthouse, a 5-minute TTL is
acceptable and reduces DNS resolver load.

## Affected Files

### This repository (`wgmesh`)

| File | Change |
|------|--------|
| `service.go` | Add `--backend`, `--model`, `--max-concurrent`, `--streaming`, `--inference-timeout` flags to `serviceAddCmd()`; pass new fields in `lighthouse.CreateSiteRequest{}` struct literal |
| `go.mod` / `go.sum` | Bump `github.com/atvirokodosprendimai/lighthouse-go` to a version that exposes AI fields in `CreateSiteRequest` |
| `eidos/spec - lighthouse - ai service gateway evolution.md` | New file: this architecture decision document (created by this spec task) |

### External repository (`lighthouse` / `lighthouse-go`)

These changes are outside this repo but are listed for sequencing:

| Repo | File | Change |
|------|------|--------|
| lighthouse | `pkg/lighthouse/types.go` | Add AI fields to `Site` struct |
| lighthouse | `pkg/lighthouse/api.go` | Accept + validate AI fields in `CreateSiteRequest`; update OpenAPI JSON |
| lighthouse | `pkg/lighthouse/xds.go` | Emit AI-aware Caddyfile and xDS cluster metadata |
| lighthouse-go | `client.go` | Add AI fields to `CreateSiteRequest` and `Site` types in the SDK |

## Test Strategy

### CLI tests (this repo)

File: `service_test.go` (extend existing test binary pattern from `main_test.go`)

1. Build test binary to `/tmp/wgmesh-test-395`.
2. Run `wgmesh service add mymodel :11434 --backend ai --model llama3 --inference-timeout 5m`
   with a mock Lighthouse HTTP server that captures the request body.
3. Assert that the captured `CreateSiteRequest` JSON contains:
   - `"backend": "ai"`
   - `"model_name": "llama3"`
   - `"inference_timeout_secs": 300`
   - `"streaming_mode": "chunked"` (default when `--backend ai` and `--streaming` not set)
4. Run without `--model` and assert that stderr contains the warning
   `Warning: --model not set`.
5. Run with `--backend ""` (default) and assert that AI fields are absent from the request body
   (backwards compatibility).

### Integration check (manual, against a real lighthouse instance)

```bash
# Register an AI service
wgmesh service add ollama :11434 \
  --secret $WGMESH_SECRET \
  --account $LIGHTHOUSE_API_KEY \
  --backend ai \
  --model llama3 \
  --max-concurrent 2 \
  --inference-timeout 5m

# Verify service appears with AI metadata
wgmesh service list --secret $WGMESH_SECRET --json \
  | jq '.[] | select(.name == "ollama") | {domain, backend, model_name, streaming_mode}'

# Curl through the edge node to verify streaming works end-to-end
curl -N https://ollama.<mesh-id>.wgmesh.dev/api/generate \
  -d '{"model":"llama3","prompt":"hello","stream":true}' \
  | head -5
```

## Estimated Complexity
medium
