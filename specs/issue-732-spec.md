   issue_title: Add email capture form to cloudroof.eu landing with trial nurture sequence
   issue_number: 732
   spec_file: specs/issue-732-spec.md

    __( O)>  ● new session · anthropic glm-4.6
   \____)    20260617_12 · /opt/wgmesh-checkout
     L L     goose is ready
I'll create an implementation specification for issue #732. First, let me understand the current codebase structure to see what already exists.
  ────────────────────────────────────────
  ▸ shell
    command: find /opt/wgmesh-checkout -type f -name "*.md" | grep -E "(spec|cloudroof|landing)" | head -20

/opt/wgmesh-checkout/pipeline-output/issue-729-spec.md
/opt/wgmesh-checkout/pipeline-output/issue-735-spec.md
/opt/wgmesh-checkout/pipeline-output/issue-539-spec.md
/opt/wgmesh-checkout/pipeline-output/issue-734-spec.md
/opt/wgmesh-checkout/pipeline-output/issue-733-spec.md
/opt/wgmesh-checkout/eidos/spec - rpc - unix socket json-rpc server and client for peer and daemon queries.md
/opt/wgmesh-checkout/eidos/spec - chimney deploy status - deploy event ingestion and last-deploy endpoint.md
/opt/wgmesh-checkout/eidos/spec - postmortem - incident documentation format.md
/opt/wgmesh-checkout/eidos/spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md
/opt/wgmesh-checkout/eidos/spec - lighthouse - cdn control plane with rest api dragonfly store xds and federated sync.md
/opt/wgmesh-checkout/eidos/spec - cli entry point - dual mode dispatch with daemon wiring and rpc server.md
/opt/wgmesh-checkout/eidos/spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md
/opt/wgmesh-checkout/eidos/spec - chimney cache control - runtime cache invalidation api.md
/opt/wgmesh-checkout/eidos/spec - chimney observability - opentelemetry instrumentation for coroot traces metrics and logs.md
/opt/wgmesh-checkout/eidos/spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md
/opt/wgmesh-checkout/eidos/spec - daemon reconciliation - peer config sync with relay routing fallback.md
/opt/wgmesh-checkout/eidos/spec - ai pipeline template - autonomous product loop for ai-native startups.md
/opt/wgmesh-checkout/eidos/spec - mesh state and access control - centralized node registry with policy-filtered peer configuration.md
/opt/wgmesh-checkout/eidos/spec - privacy dandelion - stem fluff routing for announcement privacy.md
/opt/wgmesh-checkout/eidos/spec - discovery lan and stun - multicast peer discovery and nat type detection.md
  ────────────────────────────────────────
  ▸ tree
    path: /opt/wgmesh-checkout
    depth: 2

cmd/  [515]
  eidos-lint/  [31]
    main.go  [31]
  status-gen/  [484]
    main.go  [168]
    render.go  [159]
    render_test.go  [157]
company/  [1K]
  loop-history/  [273]
    20260304-assessment.md  [24]
    20260305-assessment.md  [24]
    20260306-assessment.md  [24]
    20260307-assessment.md  [24]
    20260308-assessment.md  [24]
    20260309-assessment.md  [25]
    20260310-assessment.md  [24]
    20260311-assessment.md  [20]
    20260312-assessment.md  [17]
    20260313-assessment.md  [22]
    20260314-assessment.md  [22]
    20260315-assessment.md  [23]
  scripts/  [1K]
    goose-build-context.sh  [46]
    posthog-emit.sh  [62]
    pr-review-merge.sh  [778]
    pr-review-merge_test.sh  [88]
    sanitise.sh  [51]
    test-pr-review-merge.sh  [434]
  system-prompt.md  [114]
deploy/  [274]
  edge/  [126]
    setup.sh  [126]
  lighthouse/  [148]
    setup.sh  [148]
docs/  [10K]
  brainstorms/  [379]
    2026-05-05-bet-a-show-hn-demo-requirements.md  [214]
    2026-05-08-feature-ledger-pim-requirements.md  [165]
  dogfooding/  [111]
    README.md  [80]
    stability-log.md  [31]
  ideation/  [202]
    2026-05-05-wgmesh-bet-a-ideation.md  [202]
  plans/  [3K]
    2026-03-11-001-feat-interface-name-validation-and-faq-plan.md  [116]
    2026-03-11-002-docs-faq-mesh-secret-arbitrary-strings-plan.md  [95]
    2026-03-12-001-feat-custom-mesh-subnet-plan.md  [156]
    2026-03-12-003-chore-release-v020-plan.md  [94]
    2026-03-12-003-fix-gate-releases-behind-integration-tests-plan.md  [189]
    2026-03-12-004-fix-integration-test-data-plane-checksum-mismatch-plan.md  [152]
    2026-03-13-001-fix-pr439-copilot-review-doc-fixes-plan.md  [28]
    2026-03-13-002-fix-goose-review-zai-endpoint-and-error-detection-plan.md  [81]
    2026-03-13-003-fix-nat-punch-self-connection-prevention-plan.md  [136]
    2026-04-27-001-feat-mentisdb-workflow-instrumentation-plan.md  [430]
    2026-05-06-001-feat-bet-a-show-hn-demo-plan.md  [835]
    2026-05-06-002-fix-auto-verify-network-bug-fixes-plan.md  [383]
    2026-05-08-001-feat-feature-ledger-pim-plan.md  [422]
  pulse-reports/  [1K]
    2026-05-06_13-42.md  [38]
    2026-05-08_20-30.md  [69]
    2026-05-09_05-52.md  [54]
    2026-05-09_17-02.md  [52]
    2026-05-09_20-32.md  [48]
    2026-05-11_18-53.md  [36]
    2026-05-14_20-14.md  [36]
    2026-05-15_08-47.md  [37]
    2026-05-16_08-03.md  [37]
    2026-05-17_08-20.md  [37]
    2026-05-18_10-01.md  [37]
    2026-05-19_09-40.md  [37]
    2026-05-20_09-30.md  [37]
    2026-05-21_09-38.md  [37]
    2026-05-22_09-20.md  [37]
    2026-05-23_08-17.md  [37]
    2026-05-24_08-27.md  [37]
    2026-05-25_10-03.md  [37]
    2026-05-26_09-54.md  [37]
    2026-05-27_09-42.md  [37]
    2026-05-28_09-59.md  [37]
    2026-05-29_09-52.md  [37]
    2026-05-30_08-26.md  [37]
    2026-05-31_08-43.md  [37]
    2026-06-01_11-31.md  [37]
    2026-06-02_10-21.md  [37]
    2026-06-03_10-56.md  [37]
  research/  [0]
    a-practical-guide-to-building-agents.pdf  [0]
    practices-for-governing-agentic-ai-systems.pdf  [0]
  solutions/  [0]
    integration-issues/  [0]
    logic-errors/  [0]
    test-failures/  [0]
  use-cases/  [589]
    README.md  [21]
    hybrid-site-to-site.md  [137]
    managed-fleet.md  [165]
    multi-cloud.md  [120]
    remote-dev-team.md  [146]
  FAQ.md  [115]
  access-control.md  [313]
  centralized-mode.md  [195]
  compat-tracking.md  [27]
  evaluation-checklist.md  [288]
  index.html  [1K]
  install-verification.md  [50]
  pilot-evaluation-guide.md  [413]
  pipeline-flow.d2  [426]
  pipeline-flow.png  [0]
  pipeline-flow.svg  [896]
  quickstart.md  [334]
  troubleshooting.md  [75]
eidos/  [3K]
  eidosmeta/  [582]
    meta.go  [276]
    meta_test.go  [306]
  chimney.md  [41]
  reference - agentic engineering patterns - willison guide on agent-driven development.md  [81]
  seed.md  [34]
  spec - ai pipeline template - autonomous product loop for ai-native startups.md  [223]
  spec - centralized SSH deploy - diff-based persistent WireGuard config pushed via SSH.md  [63]
  spec - chimney - dashboard server with github api proxy and two-layer cache.md  [139]
  spec - chimney cache control - runtime cache invalidation api.md  [98]
  spec - chimney deploy status - deploy event ingestion and last-deploy endpoint.md  [125]
  spec - chimney metrics - prometheus text format endpoint for cache and request counters.md  [98]
  spec - chimney observability - opentelemetry instrumentation for coroot traces metrics and logs.md  [146]
  spec - cli entry point - dual mode dispatch with daemon wiring and rpc server.md  [123]
  spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md  [170]
  spec - daemon health - dual signal health monitoring with eviction and temporary offline.md  [64]
  spec - daemon lifecycle - secret-derived identity with interface setup and hot-reload.md  [51]
  spec - daemon reconciliation - peer config sync with relay routing fallback.md  [65]
  spec - daemon support - cache persistence collision resolution epoch management route sync and systemd.md  [77]
  spec - discovery dht core - bittorent mainline dht announce query persistence and endpoint detection.md  [96]
  spec - discovery dht rendezvous - event-driven punch with introducer selection and github registry bootstrap.md  [123]
  spec - discovery gossip - in-mesh udp peer broadcast.md  [70]
  spec - discovery lan and stun - multicast peer discovery and nat type detection.md  [80]
  spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md  [123]
  spec - first-customer - roadmap to first paying customer.md  [438]
  spec - lighthouse - cdn control plane with rest api dragonfly store xds and federated sync.md  [203]
  spec - mesh state and access control - centralized node registry with policy-filtered peer configuration.md  [48]
  spec - peer store - thread-safe registry with endpoint ranking and pub-sub.md  [65]
  spec - postmortem - incident documentation format.md  [102]
  spec - privacy dandelion - stem fluff routing for announcement privacy.md  [95]
  spec - rpc - unix socket json-rpc server and client for peer and daemon queries.md  [86]
  spec - service cli - register local services for managed ingress via lighthouse.md  [182]
evolution/  [2K]
  wgmesh-cdn-slides.html  [1K]
  wgmesh-cdn-slides.html.bak  [1K]
features/  [2K]
  archived/  [2K]
    IMPLEMENTATION_PLAN.md  [1K]
    bootstrap.md  [1K]
formal/  [432]
  Makefile  [86]
  wgmesh.spthy  [346]
memory/  [3K]
  brainstorm - 2602211225 - chimney integration with table.beerpub.dev.md  [151]
  brainstorm - 2602240805 - first paying customer for wgmesh.md  [158]
  brainstorm - 2603012302 - what do we need for a first customer.md  [146]
  decision - 2603040808 - company loop uses openrouter not direct anthropic.md  [26]
  decision - 2603151026 - decouple lighthouse from wgmesh into separate repo.md  [126]
  human.md  [28]
  learning - 2603041547 - autonomous development preferred over confirmation prompts.md  [14]
  learning - 2603041547 - branch scope drift when accumulating unrelated work.md  [10]
  learning - 2603041547 - sweep planned items after plan completion.md  [7]
  next - 2602211328 - chimney integration and postmortem followups.md  [30]
  next - 2602220031 - chimney observability phases 2-5.md  [34]
  next - 2602221420 - hostname fix plus chimney plan.md  [30]
  next - 2602221718 - chimney observability phase 2 and org dashboard.md  [22]
  next - 2603011100 - aggregated actionable items.md  [38]
  next - 2603011503 - aggregated actionable items.md  [26]
  next - 2603011717 - aggregated actionable items.md  [22]
  next - 2603011800 - aggregated actionable items.md  [26]
  next - 2603012116 - aggregated actionable items.md  [37]
  next - 2603040741 - aggregated actionable items.md  [47]
  next - 2603040854 - aggregated actionable items.md  [41]
  next - 2603040923 - aggregated actionable items.md  [51]
  next - 2603041151 - aggregated actionable items.md  [33]
  next - 2603151252 - aggregated actionable items.md  [39]
  outreach - 2602240805 - show hn post draft.md  [57]
  outreach - 2602240805 - stargazer dm template.md  [100]
  pickmeup - 2603101105 - last 10 days activity.md  [91]
  pickmeup - 2603151011 - last 5 days activity.md  [70]
  plan - 2026-02-20-2343 - pull subsections for wgmesh codebase.md  [96]
  plan - 2602211419 - chimney integration observability deploy status and cache control.md  [174]
  plan - 2602221444 - chimney org dashboard and repo split.md  [99]
  plan - 2602282207 - push subsections for autonomous company loop.md  [80]
  plan - 2603011216 - fix company loop and restructure goose build.md  [113]
  plan - 2603012134 - distributable packages deb and nix via goreleaser.md  [74]
  plan - 2603040954 - migrate observation loop to ai-pipeline-template.md  [152]
  postmortem - 2602211013 - chimney 503 incident.md  [120]
  pull - 2026-02-20-2343 - chimney dashboard server.md  [69]
  pull - 2026-02-20-2343 - wgmesh codebase overview.md  [123]
  pull - 2602211106 - mesh state access control and centralized SSH deploy.md  [134]
  pull - 2602211113 - daemon package overview.md  [141]
  pull - 2602211143 - discovery package overview.md  [138]
  push - 2602282207 - implement autonomous company loop.md  [73]
  session - 2603011733 - goose pipeline completion and testing.md  [59]
packaging/  [33]
  postinstall.sh  [7]
  preremove.sh  [4]
  wgmesh.service  [22]
pipeline-output/  [10K]
  issue-539-spec.md  [335]
  issue-539.diff  [1K]
  issue-729-spec.md  [322]
  issue-729.diff  [2K]
  issue-733-spec.md  [208]
  issue-733.diff  [407]
  issue-734-spec.md  [147]
  issue-734.diff  [2K]
  issue-735-spec.md  [102]
  issue-735.diff  [3K]
  recipe-override-implement.yaml  [58]
  recipe-override-spec.yaml  [61]
pkg/  [26K]
  crypto/  [2K]
    CLAUDE.md  [13]
    capability_version.go  [8]
    derive.go  [300]
    derive_test.go  [448]
    encrypt.go  [106]
    encrypt_test.go  [319]
    envelope.go  [328]
    envelope_compat_test.go  [287]
    envelope_test.go  [363]
    membership.go  [53]
    membership_test.go  [110]
    password.go  [40]
    rotation.go  [144]
    rotation_test.go  [221]
  daemon/  [7K]
    CLAUDE.md  [15]
    cache.go  [158]
    cache_test.go  [106]
    collision.go  [177]
    collision_test.go  [123]
    config.go  [244]
    config_test.go  [179]
    daemon.go  [1K]
    daemon_test.go  [394]
    epoch.go  [46]
    executor.go  [92]
    health_test.go  [33]
    helpers.go  [390]
    helpers_test.go  [634]
    metrics.go  [144]
    metrics_test.go  [266]
    peerstore_compat.go  [24]
    peerstore_test.go  [479]
    relay_test.go  [403]
    reload_test.go  [274]
    routes.go  [147]
    routes_test.go  [436]
    systemd.go  [220]
    systemd_test.go  [428]
    validate.go  [9]
  discovery/  [5K]
    CLAUDE.md  [14]
    dht.go  [1K]
    dht_test.go  [370]
    exchange.go  [1K]
    exchange_test.go  [703]
    gossip.go  [330]
    gossip_test.go  [220]
    init.go  [18]
    lan.go  [253]
    lan_test.go  [32]
    registry.go  [412]
    stun.go  [332]
    stun_integration_test.go  [32]
    stun_test.go  [494]
  ifname/  [247]
    validate.go  [82]
    validate_test.go  [165]
  mesh/  [2K]
    CLAUDE.md  [7]
    account.go  [63]
    account_test.go  [72]
    deploy.go  [338]
    mesh.go  [266]
    mesh_test.go  [268]
    policy.go  [262]
    policy_test.go  [717]
    services.go  [73]
    services_test.go  [107]
    types.go  [60]
  node/  [500]
    store.go  [374]
    types.go  [126]
  pilot/  [3K]
    complete.go  [244]
    complete_test.go  [329]
    config.go  [59]
    metrics.go  [255]
    metrics_test.go  [277]
    pilot.go  [318]
    pilot_test.go  [435]
    report.go  [446]
    report_test.go  [424]
    validation.go  [267]
    validation_test.go  [420]
  privacy/  [455]
    dandelion.go  [301]
    dandelion_test.go  [154]
  ratelimit/  [350]
    limiter.go  [153]
    limiter_test.go  [197]
  routes/  [280]
    routes.go  [103]
    routes_test.go  [177]
  rpc/  [1K]
    client.go  [82]
    integration_test.go  [233]
    protocol.go  [77]
    protocol_test.go  [138]
    server.go  [428]
    server_test.go  [111]
  ssh/  [388]
    CLAUDE.md  [7]
    client.go  [149]
    routes.go  [121]
    wireguard.go  [111]
  wireguard/  [1K]
    CLAUDE.md  [12]
    apply.go  [252]
    config.go  [236]
    config_test.go  [630]
    convert.go  [23]
    keys.go  [35]
    persist.go  [134]
public/  [677]
  index.html  [289]
  vpn-alternative.md  [388]
scripts/  [3K]
  workflows/  [2K]
    e2e-stalled-watcher.js  [123]
    e2e-stalled-watcher.test.js  [250]
    e2e-verify-close-handler.js  [339]
    e2e-verify-close-handler.test.js  [690]
    impl-merged-close-handler.js  [434]
    impl-merged-close-handler.test.js  [951]
  pulse.sh  [470]
  verify-install.sh  [61]
  wait-for-clean-copilot.sh  [161]
specs/  [12K]
  implemented/  [536]
    issue-24-spec.md  [98]
    issue-43-spec.md  [278]
    issue-76-spec.md  [160]
  not-implemented/  [393]
    rpc-socket-interface-spec.md  [393]
  partial/  [630]
    issue-4-spec.md  [111]
    issue-81-spec.md  [519]
  STATUS.md  [93]
  issue-176-spec.md  [1K]
  issue-178-spec.md  [193]
  issue-181-spec.md  [222]
  issue-470-spec.md  [101]
  issue-475-spec.md  [308]
  issue-492-spec.md  [421]
  issue-496-spec.md  [572]
  issue-497-spec.md  [227]
  issue-499-spec.md  [308]
  issue-505-spec.md  [419]
  issue-508-spec.md  [289]
  issue-510-spec.md  [483]
  issue-512-spec.md  [531]
  issue-520-spec.md  [190]
  issue-523-spec.md  [88]
  issue-532-spec.md  [332]
  issue-536-spec.md  [875]
  issue-539-spec.md  [468]
  issue-545-spec.md  [252]
  issue-547-spec.md  [706]
  issue-550-spec.md  [763]
  issue-551-spec.md  [521]
  issue-556-spec.md  [277]
  issue-561-spec.md  [159]
  issue-571-spec.md  [432]
  issue-573-spec.md  [117]
  issue-587-spec.md  [172]
  issue-591-spec.md  [65]
  issue-609-spec.md  [123]
  issue-641-spec.md  [226]
  issue-714-spec.md  [143]
testdata/  [24]
  compat/  [0]
    envelope/  [0]
  script/  [24]
    status-json-with-secret.txtar  [9]
    status-json.txtar  [3]
    status-text.txtar  [10]
    version.txtar  [2]
testlab/  [5K]
  cloud/  [4K]
    chaos.sh  [481]
    gen-report.py  [317]
    lib.sh  [1K]
    provision.sh  [510]
    test-cloud.sh  [1K]
  nat-relay/  [246]
    docker-compose.yml  [114]
    run-test.sh  [132]
  README.md  [75]
  Vagrantfile  [100]
  lab.sh  [115]
  lima-bridge-lab.sh  [202]
  lima-lab.sh  [220]
  test-mesh.sh  [233]
wgmesh.dev/  [203]
  index.html  [203]
AGENTS.md  [156]
BUGFIX-network-format.md  [93]
CLAUDE.md  [98]
CONTRIBUTING.md  [160]
DOCKER-COMPOSE.md  [381]
DOCKER.md  [150]
Dockerfile  [60]
ENCRYPTION.md  [305]
FEATURE_MATRIX.md  [153]
GOSSIP_TESTING.md  [43]
Makefile  [40]
README.md  [337]
ROADMAP.md  [116]
STATUS.md  [66]
STRATEGY.md  [66]
TODO.md  [346]
docker-compose.yml  [96]
example.sh  [34]
flake.lock  [61]
flake.nix  [82]
go.mod  [49]
go.sum  [645]
index.html  [244]
main.go  [1K]
main_test.go  [371]
rnd.txt  [335]
service.go  [503]
service_test.go  [307]
sonar-project.properties  [10]
test-encryption.sh  [115]
test-route-cleanup.sh  [90]
test-route-diff-scenarios.md  [156]

  ────────────────────────────────────────
  ▸ shell
    command: grep -r "cloudroof" /opt/wgmesh-checkout --include="*.md" --include="*.html" --include="*.go" 2>/dev/null | head -20

/opt/wgmesh-checkout/STRATEGY.md:Operators wiring agent fleets to edge nodes (e.g. exposing an openclaw webserver via cloudroof in minutes) need fast, ad-hoc, secure meshes. Existing tools either treat edge as second-class (Tailscale), feel bolted-on (Cloudflare Tunnel), or "work" in demos and fail at minute-3 of real ad-hoc setup.
/opt/wgmesh-checkout/STRATEGY.md:Lighthouse CDN, managed ingress, the openclaw→cloudroof path. The use case Tailscale and Cloudflare both lose.
/opt/wgmesh-checkout/STRATEGY.md:Polar.sh billing, landing repositioning at cloudroof.eu, cost tracking, distribution (Show HN, tutorial, stargazer outreach).
/opt/wgmesh-checkout/STRATEGY.md:- **Hosted control plane for someone else's wgmesh deployment** — we are not Headscale-as-a-service. cloudroof.eu uses wgmesh internally as part of a bundled edge offering; we don't sell a control plane for your mesh.
/opt/wgmesh-checkout/company/loop-history/20260313-assessment.md:Stage 0, run 16. Development momentum continues with 22 PRs merged in 7 days (Marty, ~.~). Foundation stage blockers persist: service registration CLI spec still missing after 15 runs, and cost tracking remains unconfigured despite €2400 available capital being visible in state. Infrastructure stable (chimney, cloudroof up). The fn:dev pipeline shows only 1 issue, suggesting either completion or bottleneck in AI agent workflow.
/opt/wgmesh-checkout/company/loop-history/20260311-assessment.md:Stage 0, run 13. Development activity normalized: 7 PRs merged in 7 days, down from the 29-PR sprint. Only 1 fn:dev issue queued suggests pipeline is flowing but narrow. Infrastructure stable (chimney 703ms, cloudroof 88ms). Critical: cost tracking STILL unconfigured after 13 runs - available_capital exists (€2400) but no burn rate tracking. Service registration CLI spec remains missing - our Foundation exit blocker.
/opt/wgmesh-checkout/pkg/ifname/validate.go:					"letters, digits, underscores, or hyphens (e.g. wg0, cloudroof0, mesh-1)",
/opt/wgmesh-checkout/pkg/ifname/validate_test.go:		{"custom cloudroof0", "cloudroof0"},
/opt/wgmesh-checkout/pkg/ifname/validate_test.go:		{"non-utun custom", "cloudroof0", "utun<N>"},
/opt/wgmesh-checkout/memory/pull - 2026-02-20-2343 - wgmesh codebase overview.md:Separate server binary. REST API for peer registry, DNS management (Hetzner DNS), XDS (Envoy control plane), health reporting. Rate-limited, token-auth'd. Appears to serve `cloudroof.eu`.
/opt/wgmesh-checkout/memory/brainstorm - 2603012302 - what do we need for a first customer.md:- cloudroof.eu domain (product site placeholder)
/opt/wgmesh-checkout/memory/brainstorm - 2602240805 - first paying customer for wgmesh.md:A secondary CTA points to "cloudroof.eu" (unclear what that does).
/opt/wgmesh-checkout/memory/brainstorm - 2602240805 - first paying customer for wgmesh.md:- Clarify what cloudroof.eu is and whether it's the right destination for CTAs
/opt/wgmesh-checkout/memory/brainstorm - 2602240805 - first paying customer for wgmesh.md:- Clarify cloudroof.eu and make it a coherent part of the funnel, not a mystery link
/opt/wgmesh-checkout/memory/brainstorm - 2602240805 - first paying customer for wgmesh.md:- 6 — Clarify what cloudroof.eu is and whether it's still the right CTA destination
/opt/wgmesh-checkout/memory/plan - 2602282207 - push subsections for autonomous company loop.md:- [x] Create `company/health.json` — chimney + cloudroof endpoints
/opt/wgmesh-checkout/memory/plan - 2602282207 - push subsections for autonomous company loop.md:- [x] Create `company/scripts/collect-infra.sh` — health checks for chimney + cloudroof
/opt/wgmesh-checkout/memory/plan - 2603040954 - migrate observation loop to ai-pipeline-template.md:   - Remove wgmesh-specific references (Chimney, cloudroof, Lighthouse, managed ingress)
/opt/wgmesh-checkout/memory/plan - 2602211419 - chimney integration observability deploy status and cache control.md:   - => GitHub 5xx also marks span as Error; User-Agent domain fixed (cloudroof.eu→beerpub.dev)
/opt/wgmesh-checkout/ROADMAP.md:| 3 | Landing page repositioning (cloudroof.eu) | needs-human | Currently opaque to non-engineers. ICP needs to recognize themselves above the fold. |
  ────────────────────────────────────────
  ▸ shell
    command: find /opt/wgmesh-checkout -name "*.html" -type f | head -10

/opt/wgmesh-checkout/public/index.html
/opt/wgmesh-checkout/index.html
/opt/wgmesh-checkout/wgmesh.dev/index.html
/opt/wgmesh-checkout/evolution/wgmesh-cdn-slides.html
/opt/wgmesh-checkout/docs/index.html
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/public/index.html

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>wgmesh.dev - Secure, Decentralized Mesh Networking</title>
    <link rel="icon" href="/favicon.ico" type="image/x-icon">
    <style>
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            margin: 0;
            padding: 0;
            background-color: #f4f7f6;
            color: #333;
            line-height: 1.6;
        }
        .container {
            max-width: 1000px;
            margin: 30px auto;
            padding: 20px 30px;
            background-color: #ffffff;
            border-radius: 10px;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
        }
        header {
            background-color: #0056b3;
            color: white;
            padding: 30px 20px;
            text-align: center;
            border-bottom: 5px solid #004085;
        }
        header h1 {
            margin: 0;
            font-size: 2.8em;
            letter-spacing: 1px;
        }
        header p {
            font-size: 1.2em;
            margin-top: 10px;
        }
        h2 {
            color: #0056b3;
            font-size: 2em;
            margin-top: 40px;
            border-bottom: 2px solid #e0e0e0;
            padding-bottom: 10px;
        }
        section {
            margin-bottom: 30px;
            padding: 15px 0;
        }
        ul {
            list-style-type: disc;
            margin-left: 25px;
            padding: 0;
        }
        ul li {
            margin-bottom: 8px;
        }
        .pricing-section {
            text-align: center;
            background-color: #e6f2ff;
            padding: 40px 20px;
            border-radius: 8px;
            margin-top: 40px;
            margin-bottom: 40px;
            border: 1px dashed #a0d1ff;
        }
        .pricing-section h2 {
            margin-top: 0; /* Override default h2 margin-top */
            border-bottom: none; /* No border for this h2 */
        }
        .pricing-tiers {
            display: flex;
            justify-content: center;
            gap: 25px; /* Space between cards */
            flex-wrap: wrap; /* Allow wrapping on smaller screens */
            margin-top: 30px;
        }
        .tier-card {
            background-color: #ffffff;
            border: 1px solid #d1e0ff;
            border-radius: 10px;
            padding: 30px 20px;
            width: 280px; /* Fixed width for cards */
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.05);
            transition: transform 0.2s ease, box-shadow 0.2s ease;
            display: flex; /* Flexbox for internal alignment */
            flex-direction: column;
            justify-content: space-between; /* Push button to bottom */
            text-align: left;
        }
        .tier-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 8px 25px rgba(0, 0, 0, 0.1);
        }
        .tier-card h3 {
            color: #0056b3;
            font-size: 1.6em;
            margin-top: 0;
            margin-bottom: 15px;
            text-align: center;
        }
        .tier-card p {
            font-size: 1em;
            color: #555;
            margin-bottom: 25px;
            flex-grow: 1; /* Allow description to take available space */
            text-align: center;
        }
        .cta-button {
            display: inline-block;
            background-color: #28a745; /* Green for call to action */
            color: white;
            padding: 15px 35px;
            text-decoration: none;
            border-radius: 8px;
            font-size: 1.3em;
            font-weight: bold;
            transition: background-color 0.3s ease, transform 0.2s ease;
            box-shadow: 0 4px 10px rgba(40, 167, 69, 0.3);
            text-align: center;
            width: fit-content; /* Adjust width based on content */
            margin: 0 auto; /* Center button */
        }
        .tier-card .cta-button {
            margin-top: auto; /* Push button to the bottom of the card */
            width: 100%; /* Make button full width of card */
            padding: 12px 20px;
            font-size: 1.1em;
        }
        .cta-button:hover {
            background-color: #218838;
            transform: translateY(-2px);
        }
        .cta-button:active {
            transform: translateY(0);
            box-shadow: 0 2px 5px rgba(40, 167, 69, 0.3);
        }

        /* Feature the $20 tier */
        .tier-card.featured-tier {
            border-color: #28a745; /* Green border */
            box-shadow: 0 6px 20px rgba(40, 167, 69, 0.2);
            transform: scale(1.03); /* Slightly larger */
        }
        .tier-card.featured-tier:hover {
            transform: translateY(-8px) scale(1.05);
            box-shadow: 0 10px 30px rgba(40, 167, 69, 0.3);
        }
        .tier-card.featured-tier h3 {
            color: #28a745;
        }

        .links-section a {
            color: #007bff;
            text-decoration: none;
            margin: 0 15px;
            font-weight: bold;
        }
        .links-section a:hover {
            text-decoration: underline;
        }
        footer {
            text-align: center;
            padding: 25px;
            margin-top: 50px;
            background-color: #e9ecef;
            color: #6c757d;
            border-top: 1px solid #dee2e6;
            font-size: 0.9em;
        }
        footer a {
            color: #007bff;
            text-decoration: none;
            font-weight: bold;
        }
        footer a:hover {
            text-decoration: underline;
        }

        /* Responsive adjustments for pricing tiers */
        @media (max-width: 768px) {
            .pricing-tiers {
                flex-direction: column;
                align-items: center;
            }
            .tier-card {
                width: 90%; /* Take more width on smaller screens */
                max-width: 350px; /* But not too wide */
            }
        }
    </style>
</head>
<body>
    <header>
        <h1>wgmesh.dev</h1>
        <p>The ultimate solution for secure, decentralized, and resilient mesh networks.</p>
    </header>

    <div class="container">
        <section id="pricing" class="pricing-section">
            <h2>#pricing - Choose Your Sponsorship Tier</h2>
            <p>Support the development of wgmesh and gain exclusive benefits.</p>

            <div class="pricing-tiers">
                <div class="tier-card">
                    <h3>$5 Founding Member</h3>
                    <p>Get early access to community discussions and exclusive updates.</p>
                    <a href="https://polar.sh/checkout?productId=3f5d75de-936b-49d8-a21b-4b79d9fd22c1" class="cta-button" target="_blank" rel="noopener noreferrer">
                        Become a Founding Member
                    </a>
                </div>
                <div class="tier-card featured-tier">
                    <h3>$20 Edge Node</h3>
                    <p>Access advanced network features and prioritized support.</p>
                    <a href="https://polar.sh/checkout?productId=1927e637-4cfd-4c94-8bee-c5518803bc89" class="cta-button" target="_blank" rel="noopener noreferrer">
                        Sponsor an Edge Node
                    </a>
                </div>
                <div class="tier-card">
                    <h3>$100 Mesh Operator</h3>
                    <p>Receive dedicated consultations and influence future roadmap decisions.</p>
                    <a href="https://polar.sh/checkout?productId=eb20683e-55ea-4354-9d8c-070e55a4eff5" class="cta-button" target="_blank" rel="noopener noreferrer">
                        Become a Mesh Operator
                    </a>
                </div>
            </div>
        </section>

        <section id="features">
            <h2>#features</h2>
            <p>Discover the core capabilities that make wgmesh stand out:</p>
            <ul>
                <li><strong>Truly Decentralized:</strong> No single point of control or failure.</li>
                <li><strong>WireGuard Powered:</strong> Leveraging the simplicity and security of WireGuard for robust VPN tunnels.</li>
                <li><strong>Effortless Deployment:</strong> Streamlined setup and management for all skill levels.</li>
                <li><strong>Dynamic Routing:</strong> Adapts to network changes for maximum reliability.</li>
                <li><strong>Multi-platform Support:</strong> Seamless integration across various operating systems and devices.</li>
            </ul>
        </section>

        <section id="how-it-works">
            <h2>#how-it-works</h2>
            <p>wgmesh simplifies the creation and management of secure mesh networks:</p>
            <p>It acts as an intelligent overlay network, automatically discovering peers and establishing secure WireGuard tunnels. Configuration is abstracted, allowing you to define your network logic rather than struggling with low-level details. This results in a self-healing and robust network fabric.</p>
            <!-- Placeholder for diagrams or further explanation -->
        </section>

        <section id="install">
            <h2>#install</h2>
            <p>Getting wgmesh up and running is straightforward. Here's a brief overview:</p>
            <p>1. <strong>Download:</strong> Obtain the latest wgmesh package for your system.</p>
            <p>2. <strong>Configure:</strong> Define your network peers and their policies in a simple configuration file.</p>
            <p>3. <strong>Run:</strong> Start the wgmesh daemon, and watch your network form automatically.</p>
            <p>For detailed instructions, please refer to our <a href="README.md" target="_blank" rel="noopener noreferrer">official README</a>.</p>
        </section>

        <section id="modes">
            <h2>#modes</h2>
            <p>wgmesh supports various operational modes to suit your specific networking needs:</p>
            <ul>
                <li><strong>Peer-to-Peer Mesh:</strong> Direct connections between all participants.</li>
                <li><strong>Hub-and-Spoke:</strong> Centralized routing through a main node for specific scenarios.</li>
                <li><strong>Hybrid Topologies:</strong> Combine different modes for complex network architectures.</li>
            </ul>
        </section>

        <section id="story">
            <h2>#story</h2>
            <p>wgmesh was born from a need for simpler, more resilient, and truly decentralized networking solutions. Frustrated with the complexities and single points of failure in traditional VPNs and mesh implementations, our team embarked on a mission to build a tool that empowers users to control their networks with ease and security. The project continues to evolve with community feedback and cutting-edge advancements in network technology.</p>
        </section>

        <section class="links-section" style="text-align: center;">
            <h2>Explore Further</h2>
            <p>
                <a href="https://github.com/atvirokodosprendimai/wgmeshdev" target="_blank" rel="noopener noreferrer">GitHub Repository</a> |
                <a href="https://discord.gg/your-wgmesh-discord" target="_blank" rel="noopener noreferrer">Join Our Discord Community</a> |
                <a href="README.md" target="_blank" rel="noopener noreferrer">Detailed README</a>
            </p>
        </section>
    </div>

    <footer>
        <p>&copy; 2024 wgmesh.dev. All rights reserved.</p>
        <p>Powered by WireGuard®. <a href="https://polar.sh/" target="_blank" rel="noopener noreferrer">Payment via Polar.sh</a></p>
    </footer>
</body>
</html>
  ────────────────────────────────────────
  ▸ shell
    command: cat /opt/wgmesh-checkout/pipeline-output/issue-729-spec.md

# Specification: Issue #729 - Design referral program to leverage existing customer satisfaction

## Classification
feature

## Problem Analysis
wgmesh has achieved early customer satisfaction based on positive feedback from dogfooding users. Word-of-mouth is already happening organically. However, there is no structured mechanism to:

1. **Track and incentivize referrals** - Satisfied customers cannot be formally rewarded for bringing new users
2. **Measure referral effectiveness** - No visibility into which acquisition channels or users drive growth
3. **Sustain growth momentum** - Early adopter enthusiasm needs to be captured and amplified before it fades

Current state:
- No referral tracking system exists
- No customer account management beyond basic mesh configuration
- CLI-only tooling limits non-technical referral opportunities
- Dogfood users provide positive feedback but no structured NPS collection

This feature must integrate cleanly with wgmesh's two operational modes:
- **Centralized mode** (SSH-managed nodes): Likely referral source (IT/admin users)
- **Decentralized mode** (self-discovering mesh): Likely referral target (technical teams)

## Proposed Approach

### Phase 1: Account and referral identity infrastructure

**Task 1.1: Create account package**
Create new package `pkg/account/` with:

```go
// types.go
type AccountID string
type ReferralCode string

type Account struct {
    ID           AccountID
    Email        string // Optional, for reward delivery
    ReferralCode ReferralCode
    CreatedAt    time.Time
}

type Referral struct {
    ReferrerID  AccountID
    ReferredID  AccountID
    Code        ReferralCode
    ConvertedAt time.Time // When referred account completed first mesh setup
}
```

**Task 1.2: Referral code generation**
Implement cryptographically secure referral code generation in `pkg/account/code.go`:

```go
// GenerateCode creates a unique referral code for an account
// Format: [base32(account_id_hash)][checksum][version]
// Length: 12 characters, URL-safe, case-insensitive
func GenerateCode(accountID AccountID) (ReferralCode, error)

// ValidateCode verifies checksum and extracts account ID
func ValidateCode(code ReferralCode) (AccountID, error)
```

Use HKDF-SHA256 for code derivation (consistent with existing `pkg/crypto/derive.go`). Format:
- First 8 chars: Base32(HKDF(accountID, "referral"))
- Next 3 chars: CRC24 checksum (detect typos)
- Final 1 char: Version byte (currently '1')

**Task 1.3: Local account storage**
Create `pkg/account/store.go` with file-based storage:

```go
type Store struct {
    mu       sync.RWMutex
    path     string // ~/.wgmesh/accounts.json
    accounts map[AccountID]*Account
    referrals []*Referral
}

// CreateAccount generates a new account with referral code
func (s *Store) CreateAccount(email string) (*Account, error)

// GetByCode retrieves account by referral code
func (s *Store) GetByCode(code ReferralCode) (*Account, error)

// RecordReferral records a successful referral conversion
func (s *Store) RecordReferral(referrerID, referredID AccountID, code ReferralCode) error
```

Storage format: JSON file at `~/.wgmesh/accounts.json` with atomic writes (temp file + rename).

### Phase 2: CLI integration for referral flows

**Task 2.1: Add `wgmesh account` subcommand**
Extend `main.go` with new subcommand group:

```go
var cmdAccount = &cli.Command{
    Name:  "account",
    Usage: "Manage your wgmesh account and referrals",
    Subcommands: []*cli.Command{
        cmdAccountCreate,
        cmdAccountStatus,
        cmdAccountReferrals,
    },
}

var cmdAccountCreate = &cli.Command{
    Name:  "create",
    Usage: "Create a new account",
    Flags: []cli.Flag{
        &cli.StringFlag{
            Name:  "email",
            Usage: "Email address for reward delivery (optional)",
        },
        &cli.StringFlag{
            Name:  "referral-code",
            Usage: "Referral code if you were referred (optional)",
        },
    },
    Action: func(ctx *cli.Context) error {
        // Create account, handle referral if present
    },
}
```

**Task 2.2: Implement status command**
```go
var cmdAccountStatus = &cli.Command{
    Name:  "status",
    Usage: "Show account details and referral code",
    Action: func(ctx *cli.Context) error {
        // Display:
        // - Account ID
        // - Referral code (prominent: "Share this code!")
        // - Total referrals
        // - Conversion rate (referrals that completed mesh setup)
    },
}
```

**Task 2.3: Implement referrals list command**
```go
var cmdAccountReferrals = &cli.Command{
    Name:  "referrals",
    Usage: "List all your successful referrals",
    Action: func(ctx *cli.Context) error {
        // Table output:
        // | Referred ID | Code Used | Converted At |
    },
}
```

### Phase 3: Conversion tracking

**Task 3.1: Track first-mesh completion**
Modify `pkg/daemon/daemon.go` to detect first successful mesh setup:

```go
// After first successful reconcile with >=1 peer:
if d.firstMeshComplete && !d.conversionRecorded {
    accountStore := getAccountStore()
    if account := accountStore.GetCurrent(); account != nil {
        if account.ReferredBy != "" {
            accountStore.RecordReferral(account.ReferredBy, account.ID, account.ReferralCode)
        }
    }
    d.conversionRecorded = true
}
```

**Task 3.2: Add telemetry events**
Extend `pkg/pilot/metrics.go` with referral events:

```go
type ReferralEvent struct {
    EventType string // "code_generated", "conversion", "reward_earned"
    AccountID AccountID
    Code      ReferralCode
    Timestamp time.Time
}
```

### Phase 4: Reward structure (foundation for future payments)

**Task 4.1: Define reward tiers**
Create `pkg/account/rewards.go` with reward logic:

```go
type RewardTier struct {
    ReferralsRequired int
    RewardType        string // "credit", "extension", "premium_feature"
    Value             int    // Duration in months or credit amount
}

var RewardTiers = []RewardTier{
    {1, "credit", 1},         // 1 referral = 1 month credit
    {5, "extension", 3},      // 5 referrals = 3 month extension
    {10, "premium_feature", 0}, // 10 referrals = feature unlock
}
```

**Task 4.2: Calculate pending rewards**
```go
func (s *Store) CalculateRewards(accountID AccountID) []Reward {
    // Count successful referrals
    // Match against reward tiers
    // Return unclaimed rewards
}
```

**Task 4.3: Display pending rewards in status**
Add reward summary to `wgmesh account status` output:

```
Your Referral Stats:
  Referral Code: ABC123XYZ4A
  Total Referrals: 7
  Converted: 5 (71%)

Pending Rewards:
  - 5 referrals: 3-month service extension (unclaimed)
```

### Phase 5: Documentation and UX

**Task 5.1: Create referral guide**
Add `docs/referral-program.md` with:
- How to get your referral code
- Sharing guidelines (what to avoid)
- Reward structure explanation
- FAQ ("Can I refer my team?", "When do rewards apply?")

**Task 5.2: In-app messaging**
Add post-install messaging in daemon startup:

```
First mesh setup complete!
Your referral code: ABC123XYZ4A
Share it with your team to earn rewards.
Run 'wgmesh account status' for details.
```

## Acceptance Criteria

- [ ] **Code generation**: Referral codes are 12 characters, URL-safe, typo-resistant (checksum validated)
- [ ] **Account creation**: `wgmesh account create` generates unique account with code in <100ms
- [ ] **Referral tracking**: Referred accounts record referrer ID and code on conversion
- [ ] **Status display**: `wgmesh account status` shows code, referral count, conversion rate
- [ ] **Reward calculation**: System correctly calculates pending rewards based on referral tiers
- [ ] **Data persistence**: Account/referral data survives daemon restarts (atomic file writes)
- [ ] **CLI usability**: All commands work with `--help`, clear error messages for invalid codes
- [ ] **Test coverage**: `pkg/account/` package has >80% test coverage
- [ ] **Documentation**: Referral guide exists with clear examples
- [ ] **No secrets**: No API keys or credentials hardcoded; codes are public identifiers

## Out of Scope

- **Payment processing** - No actual credit card processing or payouts in this phase
- **Email verification** - Email is optional, no verification flow
- **Web dashboard** - All referral management via CLI
- **Multi-level referrals** - No "refer-a-friend-of-friend" tracking
- **Fraud detection** - Basic code validation only; no abuse detection yet
- **API integration** - No external service dependencies (Stripe, SendGrid, etc.)
- **Compliance tools** - No tax reporting or 1099 generation
- **Time-limited campaigns** - Rewards are persistent, no expiration logic

## Affected Files

### New files
- `pkg/account/types.go` - Core account and referral data structures
- `pkg/account/code.go` - Referral code generation/validation
- `pkg/account/store.go` - Persistent account storage
- `pkg/account/rewards.go` - Reward tier calculations
- `pkg/account/store_test.go` - Tests for store persistence
- `pkg/account/code_test.go` - Tests for code generation/validation
- `docs/referral-program.md` - User-facing documentation

### Modified files
- `main.go` - Add `account` subcommand group
- `pkg/daemon/daemon.go` - Track first-mesh completion for conversion
- `go.mod` / `go.sum` - No new external dependencies (stdlib + existing crypto only)

## Test Strategy

### Unit tests
- `TestGenerateCodeUnique` - Generate 1000 codes, verify no collisions
- `TestValidateCodeChecksum` - Test typo detection (single-char errors)
- `TestStoreAtomicWrite` - Verify crash-safe persistence (write/interrupt/read)
- `TestReferralTracking` - Verify referrer ID preserved through conversion
- `TestRewardCalculation` - Test edge cases (0 referrals, exact tier boundaries)

### Integration tests
- `TestCLIReferralFlow` - End-to-end: create account → get code → refer → convert
- `TestConversionTracking` - Simulate daemon reconcile, verify conversion recorded
- `TestConcurrentAccountAccess` - Race detection on store access (`-race` flag)

### Manual verification
1. Create two accounts, verify codes are unique
2. Use referral code during account creation
3. Complete mesh setup with referred account
4. Verify referrer's status shows new referral
5. Check atomic persistence (kill daemon during write, verify data intact)

## Estimated Complexity

**Medium** - ~1,200 lines of new code across 7 files

Breakdown:
- Account/store logic: 400 lines
- Code generation/validation: 150 lines
- CLI integration: 200 lines
- Tests: 350 lines
- Documentation: 100 lines

**Risks:**
- File storage race conditions (mitigated by atomic writes)
- Code collisions at scale (mitigated by HKDF + sufficient entropy)
- Conversion tracking false positives (mitigated by explicit first-mesh check)

**Dependencies:**
- None (uses existing `pkg/crypto/` for HKDF)
- Go stdlib only (encoding, crypto, filesystem)-32602: Could not parse tool arguments: {"path":"/opt/wgmesh-checkout/specs/issue-732-spec.md","content":"# Specification: Issue #732 - Add email capture form to cloudroof.eu landing with trial nurture sequence\n\n## Classification\nfeature\n\n## Problem Analysis\n\nCurrent state:\n- The cloudroof.eu landing page (at `/public/index.html` in this repo) exists but has no mechanism for visitor engagement\n- No email capture means no way to build a prospect list or follow up with interested visitors\n- Visitors cannot easily start a trial without immediate commitment\n- No nurture sequence exists to educate prospects and drive conversions\n\nBusiness impact:\n- **Lost leads**: Interested visitors leave without留下 contact information\n- **No remarketing**: Cannot re-engage visitors who showed interest but didn't convert\n- **Higher acquisition cost**: Every conversion must come from paid or organic traffic without email list building\n- **No trial pipeline**: Missing \"try before buy\" path that reduces conversion friction\n\nTechnical context:\n- cloudroof.eu is referenced in STRATEGY.md as the product site for the wgmesh-based edge offering\n- The landing page at `/public/index.html` currently shows a sponsorship pricing model via Polar.sh (unrelated to cloudroof.eu's B2B positioning)\n- No backend infrastructure exists for email storage or automation\n- wgmesh codebase is Go-based but this feature requires static web + email service integration\n\n## Proposed Approach\n\n### Phase 1: Email capture form and landing page updates\n\n**Task 1.1: Update cloudroof.eu landing page**\nModify `/public/index.html` to serve as cloudroof.eu landing with:\n\n1. **Hero section with email capture**\n   - Headline: \"Secure Mesh Networking for Edge Infrastructure\"\n   - Subheadline emphasizing use case (ad-hoc fleet wiring, managed ingress)\n   - Email form with \"Start Free Trial\" CTA\n   - Trust indicators (no credit card required, 14-day trial, self-service setup)\n\n2. **Email capture form HTML**\n```html\n<form id=\"email-capture\" method=\"POST\" action=\"/api/trial-signup\">\n  <input type=\"email\" name=\"email\" placeholder=\"work@company.com\" required>\n  <input type=\"hidden\" name=\"source\" value=\"cloudroof-landing\">\n  <button type=\"submit\">Start 14-Day Free Trial</button>\n  <div class=\"form-message\" style=\"display:none\"></div>\n</form>\n```\n\n3. **Form validation and UX**\n   - Client-side email format validation\n   - Loading state during submission\n   - Success message: \"Check your inbox for trial setup instructions\"\n   - Error handling for duplicate signups\n\n**Task 1.2: Create dedicated cloudroof landing content**\nAdd new section below hero with:\n\n- Value proposition: \"Wire your edge fleet to the internet in minutes\"\n- Use case examples: \"Expose web servers, APIs, internal tools without public IPs\"\n- How it works: 3-step visual (install wgmesh → register services → get HTTPS endpoint)\n- Integration with existing `/public/vpn-alternative.md` content as \"Why wgmesh\" section\n\n### Phase 2: Email storage and validation API\n\n**Task 2.1: Create trial signup API endpoint**\nCreate new file `cmd/lighthouse/trial.go` (lighthouse is the CDN control plane):\n\n```go\npackage main\n\ntype TrialSignupRequest struct {\n    Email  string `json:\"email\"`\n    Source string `json:\"source\"` // \"cloudroof-landing\", \"docs\", \"referral\"\n}\n\ntype TrialSignupResponse struct {\n    Success   bool   `json:\"success\"`\n    Message   string `json:\"message\"`\n    TrialID   string `json:\"trial_id,omitempty\"`\n    Exists    bool   `json:\"exists\"` // true if email already registered\n}\n\n// POST /api/trial-signup\nfunc handleTrialSignup(w http.ResponseWriter, r *http.Request) {\n    var req TrialSignupRequest\n    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {\n        http.Error(w, \"invalid request\", 400)\n        return\n    }\n\n    // Validate email format\n    if !isValidEmail(req.Email) {\n        respondJSON(w, TrialSignupResponse{Success: false, Message: \"Invalid email format\"})\n        return\n    }\n\n    // Check for existing trial\n    if exists := trialStore.Exists(req.Email); exists {\n        respondJSON(w, TrialSignupResponse{Success: true, Exists: true, \n            Message: \"You already have an active trial. Check your inbox.\"})\n        return\n    }\n\n    // Generate trial ID and store\n    trialID := generateTrialID()\n    trial := &Trial{\n        ID:        trialID,\n        Email:     req.Email,\n        Source:    req.Source,\n        CreatedAt: time.Now(),\n    }\n\n    if err := trialStore.Create(trial); err != nil {\n        http.Error(w, \"storage error\", 500)\n        return\n    }\n\n    // Trigger welcome email (async)\n    go nurture.SendWelcomeEmail(trial)\n\n    respondJSON(w, TrialSignupResponse{\n        Success: true,\n        Message: \"Check your inbox for trial setup instructions\",\n        TrialID: trialID,\n    })\n}\n```\n\n**Task 2.2: Implement trial storage**\nAdd `pkg/lighthouse/trial/store.go`:\n\n```go\ntype Trial struct {\n    ID        string\n    Email     string\n    Source    string\n    CreatedAt time.Time\n    Status    string // \"pending\", \"active\", \"converted\", \"expired\"\n}\n\ntype Store struct {\n    db *sql.DB // SQLite for simplicity\n}\n\nfunc (s *Store) Create(trial *Trial) error\nfunc (s *Store) Exists(email string) bool\nfunc (s *Store) GetByID(id string) (*Trial, error)\nfunc (s *Store) GetByEmail(email string) (*Trial, error)\n```\n\nDatabase schema (SQLite):\n```sql\nCREATE TABLE trials (\n    id TEXT PRIMARY KEY,\n    email TEXT UNIQUE NOT NULL,\n    source TEXT,\n    created_at DATETIME,\n    status TEXT DEFAULT 'pending'\n);\nCREATE INDEX idx_trials_email ON trials(email);\n```\n\n### Phase 3: Email nurture sequence\n\n**Task 3.1: Define nurture sequence timeline**\nCreate `pkg/nurture/sequence.go` with email schedule:\n\n```go\ntype EmailTemplate struct {\n    Delay       time.Duration // Send after trial signup\n    Subject     string\n    TemplateID  string\n    TrackingID  string\n}\n\nvar NurtureSequence = []EmailTemplate{\n    {\n        Delay:      0 * time.Minute, // Immediate\n        Subject:    \"Welcome to cloudroof.eu - Your 14-Day Trial Starts Now\",\n        TemplateID: \"trial-welcome\",\n        TrackingID: \"trial_welcome\",\n    },\n    {\n        Delay:      1 * time.Day,\n        Subject:    \"Day 1: Wire your first service to the internet\",\n        TemplateID: \"trial-day-1\",\n        TrackingID: \"trial_day_1\",\n    },\n    {\n        Delay:      3 * time.Day,\n        Subject:    \"Day 3: Pro tip - Custom domains for your services\",\n        TemplateID: \"trial-day-3\",\n        TrackingID: \"trial_day_3\",\n    },\n    {\n        Delay:      7 * time.Day,\n        Subject:    \"Week 1 check-in - How's your trial going?\",\n        TemplateID: \"trial-week-1\",\n        TrackingID: \"trial_week_1\",\n    },\n    {\n        Delay:      11 * time.Day,\n        Subject:    \"3 days left - Extend your trial or upgrade\",\n        TemplateID: \"trial-reminder\",\n        TrackingID: \"trial_reminder\",\n    },\n}\n```\n\n**Task 3.2: Email template content**\nCreate templates directory `cmd/lighthouse/templates/`:\n\n**trial-welcome.html**:\n```html\n<h1>Your cloudroof.eu trial is ready</h1>\n<p>Hi {{.Email}},</p>\n<p>You're 14 days away from secure, managed edge ingress. Let's get started.</p>\n<h2>Step 1: Install wgmesh</h2>\n<pre><code>curl -sSL https://get.wgmesh.dev | sh</code></pre>\n<h2>Step 2: Register your edge service</h2>\n<pre><code>wgmesh service register --name my-api --port 8080</code></pre>\n<h2>Step 3: Access your service</h2>\n<p>Your service is now available at: <code>{{.ServiceURL}}</code></p>\n<p>Questions? Reply to this email - we answer every one.</p>\n```\n\n**trial-day-1.html**: Focus on first win (expose a service)\n**trial-day-3.html**: Feature reveal (custom domains)\n**trial-week-1.html**: Check-in and help offer\n**trial-reminder.html**: Urgency and upgrade path\n\n**Task 3.3: Email sending implementation**\nCreate `pkg/nurture/sender.go`:\n\n```go\ntype Sender struct {\n    from     string\n    replyTo  string\n    provider string // \"sendgrid\", \"aws-ses\", \"smtp\"\n}\n\nfunc (s *Sender) SendTemplate(to string, templateID string, data map[string]interface{}) error\n```\n\nUse environment-based provider selection:\n```bash\n# Default: SMTP (self-hosted)\nNURTURE_EMAIL_PROVIDER=smtp\nSMTP_HOST=smtp.example.com\nSMTP_USER=apikey\nSMTP_PASS=sendgrid-api-key\n\n# Production: SendGrid\nNURTURE_EMAIL_PROVIDER=sendgrid\nSENDGRID_API_KEY=SG.xxx\n```\n\n### Phase 4: Nurture automation and tracking\n\n**Task 4.1: Create nurture worker**\nAdd `cmd/lighthouse/nurture-worker.go`:\n\n```go\nfunc main() {\n    store := trial.NewStore()\n    sender := nurture.NewSender()\n\n    ticker := time.NewTicker(1 * time.Hour)\n    for range ticker.C {\n        trials, _ := store.GetPendingNurture()\n\n        for _, trial := range trials {\n            elapsed := time.Since(trial.CreatedAt)\n            \n            for _, email := range nurture.NurtureSequence {\n                if elapsed >= email.Delay && !emailSent(trial, email.TrackingID) {\n                    sender.SendTemplate(trial.Email, email.TemplateID, map[string]interface{}{\n                        \"Email\": trial.Email,\n                        \"TrialID\": trial.ID,\n                    })\n                    markEmailSent(trial, email.TrackingID)\n                }\n            }\n        }\n    }\n}\n```\n\n**Task 4.2: Email tracking**\nAdd trial_email_log table:\n```sql\nCREATE TABLE trial_email_log (\n    id INTEGER PRIMARY KEY AUTOINCREMENT,\n    trial_id TEXT,\n    tracking_id TEXT,\n    sent_at DATETIME,\n    opened_at DATETIME,\n    clicked_at DATETIME,\n    FOREIGN KEY (trial_id) REFERENCES trials(id)\n);\n```\n\n**Task 4.3: Unsubscribe and preferences**\nAdd `POST /api/unsubscribe` endpoint:\n```go\nfunc handleUnsubscribe(w http.ResponseWriter, r *http.Request) {\n    trialID := r.URL.Query().Get(\"trial\")\n    email := r.URL.Query().Get(\"email\")\n    \n    trialStore.UpdateStatus(trialID, \"unsubscribed\")\n    \n    // Render confirmation page\n    renderTemplate(w, \"unsubscribed.html\", nil)\n}\n```\n\nInclude unsubscribe link in all email footers:\n```html\n<p style=\"font-size: 12px; color: #666;\">\n  <a href=\"{{.UnsubscribeURL}}\">Unsubscribe</a> | \n  <a href=\"{{.PreferencesURL}}\">Manage preferences</a>\n</p>\n```\n\n### Phase 5: Trial-to-production conversion\n\n**Task 5.1: Trial expiration handling**\nAdd to nurture-worker:\n\n```go\n// Check for expiring trials\ntrialsExpiring := store.GetExpiring(time.Now().Add(3 * time.Day))\nfor _, trial := range trialsExpiring {\n    // Send \"3 days left\" email if not already sent\n    if !emailSent(trial, \"trial_3day_reminder\") {\n        sender.SendTemplate(trial.Email, \"trial-expiring\", map[string]interface{}{\n            \"DaysLeft\": 3,\n            \"ExtendURL\": \"https://cloudroof.eu/extend?trial=\" + trial.ID,\n        })\n        markEmailSent(trial, \"trial_3day_reminder\")\n    }\n}\n\n// Check for expired trials\ntrialsExpired := store.GetExpired(time.Now())\nfor _, trial := range trialsExpired {\n    trialStore.UpdateStatus(trial.ID, \"expired\")\n    sender.SendTemplate(trial.Email, \"trial-expired\", nil)\n}\n```\n\n**Task 5.2: Conversion tracking**\nAdd to `pkg/lighthouse/trial/store.go`:\n\n```go\nfunc (s *Store) MarkConverted(trialID string, planID string) error {\n    _, err := s.db.Exec(`\n        UPDATE trials \n        SET status = 'converted', converted_at = ?, plan_id = ?\n        WHERE id = ?\n    `, time.Now(), planID, trialID)\n    return err\n}\n\nfunc (s *Store) ConversionMetrics() (*ConversionMetrics, error) {\n    // Returns: signup_count, active_count, converted_count, expired_count\n}\n```\n\n**Task 5.3: Conversion email**\nCreate `trial-converted.html` template:\n```html\n<h1>Welcome to cloudroof.eu Pro</h1>\n<p>Your trial has converted to the {{.PlanName}} plan.</p>\n<p>Next steps:</p>\n<ul>\n  <li>Configure your billing method</li>\n  <li>Review your service endpoints</li>\n  <li>Set up team access</li>\n</ul>\n```\n\n## Acceptance Criteria\n\n- [ ] **Landing page form**: Email capture form visible above fold, functional validation\n- [ ] **Form submission**: POST to `/api/trial-signup` returns success/error in <500ms\n- [ ] **Email validation**: Rejects invalid formats, detects existing emails\n- [ ] **Trial storage**: SQLite persists trials with unique email constraint\n- [ ] **Welcome email**: Sent immediately after signup, contains setup instructions\n- [ ] **Nurture sequence**: All 5 emails sent at correct intervals (0d, 1d, 3d, 7d, 11d)\n- [ ] **Email templates**: All templates render with trial-specific data\n- [ ] **Unsubscribe**: One-click unsubscribe functional in all emails\n- [ ] **Tracking**: Email sends logged to trial_email_log table\n- [ ] **Expiration**: Trials marked expired after 14 days, notification sent\n- [ ] **Conversion**: Converted trials tracked with plan and timestamp\n- [ ] **No secrets**: All credentials via environment variables, none in code\n- [ ] **Documentation**: Setup guide for email provider config and template customization\n\n## Out of Scope\n\n- **A/B testing** - No landing page variant testing in this phase\n- **Lead scoring** - No engagement-based scoring beyond trial status\n- **Marketing automation** - No complex drip campaign logic (fixed sequence only)\n- **CRM integration** - No Salesforce/HubSpot sync\n- **Advanced analytics** - No email open/click tracking beyond basic logging\n- **Multi-language** - English templates only\n- **Social proof** - No testimonial collection in this phase\n- **Referral integration** - Email capture independent of issue #729 referral system\n- **Payment processing** - Trial is self-service; payment handled separately post-trial\n- **SSO/enterprise** - No SAML/LDAP integration in trial flow\n\n## Affected Files\n\n### New files\n- `public/index.html` - Updated cloudroof.eu landing with email form (modifies existing)\n- `pkg/lighthouse/trial/store.go` - Trial storage and queries\n- `pkg/lighthouse/trial/store_test.go` - Trial storage tests\n- `pkg/nurture/sequence.go` - Nurture timeline and template definitions\n- `pkg/nurture/sender.go` - Email sending abstraction\n- `pkg/nurture/sender_test.go` - Email sender tests\n- `cmd/lighthouse/trial.go` - Trial signup API handler\n- `cmd/lighthouse/nurture-worker.go` - Background nurture email worker\n- `cmd/lighthouse/templates/trial-welcome.html` - Immediate welcome email\n- `cmd/lighthouse/templates/trial-day-1.html` - Day 1 tip email\n- `cmd/lighthouse/templates/trial-day-3.html` - Day 3 feature email\n- `cmd/lighthouse/templates/trial-week-1.html` - Week 1 check-in email\n- `cmd/lighthouse/templates/trial-reminder.html` - 3-day reminder email\n- `cmd/lighthouse/templates/trial-expired.html` - Expiration notice\n- `cmd/lighthouse/templates/trial-converted.html` - Conversion confirmation\n- `docs/cloudroof-trial-setup.md` - Trial system configuration guide\n\n### Modified files\n- `cmd/lighthouse/main.go` - Add trial API routes and nurture worker startup\n- `go.mod` / `go.sum` - Add email provider library (e.g., sendgrid-go or smtp wrapper)\n\n### Database migrations\n- `cmd/lighthouse/migrations/001_trials.sql` - Create trials and trial_email_log tables\n\n## Test Strategy\n\n### Unit tests\n- `TestTrialStorageCreateGet` - Verify trial creation and retrieval\n- `TestTrialExistsDuplicate` - Test unique email constraint\n- `TestEmailValidation` - Test valid/invalid email formats\n- `TestNurtureSequenceTiming` - Verify correct delays for each email\n- `TestTemplateRendering` - Ensure all templates render with sample data\n\n### Integration tests\n- `TestTrialSignupFlow` - End-to-end: form submit → API → storage → welcome email\n- `TestNurtureWorkerExecution` - Mock time, verify sequence sends in order\n- `TestUnsubscribeFlow` - Verify unsubscribe updates trial status\n- `TestExpirationHandling` - Mock expired trial, verify status change and email\n\n### Manual verification\n1. Visit cloudroof.eu landing page\n2. Submit email through capture form\n3. Receive welcome email within 1 minute\n4. Verify email contains valid setup instructions\n5. Check database for trial record\n6. Advance system time by 24 hours\n7. Verify Day 1 email sent\n8. Test unsubscribe link\n9. Verify no further emails received after unsubscribe\n\n### Email testing checklist\n- [ ] Test with Gmail (tabs, spam filtering)\n- [ ] Test with Outlook/Office 365\n- [ ] Test unsubscribe link functionality\n- [ ] Test tracking links render correctly\n- [ ] Test plain text fallback for non-HTML clients\n\n## Estimated Complexity\n\n**Medium-High** - ~1,800 lines of new code across 15 files\n\nBreakdown:\n- Trial storage and API: 400 lines\n- Nurture sequence and worker: 350 lines\n- Email templates (HTML): 600 lines\n- Landing page form: 150 lines\n- Tests: 300 lines\n\n**Risks:**\n- **Email deliverability**: Spam filters may block nurture emails (mitigated by SPF/DKIM setup)\n- **Provider rate limits**: Free SendGrid tier has daily limits (mitigated by provider abstraction)\n- **Template rendering bugsA tool call could not be parsed — the response may have been truncated. Try breaking the task into smaller steps or resending your message.
