package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// boundRow is one line of the bound table: the map field it covers and how
// that map is kept from growing without limit (ADR-0007 Decision 1).
type boundRow struct {
	field string // "Struct.field"
	file  string // the path in boundFiles
	kind  string // one of boundKinds
	how   string // one line naming the function that releases entries
	lock  string // the lock that guards every access, or why none is needed
}

// boundKinds are the rule kinds of ADR-0007 Decision 1. hard-cap is the one
// added by T4: a map bounded only by a size limit (node.DefaultMaxPeers).
var boundKinds = map[string]bool{
	"replaced-per-tick":   true,
	"swept-vs-active-set": true,
	"age-sweep":           true,
	"age-sweep-cap":       true,
	"store-membership":    true,
	"deadline-sweep":      true,
	"paired-set-clear":    true,
	"session-local":       true,
	"hard-cap":            true,
}

// boundFiles are the long-lived structs on the peer path, relative to pkg/daemon.
var boundFiles = []string{
	"daemon.go",
	"../discovery/dht.go",
	"../discovery/exchange.go",
	"../discovery/gossip.go",
	"../discovery/lan.go",
	"../node/store.go",
	"../discovery/replay_cache.go",
}

// boundTable has one row per map-typed struct field of boundFiles. The lock
// column was read from every non-test access of the field (ADR-0007, M's rule
// of 2026-11-16: a field touched by more than one goroutine is under a lock).
var boundTable = []boundRow{
	{"Daemon.lastAppliedPeerConfigs", "daemon.go", "swept-vs-active-set", "applyDesiredPeerConfigs deletes the WireGuard peers that are not desired; evictPeerFromPool and attemptPeerReconnect delete one key", "d.appliedMu"},
	{"Daemon.relayRoutes", "daemon.go", "replaced-per-tick", "reconcile assigns a fresh map every tick; evictPeerFromPool deletes one key", "d.relayMu"},
	{"Daemon.directStableCycles", "daemon.go", "replaced-per-tick", "reconcile assigns a fresh map every tick; evictPeerFromPool deletes one key", "d.relayMu"},
	{"Daemon.peerHealthFailures", "daemon.go", "swept-vs-active-set", "checkPeerHealth deletes keys outside the active set at the end of every pass", "d.healthMu"},
	{"Daemon.lastPeerTransferTotal", "daemon.go", "swept-vs-active-set", "checkPeerHealth deletes keys outside the active set at the end of every pass", "d.healthMu"},
	{"Daemon.probeSessions", "daemon.go", "swept-vs-active-set", "cleanupProbeSessions (probePeersOverMesh, every second) closes and deletes sessions outside the active set", "d.probeMu"},
	{"Daemon.probeFailures", "daemon.go", "swept-vs-active-set", "cleanupProbeSessions deletes keys outside the active set", "d.probeMu"},
	{"Daemon.temporaryOffline", "daemon.go", "deadline-sweep", "sweepTemporaryOffline, called by staleCleanupTick every minute, deletes entries past their deadline", "d.offlineMu"},
	{"desiredPeerConfig.allowed", "daemon.go", "session-local", "built and read inside one reconcile call, then dropped with the desired map", "none: never shared between goroutines"},
	{"DHTDiscovery.contactedPeers", "../discovery/dht.go", "age-sweep-cap", "markContacted sweeps entries older than ContactDedupWindow above contactSweepThreshold and refuses new addresses at MaxContactedPeers", "d.mu"},
	{"DHTDiscovery.controlPeers", "../discovery/dht.go", "store-membership", "sweepPeerKeyed (first step of checkStaleHandshakes) deletes keys the PeerStore no longer holds", "d.mu"},
	{"DHTDiscovery.rendezvousBackoff", "../discovery/dht.go", "store-membership", "sweepPeerKeyed deletes keys the PeerStore no longer holds; recordRendezvousAttempt deletes on success", "d.mu"},
	{"rendezvousState.offers", "../discovery/exchange.go", "session-local", "the session is deleted when the pair starts or after RendezvousSessionTTL (handleRendezvousOffer)", "pe.rendezvousMu"},
	{"rendezvousState.endpoints", "../discovery/exchange.go", "session-local", "the session is deleted when the pair starts or after RendezvousSessionTTL (handleRendezvousOffer)", "pe.rendezvousMu"},
	{"PeerExchange.pendingReplies", "../discovery/exchange.go", "paired-set-clear", "setPendingReplyChannel is paired with clearPendingReplyChannel by the caller", "pe.pendingMu"},
	{"PeerExchange.rendezvousSessions", "../discovery/exchange.go", "age-sweep", "handleRendezvousOffer deletes sessions older than RendezvousSessionTTL on every offer", "pe.rendezvousMu"},
	{"PeerExchange.activePunches", "../discovery/exchange.go", "age-sweep", "handleRendezvousOffer deletes entries older than twice RendezvousPunchCooldown on every offer", "pe.rendezvousMu"},
	{"PeerExchange.rendezvousStarts", "../discovery/exchange.go", "age-sweep", "handleRendezvousOffer deletes entries older than twice RendezvousStartCooldown on every offer", "pe.rendezvousMu"},
	{"PeerExchange.lastPacketLog", "../discovery/exchange.go", "age-sweep-cap", "allowLog sweeps stale entries once per cooldown and stops remembering at maxLogThrottleEntries", "pe.logMu"},
	{"PeerStore.peers", "../node/store.go", "hard-cap", "Decide refuses a new key at DefaultMaxPeers (ReasonPeerCap); CleanupStale, Remove and Withdraw delete", "ps.mu"},
	{"PeerStore.tombstones", "../node/store.go", "age-sweep-cap", "tombstone drops expired entries, then the oldest, at DefaultMaxPeers; isTombstoned drops an expired one", "ps.mu"},
	{"PeerStore.ipHeldLogged", "../node/store.go", "hard-cap", "Offer replaces the map with an empty one when it reaches DefaultMaxPeers", "ps.mu"},
	{"nonceCache.seen", "../discovery/replay_cache.go", "age-sweep-cap", "Seen drops entries older than ttl, then the oldest at capacity (drop)", "c.mu"},
}

// checkBoundTable lists every map-typed struct field of the files in src (path
// to source text) and compares it with table. It is a heuristic over field
// declarations: a map inside a nested type, a slice of maps, a sync.Map or a
// map declared in a file that is not in src is outside it. A finding starts
// with "unlisted", "stale" or "invalid".
func checkBoundTable(src map[string]string, table []boundRow) []string {
	var out []string
	found := map[string]bool{}
	paths := make([]string, 0, len(src))
	for p := range src {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := parser.ParseFile(token.NewFileSet(), path, src[path], 0)
		if err != nil {
			out = append(out, "invalid: cannot parse "+path+": "+err.Error())
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, fld := range st.Fields.List {
				if _, isMap := fld.Type.(*ast.MapType); !isMap {
					continue
				}
				for _, name := range fld.Names {
					found[path+"|"+ts.Name.Name+"."+name.Name] = true
				}
			}
			return true
		})
	}
	listed := map[string]bool{}
	for _, r := range table {
		key := r.file + "|" + r.field
		listed[key] = true
		if !boundKinds[r.kind] || r.how == "" || r.lock == "" {
			out = append(out, "invalid row "+r.field+" in "+r.file+": needs a known rule kind, a how and a lock")
		}
		if _, read := src[r.file]; read && !found[key] {
			out = append(out, "stale row "+r.field+" in "+r.file+": the field no longer exists; delete the row")
		}
	}
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !listed[k] {
			file, field, _ := strings.Cut(k, "|")
			out = append(out, "unlisted map field "+field+" in "+file+": add a row to boundTable naming its bound (rule kinds: ADR-0007 Decision 1)")
		}
	}
	return out
}

func readBoundFiles(t *testing.T) map[string]string {
	t.Helper()
	src := map[string]string{}
	for _, p := range boundFiles {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		src[p] = string(b)
	}
	return src
}

func findingsWithPrefix(findings []string, prefix string) []string {
	var out []string
	for _, f := range findings {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// TestEveryMapFieldHasABound fails on a map-typed struct field of the peer
// path without a row in boundTable, or with an invalid one.
func TestEveryMapFieldHasABound(t *testing.T) {
	findings := checkBoundTable(readBoundFiles(t), boundTable)
	bad := append(findingsWithPrefix(findings, "unlisted"), findingsWithPrefix(findings, "invalid")...)
	if len(bad) > 0 {
		t.Fatalf("%d finding(s):\n%s", len(bad), strings.Join(bad, "\n"))
	}
}

// TestBoundTableHasNoStaleRows fails on a row whose field no longer exists.
func TestBoundTableHasNoStaleRows(t *testing.T) {
	stale := findingsWithPrefix(checkBoundTable(readBoundFiles(t), boundTable), "stale")
	if len(stale) > 0 {
		t.Fatalf("%d finding(s):\n%s", len(stale), strings.Join(stale, "\n"))
	}
}

// TestGuardFailsOnUnlistedMapField proves the checker on a synthetic file: one
// map field without a row is one finding naming it, and a row for a field that
// is gone is one stale finding.
func TestGuardFailsOnUnlistedMapField(t *testing.T) {
	const file = "synthetic.go"
	src := map[string]string{file: `package x

import "time"

type Holder struct {
	known    map[string]int
	seenAt   map[string]time.Time
	notAMap  []string
}
`}
	table := []boundRow{{field: "Holder.known", file: file, kind: "session-local", how: "dies with the holder", lock: "none"}}
	got := checkBoundTable(src, table)
	if len(got) != 1 || !strings.Contains(got[0], "Holder.seenAt") {
		t.Fatalf("want one finding naming Holder.seenAt, got %q", got)
	}

	table = append(table, boundRow{field: "Holder.gone", file: file, kind: "session-local", how: "x", lock: "none"})
	got = checkBoundTable(src, table)
	if len(got) != 2 || !strings.Contains(strings.Join(got, "\n"), "stale row Holder.gone") {
		t.Fatalf("want a stale finding for Holder.gone and the unlisted one, got %q", got)
	}
}
