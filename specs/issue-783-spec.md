   issue_title: Add onboarding checklist widget: 'Time to first mesh' progress tracker on dashboard
   issue_number: 783
   spec_file: specs/issue-783-spec.md

    __( O)>  ● new session · anthropic glm-4.6
   \____)    20260620_5 · /opt/wgmesh-checkout
     L L     goose is ready
I'll create an implementation specification for the onboarding checklist widget. Let me first explore the project structure to understand the codebase better.
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
  outreach - 2602240805 - show hn post draft.md  [69]
  outreach - 2602240805 - stargazer dm template.md  [93]
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
pipeline-output/  [32K]
  implementation-summary.md  [161]
  issue-539-spec.md  [335]
  issue-539.diff  [1K]
  issue-729-spec.md  [322]
  issue-729.diff  [2K]
  issue-732-spec.md  [1K]
  issue-732.diff  [2K]
  issue-733-spec.md  [208]
  issue-733.diff  [407]
  issue-734-spec.md  [147]
  issue-734.diff  [2K]
  issue-735-spec.md  [102]
  issue-735.diff  [3K]
  issue-745-spec.md  [101]
  issue-745.diff  [623]
  issue-746-spec.md  [314]
  issue-746.diff  [5K]
  issue-752-spec.md  [101]
  issue-752.diff  [1K]
  issue-753-spec.md  [196]
  issue-753.diff  [516]
  issue-759-spec.md  [295]
  issue-759.diff  [1K]
  issue-766-spec.md  [219]
  issue-766.diff  [1K]
  issue-767-implementation-summary.md  [115]
  issue-767-spec.md  [131]
  issue-767.diff  [684]
  issue-773-spec.md  [2K]
  issue-773.diff  [614]
  issue-774-spec.md  [226]
  issue-774.diff  [1K]
  issue-779-spec.md  [184]
  issue-779.diff  [1K]
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
public/  [310]
  index.html  [310]
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
wgmesh.dev/  [224]
  index.html  [224]
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
README.md  [335]
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
index.html  [265]
main.go  [1K]
main_test.go  [371]
pp.html  [35]
rnd.txt  [335]
service.go  [503]
service_test.go  [307]
sonar-project.properties  [10]
test-encryption.sh  [115]
test-route-cleanup.sh  [90]
test-route-diff-scenarios.md  [156]

  ────────────────────────────────────────
  ▸ analyze
    path: /opt/wgmesh-checkout/pkg
    max_depth: 2

96 files, 26070L, 913F, 107C (depth=2)
(6 files skipped: no parser)
go 100%

crypto/
  capability_version.go [8L, 0F]
  derive.go [300L, 12F, 1C]
  derive_test.go [448L, 20F]
  encrypt.go [106L, 2F]
  encrypt_test.go [319L, 6F]
  envelope.go [328L, 10F, 3C]
  envelope_compat_test.go [287L, 12F, 1C]
  envelope_test.go [363L, 3F]
  membership.go [53L, 3F]
  membership_test.go [110L, 6F]
  password.go [40L, 2F]
  rotation.go [144L, 8F, 3C]
  rotation_test.go [221L, 6F]
daemon/
  cache.go [158L, 5F, 2C]
  cache_test.go [106L, 3F]
  collision.go [177L, 7F, 1C]
  collision_test.go [123L, 5F]
  config.go [244L, 7F, 2C]
  config_test.go [179L, 10F]
  daemon.go [1811L, 74F, 10C]
  daemon_test.go [394L, 9F]
  epoch.go [46L, 5F, 1C]
  executor.go [92L, 10F, 4C]
  health_test.go [33L, 1F]
  helpers.go [390L, 16F, 1C]
  helpers_test.go [634L, 19F, 2C]
  metrics.go [144L, 8F]
  metrics_test.go [266L, 11F]
  peerstore_compat.go [24L, 1F]
  peerstore_test.go [479L, 23F]
  relay_test.go [403L, 18F]
  reload_test.go [274L, 16F]
  routes.go [147L, 6F]
  routes_test.go [436L, 10F]
  systemd.go [220L, 5F, 1C]
  systemd_test.go [428L, 16F]
  validate.go [9L, 1F]
discovery/
  dht.go [1472L, 49F, 5C]
  dht_test.go [370L, 16F]
  exchange.go [1166L, 40F, 5C]
  exchange_test.go [703L, 17F]
  gossip.go [330L, 10F, 1C]
  gossip_test.go [220L, 8F]
  init.go [18L, 2F]
  lan.go [253L, 9F, 1C]
  lan_test.go [32L, 1F]
  registry.go [412L, 8F, 2C]
  stun.go [332L, 8F, 1C]
  stun_integration_test.go [32L, 1F]
  stun_test.go [494L, 14F]
ifname/
  validate.go [82L, 1F]
  validate_test.go [165L, 3F]
mesh/
  account.go [63L, 2F, 1C]
  account_test.go [72L, 4F]
  deploy.go [338L, 8F]
  mesh.go [266L, 9F]
  mesh_test.go [268L, 5F]
  policy.go [262L, 10F]
  policy_test.go [717L, 13F]
  services.go [73L, 2F, 2C]
  services_test.go [107L, 4F]
  types.go [60L, 0F, 5C]
node/
  store.go [374L, 22F, 3C]
  types.go [126L, 2F, 7C]
pilot/
  complete.go [244L, 5F, 2C]
  complete_test.go [329L, 14F]
  config.go [59L, 4F]
  metrics.go [255L, 17F, 1C]
  metrics_test.go [277L, 18F]
  pilot.go [318L, 13F, 6C]
  pilot_test.go [435L, 14F]
  report.go [446L, 6F, 1C]
  report_test.go [424L, 25F]
  validation.go [267L, 11F, 2C]
  validation_test.go [420L, 17F]
privacy/
  dandelion.go [301L, 14F, 4C]
  dandelion_test.go [154L, 9F]
ratelimit/
  limiter.go [153L, 8F, 3C]
  limiter_test.go [197L, 11F]
routes/
  routes.go [103L, 3F, 1C]
  routes_test.go [177L, 12F]
rpc/
  client.go [82L, 3F, 1C]
  integration_test.go [233L, 1F]
  protocol.go [77L, 0F, 8C]
  protocol_test.go [138L, 5F]
  server.go [428L, 15F, 4C]
  server_test.go [111L, 4F]
ssh/
  client.go [149L, 6F, 1C]
  routes.go [121L, 3F]
  wireguard.go [111L, 6F]
wireguard/
  apply.go [252L, 9F, 4C]
  config.go [236L, 6F, 4C]
  config_test.go [630L, 3F]
  convert.go [23L, 1F]
  keys.go [35L, 1F]
  persist.go [134L, 5F]

  ────────────────────────────────────────
  ▸ shell
    command: find . -name "*.spec.md" -o -name "*spec.md" | head -20

./pipeline-output/issue-753-spec.md
./pipeline-output/issue-732-spec.md
./pipeline-output/issue-745-spec.md
./pipeline-output/issue-729-spec.md
./pipeline-output/issue-774-spec.md
./pipeline-output/issue-735-spec.md
./pipeline-output/issue-766-spec.md
./pipeline-output/issue-539-spec.md
./pipeline-output/issue-767-spec.md
./pipeline-output/issue-752-spec.md
./pipeline-output/issue-779-spec.md
./pipeline-output/issue-773-spec.md
./pipeline-output/issue-746-spec.md
./pipeline-output/issue-759-spec.md
./pipeline-output/issue-734-spec.md
./pipeline-output/issue-733-spec.md
./specs/issue-176-spec.md
./specs/issue-561-spec.md
./specs/issue-573-spec.md
./specs/issue-496-spec.md
  ────────────────────────────────────────
  ▸ shell
    command: cat ./specs/issue-539-spec.md

# Specification: Issue #539

## Classification
feature

## Deliverables
code

## Problem Analysis

Android's VPN Service API creates a TUN device and hands the application a raw file descriptor
(`int`) for it. The application does **not** create the interface — the OS does — and the
application has no interface name to pass to `wg` or `ip`. As a result the existing
`setupWireGuard()` flow in `pkg/daemon/daemon.go` (which calls `createInterface`,
`configureInterface`, `setInterfaceAddress`, `setInterfaceUp`) cannot be used on Android.

What is needed is a way to start the mesh daemon when the caller already holds a TUN fd and
a raw WireGuard private key, so that:

1. The daemon **skips** `createInterface` / `setInterfaceAddress` / `setInterfaceUp` (those
   steps were done by the Android VPN Service before handing us the fd).
2. The daemon **uses** the fd to configure WireGuard peers via the userspace WireGuard
   control interface (e.g. `wireguard-go` unix socket) or directly via
   `golang.zx2c4.com/wireguard`.
3. On `Stop()` the daemon tears down gracefully without trying to `deleteInterface` (Android
   closes the fd itself when the VPN Service is revoked).

The change must be **additive**: existing Linux/macOS users who never pass an fd must
experience zero behaviour change.

The embeddable API lives in (or will live in, per spec #536) `pkg/wgmesh`. Callers there
call:

```go
m, err := wgmesh.Start(ctx, "wgmesh://v1/<base64>", wgmesh.WithTunFd(fd))
```

On Android the VPN Service calls:

```go
m, err := wgmesh.StartWithFd(ctx, key, fd)
```

Both entry points converge on the same daemon path.

## Implementation Tasks

### Task 1: Add `TunFd` and `TunPrivateKey` to `DaemonOpts` in `pkg/daemon/config.go`

In `pkg/daemon/config.go`, add two new fields to the `DaemonOpts` struct:

```go
// DaemonOpts holds options for the daemon
type DaemonOpts struct {
	Secret              string
	InterfaceName       string
	WGListenPort        int
	AdvertiseRoutes     []string
	LogLevel            string
	Privacy             bool
	Gossip              bool
	DisableLANDiscovery bool
	Introducer          bool
	DisableIPv6         bool
	ForceRelay          bool
	DisablePunching     bool
	MeshSubnet          string // Custom mesh subnet CIDR (e.g. "192.168.100.0/24")
	TunFd               int    // Pre-created TUN file descriptor (Android VPN API). 0 = unused.
	TunPrivateKey       []byte // Raw 32-byte WireGuard private key for TunFd mode.
}
```

Add two new fields to the `Config` struct (keep all existing fields unchanged):

```go
type Config struct {
	Secret          string
	Keys            *crypto.DerivedKeys
	InterfaceName   string
	WGListenPort    int
	AdvertiseRoutes []string
	LogLevel        string
	Privacy         bool
	Gossip          bool
	LANDiscovery    bool
	Introducer      bool
	DisableIPv6     bool
	ForceRelay      bool
	DisablePunching bool
	CustomSubnet    *net.IPNet
	TunFd           int    // 0 means "not set / create interface the normal way"
	TunPrivateKey   []byte // non-nil in TunFd mode; 32 raw WG private key bytes
}
```

In `NewConfig`, add the following validation block **before** the `return &Config{...}` literal
(i.e. after the `customSubnet` validation block and before the `return` statement), and then
add the two fields to the returned struct literal:

```go
	// Validate TunFd options before building the Config.
	if opts.TunFd != 0 {
		if len(opts.TunPrivateKey) != 32 {
			return nil, fmt.Errorf("TunPrivateKey must be exactly 32 bytes when TunFd is set")
		}
	}

	return &Config{
		// ... all existing fields unchanged ...
		TunFd:         opts.TunFd,
		TunPrivateKey: opts.TunPrivateKey,
	}, nil
```

Replace the existing `return &Config{...}, nil` statement with the version above that
includes the two new fields. All other fields in the literal remain unchanged.

### Task 2: Add `isTunFdMode()` helper and modify `setupWireGuard` in `pkg/daemon/daemon.go`

Add a private helper on `*Daemon`:

```go
// isTunFdMode returns true when the daemon was started with a pre-created TUN fd
// (Android VPN API). In this mode the WireGuard interface is managed externally.
func (d *Daemon) isTunFdMode() bool {
	return d.config.TunFd != 0
}
```

Modify `setupWireGuard` so that the entire body executes only in normal (non-fd) mode.
Specifically, change the function as follows — **replace** the existing `setupWireGuard`:

```go
// setupWireGuard creates and configures the WireGuard interface.
// In TunFd mode (Android) the interface was already created by the OS; only
// configure WireGuard keys and peers via the fd-based backend.
func (d *Daemon) setupWireGuard() error {
	if d.isTunFdMode() {
		return d.setupWireGuardFromFd()
	}

	log.Printf("Setting up WireGuard interface %s...", d.config.InterfaceName)

	// Check if interface exists
	if interfaceExists(d.config.InterfaceName) {
		existingPort := getWGInterfacePort(d.config.InterfaceName)
		if existingPort == d.config.WGListenPort {
			log.Printf("Interface %s exists with same port, resetting...", d.config.InterfaceName)
		} else {
			log.Printf("Interface %s exists, resetting...", d.config.InterfaceName)
		}
		if err := resetInterface(d.config.InterfaceName); err != nil {
			return fmt.Errorf("failed to reset interface: %w", err)
		}
	} else {
		if err := createInterface(d.config.InterfaceName); err != nil {
			return fmt.Errorf("failed to create interface: %w", err)
		}
	}

	listenPort := d.config.WGListenPort
	if isPortInUse(listenPort) {
		availablePort := findAvailablePort(listenPort + 1)
		if availablePort == 0 {
			return fmt.Errorf("port %d is in use and no available ports found (try --listen-port with a different port)", listenPort)
		}
		log.Printf("Port %d is in use, using port %d instead", listenPort, availablePort)
		listenPort = availablePort
		d.config.WGListenPort = availablePort
	}

	if err := configureInterface(d.config.InterfaceName, d.localNode.WGPrivateKey, listenPort); err != nil {
		return fmt.Errorf("failed to configure interface: %w", err)
	}

	if err := setInterfaceAddress(d.config.InterfaceName, fmt.Sprintf("%s/%d", d.localNode.MeshIP, d.config.PrefixLen())); err != nil {
		return fmt.Errorf("failed to set IP address: %w", err)
	}
	if d.localNode.MeshIPv6 != "" {
		if err := setInterfaceAddress(d.config.InterfaceName, d.localNode.MeshIPv6+"/64"); err != nil {
			return fmt.Errorf("failed to set IPv6 address: %w", err)
		}
	}

	if err := setInterfaceUp(d.config.InterfaceName); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	log.Printf("WireGuard interface %s ready on port %d", d.config.InterfaceName, listenPort)
	return nil
}
```

Add the new `setupWireGuardFromFd` method to `pkg/daemon/daemon.go`:

```go
// setupWireGuardFromFd configures the WireGuard stack using a pre-existing TUN fd
// (returned by Android VPN Service). It stores the supplied private key in
// localNode so the reconcile loop can configure peers normally.
// Must be called only from the daemon goroutine before the reconcile loop
// starts (no concurrent localNode access at that point).
func (d *Daemon) setupWireGuardFromFd() error {
	if len(d.config.TunPrivateKey) != 32 {
		return fmt.Errorf("setupWireGuardFromFd: TunPrivateKey must be 32 bytes")
	}

	// Encode the raw key as base64 (WireGuard standard encoding).
	privKeyB64 := base64.StdEncoding.EncodeToString(d.config.TunPrivateKey)

	// localNode is initialised by initLocalNode before setupWireGuard is called.
	// If for any reason it is nil (e.g. in unit tests), create a minimal struct.
	// No mutex is needed here because setupWireGuard is called sequentially
	// during daemon startup, before the reconcile goroutines are launched.
	if d.localNode == nil {
		d.localNode = &LocalNode{}
	}
	d.localNode.WGPrivateKey = privKeyB64

	log.Printf("WireGuard configured from pre-created TUN fd %d", d.config.TunFd)
	return nil
}
```

This method must import `"encoding/base64"` — add it to the imports in `daemon.go` if not
already present.

### Task 3: Skip interface teardown in TunFd mode in `pkg/daemon/daemon.go`

Modify `teardownWireGuard` to skip `setInterfaceDown` / `deleteInterface` when in fd mode
(Android controls the fd lifetime):

```go
func (d *Daemon) teardownWireGuard() {
	if d == nil || d.config == nil || d.config.InterfaceName == "" {
		return
	}
	if d.isTunFdMode() {
		log.Printf("[Shutdown] TunFd mode: skipping interface teardown (fd %d)", d.config.TunFd)
		return
	}
	if err := setInterfaceDown(d.config.InterfaceName); err != nil {
		log.Printf("[Shutdown] Failed to bring down interface %s: %v", d.config.InterfaceName, err)
	}
	if err := deleteInterface(d.config.InterfaceName); err != nil {
		log.Printf("[Shutdown] Failed to delete interface %s: %v", d.config.InterfaceName, err)
		return
	}
	log.Printf("[Shutdown] WireGuard interface %s removed", d.config.InterfaceName)
}
```

### Task 4: Add `WithTunFd` option to the embeddable API in `pkg/wgmesh/wgmesh.go`

The `pkg/wgmesh` package is introduced by spec #536. This task **adds** two items to that
file (or creates the file if #536 has not yet been merged):

1. A new field in the internal `options` struct:

```go
type options struct {
	ifaceName       string
	listenPort      int
	advertiseRoutes []string
	logLevel        string
	gossip          bool
	privacy         bool
	noLAN           bool
	disableIPv6     bool
	forceRelay      bool
	disablePunching bool
	introducer      bool
	meshSubnet      string
	stateDir        string
	tunFd           int    // 0 = normal mode; >0 = Android fd mode
	tunPrivateKey   []byte // required when tunFd > 0
}
```

2. Two new exported option functions:

```go
// WithTunFd instructs the daemon to use a pre-created TUN file descriptor
// instead of creating a WireGuard interface. fd must be the value returned by
// Android's VpnService.Builder.establish(). key must be the 32-byte raw
// WireGuard private key that was provisioned for this node.
//
// This option is mutually exclusive with WithInterface.
func WithTunFd(fd int, key []byte) Option {
	return func(o *options) {
		o.tunFd = fd
		o.tunPrivateKey = key
	}
}
```

3. Wire the new fields through `Start`:

Inside the `Start` function, in the `daemon.DaemonOpts{...}` literal, add:

```go
		TunFd:         o.tunFd,
		TunPrivateKey: o.tunPrivateKey,
```

### Task 5: Add `StartWithFd` convenience function in `pkg/wgmesh/wgmesh.go`

Add the following function at the end of `pkg/wgmesh/wgmesh.go`. This is the primary
Android-targeted entry point that matches the signature requested in the issue:

```go
// StartWithFd starts a mesh node using a pre-created TUN file descriptor,
// as returned by Android's VpnService.Builder.establish().
//
// key must be a 32-byte raw WireGuard private key.
// fd must be the file descriptor returned by the Android VPN API.
// token must be the shared mesh secret in "wgmesh://v1/<base64>" format or a
// plain passphrase.
//
// The node runs until ctx is cancelled or Stop is called.
func StartWithFd(ctx context.Context, token string, key []byte, fd int, opts ...Option) (*Mesh, error) {
	opts = append([]Option{WithTunFd(fd, key)}, opts...)
	return Start(ctx, token, opts...)
}
```

### Task 6: Add unit tests in `pkg/daemon/daemon_fd_test.go`

Create the new file `pkg/daemon/daemon_fd_test.go`:

```go
package daemon

import (
	"testing"
)

// TestIsTunFdMode verifies the isTunFdMode helper.
func TestIsTunFdMode(t *testing.T) {
	tests := []struct {
		name  string
		tunFd int
		want  bool
	}{
		{"zero fd = normal mode", 0, false},
		{"positive fd = tun mode", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Daemon{config: &Config{TunFd: tt.tunFd}}
			if got := d.isTunFdMode(); got != tt.want {
				t.Errorf("isTunFdMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSetupWireGuardFromFd verifies that setupWireGuardFromFd stores the
// base64-encoded private key in localNode and does not attempt to create or
// configure a system WireGuard interface.
func TestSetupWireGuardFromFd(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: key,
		},
	}

	if err := d.setupWireGuardFromFd(); err != nil {
		t.Fatalf("setupWireGuardFromFd() error: %v", err)
	}

	if d.localNode == nil {
		t.Fatal("localNode is nil after setupWireGuardFromFd")
	}
	if d.localNode.WGPrivateKey == "" {
		t.Error("localNode.WGPrivateKey is empty after setupWireGuardFromFd")
	}
}

// TestSetupWireGuardFromFd_BadKey verifies that an invalid key length is rejected.
func TestSetupWireGuardFromFd_BadKey(t *testing.T) {
	d := &Daemon{
		config: &Config{
			TunFd:         5,
			TunPrivateKey: []byte{1, 2, 3}, // too short
		},
	}
	if err := d.setupWireGuardFromFd(); err == nil {
		t.Error("expected error for short TunPrivateKey, got nil")
	}
}

// TestTeardownWireGuardTunFdMode verifies that teardownWireGuard is a no-op
// (does not call deleteInterface) when in TunFd mode.
func TestTeardownWireGuardTunFdMode(t *testing.T) {
	// Replace cmdExecutor with a recorder to assert no wg/ip commands are run.
	orig := cmdExecutor
	defer func() { cmdExecutor = orig }()
	rec := &recordingExecutor{}
	cmdExecutor = rec

	d := &Daemon{
		config: &Config{
			InterfaceName: "wg0",
			TunFd:         5,
		},
	}
	d.teardownWireGuard()

	if len(rec.commands) != 0 {
		t.Errorf("teardownWireGuard in TunFd mode ran unexpected commands: %v", rec.commands)
	}
}

// recordingExecutor captures all commands issued through cmdExecutor.
type recordingExecutor struct {
	commands [][]string
}

func (r *recordingExecutor) Command(name string, args ...string) Cmd {
	r.commands = append(r.commands, append([]string{name}, args...))
	return &noopCmd{}
}

// noopCmd is a Cmd that does nothing.
type noopCmd struct{}

func (n *noopCmd) Output() ([]byte, error)                 { return nil, nil }
func (n *noopCmd) CombinedOutput() ([]byte, error)         { return nil, nil }
func (n *noopCmd) Run() error                              { return nil }
func (n *noopCmd) Start() error                            { return nil }
func (n *noopCmd) Wait() error                             { return nil }
func (n *noopCmd) SetStdin(_ interface{})                  {}
func (n *noopCmd) SetStdout(_ interface{})                 {}
func (n *noopCmd) SetStderr(_ interface{})                 {}
```

**Note:** Check the existing `CommandExecutor` and `Cmd` interface definitions in
`pkg/daemon/helpers.go` or a `_test.go` file. Adapt the `recordingExecutor` to match the
exact interface methods already defined. Do not introduce a new interface — reuse the
existing one.

## Affected Files

| File | Change |
|------|--------|
| `pkg/daemon/config.go` | Add `TunFd int` and `TunPrivateKey []byte` to `DaemonOpts` and `Config`; validate in `NewConfig` |
| `pkg/daemon/daemon.go` | Add `isTunFdMode()`, `setupWireGuardFromFd()`; modify `setupWireGuard()` and `teardownWireGuard()` |
| `pkg/wgmesh/wgmesh.go` | Add `tunFd`/`tunPrivateKey` to `options`; add `WithTunFd()` option; add `StartWithFd()` function; wire through `Start()` |
| `pkg/daemon/daemon_fd_test.go` | New file: unit tests for fd mode helpers |

## Test Strategy

1. Run `go test ./pkg/daemon/... -run TestIsTunFdMode` — must pass.
2. Run `go test ./pkg/daemon/... -run TestSetupWireGuardFromFd` — must pass.
3. Run `go test ./pkg/daemon/... -run TestSetupWireGuardFromFd_BadKey` — must pass.
4. Run `go test ./pkg/daemon/... -run TestTeardownWireGuardTunFdMode` — must pass.
5. Run `go build ./...` — must compile without errors.
6. Run `go test -race ./pkg/daemon/...` — must pass with no race conditions.
7. Existing test suite `go test ./...` must remain green (no regressions).

## Estimated Complexity
lowNow let me check for any existing dashboard or web UI components:
  ────────────────────────────────────────
  ▸ shell
    command: find . -type f -name "*.go" | xargs grep -l "dashboard\|widget\|checklist\|onboard" 2>/dev/null | head -20

(no output)
  ────────────────────────────────────────
  ▸ shell
    command: cat ./main.go

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/mesh"
	"github.com/atvirokodosprendimai/wgmesh/pkg/pilot"
	"github.com/atvirokodosprendimai/wgmesh/pkg/rpc"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	// Import discovery to register the DHT factory via init()
	_ "github.com/atvirokodosprendimai/wgmesh/pkg/discovery"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

func versionOutput() string {
	return "wgmesh version " + version
}

func main() {
	// Check for version flags first (--version or -v)
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(versionOutput())
			return
		}
	}

	// Check for subcommands first
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println(versionOutput())
			return
		case "join":
			joinCmd()
			return
		case "init":
			initCmd()
			return
		case "status":
			statusCmd()
			return
		case "test-peer":
			testPeerCmd()
			return
		case "qr":
			qrCmd()
			return
		case "install-service":
			installServiceCmd()
			return
		case "uninstall-service":
			uninstallServiceCmd()
			return
		case "rotate-secret":
			rotateSecretCmd()
			return
		case "mesh":
			meshCmd()
			return
		case "peers":
			peersCmd()
			return
		case "service":
			serviceCmd()
			return
		case "pilot":
			pilotCmd()
			return
		}
	}

	// Original CLI mode
	var (
		stateFile  = flag.String("state", filepath.Join(defaultStateDir, "mesh-state.json"), "Path to mesh state file")
		addNode    = flag.String("add", "", "Add node (format: hostname:ip:ssh_host[:ssh_port])")
		removeNode = flag.String("remove", "", "Remove node by hostname")
		list       = flag.Bool("list", false, "List all nodes")
		listSimple = flag.Bool("list-simple", false, "List all nodes in simple format (hostname ip)")
		deploy     = flag.Bool("deploy", false, "Deploy configuration to all nodes")
		init       = flag.Bool("init", false, "Initialize new mesh")
		network    = flag.String("network", "", "Custom mesh network CIDR for init (default: 10.99.0.0/16)")
		encrypt    = flag.Bool("encrypt", false, "Encrypt state file with password (asks for password)")
	)

	flag.Parse()

	// Handle encryption flag
	if *encrypt {
		var password string
		var err error

		if *init {
			// For init, ask for password twice
			password, err = crypto.ReadPasswordTwice("Enter encryption password: ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
				os.Exit(1)
			}
		} else {
			// For other operations, ask once
			password, err = crypto.ReadPassword("Enter encryption password: ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
				os.Exit(1)
			}
		}

		mesh.SetEncryptionPassword(password)
	}

	if *init {
		if err := mesh.InitializeWithNetwork(*stateFile, *network); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize mesh: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Mesh initialized successfully")
		return
	}

	m, err := mesh.Load(*stateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load mesh state: %v\n", err)
		os.Exit(1)
	}

	switch {
	case *addNode != "":
		if err := m.AddNode(*addNode); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to add node: %v\n", err)
			os.Exit(1)
		}
		if err := m.Save(*stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Node added successfully\n")

	case *removeNode != "":
		if err := m.RemoveNode(*removeNode); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove node: %v\n", err)
			os.Exit(1)
		}
		if err := m.Save(*stateFile); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Node removed successfully\n")

	case *list:
		m.List()

	case *listSimple:
		m.ListSimple()

	case *deploy:
		if err := m.Deploy(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to deploy: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Deployment completed successfully")

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`wgmesh - WireGuard mesh network builder

FLAGS:
  --version, -v               Show version information
  -state <file>    Path to mesh state file (default: ` + filepath.Join(defaultStateDir, "mesh-state.json") + `)
  -add <spec>      Add node (format: hostname:ip:ssh_host[:ssh_port])
  -remove <name>   Remove node by hostname
  -list            List all nodes
  -deploy          Deploy configuration to all nodes
  -init            Initialize new mesh state file
  -network <CIDR>  Custom mesh network for init (default: 10.99.0.0/16)
  -encrypt         Encrypt state file with password

SUBCOMMANDS (centralized mode):
  mesh list [--state <file>] [--encrypt]  List hostnames and mesh IPs

SUBCOMMANDS (decentralized mode):
  init --secret                 Generate a new mesh secret
	join --secret <SECRET>        Join a mesh network
	     [--account <cr_...>]    Save Lighthouse API key for service commands
	     [--mesh-subnet CIDR]    Custom mesh subnet (e.g. 192.168.100.0/24)
	     [--no-lan-discovery]     Disable LAN multicast discovery
	     [--no-ipv6]              Ignore IPv6 endpoints for connectivity
	     [--force-relay]          Prefer relay path for non-LAN peers
	     [--no-punching]          Disable NAT port punching/rendezvous
	     [--introducer]           Enable rendezvous introducer role
  status --secret <SECRET>      Show mesh status
  qr --secret <SECRET>          Display secret as QR code (text)
	install-service --secret ...  Install systemd service
	     [--account <cr_...>]    Save Lighthouse API key for service commands
	     [--no-lan-discovery]     Disable LAN multicast discovery in service
	     [--no-ipv6]              Ignore IPv6 endpoints in service
	     [--force-relay]          Prefer relay path in service
	     [--no-punching]          Disable NAT punching in service
	     [--introducer]           Enable rendezvous introducer role in service
  uninstall-service             Remove systemd service
  rotate-secret                 Rotate mesh secret

QUERY SUBCOMMANDS (decentralized mode):
  peers list                    List all active peers
  peers count                   Show peer statistics
  peers get <pubkey>            Get specific peer details

  Note: These commands query a running daemon via RPC socket.
  The daemon must be started with 'wgmesh join' first.

EXAMPLES:
  # Show version
  wgmesh --version
  wgmesh -v

  # Decentralized mode (automatic peer discovery):
  wgmesh init --secret                          # Generate a new mesh secret
  wgmesh join --secret "wgmesh://v1/K7x2..."    # Join mesh on this node
  wgmesh join --secret "..." --account cr_123    # Join and save API key
  wgmesh join --secret "..." --privacy           # Join with Dandelion++ privacy
  wgmesh join --secret "..." --gossip            # Enable in-mesh gossip

  # Query running daemon:
  wgmesh peers list                              # List all active peers
  wgmesh peers count                             # Show peer counts
  wgmesh peers get <pubkey>                      # Get specific peer info

  # Centralized mode (SSH-based deployment):
  wgmesh -init -encrypt                         # Initialize encrypted state
  wgmesh -add node1:10.99.0.1:192.168.1.10     # Add a node
  wgmesh -deploy                               # Deploy to all nodes
  wgmesh mesh list                             # List hostnames and mesh IPs`)
}

// initCmd handles the "init --secret" subcommand
func initCmd() {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	secretMode := fs.Bool("secret", false, "Generate a new mesh secret")
	fs.Parse(os.Args[2:])

	if *secretMode {
		secret, err := daemon.GenerateSecret()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate secret: %v\n", err)
			os.Exit(1)
		}

		uri := daemon.FormatSecretURI(secret)
		fmt.Println("Generated mesh secret:")
		fmt.Println()
		fmt.Println(uri)
		fmt.Println()
		fmt.Println("Share this secret with all nodes that should join the mesh.")
		fmt.Println("Run: wgmesh join --secret \"" + uri + "\"")
		return
	}

	fs.Usage()
	os.Exit(1)
}

// joinCmd handles the "join --secret" subcommand
func joinCmd() {
	fs := flag.NewFlagSet("join", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	account := fs.String("account", "", "Lighthouse API key (cr_...) — saved for service commands")
	stateDir := fs.String("state-dir", defaultStateDir, "State directory for account config")
	advertiseRoutes := fs.String("advertise-routes", "", "Comma-separated list of routes to advertise")
	listenPort := fs.Int("listen-port", 51820, "WireGuard listen port")
	iface := fs.String("interface", "", "WireGuard interface name (default: wg0 on non-macOS, utun20 on macOS)")
	logLevel := fs.String("log-level", "info", "Log level (debug, info, warn, error)")
	privacyMode := fs.Bool("privacy", false, "Enable privacy mode (Dandelion++ relay)")
	gossipMode := fs.Bool("gossip", false, "Enable in-mesh gossip")
	socketPath := fs.String("socket-path", "", "RPC socket path (auto-detected if empty)")
	noLANDiscovery := fs.Bool("no-lan-discovery", false, "Disable LAN multicast discovery")
	noIPv6 := fs.Bool("no-ipv6", false, "Ignore IPv6 endpoints for connectivity")
	forceRelay := fs.Bool("force-relay", false, "Prefer relay path for non-LAN peers")
	noPunching := fs.Bool("no-punching", false, "Disable NAT port punching/rendezvous")
	introducerMode := fs.Bool("introducer", false, "Allow this node to act as rendezvous introducer")
	meshSubnet := fs.String("mesh-subnet", "", "Custom mesh subnet CIDR (e.g. 192.168.100.0/24)")
	pprofAddr := fs.String("pprof", "", "Enable pprof HTTP server (e.g. localhost:6060)")
	metricsAddr := fs.String("metrics", "", "Enable Prometheus metrics server (e.g. :9090)")
	fs.Parse(os.Args[2:])

	// If secret not provided via flag, try environment variables
	if *secret == "" {
		if envSecret := os.Getenv("WGMESH_SECRET"); envSecret != "" {
			*secret = envSecret
		} else if secretFile := os.Getenv("WGMESH_SECRET_FILE"); secretFile != "" {
			secretBytes, err := os.ReadFile(secretFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading secret file %s: %v\n", secretFile, err)
				os.Exit(1)
			}
			*secret = strings.TrimSpace(string(secretBytes))
		}
	}

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh join --secret <SECRET>")
		fmt.Fprintln(os.Stderr, "       or set WGMESH_SECRET environment variable")
		fmt.Fprintln(os.Stderr, "       or set WGMESH_SECRET_FILE environment variable")
		os.Exit(1)
	}

	// Save account API key if provided
	handleAccountFlag(*stateDir, *account)

	// Parse advertise routes
	var routes []string
	if *advertiseRoutes != "" {
		routes = strings.Split(*advertiseRoutes, ",")
		for i, r := range routes {
			routes[i] = strings.TrimSpace(r)
		}
	}

	// Create daemon config
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{
		Secret:              *secret,
		InterfaceName:       *iface,
		WGListenPort:        *listenPort,
		AdvertiseRoutes:     routes,
		LogLevel:            *logLevel,
		Privacy:             *privacyMode,
		Gossip:              *gossipMode,
		DisableLANDiscovery: *noLANDiscovery,
		DisableIPv6:         *noIPv6,
		ForceRelay:          *forceRelay,
		DisablePunching:     *noPunching,
		Introducer:          *introducerMode,
		MeshSubnet:          *meshSubnet,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	// Configure logging before creating the daemon (must be done in main,
	// not inside library code like NewDaemon).
	daemon.ConfigureLogging(cfg.LogLevel)

	// Create and run daemon with DHT discovery
	d, err := daemon.NewDaemon(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create daemon: %v\n", err)
		os.Exit(1)
	}

	// Start pprof HTTP server if requested (for profiling/flame graphs)
	if *pprofAddr != "" {
		go func() {
			log.Printf("pprof server listening on %s", *pprofAddr)
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				log.Printf("pprof server error: %v", err)
			}
		}()
	}

	// Start Prometheus metrics server if requested
	if *metricsAddr != "" {
		daemon.RegisterMetrics()
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		go func() {
			log.Printf("metrics server listening on %s", *metricsAddr)
			if err := http.ListenAndServe(*metricsAddr, metricsMux); err != nil {
				log.Printf("metrics server error: %v", err)
			}
		}()
	}

	// Setup RPC server
	rpcSocketPath := *socketPath
	if rpcSocketPath == "" {
		// Import here to avoid circular dependency
		rpcSocketPath = getRPCSocketPath()
	}

	// Create RPC server with callback functions
	rpcServer, err := createRPCServer(d, rpcSocketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create RPC server: %v\n", err)
	} else {
		d.SetRPCServer(rpcServer)
		fmt.Printf("RPC socket configured: %s (will start after DHT discovery)\n", rpcSocketPath)
	}

	fmt.Println("Initializing mesh node with DHT discovery...")
	if *privacyMode {
		fmt.Println("Privacy mode enabled (Dandelion++ relay)")
	}
	if *gossipMode {
		fmt.Println("In-mesh gossip enabled")
	}
	if *noLANDiscovery {
		fmt.Println("LAN discovery disabled")
	}
	if *noIPv6 {
		fmt.Println("IPv6 connectivity disabled")
	}
	if *forceRelay {
		fmt.Println("Force relay mode enabled")
	}
	if *noPunching {
		fmt.Println("NAT punching disabled")
	}
	if *introducerMode {
		fmt.Println("Rendezvous introducer enabled")
	}

	if err := d.RunWithDHTDiscovery(); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}
}

// testPeerCmd tests direct peer exchange connectivity
func testPeerCmd() {
	fs := flag.NewFlagSet("test-peer", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	peerAddr := fs.String("peer", "", "Peer address to test (IP:PORT)")
	listenPort := fs.Int("port", 0, "Local port to listen on (0 = random)")
	fs.Parse(os.Args[2:])

	if *secret == "" || *peerAddr == "" {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh test-peer --secret <SECRET> --peer <IP:PORT>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "This tests direct UDP connectivity to another wgmesh node.")
		fmt.Fprintln(os.Stderr, "Run 'wgmesh join' on the peer first, note its exchange port,")
		fmt.Fprintln(os.Stderr, "then test with: wgmesh test-peer --secret <SECRET> --peer <PEER_IP>:<EXCHANGE_PORT>")
		os.Exit(1)
	}

	cfg, err := daemon.NewConfig(daemon.DaemonOpts{Secret: *secret})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Testing peer exchange with %s\n", *peerAddr)
	fmt.Printf("Network ID: %x\n", cfg.Keys.NetworkID[:8])

	// Create UDP socket
	addr := &net.UDPAddr{Port: *listenPort}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to bind UDP: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Listening on port %d\n", conn.LocalAddr().(*net.UDPAddr).Port)

	// Resolve peer
	peerUDP, err := net.ResolveUDPAddr("udp", *peerAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve peer: %v\n", err)
		os.Exit(1)
	}

	// Create and send test message
	announcement := crypto.CreateAnnouncement("test-pubkey", "10.0.0.1", "test:51820", false, nil, nil, "", "", "")
	data, err := crypto.SealEnvelope(crypto.MessageTypeHello, announcement, cfg.Keys.GossipKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create message: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Sending HELLO to %s (%d bytes)...\n", *peerAddr, len(data))
	_, err = conn.WriteToUDP(data, peerUDP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send: %v\n", err)
		os.Exit(1)
	}

	// Wait for response
	fmt.Println("Waiting for response (10s timeout)...")
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 65536)
	n, from, err := conn.ReadFromUDP(buf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "No response: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Possible issues:")
		fmt.Fprintln(os.Stderr, "- Peer not running or wrong port")
		fmt.Fprintln(os.Stderr, "- Firewall blocking UDP")
		fmt.Fprintln(os.Stderr, "- Different secrets (different gossip keys)")
		os.Exit(1)
	}

	fmt.Printf("Received %d bytes from %s\n", n, from.String())

	// Try to decrypt
	envelope, reply, err := crypto.OpenEnvelope(buf[:n], cfg.Keys.GossipKey)
	if err != nil {
		fmt.Printf("Failed to decrypt (wrong secret?): %v\n", err)
		os.Exit(1)
	}

	fmt.Println("SUCCESS! Peer exchange working!")
	fmt.Printf("  Message type: %s\n", envelope.MessageType)
	fmt.Printf("  Peer pubkey: %s\n", reply.WGPubKey)
	fmt.Printf("  Peer mesh IP: %s\n", reply.MeshIP)
}

// statusCmd handles the "status --secret" subcommand
// StatusOutput defines the JSON structure for status output
type StatusOutput struct {
	Interface      string `json:"interface"`
	NetworkID      string `json:"network_id"`
	MeshSubnet     string `json:"mesh_subnet"`
	MeshIPv6Prefix string `json:"mesh_ipv6_prefix"`
	GossipPort     int    `json:"gossip_port"`
	RendezvousID   string `json:"rendezvous_id"`
	ServiceStatus  string `json:"service_status,omitempty"`
}

func statusCmd() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	iface := fs.String("interface", "", "WireGuard interface name (default: wg0 on non-macOS, utun20 on macOS)")
	meshSubnet := fs.String("mesh-subnet", "", "Custom mesh subnet CIDR (e.g. 192.168.100.0/24)")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh status --secret <SECRET>")
		os.Exit(1)
	}

	// Create config to derive keys
	cfg, err := daemon.NewConfig(daemon.DaemonOpts{
		Secret:        *secret,
		InterfaceName: *iface,
		MeshSubnet:    *meshSubnet,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config: %v\n", err)
		os.Exit(1)
	}

	// Prepare output
	output := StatusOutput{
		Interface:      cfg.InterfaceName,
		NetworkID:      fmt.Sprintf("%x", cfg.Keys.NetworkID[:8]),
		MeshIPv6Prefix: formatIPv6Prefix(cfg.Keys.MeshPrefixV6),
		GossipPort:     int(cfg.Keys.GossipPort),
		RendezvousID:   fmt.Sprintf("%x", cfg.Keys.RendezvousID),
	}

	// Set mesh subnet based on custom or derived
	if cfg.CustomSubnet != nil {
		output.MeshSubnet = cfg.CustomSubnet.String()
	} else {
		output.MeshSubnet = fmt.Sprintf("10.%d.0.0/16", cfg.Keys.MeshSubnet[0])
	}

	// Get service status if available
	status, err := daemon.ServiceStatus()
	if err == nil {
		output.ServiceStatus = status
	}

	// Output in requested format
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(output); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to encode JSON: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Text format (original behavior)
		fmt.Printf("Mesh Status\n")
		fmt.Printf("===========\n")
		fmt.Printf("Interface: %s\n", output.Interface)
		fmt.Printf("Network ID: %s\n", output.NetworkID)
		if cfg.CustomSubnet != nil {
			fmt.Printf("Mesh Subnet: %s (custom)\n", output.MeshSubnet)
		} else {
			fmt.Printf("Mesh Subnet: %s\n", output.MeshSubnet)
		}
		fmt.Printf("Mesh IPv6 Prefix: %s\n", output.MeshIPv6Prefix)
		fmt.Printf("Gossip Port: %d\n", output.GossipPort)
		fmt.Printf("Rendezvous ID: %s\n", output.RendezvousID)
		fmt.Println()

		if output.ServiceStatus != "" {
			fmt.Printf("Service Status: %s\n", output.ServiceStatus)
		}

		fmt.Println()
		fmt.Println("(Run 'wg show' to see connected peers)")
	}
}

// qrCmd handles the "qr" subcommand - displays secret as a text-based QR code
func qrCmd() {
	fs := flag.NewFlagSet("qr", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret to encode as QR code")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh qr --secret <SECRET>")
		os.Exit(1)
	}

	uri := *secret
	if !strings.HasPrefix(uri, daemon.URIPrefix) {
		uri = daemon.FormatSecretURI(*secret)
	}

	fmt.Println("Mesh Secret QR Code")
	fmt.Println("====================")
	fmt.Println()
	fmt.Printf("URI: %s\n", uri)
	fmt.Println()

	// Generate a simple text-based QR representation
	// For a real QR code, the go-qrcode library would be used
	printTextQR(uri)

	fmt.Println()
	fmt.Println("Scan this QR code or copy the URI to join the mesh.")
}

// printTextQR prints a simple text-based representation of the secret
func printTextQR(data string) {
	// Generate a simple visual representation using Unicode block characters
	// This is a placeholder - a real implementation would use go-qrcode
	const maxLineWidth = 40 // Maximum characters per line for readability
	width := len(data)
	if width > maxLineWidth {
		width = maxLineWidth
	}

	border := strings.Repeat("██", width+2)
	fmt.Println(border)
	fmt.Printf("██%s██\n", strings.Repeat("  ", width))

	// Print the data in a box format for easy reading
	for i := 0; i < len(data); i += width {
		end := i + width
		if end > len(data) {
			end = len(data)
		}
		chunk := data[i:end]
		padding := strings.Repeat(" ", (width-len(chunk))*2)
		fmt.Printf("██  %s%s  ██\n", chunk, padding)
	}

	fmt.Printf("██%s██\n", strings.Repeat("  ", width))
	fmt.Println(border)
}

func formatIPv6Prefix(prefix [8]byte) string {
	return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x::/64",
		prefix[0], prefix[1],
		prefix[2], prefix[3],
		prefix[4], prefix[5],
		prefix[6], prefix[7],
	)
}

// handleAccountFlag processes the --account flag by saving the API key if provided.
func handleAccountFlag(stateDir, apiKey string) {
	if apiKey != "" {
		saveAccountAPIKey(stateDir, apiKey)
	}
}

// saveAccountAPIKey saves the provided API key to the account config file.
// It properly handles missing vs corrupt files and includes the path in error messages.
func saveAccountAPIKey(stateDir, apiKey string) {
	accountPath := filepath.Join(stateDir, "account.json")

	// Try to load existing account first
	acct, err := mesh.LoadAccount(accountPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing account file; start from an empty account.
			acct = mesh.AccountConfig{}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: failed to load existing account from %s: %v\n", accountPath, err)
			fmt.Fprintln(os.Stderr, "Aborting account update to avoid overwriting potentially valid data.")
			// Do not overwrite a potentially valid but unreadable account file.
			return
		}
	}

	acct.APIKey = apiKey
	if err := mesh.SaveAccount(accountPath, acct); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save account to %s: %v\n", accountPath, err)
	} else {
		fmt.Println("Lighthouse API key saved for service commands.")
	}
}

// installServiceCmd handles the "install-service" subcommand
func installServiceCmd() {
	fs := flag.NewFlagSet("install-service", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (required)")
	account := fs.String("account", "", "Lighthouse API key (cr_...) — saved for service commands")
	stateDir := fs.String("state-dir", defaultStateDir, "State directory for account config")
	iface := fs.String("interface", "", "WireGuard interface name (default: wg0 on non-macOS, utun20 on macOS)")
	listenPort := fs.Int("listen-port", 51820, "WireGuard listen port")
	advertiseRoutes := fs.String("advertise-routes", "", "Comma-separated routes to advertise")
	privacyMode := fs.Bool("privacy", false, "Enable privacy mode")
	gossipMode := fs.Bool("gossip", false, "Enable in-mesh gossip")
	noLANDiscovery := fs.Bool("no-lan-discovery", false, "Disable LAN multicast discovery")
	noIPv6 := fs.Bool("no-ipv6", false, "Ignore IPv6 endpoints for connectivity")
	forceRelay := fs.Bool("force-relay", false, "Prefer relay path for non-LAN peers")
	noPunching := fs.Bool("no-punching", false, "Disable NAT port punching/rendezvous")
	introducerMode := fs.Bool("introducer", false, "Allow this node to act as rendezvous introducer")
	meshSubnet := fs.String("mesh-subnet", "", "Custom mesh subnet CIDR (e.g. 192.168.100.0/24)")
	fs.Parse(os.Args[2:])

	if *secret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh install-service --secret <SECRET>")
		os.Exit(1)
	}

	// Save account API key if provided
	handleAccountFlag(*stateDir, *account)

	var routes []string
	if *advertiseRoutes != "" {
		routes = strings.Split(*advertiseRoutes, ",")
		for i, r := range routes {
			routes[i] = strings.TrimSpace(r)
		}
	}

	cfg := daemon.SystemdServiceConfig{
		Secret:              *secret,
		InterfaceName:       *iface,
		ListenPort:          *listenPort,
		AdvertiseRoutes:     routes,
		Privacy:             *privacyMode,
		Gossip:              *gossipMode,
		DisableLANDiscovery: *noLANDiscovery,
		DisableIPv6:         *noIPv6,
		ForceRelay:          *forceRelay,
		DisablePunching:     *noPunching,
		Introducer:          *introducerMode,
		MeshSubnet:          *meshSubnet,
	}

	fmt.Println("Installing wgmesh systemd service...")
	if err := daemon.InstallSystemdService(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to install service: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Service installed and started successfully!")
	fmt.Println("Check status with: systemctl status wgmesh")
}

// uninstallServiceCmd handles the "uninstall-service" subcommand
func uninstallServiceCmd() {
	fmt.Println("Removing wgmesh systemd service...")
	if err := daemon.UninstallSystemdService(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to uninstall service: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Service removed successfully!")
}

// rotateSecretCmd handles the "rotate-secret" subcommand
func rotateSecretCmd() {
	fs := flag.NewFlagSet("rotate-secret", flag.ExitOnError)
	currentSecret := fs.String("current", "", "Current mesh secret (required)")
	newSecret := fs.String("new", "", "New mesh secret (auto-generated if empty)")
	gracePeriod := fs.Duration("grace", 24*time.Hour, "Grace period for dual-secret mode")
	fs.Parse(os.Args[2:])

	if *currentSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: --current is required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh rotate-secret --current <OLD_SECRET> [--new <NEW_SECRET>] [--grace 24h]")
		os.Exit(1)
	}

	// Generate new secret if not provided
	if *newSecret == "" {
		secret, err := daemon.GenerateSecret()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate new secret: %v\n", err)
			os.Exit(1)
		}
		*newSecret = secret
	}

	// Derive keys from old secret for signing
	oldKeys, err := crypto.DeriveKeys(*currentSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to derive keys from current secret: %v\n", err)
		os.Exit(1)
	}

	// Create rotation announcement
	announcement, err := crypto.GenerateRotationAnnouncement(oldKeys.MembershipKey[:], *newSecret, *gracePeriod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create rotation announcement: %v\n", err)
		os.Exit(1)
	}

	_ = announcement // Would be broadcast via gossip in a running mesh

	newURI := daemon.FormatSecretURI(*newSecret)

	fmt.Println("Secret Rotation Initiated")
	fmt.Println("=========================")
	fmt.Printf("Grace Period: %v\n", *gracePeriod)
	fmt.Printf("New Secret URI: %s\n", newURI)
	fmt.Println()
	fmt.Println("During the grace period, both secrets will be accepted.")
	fmt.Printf("After %v, all nodes should use the new secret.\n", *gracePeriod)
	fmt.Println()
	fmt.Println("Share the new secret with all nodes:")
	fmt.Printf("  wgmesh join --secret \"%s\"\n", newURI)
}

// meshCmd handles the "mesh" subcommand for centralized mesh management
func meshCmd() {
	// Check for action subcommand first
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: action required")
		fmt.Fprintln(os.Stderr, "Usage: wgmesh mesh <action> [options]")
		fmt.Fprintln(os.Stderr, "Actions: list")
		os.Exit(1)
	}

	action := os.Args[2]

	fs := flag.NewFlagSet("mesh "+action, flag.ExitOnError)
	stateFile := fs.String("state", "mesh-state.json", "Path to mesh state file")
	encrypt := fs.Bool("encrypt", false, "Encrypt state file with password")
	fs.Parse(os.Args[3:])

	// Handle encryption flag if set
	if *encrypt {
		password, err := crypto.ReadPassword("Enter encryption password: ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
			os.Exit(1)
		}
		mesh.SetEncryptionPassword(password)
	}

	// Load mesh state
	m, err := mesh.Load(*stateFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load mesh state: %v\n", err)
		os.Exit(1)
	}

	switch action {
	case "list":
		m.ListSimple()
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		fmt.Fprintln(os.Stderr, "Available actions: list")
		os.Exit(1)
	}
}

// getRPCSocketPath determines the RPC socket path (uses rpc.GetSocketPath)
func getRPCSocketPath() string {
	return rpc.GetSocketPath()
}

// createRPCServer creates an RPC server for the daemon
func createRPCServer(d *daemon.Daemon, socketPath string) (daemon.RPCServer, error) {
	config := rpc.ServerConfig{
		SocketPath: socketPath,
		Version:    version,
		GetPeers: func() []*rpc.PeerData {
			rpcPeers := d.GetRPCPeers()
			result := make([]*rpc.PeerData, len(rpcPeers))
			for i, p := range rpcPeers {
				result[i] = &rpc.PeerData{
					WGPubKey:         p.WGPubKey,
					Hostname:         p.Hostname,
					MeshIP:           p.MeshIP,
					Endpoint:         p.Endpoint,
					LastSeen:         p.LastSeen,
					DiscoveredVia:    p.DiscoveredVia,
					RoutableNetworks: p.RoutableNetworks,
					LatencyMs:        p.LatencyMs,
				}
			}
			return result
		},
		GetPeer: func(pubKey string) (*rpc.PeerData, bool) {
			peer, exists := d.GetRPCPeer(pubKey)
			if !exists {
				return nil, false
			}
			return &rpc.PeerData{
				WGPubKey:         peer.WGPubKey,
				Hostname:         peer.Hostname,
				MeshIP:           peer.MeshIP,
				Endpoint:         peer.Endpoint,
				LastSeen:         peer.LastSeen,
				DiscoveredVia:    peer.DiscoveredVia,
				RoutableNetworks: peer.RoutableNetworks,
				LatencyMs:        peer.LatencyMs,
			}, true
		},
		GetPeerCounts: d.GetRPCPeerCounts,
		GetStatus: func() *rpc.StatusData {
			status := d.GetRPCStatus()
			if status == nil {
				return nil
			}
			return &rpc.StatusData{
				MeshIP:    status.MeshIP,
				PubKey:    status.PubKey,
				Uptime:    status.Uptime,
				Interface: status.Interface,
			}
		},
	}

	return rpc.NewServer(config)
}

// peersCmd handles the "peers" subcommand for querying the daemon via RPC
func peersCmd() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh peers <list|count|get>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  list            List all active peers")
		fmt.Fprintln(os.Stderr, "  count           Show peer counts")
		fmt.Fprintln(os.Stderr, "  get <pubkey>    Get specific peer by public key")
		os.Exit(1)
	}

	action := os.Args[2]
	socketPath := os.Getenv("WGMESH_SOCKET")
	if socketPath == "" {
		socketPath = getRPCSocketPath()
	}

	client, err := rpc.NewClient(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Is wgmesh daemon running?")
		fmt.Fprintln(os.Stderr, "  Start with: wgmesh join --secret <SECRET>")
		fmt.Fprintf(os.Stderr, "  Socket path: %s\n", socketPath)
		os.Exit(1)
	}
	defer client.Close()

	switch action {
	case "list":
		handlePeersList(client)
	case "count":
		handlePeersCount(client)
	case "get":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "Usage: wgmesh peers get <pubkey>")
			os.Exit(1)
		}
		handlePeersGet(client, os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "Unknown action: %s\n", action)
		fmt.Fprintln(os.Stderr, "Available actions: list, count, get")
		os.Exit(1)
	}
}

func handlePeersList(client *rpc.Client) {
	result, err := client.Call("peers.list", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC error: %v\n", err)
		os.Exit(1)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid response format")
		os.Exit(1)
	}

	peersData, ok := resultMap["peers"].([]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid peers data")
		os.Exit(1)
	}

	if len(peersData) == 0 {
		fmt.Println("No active peers")
		return
	}

	fmt.Printf("%-20s %-19s %-15s %-25s %-10s %-10s %s\n", "HOSTNAME", "PUBLIC KEY", "MESH IP", "ENDPOINT", "LAST SEEN", "LATENCY", "DISCOVERED VIA")
	fmt.Println(strings.Repeat("-", 130))

	for _, peerData := range peersData {
		peer, ok := peerData.(map[string]interface{})
		if !ok {
			continue
		}

		pubkey, ok := peer["pubkey"].(string)
		if !ok {
			continue
		}
		pubkeyShort := pubkey
		if len(pubkeyShort) > 16 {
			pubkeyShort = pubkeyShort[:16] + "..."
		}

		hostname, _ := peer["hostname"].(string)
		if hostname == "" {
			hostname = pubkeyShort
		}
		if len(hostname) > 20 {
			hostname = hostname[:17] + "..."
		}

		meshIP, _ := peer["mesh_ip"].(string)
		endpoint, _ := peer["endpoint"].(string)
		lastSeen, _ := peer["last_seen"].(string)

		lastSeenTime, err := time.Parse(time.RFC3339, lastSeen)
		lastSeenStr := "unknown"
		if err == nil {
			lastSeenStr = formatDuration(time.Since(lastSeenTime))
		}

		latencyStr := "-"
		if v, ok := peer["latency_ms"]; ok && v != nil {
			if ms, ok := v.(float64); ok {
				latencyStr = fmt.Sprintf("%.1fms", ms)
			}
		}

		var discoveredViaStr []string
		if v, ok := peer["discovered_via"]; ok {
			if discoveredVia, ok := v.([]interface{}); ok {
				for _, item := range discoveredVia {
					if s, ok := item.(string); ok {
						discoveredViaStr = append(discoveredViaStr, s)
					}
				}
			}
		}

		fmt.Printf("%-20s %-19s %-15s %-25s %-10s %-10s %s\n", hostname, pubkeyShort, meshIP, endpoint, lastSeenStr, latencyStr, strings.Join(discoveredViaStr, ","))
	}
}

func handlePeersCount(client *rpc.Client) {
	result, err := client.Call("peers.count", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC error: %v\n", err)
		os.Exit(1)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid response format")
		os.Exit(1)
	}

	active, ok1 := resultMap["active"].(float64)
	total, ok2 := resultMap["total"].(float64)
	dead, ok3 := resultMap["dead"].(float64)
	if !ok1 || !ok2 || !ok3 {
		fmt.Fprintln(os.Stderr, "Invalid peer count data")
		os.Exit(1)
	}

	fmt.Printf("Peer Statistics\n")
	fmt.Printf("===============\n")
	fmt.Printf("Active peers: %d\n", int(active))
	fmt.Printf("Total peers:  %d\n", int(total))
	fmt.Printf("Dead peers:   %d\n", int(dead))
}

func handlePeersGet(client *rpc.Client, pubkey string) {
	result, err := client.Call("peers.get", map[string]interface{}{"pubkey": pubkey})
	if err != nil {
		fmt.Fprintf(os.Stderr, "RPC error: %v\n", err)
		os.Exit(1)
	}

	peer, ok := result.(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "Invalid response format")
		os.Exit(1)
	}

	pubkeyStr, _ := peer["pubkey"].(string)
	meshIP, _ := peer["mesh_ip"].(string)
	endpoint, _ := peer["endpoint"].(string)
	lastSeen, _ := peer["last_seen"].(string)

	fmt.Printf("Peer Information\n")
	fmt.Printf("================\n")
	fmt.Printf("Public Key:     %s\n", pubkeyStr)
	fmt.Printf("Mesh IP:        %s\n", meshIP)
	fmt.Printf("Endpoint:       %s\n", endpoint)
	fmt.Printf("Last Seen:      %s\n", lastSeen)

	if v, ok := peer["discovered_via"]; ok {
		if discoveredVia, ok := v.([]interface{}); ok && len(discoveredVia) > 0 {
			discoveredViaStr := make([]string, 0, len(discoveredVia))
			for _, item := range discoveredVia {
				if s, ok := item.(string); ok {
					discoveredViaStr = append(discoveredViaStr, s)
				}
			}
			if len(discoveredViaStr) > 0 {
				fmt.Printf("Discovered Via: %s\n", strings.Join(discoveredViaStr, ", "))
			}
		}
	}

	if routesVal, ok := peer["routable_networks"]; ok {
		if routes, ok := routesVal.([]interface{}); ok && len(routes) > 0 {
			routeStrs := make([]string, 0, len(routes))
			for _, r := range routes {
				if routeStr, ok := r.(string); ok {
					routeStrs = append(routeStrs, routeStr)
				}
			}
			if len(routeStrs) > 0 {
				fmt.Printf("Routes:         %s\n", strings.Join(routeStrs, ", "))
			}
		}
	}

	if v, ok := peer["latency_ms"]; ok && v != nil {
		if ms, ok := v.(float64); ok {
			fmt.Printf("Latency:        %.1f ms\n", ms)
		}
	} else {
		fmt.Printf("Latency:        -\n")
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// pilotCmd handles the "pilot" subcommand
func pilotCmd() {
	if len(os.Args) < 3 {
		printPilotUsage()
		os.Exit(1)
	}

	subcommand := os.Args[2]

	switch subcommand {
	case "init":
		runPilotInit()
	case "start":
		runPilotStart()
	case "status":
		runPilotStatus()
	case "report":
		runPilotReport()
	case "complete":
		runPilotComplete()
	case "validate":
		runPilotValidate()
	default:
		fmt.Fprintf(os.Stderr, "Unknown pilot subcommand: %s\n", subcommand)
		printPilotUsage()
		os.Exit(1)
	}
}

// printPilotUsage shows pilot command usage
func printPilotUsage() {
	fmt.Println(`wgmesh pilot - Pilot evaluation management

SUBCOMMANDS:
  pilot init [--org <name>] [--contact <email>] [--nodes <n>] [--mode <mode>] [--duration <days>]
                          Initialize a new pilot evaluation
  pilot start              Start the pilot evaluation
  pilot status            Show pilot status and progress
  pilot report [--format <format>] [--output <file>]
                          Generate pilot evaluation report
  pilot complete          Finalize pilot and generate summary
  pilot validate          Run pilot validation checks

OPTIONS:
  --org <name>            Organization name (required for init)
  --contact <email>       Contact email (required for init)
  --nodes <n>             Expected number of nodes (default: 3)
  --mode <mode>           Operational mode: decentralized or centralized (default: decentralized)
  --duration <days>       Pilot duration in days (default: 30)
  --format <format>       Report format: console, json, html (default: console)
  --output <file>         Output file path (default: stdout)`)
}

// runPilotInit initializes a new pilot
func runPilotInit() {
	var org, contact, mode string
	var nodes, duration int

	// Parse flags
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--org":
			if i+1 < len(args) {
				org = args[i+1]
				i++
			}
		case "--contact":
			if i+1 < len(args) {
				contact = args[i+1]
				i++
			}
		case "--nodes":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &nodes)
				i++
			}
		case "--mode":
			if i+1 < len(args) {
				mode = args[i+1]
				i++
			}
		case "--duration":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &duration)
				i++
			}
		}
	}

	// Set defaults
	if nodes == 0 {
		nodes = 3
	}
	if mode == "" {
		mode = "decentralized"
	}
	if duration == 0 {
		duration = 30
	}

	// Validate required flags
	if org == "" {
		fmt.Fprintln(os.Stderr, "Error: --org is required")
		printPilotUsage()
		os.Exit(1)
	}
	if contact == "" {
		fmt.Fprintln(os.Stderr, "Error: --contact is required")
		printPilotUsage()
		os.Exit(1)
	}

	p := pilot.New(pilot.PilotConfigPath)
	if err := p.Initialize(org, contact, nodes, mode, duration); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize pilot: %v\n", err)
		os.Exit(1)
	}

	if err := p.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save pilot configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pilot initialized successfully: %s\n", p.Config().PilotID)
	fmt.Printf("Configuration saved to %s\n", pilot.PilotConfigPath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Deploy wgmesh to your pilot nodes\n")
	fmt.Printf("  2. Run: wgmesh pilot start\n")
	fmt.Printf("  3. Monitor progress: wgmesh pilot status\n")
}

// runPilotStart starts the pilot
func runPilotStart() {
	p := pilot.New(pilot.PilotConfigPath)

	// Load existing configuration
	if err := p.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pilot configuration: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you have run 'wgmesh pilot init' first")
		os.Exit(1)
	}

	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start pilot: %v\n", err)
		os.Exit(1)
	}

	if err := p.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save pilot state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Pilot started: %s\n", p.Config().PilotID)
	fmt.Printf("Start date: %s\n", p.Config().StartDate.Format("2006-01-02"))
	fmt.Printf("End date: %s\n", p.Config().EndDate.Format("2006-01-02"))
	fmt.Printf("Duration: %d days\n", int(p.Config().EndDate.Sub(p.Config().StartDate).Hours()/24))
	fmt.Printf("\nUse 'wgmesh pilot status' to check progress\n")
}

// runPilotStatus shows pilot status
func runPilotStatus() {
	p := pilot.New(pilot.PilotConfigPath)

	if err := p.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pilot configuration: %v\n", err)
		fmt.Fprintln(os.Stderr, "Make sure you have run 'wgmesh pilot init' first")
		os.Exit(1)
	}

	state, err := p.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get pilot status: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("wgmesh Pilot Status\n")
	fmt.Printf("====================\n\n")
	fmt.Printf("Pilot ID: %s\n", state.Config.PilotID)
	fmt.Printf("Organization: %s\n", state.Config.Organization)
	fmt.Printf("Contact: %s\n", state.Config.ContactEmail)
	fmt.Printf("Mode: %s\n", state.Config.Mode)
	fmt.Printf("Node Count: %d\n\n", state.Config.NodeCount)

	fmt.Printf("Timeline\n")
	fmt.Printf("--------\n")
	fmt.Printf("Start: %s\n", state.Config.StartDate.Format("2006-01-02"))
	fmt.Printf("End: %s\n", state.Config.EndDate.Format("2006-01-02"))
	fmt.Printf("Current: Day %d of %d\n", state.DaysElapsed, int(state.Config.EndDate.Sub(state.Config.StartDate).Hours()/24))
	fmt.Printf("Phase: %s\n\n", state.CurrentPhase)

	fmt.Printf("Milestones\n")
	fmt.Printf("----------\n")
	milestoneOrder := []string{"baseline", "mesh_stability", "production_traffic", "advanced_scenarios"}
	for _, key := range milestoneOrder {
		milestone := state.Config.Milestones[key]
		if milestone == nil {
			continue
		}
		status := "○"
		if milestone.Completed {
			status = "✓"
		}
		fmt.Printf("%s %s", status, milestone.Name)
		if milestone.Completed {
			fmt.Printf(" (completed Day %d)", int(milestone.CompletedAt.Sub(state.Config.StartDate).Hours()/24))
		} else {
			targetDay := int(milestone.TargetDate.Sub(state.Config.StartDate).Hours() / 24)
			fmt.Printf(" (target: Day %d)", targetDay)
		}
		fmt.Printf("\n")
	}
}

// runPilotReport generates a pilot report
func runPilotReport() {
	var format, output string

	// Parse flags
	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--output":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		}
	}

	// Set defaults
	if format == "" {
		format = "console"
	}

	p := pilot.New(pilot.PilotConfigPath)

	if err := p.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pilot configuration: %v\n", err)
		os.Exit(1)
	}

	reportFormat := pilot.FormatConsole
	switch format {
	case "json":
		reportFormat = pilot.FormatJSON
	case "html":
		reportFormat = pilot.FormatHTML
	case "console":
		// default
	default:
		fmt.Fprintf(os.Stderr, "Unknown format: %s (use console, json, or html)\n", format)
		os.Exit(1)
	}

	if err := p.GenerateReport(reportFormat, output); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate report: %v\n", err)
		os.Exit(1)
	}
}

// runPilotComplete finalizes the pilot
func runPilotComplete() {
	p := pilot.New(pilot.PilotConfigPath)

	if err := p.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pilot configuration: %v\n", err)
		os.Exit(1)
	}

	report, err := p.Complete()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to complete pilot: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(report.FormatConsole())

	// Also save JSON output if output path is provided
	args := os.Args[3:]
	outputPath := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "--output" && i+1 < len(args) {
			outputPath = args[i+1]
			break
		}
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(report.FormatJSON()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write report: %v\n", err)
		} else {
			fmt.Printf("\nReport saved to %s\n", outputPath)
		}
	}
}

// runPilotValidate runs validation checks
func runPilotValidate() {
	p := pilot.New(pilot.PilotConfigPath)

	if err := p.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load pilot configuration: %v\n", err)
		os.Exit(1)
	}

	result, err := p.Validate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to validate pilot: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result.FormatConsole())

	// Set exit code based on validation result
	if !result.Passed {
		os.Exit(1)
	}
}
  ────────────────────────────────────────
  ▸ shell
    command: cat ./pkg/pilot/pilot.go

package pilot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// PilotConfigPath is the default path for pilot configuration
	PilotConfigPath = "/etc/wgmesh/pilot.yaml"
	// DefaultPilotDuration is the default duration in days
	DefaultPilotDuration = 30
)

// Milestone represents a pilot evaluation milestone
type Milestone struct {
	Name        string    `yaml:"name"`
	Completed   bool      `yaml:"completed"`
	CompletedAt time.Time `yaml:"completed_at,omitempty"`
	TargetDate  time.Time `yaml:"target_date,omitempty"`
}

// Phase represents a pilot phase with its milestones
type Phase struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	StartDay    int          `yaml:"start_day"`
	EndDay      int          `yaml:"end_day"`
	Milestones  []*Milestone `yaml:"milestones"`
}

// MetricsTarget defines evaluation targets
type MetricsTarget struct {
	MeshConnectivity  float64 `yaml:"mesh_connectivity"`   // percent
	PeerDiscoveryTime int     `yaml:"peer_discovery_time"` // seconds
	RoutePropagation  int     `yaml:"route_propagation"`   // seconds
	ThroughputMbps    float64 `yaml:"throughput_mbps"`
	LatencyOverheadMs int     `yaml:"latency_overhead_ms"`
}

// Config represents the pilot configuration
type Config struct {
	PilotID        string                `yaml:"pilot_id"`
	Organization   string                `yaml:"organization"`
	ContactEmail   string                `yaml:"contact_email"`
	StartDate      time.Time             `yaml:"start_date"`
	EndDate        time.Time             `yaml:"end_date"`
	Mode           string                `yaml:"mode"` // centralized or decentralized
	NodeCount      int                   `yaml:"node_count"`
	Milestones     map[string]*Milestone `yaml:"milestones"`
	MetricsTargets MetricsTarget         `yaml:"metrics_targets"`
}

// State represents the current pilot state
type State struct {
	Config       *Config
	CurrentPhase string
	DaysElapsed  int
	Started      bool
	Completed    bool
	mu           sync.RWMutex
}

// Pilot manages the pilot evaluation lifecycle
type Pilot struct {
	configPath string
	state      *State
	metrics    *Metrics
	mu         sync.RWMutex
}

// New creates a new Pilot instance
func New(configPath string) *Pilot {
	if configPath == "" {
		configPath = PilotConfigPath
	}
	return &Pilot{
		configPath: configPath,
		state: &State{
			Config: &Config{
				Milestones: make(map[string]*Milestone),
			},
		},
		metrics: NewMetrics(),
	}
}

// Initialize creates a new pilot configuration
func (p *Pilot) Initialize(org, contact string, nodeCount int, mode string, durationDays int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate inputs
	if org == "" {
		return fmt.Errorf("organization name is required")
	}
	if contact == "" {
		return fmt.Errorf("contact email is required")
	}
	if nodeCount < 1 {
		return fmt.Errorf("node count must be at least 1")
	}
	if mode != "centralized" && mode != "decentralized" {
		return fmt.Errorf("mode must be 'centralized' or 'decentralized'")
	}
	if durationDays < 7 {
		return fmt.Errorf("duration must be at least 7 days")
	}

	now := time.Now()
	pilotID := generatePilotID(org, now)

	p.state.Config = &Config{
		PilotID:      pilotID,
		Organization: org,
		ContactEmail: contact,
		StartDate:    now,
		EndDate:      now.AddDate(0, 0, durationDays),
		Mode:         mode,
		NodeCount:    nodeCount,
		Milestones:   initializeMilestones(now, durationDays),
		MetricsTargets: MetricsTarget{
			MeshConnectivity:  99.9,
			PeerDiscoveryTime: 60,
			RoutePropagation:  30,
			ThroughputMbps:    80.0,
			LatencyOverheadMs: 20,
		},
	}

	p.state.Started = false
	p.state.Completed = false

	return nil
}

// Start begins the pilot evaluation
func (p *Pilot) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state.Started {
		return fmt.Errorf("pilot already started")
	}

	if p.state.Config.PilotID == "" {
		return fmt.Errorf("pilot not initialized")
	}

	p.state.Started = true

	// Start metrics collection
	p.metrics.Start(p.state.Config.PilotID)

	return nil
}

// Status returns the current pilot state
func (p *Pilot) Status() (*State, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.state.Started {
		return nil, fmt.Errorf("pilot not started")
	}

	// Update days elapsed
	p.state.DaysElapsed = int(time.Since(p.state.Config.StartDate).Hours() / 24)

	// Update current phase
	p.state.CurrentPhase = getCurrentPhase(p.state.DaysElapsed)

	return p.state, nil
}

// Config returns the pilot configuration
func (p *Pilot) Config() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Config
}

// IsStarted returns whether the pilot has been started
func (p *Pilot) IsStarted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Started
}

// MarkMilestoneComplete marks a milestone as completed
func (p *Pilot) MarkMilestoneComplete(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	milestone, exists := p.state.Config.Milestones[name]
	if !exists {
		return fmt.Errorf("milestone not found: %s", name)
	}

	milestone.Completed = true
	milestone.CompletedAt = time.Now()

	return nil
}

// IsMilestoneComplete checks if a milestone is completed
func (p *Pilot) IsMilestoneComplete(name string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	milestone, exists := p.state.Config.Milestones[name]
	if !exists {
		return false
	}
	return milestone.Completed
}

// Save persists the pilot state to disk
func (p *Pilot) Save() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(p.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return saveConfig(p.state.Config, p.configPath)
}

// Load loads the pilot state from disk
func (p *Pilot) Load() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	config, err := loadConfig(p.configPath)
	if err != nil {
		return err
	}

	p.state.Config = config
	p.state.Started = !config.StartDate.IsZero()
	p.state.Completed = false

	return nil
}

// generatePilotID creates a unique pilot ID from organization and date
func generatePilotID(org string, t time.Time) string {
	// Normalize organization name: lowercase, replace spaces with hyphens
	orgNormalized := strings.ToLower(org)
	orgNormalized = strings.ReplaceAll(orgNormalized, " ", "-")
	orgNormalized = strings.ReplaceAll(orgNormalized, "/", "-")

	// Remove any non-alphanumeric characters except hyphens
	cleaned := ""
	for _, r := range orgNormalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			cleaned += string(r)
		}
	}

	return fmt.Sprintf("%s-%s", cleaned, t.Format("20060102"))
}

// initializeMilestones creates the standard four-phase milestones
func initializeMilestones(startDate time.Time, durationDays int) map[string]*Milestone {
	milestones := make(map[string]*Milestone)

	// Phase 1: Baseline Setup (Days 1-3)
	milestones["baseline"] = &Milestone{
		Name:       "Baseline Setup",
		TargetDate: startDate.AddDate(0, 0, 3),
		Completed:  false,
	}

	// Phase 2: Mesh Stability (Days 4-7)
	milestones["mesh_stability"] = &Milestone{
		Name:       "Mesh Stability",
		TargetDate: startDate.AddDate(0, 0, 7),
		Completed:  false,
	}

	// Phase 3: Production Traffic (Days 8-14)
	milestones["production_traffic"] = &Milestone{
		Name:       "Production Traffic",
		TargetDate: startDate.AddDate(0, 0, 14),
		Completed:  false,
	}

	// Phase 4: Advanced Scenarios (Days 15-30)
	milestones["advanced_scenarios"] = &Milestone{
		Name:       "Advanced Scenarios",
		TargetDate: startDate.AddDate(0, 0, durationDays),
		Completed:  false,
	}

	return milestones
}

// getCurrentPhase determines the current phase based on days elapsed
func getCurrentPhase(daysElapsed int) string {
	switch {
	case daysElapsed <= 3:
		return "Baseline Setup"
	case daysElapsed <= 7:
		return "Mesh Stability"
	case daysElapsed <= 14:
		return "Production Traffic"
	default:
		return "Advanced Scenarios"
	}
}
  ────────────────────────────────────────
  ▸ shell
    command: ls -la ./docs/

total 1848
drwxr-xr-x 10 wgmesh wgmesh    4096 Jun 19 23:05 .
drwxr-xr-x 24 wgmesh wgmesh    4096 Jun 19 23:40 ..
-rw-r--r--  1 wgmesh wgmesh    8880 Jun 11 05:32 access-control.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 brainstorms
-rw-r--r--  1 wgmesh wgmesh    5152 Jun 11 05:32 centralized-mode.md
-rw-r--r--  1 wgmesh wgmesh    1135 Jun 11 05:32 compat-tracking.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 dogfooding
-rw-r--r--  1 wgmesh wgmesh   12616 Jun 16 08:43 evaluation-checklist.md
-rw-r--r--  1 wgmesh wgmesh    4596 Jun 11 05:32 FAQ.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 ideation
-rw-r--r--  1 wgmesh wgmesh   78981 Jun 19 07:05 index.html
-rw-r--r--  1 wgmesh wgmesh    2157 Jun 11 05:32 install-verification.md
-rw-r--r--  1 wgmesh wgmesh   11273 Jun 16 08:43 pilot-evaluation-guide.md
-rw-r--r--  1 wgmesh wgmesh    9490 Jun 19 21:19 pipeline-flow.d2
-rw-r--r--  1 wgmesh wgmesh 1578777 Jun 11 05:32 pipeline-flow.png
-rw-r--r--  1 wgmesh wgmesh   93545 Jun 11 05:32 pipeline-flow.svg
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 plans
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 pulse-reports
-rw-r--r--  1 wgmesh wgmesh   10212 Jun 11 05:32 quickstart.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 research
drwxr-xr-x  5 wgmesh wgmesh    4096 Jun 11 05:32 solutions
-rw-r--r--  1 wgmesh wgmesh    1603 Jun 11 05:32 troubleshooting.md
drwxr-xr-x  2 wgmesh wgmesh    4096 Jun 11 05:32 use-cases
  ────────────────────────────────────────
  ▸ shell
    command: cat ./docs/evaluation-checklist.md

# Evaluation Checklist for Network Administrators

Use this checklist to assess whether wgmesh fits your use case in roughly 15 minutes.
Work through each section top to bottom; the final section gives you a clear go/no-go recommendation.

---

## Section 1 — Infrastructure Requirements (5 minutes)

Mark each item ✅ (met), ❌ (not met), or ⚠️ (needs investigation).

### 1.1 — Kernel / OS

| Requirement | Notes | Status |
|-------------|-------|--------|
| Linux kernel ≥ 5.6, **or** macOS with `wireguard-go` installed | WireGuard is built into Linux 5.6+; older kernels need the DKMS module | |
| `wireguard-tools` package installed (`wg` command available) | `apt install wireguard-tools` / `yum install wireguard-tools` | |
| Root access or `CAP_NET_ADMIN` capability on each node | Required to create and configure WireGuard interfaces | |

### 1.2 — Network

| Requirement | Notes | Status |
|-------------|-------|--------|
| Outbound UDP is not completely blocked on every node | At minimum, one node must be reachable over UDP for direct connections | |
| At least one node has a public IP **or** NAT traversal (UDP hole-punching) is acceptable | Nodes behind symmetric NAT may require a relay path | |
| Nodes can reach the public internet for initial DHT discovery | Outbound UDP to DHT bootstrap nodes, or HTTPS to GitHub API for the registry fallback | |

### 1.3 — Storage and State

| Requirement | Notes | Status |
|-------------|-------|--------|
| `/var/lib/wgmesh/` is writable on each node | Stores WireGuard keypair and peer cache; ~few KB per node | |
| Systemd available (if persistent service is required) | `wgmesh install-service` creates a `wgmesh.service` unit | |

**Section 1 verdict:** If any item is ❌, resolve it before proceeding. Items marked ⚠️ are addressed in Section 3.

---

## Section 2 — Use Case Fit (5 minutes)

Answer each question and tally the mode recommendations.

### 2.1 — Topology Questions

**Q1: How many nodes will the mesh contain at steady state?**
- ≤ 50 nodes → **decentralized mode preferred**
- 50–200 nodes → **either mode works; decentralized scales to this range**
- > 200 nodes → ⚠️ evaluate centralized mode or contact the project for guidance

**Q2: Who manages node additions and removals?**
- Nodes join/leave autonomously (e.g., auto-scaling, developer laptops) → **decentralized mode**
- An operator controls all changes via SSH from a central host → **centralized mode**

**Q3: Are all nodes behind NAT, or do some have public IPs?**
- All behind NAT → decentralized mode with UDP hole-punching; check that at least one bootstrap peer is reachable
- At least one node has a public IP → direct endpoint configuration; both modes handle this

**Q4: Do you require site-to-site routing (advertising subnets)?**
- Yes → both modes support `--advertise-routes`; decentralized mode propagates routes automatically
- No → no impact on mode choice

### 2.2 — Operational Questions

**Q5: Is there a central operations team that manages WireGuard state?**
- Yes → **centralized mode** (state file in `mesh-state.json`, SSH-based deployment)
- No / decentralized DevOps → **decentralized mode**

**Q6: Do nodes need to discover each other without pre-sharing IP addresses?**
- Yes (dynamic IPs, cloud auto-scaling, remote workers) → **decentralized mode required**
- No (static IP fleet) → either mode works

**Q7: Is an encrypted state file at rest required?**
- Yes → centralized mode supports AES-256-GCM + PBKDF2 encryption of `mesh-state.json`
- No preference → either mode

### 2.3 — Mode Recommendation

Count your answers:
- Mostly decentralized → proceed with **decentralized mode** (`wgmesh join`)
- Mostly centralized → proceed with **centralized mode** (`wgmesh -deploy`)
- Mixed → decentralized mode is the default; use centralized for operator-controlled fleets

---

## Section 3 — Decision Framework: wgmesh vs Alternatives (2 minutes)

Use this table to confirm wgmesh is the right tool. If a competing tool better fits your profile, the ❌ cells explain why.

| Scenario | wgmesh | Tailscale | Netmaker | innernet |
|----------|--------|-----------|----------|----------|
| No coordination server to host or trust | ✅ (DHT-based, no server) | ❌ (requires Tailscale control plane) | ❌ (requires Netmaker server) | ❌ (requires innernet server) |
| Self-hosted, open-source, auditable | ✅ | ❌ (SaaS) | ✅ (self-hosted) | ✅ (self-hosted) |
| Serverless peer discovery (NAT traversal included) | ✅ | ✅ | ❌ (server required) | ❌ (server required) |
| Share one secret to add any node | ✅ | ❌ (ACL/invite required) | ❌ (token per node) | ❌ (certificate per node) |
| macOS and Linux support | ✅ | ✅ | ✅ | ✅ |
| Windows support | ❌ (not yet) | ✅ | ✅ | ❌ |
| Web UI / dashboard | ❌ | ✅ | ✅ | ❌ |
| Per-node access control lists | ✅ (centralized mode policy engine) | ✅ | ✅ | ✅ |
| No dedicated coordination server to operate | ✅ (DHT + GitHub Issues registry) | ❌ (requires Tailscale control plane) | ❌ (requires Netmaker server) | ❌ (requires innernet server) |

**Choose wgmesh when:** you want a coordination-server-free, self-hosted mesh where any node can join with a shared secret, and you do not need Windows support or a web dashboard.

**Choose an alternative when:** you need Windows clients, a web UI, or an enterprise-grade access-control system with per-user identity.

---

## Section 4 — Evaluation Test Scenarios (3 minutes to read; 15–30 minutes to run)

Run these tests during your pilot to confirm wgmesh behaves as expected.

### Test A — Two-node basic mesh (decentralized mode)

**Setup:** Two hosts (can be VMs or VPS). Both must have outbound internet access.

**Steps:**
1. On **host-1**: `wgmesh init --secret` → copy the printed secret.
2. On **host-1**: `sudo wgmesh join --secret "<secret>" --log-level debug`
3. On **host-2**: `sudo wgmesh join --secret "<secret>" --log-level debug`
4. Wait up to 30 seconds for DHT discovery.
5. On **host-1**: `wgmesh peers list`

**Pass criteria:**
- `wgmesh peers list` shows **host-2** with a mesh IP and a non-stale `LAST SEEN` timestamp.
- `ping <host-2-mesh-ip>` from **host-1** succeeds.
- `sudo wg show` on **host-1** shows **host-2** with a recent `latest-handshake`.

**Fail indicator:** No peers appear after 2 minutes → check UDP outbound; run the daemon in foreground with `--log-level debug` and look for `[dht]` lines in the output.

---

### Test B — NAT traversal (both hosts behind NAT)

**Setup:** Two hosts behind different NAT gateways (e.g., two cloud VMs in different VPCs without public IPs, or two developer laptops on different home networks).

**Steps:**
1. Repeat Test A steps with both hosts behind NAT.
2. Observe `ENDPOINT` column in `wgmesh peers list`.

**Pass criteria:**
- Peer appears with an `ENDPOINT` of the form `<public-ip>:<port>` (hole-punched) **or** `(relayed)`.
- `ping <mesh-ip>` succeeds in either case.

**Note:** Symmetric NAT on both ends may result in `(relayed)` — this is expected and functional.

---

### Test C — Subnet advertisement (site-to-site routing)

**Setup:** **host-1** has a private subnet `192.168.10.0/24` behind it (or a loopback alias for testing).

**Steps:**
1. On **host-1**:
   ```bash
   sudo wgmesh join --secret "<secret>" --advertise-routes "192.168.10.0/24"
   ```
2. On **host-2**: `wgmesh peers list` → confirm **host-1** shows `ROUTES: 192.168.10.0/24`.
3. On **host-2**: `ip route get 192.168.10.1` → output should route through `wg0`.
4. On **host-2**: `ping 192.168.10.1` (or a host in that subnet).

**Pass criteria:**
- Route `192.168.10.0/24 via <host-1-mesh-ip> dev wg0` is present on **host-2**.
- Traffic to the subnet is forwarded correctly.

---

### Test D — Node restart / persistence

**Steps:**
1. After Test A is passing, stop the daemon on **host-1**: `sudo systemctl stop wgmesh` (or Ctrl-C).
2. Restart: `sudo systemctl start wgmesh` (or re-run `sudo wgmesh join ...`).
3. On **host-1**: wait 30 seconds, then `wgmesh peers list`.

**Pass criteria:**
- **host-2** reappears in the peer list within 60 seconds without manual intervention.
- Same mesh IPs are used as before (deterministic from secret).

---

### Test E — Adding a third node

**Steps:**
1. While Test A is passing, bring up **host-3** with the same secret.
2. On **host-1** and **host-2**: `wgmesh peers list` after 30 seconds.

**Pass criteria:**
- All three nodes see each other.
- `wgmesh peers count` returns `2` on each node (two remote peers).

---

## Section 5 — Pilot Setup Recommendations

### Minimal pilot topology

- **2–3 nodes**: one with a public IP (or Hetzner/DigitalOcean cheapest VPS), one or two behind NAT.
- **Duration**: 48–72 hours continuous operation to confirm stability and peer reconnection after restarts.
- **Monitoring**: tail daemon logs (`journalctl -u wgmesh -f`) and periodically run `wgmesh peers list`.

### Recommended pilot sequence

1. **Day 1 — Basic connectivity (Test A + Test B):** Validate peer discovery and NAT traversal.
2. **Day 1–2 — Subnet routing (Test C):** If site-to-site is required, validate route propagation.
3. **Day 2 — Persistence (Test D):** Simulate reboots and daemon restarts; confirm auto-recovery.
4. **Day 2–3 — Scale (Test E):** Add the third node; confirm mesh expands correctly.
5. **Day 3 — Load and stability:** Keep the mesh running for 24 hours; check for unexpected disconnections in logs.

### Success criteria for go decision

All of the following must be true after the pilot:

- [ ] All pilot nodes appear in `wgmesh peers list` on each other within 60 seconds of starting.
- [ ] `ping <mesh-ip>` succeeds between every node pair.
- [ ] Daemon survives a node reboot without manual reconfiguration.
- [ ] If subnet routing is required: routes are present and traffic flows.
- [ ] No unexplained crashes in `journalctl -u wgmesh` over 24 hours of operation.

### No-go triggers

Stop the evaluation and investigate (or file a bug) if:

- Peers never appear after 5 minutes with `--log-level debug` showing no DHT activity.
- `ping` between mesh IPs times out despite both peers showing in `wgmesh peers list`.
- Daemon exits unexpectedly within 24 hours of running.
- Required feature (e.g., Windows support, web UI) is in the ❌ column of Section 3.

---

## Summary: Go / No-Go Decision

| Signal | Go | No-Go |
|--------|----|-------|
| Section 1: All infrastructure items ✅ | ✅ | Any ❌ unresolved |
| Section 2: Mode recommendation is clear | ✅ | No clear mode fit |
| Section 3: wgmesh column fits your scenario | ✅ | Competing tool fits better |
| Section 4: Tests A–D pass | ✅ | Any test fails after troubleshooting |
| Section 5: Pilot success criteria met | ✅ | Any criterion unmet |

If all rows are **Go**: proceed to production rollout using [docs/quickstart.md](quickstart.md) (decentralized) or [docs/centralized-mode.md](centralized-mode.md) (centralized).

If any row is **No-Go**: file an issue at https://github.com/atvirokodosprendimai/wgmesh/issues with the failing section and debug output.

---

## Section 6 — Structured 30-Day Pilot (Optional)

For a more rigorous, multi-week evaluation with automated milestone tracking and metrics collection, use the built-in pilot framework. This is recommended for production-readiness assessments.

See [pilot-evaluation-guide.md](pilot-evaluation-guide.md) for the full walkthrough.

### Quick pilot setup

```bash
# 1. Initialize pilot with your organization details
wgmesh pilot init --org "Your Org" --contact admin@yourorg.com --nodes 5

# 2. Generate a mesh secret
wgmesh init --secret

# 3. Start the pilot clock
wgmesh pilot start

# 4. Deploy to pilot nodes
wgmesh join --secret "wgmesh://v1/..."

# 5. Monitor progress at any time
wgmesh pilot status
wgmesh pilot validate
wgmesh pilot report

# 6. Finalize after evaluation period
wgmesh pilot complete
```

### Pilot milestones

| Phase | Days | Goal |
|-------|------|------|
| Baseline Setup | 1–3 | Successful deployment and basic connectivity |
| Mesh Stability | 4–7 | ≥99.9% connectivity uptime, zero crashes |
| Production Traffic | 8–14 | Throughput and latency within targets |
| Advanced Scenarios | 15–30 | NAT traversal, relay fallback, secret rotation |

### Pilot success criteria

- [ ] Pilot report shows ≥99.9% mesh connectivity
- [ ] All four milestones completed
- [ ] Overall rating: Good or Excellent
- [ ] Zero daemon crashes over the evaluation period
  ────────────────────────────────────────
  ▸ shell
    command: cat ./docs/pilot-evaluation-guide.md

# wgmesh Pilot Evaluation Guide

This guide walks network administrators through a structured 30-day evaluation of wgmesh
using the built-in pilot framework. By the end of this evaluation you will have measurable
data on mesh stability, peer discovery, NAT traversal, and operational readiness.

---

## Prerequisites

Before starting the pilot, ensure your environment meets these requirements:

| Requirement | Details |
|-------------|---------|
| Linux kernel ≥ 5.6 | WireGuard is built into Linux 5.6+ |
| `wireguard-tools` installed | `apt install wireguard-tools` or equivalent |
| Root access or `CAP_NET_ADMIN` | Required to create WireGuard interfaces |
| Outbound UDP not blocked | At least one node must be reachable over UDP |
| 2–5 test nodes | Mix of public IP and NAT-behind nodes recommended |
| `/etc/wgmesh/` writable | Stores pilot configuration and peer cache |
| Systemd (optional) | For persistent service via `install-service` |

See [evaluation-checklist.md](evaluation-checklist.md) for a detailed pre-evaluation checklist.

---

## Quick Start

### 1. Initialize the Pilot (Day 0)

On your primary evaluation node:

```bash
wgmesh pilot init \
  --org "Your Organization" \
  --contact admin@yourorg.com \
  --nodes 5 \
  --mode decentralized \
  --duration 30
```

This creates `/etc/wgmesh/pilot.yaml` with default milestones and metrics targets.

### 2. Generate a Mesh Secret

```bash
wgmesh init --secret
```

Copy the printed `wgmesh://v1/...` URI.

### 3. Start the Pilot

```bash
wgmesh pilot start
```

This starts the 30-day evaluation clock and begins metrics collection.

### 4. Deploy to Pilot Nodes

On each node, join the mesh:

```bash
wgmesh join --secret "wgmesh://v1/..."
```

For persistent operation:

```bash
wgmesh install-service --secret "wgmesh://v1/..."
```

### 5. Monitor Progress

```bash
wgmesh pilot status       # Current phase, milestones, days elapsed
wgmesh pilot validate     # Run health checks
wgmesh pilot report       # Generate evaluation report
```

---

## Four-Phase Milestone Structure

The pilot progresses through four phases, each with specific validation criteria.

### Phase 1: Baseline Setup (Days 1–3)

**Goal:** Successful deployment and basic connectivity.

**Tasks:**
- Install wgmesh on all pilot nodes
- Configure mesh secret and join mesh
- Verify peer discovery across all nodes
- Run `wgmesh pilot status` to confirm Phase 1 progress

**Validation:**
- [ ] All peers visible in `wgmesh peers list`
- [ ] `ping <mesh-ip>` succeeds between all peer pairs
- [ ] No interface churn (WireGuard restart loops)
- [ ] `wgmesh pilot validate` reports no errors

**Commands to verify:**
```bash
wgmesh peers list                    # Should show all pilot nodes
wg show                              # Check latest handshake times
ping <mesh-ip-of-another-node>       # Verify connectivity
wgmesh pilot validate                # Run health checks
```

**Mark milestone complete:**
```bash
# Milestones are tracked automatically; you can also manually mark:
# (via the pilot status reporting — all milestones appear in reports)
```

---

### Phase 2: Mesh Stability (Days 4–7)

**Goal:** Verify mesh stability under normal operations.

**Tasks:**
- Run continuous connectivity tests (24h soak)
- Verify route propagation after network changes
- Test graceful node restart and reconnection
- Log key metrics: connection uptime, discovery success rate

**Validation:**
- [ ] ≥99.9% connectivity uptime between all nodes
- [ ] All routes propagate within 30 seconds of topology change
- [ ] Zero daemon crashes or WireGuard interface crashes
- [ ] NAT type detection completed for all nodes

**Commands to verify:**
```bash
wgmesh pilot status                  # Check milestone progress
wgmesh peers count                   # Verify all peers still active
journalctl -u wgmesh --since "24 hours ago" | grep -i error
wgmesh pilot report                  # Generate progress report
```

**Simulating a node restart:**
```bash
# On the test node:
sudo systemctl restart wgmesh        # Or: Ctrl-C and re-run join

# Wait 60 seconds, then from another node:
wgmesh peers list                    # Node should reappear
```

---

### Phase 3: Production Traffic Simulation (Days 8–14)

**Goal:** Validate under realistic workload.

**Tasks:**
- Route application traffic through mesh
- Measure throughput and latency
- Test with intermittent network failures (simulated outages)
- Exercise all discovery layers (registry, LAN, DHT, gossip)

**Validation:**
- [ ] Throughput ≥80% of native WireGuard baseline
- [ ] Latency overhead <20ms compared to native WireGuard
- [ ] Successful recovery from simulated network partitions
- [ ] All discovery layers successfully used

**Throughput testing:**
```bash
# On node A (server):
iperf3 -s -B <mesh-ip-a>

# On node B (client):
iperf3 -c <mesh-ip-a> -t 30         # 30-second throughput test
```

**Latency testing:**
```bash
# Native WireGuard latency:
ping -c 100 <mesh-ip> | tail -1

# Compare with direct IP latency to assess overhead
```

**Simulating network failure:**
```bash
# Temporarily block traffic on one node:
sudo iptables -A OUTPUT -d <other-node-ip> -j DROP

# Wait 30 seconds, then restore:
sudo iptables -D OUTPUT -d <other-node-ip> -j DROP

# Verify reconnection:
wgmesh peers list
```

---

### Phase 4: Advanced Scenarios & NAT Traversal (Days 15–30)

**Goal:** Stress-test edge cases and operational workflows.

**Tasks:**
- Deploy nodes behind diverse NAT types (Full Cone, Symmetric, etc.)
- Test relay fallback when direct connection fails
- Verify secret rotation workflow
- Exercise operational procedures: daemon restart, config reload, node add/remove

**Validation:**
- [ ] Successful hole-punching across all NAT type combinations
- [ ] Relay fallback engages within 60 seconds of direct path failure
- [ ] Zero secret leaks or key derivation failures
- [ ] Clean node addition/removal with no orphaned WireGuard configs

**Adding a new node mid-pilot:**
```bash
# On the new node:
wgmesh join --secret "wgmesh://v1/..."

# From existing nodes:
wgmesh peers list                    # New node should appear within 60s
```

**Removing a node:**
```bash
# On the node being removed:
sudo wgmesh uninstall-service        # If running as service
sudo systemctl stop wgmesh           # Stop the daemon

# Other nodes will mark it as stale after timeout
```

**Secret rotation:**
```bash
wgmesh rotate-secret --current "wgmesh://v1/old-secret"
# Follow printed instructions to deploy new secret
```

---

## Reports and Metrics

### Generating Reports

```bash
# Console report (default)
wgmesh pilot report

# JSON export for automated analysis
wgmesh pilot report --format json --output pilot-report.json

# HTML report for executive summary
wgmesh pilot report --format html --output pilot-report.html
```

### Default Metrics Targets

| Metric | Target | Description |
|--------|--------|-------------|
| Mesh Connectivity | ≥99.9% | Uptime between all peer pairs |
| Peer Discovery Time | ≤60s | Time for all nodes to discover each other |
| Route Propagation | ≤30s | Time for route changes to propagate |
| Throughput | ≥80 Mbps | Compared to native WireGuard |
| Latency Overhead | ≤20ms | Additional latency vs native WireGuard |

### Report Sections

Each report contains:

1. **Milestone Status** — Progress through the four phases
2. **Key Metrics** — Connectivity, discovery, route propagation, restart counts
3. **Discovery Layer Distribution** — Usage of Registry, LAN, DHT, and Gossip layers
4. **NAT Types Detected** — Breakdown of NAT types across pilot nodes
5. **Issues / Warnings** — Any metrics below targets or operational issues
6. **Next Steps** — Guidance on what to do next

---

## Completing the Pilot

### Final Evaluation

```bash
wgmesh pilot complete
```

This produces a final report with:

- **Overall Rating**: Excellent / Good / Fair / Poor
- **Recommendation**: Production readiness assessment
- **Milestone Summary**: Which milestones were completed
- **Metrics Summary**: Final metrics across the evaluation period

### Rating Criteria

| Rating | Score | Meaning |
|--------|-------|---------|
| Excellent | ≥90 | Ready for production deployment |
| Good | 70–89 | Suitable for production with monitoring |
| Fair | 50–69 | Requires investigation before production |
| Poor | <50 | Not recommended for production |

### Saving Results

```bash
# Save final report as JSON
wgmesh pilot complete --output pilot-final.json

# The console output shows the human-readable summary
```

---

## Troubleshooting

### Peers Not Discovered

**Symptoms:** `wgmesh peers list` shows no peers after 5+ minutes.

**Check:**
```bash
# Verify both nodes use the same secret
wgmesh status --secret "wgmesh://v1/..."   # Compare Network ID

# Check UDP connectivity
wgmesh test-peer --secret "wgmesh://v1/..." --peer <ip:port>

# Check firewall rules
sudo iptables -L -n | grep -i drop
```

**Common causes:**
- Different secrets on different nodes (Network IDs won't match)
- Firewall blocking outbound UDP
- No internet access for DHT bootstrap

### High Latency

**Symptoms:** Latency overhead exceeds 20ms target.

**Check:**
```bash
# Compare native vs mesh latency
ping -c 10 <direct-ip>     # Direct latency
ping -c 10 <mesh-ip>       # Mesh latency

# Check if relay is being used
wgmesh peers list          # Look for "(relayed)" in endpoint
```

**Common causes:**
- Relay path instead of direct (Symmetric NAT)
- High baseline latency between regions

### Daemon Crashes

**Symptoms:** Daemon exits unexpectedly, shown in pilot report as restart count.

**Check:**
```bash
journalctl -u wgmesh --since "1 hour ago" | grep -i "panic\|fatal\|error"
```

**Action:** File an issue at https://github.com/atvirokodosprendimai/wgmesh/issues
with the pilot report and relevant log output.

### Connectivity Drops

**Symptoms:** Pilot report shows mesh connectivity below 99.9%.

**Check:**
```bash
# Monitor connectivity in real-time
watch -n 5 'wg show wg0 | grep latest-handshake'

# Check for WireGuard interface issues
sudo wg show wg0
```

---

## FAQ

**Q: Can I run the pilot with fewer than 5 nodes?**
A: Yes. Set `--nodes` to your actual count. The minimum is 1, but 2–3 is recommended
   for meaningful evaluation.

**Q: Can I shorten the pilot duration?**
A: Yes. The minimum is 7 days (`--duration 7`), but the full 30-day evaluation
   provides the most comprehensive results.

**Q: Do I need a separate node for pilot management?**
A: No. The pilot commands run on any node with the pilot configuration. Typically
   you run them on your primary evaluation node.

**Q: Can I run multiple pilots simultaneously?**
A: No. The current implementation supports one pilot at a time per configuration
   file. Use separate configuration paths if needed.

**Q: What happens if I restart the daemon?**
A: Daemon restarts are tracked in pilot metrics. A few restarts during evaluation
   are acceptable; excessive restarts will lower the overall rating.

**Q: How do I reset and start over?**
A: Remove the pilot configuration and reinitialize:
```bash
sudo rm /etc/wgmesh/pilot.yaml
wgmesh pilot init --org "..." --contact "..."
wgmesh pilot start
```Now I'll create the specification for the onboarding checklist widget:-32602: Could not parse tool arguments: {"path":"/opt/wgmesh-checkout/specs/issue-783-spec.md","content":"# Specification: Issue #783\n\n## Classification\nfeature\n\n## Problem Analysis\n\nNew users of wgmesh face a \"cold start\" problem: after running `wgmesh join`, there is\nno clear indication of what steps are needed to achieve a fully functional mesh network.\nUsers must manually check peer discovery, WireGuard interface status, route propagation,\nand network connectivity without structured guidance.\n\nThe current CLI provides individual commands (`wgmesh peers list`, `wg show`, `ping`,\netc.) but lacks an integrated view of \"time to first mesh\" — the critical path from\ninstallation to first successful mesh connection. This creates several issues:\n\n1. **Unclear progress** — Users don't know if they're waiting normally for DHT discovery\n   or if something is misconfigured.\n\n2. **No troubleshooting guidance** — When peers don't appear, users must manually\n   diagnose across multiple commands and documentation files.\n\n3. **Incomplete verification** — Users may assume the mesh is working when WireGuard\n   shows peers but actual connectivity hasn't been verified.\n\n4. **No dashboard** — While wgmesh is CLI-first, a visual progress tracker would\n   significantly improve onboarding experience for new users.\n\nThis spec proposes an \"onboarding checklist widget\" that displays as an ASCII-based\ndashboard in the terminal, showing real-time progress toward a working mesh network.\nThe widget is designed to be:\n- Always available during initial mesh setup\n- Non-intrusive for experienced users (can be dismissed)\n- Actionable with clear next steps\n- Integrated with existing pilot evaluation framework\n\n## Proposed Approach\n\n### Core Concept: Terminal-Based Dashboard Widget\n\nAdd a new subcommand `wgmesh onboard` that launches a real-time progress dashboard\nshowing the onboarding checklist. The widget updates automatically as the mesh\ninitializes and peers are discovered.\n\n**Key Design Decisions:**\n\n1. **Terminal-based, not web UI** — Fits wgmesh's CLI-first architecture; no\n   additional dependencies or services required.\n\n2. **Automatic detection** — Widget reads actual daemon state via RPC socket and\n   system status via `wg show` and network checks.\n\n3. **Non-blocking** — Widget runs in a separate goroutine with periodic updates;\n   user can exit with `q` or Ctrl-C without disrupting the daemon.\n\n4. **Pilot integration** — Checklist items align with pilot milestones; the widget\n   serves as the \"day 1\" view for new installations.\n\n### Task 1: Create `pkg/onboard` Package\n\nCreate a new package `pkg/onboard` with the following structure:\n\n```\npkg/onboard/\n├── checklist.go      # Checklist definition and state tracking\n├── dashboard.go      # Terminal UI rendering (tcell-based)\n├── monitor.go       # Daemon and network state monitoring\n└── onboarding_test.go\n```\n\n**checklist.go** — Define the onboarding checklist items:\n\n```go\npackage onboard\n\nimport \"time\"\n\n// CheckItem represents a single onboarding checklist item\ntype CheckItem struct {\n    ID          string\n    Name        string\n    Description string\n    Status      ItemStatus\n    LastChecked time.Time\n    Error       string\n    ActionHint  string\n}\n\ntype ItemStatus int\n\nconst (\n    StatusPending ItemStatus = iota\n    StatusInProgress\n    StatusComplete\n    StatusFailed\n    StatusSkipped\n)\n\n// Checklist represents the onboarding checklist\ntype Checklist struct {\n    items []*CheckItem\n    mu    sync.RWMutex\n}\n\n// NewChecklist creates a new onboarding checklist\nfunc NewChecklist() *Checklist {\n    return &Checklist{\n        items: []*CheckItem{\n            {\n                ID:          \"wireguard_interface\",\n                Name:        \"WireGuard Interface\",\n                Description: \"WireGuard interface wg0 is up and configured\",\n                Status:      StatusPending,\n                ActionHint:  \"Check: sudo wg show wg0\",\n            },\n            {\n                ID:          \"dht_bootstrap\",\n                Name:        \"DHT Bootstrap\",\n                Description: \"Connected to DHT bootstrap nodes for peer discovery\",\n                Status:      StatusPending,\n                ActionHint:  \"Check: journalctl -u wgmesh | grep -i dht\",\n            },\n            {\n                ID:          \"peer_discovery\",\n                Name:        \"Peer Discovery\",\n                Description: \"At least one peer discovered (via DHT, LAN, or registry)\",\n                Status:      StatusPending,\n                ActionHint:  \"Run: wgmesh peers list\",\n            },\n            {\n                ID:          \"wireguard_handshake\",\n                Name:        \"WireGuard Handshake\",\n                Description: \"Successful WireGuard handshake with at least one peer\",\n                Status:      StatusPending,\n                ActionHint:  \"Check: sudo wg show wg0 | grep handshake\",\n            },\n            {\n                ID:          \"mesh_connectivity\",\n                Name:        \"Mesh Connectivity\",\n                Description: \"Can ping at least one peer mesh IP\",\n                Status:      StatusPending,\n                ActionHint:  \"Run: ping <mesh-ip>\",\n            },\n            {\n                ID:          \"route_propagation\",\n                Name:        \"Route Propagation\",\n                Description: \"Advertised routes propagated to peers (if any)\",\n                Status:      StatusSkipped, // Skipped if no routes advertised\n                ActionHint:  \"Run: ip route show | grep wg0\",\n            },\n        },\n    }\n}\n\n// Items returns a snapshot of current checklist items\nfunc (c *Checklist) Items() []*CheckItem {\n    c.mu.RLock()\n    defer c.mu.RUnlock()\n    \n    result := make([]*CheckItem, len(c.items))\n    for i, item := range c.items {\n        itemCopy := *item\n        result[i] = &itemCopy\n    }\n    return result\n}\n\n// UpdateItem updates the status of a checklist item\nfunc (c *Checklist) UpdateItem(id string, status ItemStatus, errMsg string) {\n    c.mu.Lock()\n    defer c.mu.Unlock()\n    \n    for _, item := range c.items {\n        if item.ID == id {\n            item.Status = status\n            item.LastChecked = time.Now()\n            item.Error = errMsg\n            break\n        }\n    }\n}\n```\n\n**monitor.go** — Monitor daemon and network state:\n\n```go\npackage onboard\n\nimport (\n    \"context\"\n    \"net\"\n    \"os/exec\"\n    \"strings\"\n    \"time\"\n\n    \"github.com/atvirokodosprendimai/wgmesh/pkg/rpc\"\n)\n\n// Monitor checks daemon and network state\ntype Monitor struct {\n    checklist *Checklist\n    client    *rpc.Client\n    interval  time.Duration\n}\n\n// NewMonitor creates a new state monitor\nfunc NewMonitor(socketPath string, checklist *Checklist) (*Monitor, error) {\n    client, err := rpc.NewClient(socketPath)\n    if err != nil {\n        return nil, err\n    }\n    \n    return &Monitor{\n        checklist: checklist,\n        client:    client,\n        interval:  5 * time.Second,\n    }, nil\n}\n\n// Run starts the monitoring loop\nfunc (m *Monitor) Run(ctx context.Context) {\n    ticker := time.NewTicker(m.interval)\n    defer ticker.Stop()\n    \n    for {\n        select {\n        case <-ctx.Done():\n            return\n        case <-ticker.C:\n            m.checkAll()\n        }\n    }\n}\n\n// checkAll runs all checks and updates the checklist\nfunc (m *Monitor) checkAll() {\n    m.checkWireGuardInterface()\n    m.checkDHTBootstrap()\n    m.checkPeerDiscovery()\n    m.checkWireGuardHandshake()\n    m.checkMeshConnectivity()\n    m.checkRoutePropagation()\n}\n\n// checkWireGuardInterface checks if wg0 interface exists and is up\nfunc (m *Monitor) checkWireGuardInterface() {\n    // Use 'ip link show wg0' or 'wg show wg0'\n    cmd := exec.Command(\"wg\", \"show\", \"wg0\")\n    output, err := cmd.CombinedOutput()\n    \n    if err != nil {\n        m.checklist.UpdateItem(\"wireguard_interface\", StatusFailed, \"interface not found\")\n        return\n    }\n    \n    if strings.Contains(string(output), \"listening port\") {\n        m.checklist.UpdateItem(\"wireguard_interface\", StatusComplete, \"\")\n    } else {\n        m.checklist.UpdateItem(\"wireguard_interface\", StatusInProgress, \"interface exists but not configured\")\n    }\n}\n\n// checkDHTBootstrap checks DHT bootstrap progress via daemon logs or metrics\nfunc (m *Monitor) checkDHTBootstrap() {\n    // Query daemon for peer counts to infer DHT activity\n    result, err := m.client.Call(\"peers.count\", nil)\n    if err != nil {\n        m.checklist.UpdateItem(\"dht_bootstrap\", StatusInProgress, \"waiting for DHT connection\")\n        return\n    }\n    \n    resultMap, ok := result.(map[string]interface{})\n    if !ok {\n        m.checklist.UpdateItem(\"dht_bootstrap\", StatusFailed, \"invalid response\")\n        return\n    }\n    \n    // If we got a response, daemon is running and likely connected to DHT\n    m.checklist.UpdateItem(\"dht_bootstrap\", StatusComplete, \"\")\n}\n\n// checkPeerDiscovery checks if any peers are discovered\nfunc (m *Monitor) checkPeerDiscovery() {\n    result, err := m.client.Call(\"peers.list\", nil)\n    if err != nil {\n        m.checklist.UpdateItem(\"peer_discovery\", StatusInProgress, \"querying daemon\")\n        return\n    }\n    \n    resultMap, ok := result.(map[string]interface{})\n    if !ok {\n        m.checklist.UpdateItem(\"peer_discovery\", StatusFailed, \"invalid response\")\n        return\n    }\n    \n    peersData, ok := resultMap[\"peers\"].([]interface{})\n    if !ok {\n        m.checklist.UpdateItem(\"peer_discovery\", StatusFailed, \"invalid peer data\")\n        return\n    }\n    \n    if len(peersData) > 0 {\n        m.checklist.UpdateItem(\"peer_discovery\", StatusComplete, \"\")\n    } else {\n        m.checklist.UpdateItem(\"peer_discovery\", StatusInProgress, \"waiting for peer discovery\")\n    }\n}\n\n// checkWireGuardHandshake checks if any WireGuard handshake occurred\nfunc (m *Monitor) checkWireGuardHandshake() {\n    cmd := exec.Command(\"wg\", \"show\", \"wg0\")\n    output, err := cmd.CombinedOutput()\n    \n    if err != nil {\n        m.checklist.UpdateItem(\"wireguard_handshake\", StatusFailed, \"cannot query WireGuard\")\n        return\n    }\n    \n    // Look for \"latest handshake\" with recent timestamp\n    outputStr := string(output)\n    if strings.Contains(outputStr, \"latest handshake\") {\n        // Simple heuristic: if handshake line exists, mark complete\n        // A more sophisticated check would parse the timestamp\n        m.checklist.UpdateItem(\"wireguard_handshake\", StatusComplete, \"\")\n    } else {\n        m.checklist.UpdateItem(\"wireguard_handshake\", StatusInProgress, \"waiting for handshake\")\n    }\n}\n\n// checkMeshConnectivity checks if we can ping a peer mesh IP\nfunc (m *Monitor) checkMeshConnectivity() {\n    // Get peers from daemon\n    result, err := m.client.Call(\"peers.list\", nil)\n    if err != nil {\n        m.checklist.UpdateItem(\"mesh_connectivity\", StatusInProgress, \"no peers to test\")\n        return\n    }\n    \n    resultMap, ok := result.(map[string]interface{})\n    if !ok {\n        return\n    }\n    \n    peersData, ok := resultMap[\"peers\"].([]interface{})\n    if !ok || len(peersData) == 0 {\n        return\n    }\n    \n    // Try to ping the first peer's mesh IP\n    for _, peerData := range peersData {\n        peer, ok := peerData.(map[string]interface{})\n        if !ok {\n            continue\n        }\n        \n        meshIP, ok := peer[\"mesh_ip\"].(string)\n        if !ok || meshIP == \"\" {\n            continue\n        }\n        \n        // Ping test (single packet, 2 second timeout)\n        cmd := exec.Command(\"ping\", \"-c\", \"1\", \"-W\", \"2\", meshIP)\n        err := cmd.Run()\n        \n        if err == nil {\n            m.checklist.UpdateItem(\"mesh_connectivity\", StatusComplete, \"\")\n            return\n        }\n    }\n    \n    m.checklist.UpdateItem(\"mesh_connectivity\", StatusInProgress, \"ping test failed\")\n}\n\n// checkRoutePropagination checks if advertised routes are present\nfunc (m *Monitor) checkRoutePropagation() {\n    cmd := exec.Command(\"ip\", \"route\", \"show\")\n    output, err := cmd.CombinedOutput()\n    \n    if err != nil {\n        m.checklist.UpdateItem(\"route_propagation\", StatusFailed, \"cannot query routes\")\n        return\n    }\n    \n    outputStr := string(output)\n    if strings.Contains(outputStr, \"dev wg0\") || strings.Contains(outputStr, \"dev utun\") {\n        m.checklist.UpdateItem(\"route_propagation\", StatusComplete, \"\")\n    } else {\n        // If no routes are advertised, this is normal\n        m.checklist.UpdateItem(\"route_propagation\", StatusSkipped, \"no advertised routes\")\n    }\n}\n\n// Close closes the RPC client connection\nfunc (m *Monitor) Close() {\n    if m.client != nil {\n        m.client.Close()\n    }\n}\n```\n\n### Task 2: Create Dashboard Renderer\n\n**dashboard.go** — Terminal UI using tcell library:\n\n```go\npackage onboard\n\nimport (\n    \"fmt\"\n\n    \"github.com/gdamore/tcell/v2\"\n    \"github.com/rivo/tview\"\n)\n\n// Dashboard renders the onboarding checklist in terminal UI\ntype Dashboard struct {\n    app       *tview.Application\n    checklist *Checklist\n    monitor   *Monitor\n    table     *tview.Table\n}\n\n// NewDashboard creates a new dashboard widget\nfunc NewDashboard(checklist *Checklist, monitor *Monitor) *Dashboard {\n    d := &Dashboard{\n        checklist: checklist,\n        monitor:   monitor,\n    }\n    \n    d.app = tview.NewApplication()\n    d.createUI()\n    \n    return d\n}\n\n// createUI builds the terminal UI\nfunc (d *Dashboard) createUI() {\n    // Create main table for checklist items\n    d.table = tview.NewTable().\n        SetBorders(true).\n        SetBorderColor(tcell.ColorWhite).\n        SetTitle(\" wgmesh Onboarding Checklist \").\n        SetTitleColor(tcell.ColorYellow).\n        SetBorderPadding(1, 1, 1, 1)\n    \n    // Add header row\n    headers := []string{\"Status\", \"Check\", \"Description\", \"Action\"}\n    for col, header := range headers {\n        cell := tview.NewTableCell(header).\n            SetTextColor(tcell.ColorYellow).\n            SetAttributes(tcell.AttrBold)\n        d.table.SetCell(0, col, cell)\n    }\n    \n    // Add initial rows for each checklist item\n    items := d.checklist.Items()\n    for row, item := range items {\n        d.updateRow(row+1, item) // +1 to skip header\n    }\n    \n    // Add flex layout with instructions\n    flex := tview.NewFlex().\n        SetDirection(tview.FlexRow).\n        AddItem(d.table, 0, 1).\n        AddItem(d.createInstructions(), 3, 0)\n    \n    d.app.SetRoot(flex, true)\n    \n    // Set up keyboard handler\n    d.app.SetInputCapture(d.handleInput)\n}\n\n// createInstructions creates the instruction footer\nfunc (d *Dashboard) createInstructions() *tview.TextView {\n    return tview.NewTextView().\n        SetText(`[yellow]Controls:[yellow]\n  [q] or [Esc] - Quit\n  [r] - Refresh now\n  [?] - Help\n\n[green]Tip: Keep this dashboard open while wgmesh discovers peers.[green]`)\n}\n\n// updateRow updates a single row in the table\nfunc (d *Dashboard) updateRow(row int, item *CheckItem) {\n    // Status column with icon\n    statusCell := tview.NewTableCell(d.statusIcon(item.Status)).\n        SetTextColor(d.statusColor(item.Status))\n    d.table.SetCell(row, 0, statusCell)\n    \n    // Name column\n    nameCell := tview.NewTableCell(item.Name).\n        SetTextColor(tcell.ColorWhite)\n    d.table.SetCell(row, 1, nameCell)\n    \n    // Description column\n    descCell := tview.NewTableCell(item.Description).\n        SetTextColor(tcell.ColorGray).\n        SetMaxWidth(50)\n    d.table.SetCell(row, 2, descCell)\n    \n    // Action hint column\n    actionCell := tview.NewTableCell(item.ActionHint).\n        SetTextColor(tcell.ColorCyan).\n        SetMaxWidth(30)\n    d.table.SetCell(row, 3, actionCell)\n}\n\n// statusIcon returns the icon for a status\nfunc (d *Dashboard) statusIcon(status ItemStatus) string {\n    switch status {\n    case StatusComplete:\n        return \"✓\"\n    case StatusInProgress:\n        return \"⟳\"\n    case StatusFailed:\n        return \"✗\"\n    case StatusSkipped:\n        return \"○\"\n    default:\n        return \"○\"\n    }\n}\n\n// statusColor returns the color for a status\nfunc (d *Dashboard) statusColor(status ItemStatus) tcell.Color {\n    switch status {\n    case StatusComplete:\n        return tcell.ColorGreen\n    case StatusInProgress:\n        return tcell.ColorYellow\n    case StatusFailed:\n        return tcell.ColorRed\n    case StatusSkipped:\n        return tcell.ColorGray\n    default:\n        return tcell.ColorGray\n    }\n}\n\n// handleInput processes keyboard input\nfunc (d *Dashboard) handleInput(event *tcell.EventKey) *tcell.EventKey {\n    switch event.Key() {\n    case tcell.KeyEsc, tcell.KeyCtrlC:\n        d.app.Stop()\n        return nil\n    case tcell.KeyRune:\n        switch event.Rune() {\n        case 'q', 'Q':\n            d.app.Stop()\n            return nil\n        case 'r', 'R':\n            // Force refresh\n            d.refresh()\n            return nil\n        case '?':\n            // Show help (could be a modal)\n            return nil\n        }\n    }\n    return event\n}\n\n// refresh updates the UI with current checklist state\nfunc (d *Dashboard) refresh() {\n    items := d.checklist.Items()\n    for row, item := range items {\n        d.updateRow(row+1, item)\n    }\n    d.app.Draw()\n}\n\n// Run starts the dashboard with auto-refresh\nfunc (d *Dashboard) Run() error {\n    // Start monitor in background\n    go d.monitor.Run(d.app.GetContext())\n    \n    // Set up periodic UI refresh\n    go func() {\n        ticker := time.NewTicker(2 * time.Second)\n        defer ticker.Stop()\n        \n        for {\n            select {\n            case <-d.app.GetContext().Done():\n                return\n            case <-ticker.C:\n                d.app.QueueUpdateDraw(func() {\n                    d.refresh()\n                })\n            }\n        }\n    }()\n    \n    return d.app.Run()\n}\n\n// Close closes the dashboard and monitor\nfunc (d *Dashboard) Close() {\nA tool call could not be parsed — the response may have been truncated. Try breaking the task into smaller steps or resending your message.
