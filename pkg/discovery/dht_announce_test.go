package discovery

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// serverCalls returns every call in dht.go whose selector is name, keyed by
// the name of the enclosing function.
func serverCalls(t *testing.T, name string) map[string][]*ast.CallExpr {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "dht.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string][]*ast.CallExpr{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
					calls[fn.Name.Name] = append(calls[fn.Name.Name], c)
				}
			}
			return true
		})
	}
	return calls
}

func TestAnnounceCallsUseTraversal(t *testing.T) {
	if old := serverCalls(t, "Announce"); len(old) != 0 {
		t.Errorf("deprecated Announce still called in %d function(s): %v", len(old), old)
	}
	total := 0
	for _, cs := range serverCalls(t, "AnnounceTraversal") {
		total += len(cs)
	}
	if total != 3 {
		t.Errorf("AnnounceTraversal calls in dht.go = %d, want 3", total)
	}
	// The announce site passes announceOpts(port)... and nothing else.
	cs := serverCalls(t, "AnnounceTraversal")["announceToInfohash"]
	if len(cs) != 1 || len(cs[0].Args) != 2 || !cs[0].Ellipsis.IsValid() {
		t.Fatalf("announceToInfohash must call AnnounceTraversal(infohash, announceOpts(port)...) once")
	}
	opts, ok := cs[0].Args[1].(*ast.CallExpr)
	if !ok {
		t.Fatalf("announceToInfohash second argument is not a call")
	}
	if id, ok := opts.Fun.(*ast.Ident); !ok || id.Name != "announceOpts" {
		t.Errorf("announceToInfohash second argument is not announceOpts(...)")
	}
}

func TestAnnounceOptsCarryPortOnlyWhenNonZero(t *testing.T) {
	if got := announceOpts(0); len(got) != 0 {
		t.Errorf("announceOpts(0) has %d options, want 0", len(got))
	}
	if got := announceOpts(51820); len(got) != 1 {
		t.Errorf("announceOpts(51820) has %d options, want 1", len(got))
	}
}

func TestQueryAndBootstrapSitesPassNoAnnounceOption(t *testing.T) {
	calls := serverCalls(t, "AnnounceTraversal")
	for _, fn := range []string{"queryInfohash", "attemptBootstrapLookup"} {
		cs := calls[fn]
		if len(cs) != 1 {
			t.Errorf("%s: %d AnnounceTraversal calls, want 1", fn, len(cs))
			continue
		}
		if len(cs[0].Args) != 1 || cs[0].Ellipsis.IsValid() {
			t.Errorf("%s: AnnounceTraversal must take the infohash only, got %d arguments", fn, len(cs[0].Args))
		}
	}
}
