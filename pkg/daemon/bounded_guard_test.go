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

var boundTable = []boundRow{}

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
