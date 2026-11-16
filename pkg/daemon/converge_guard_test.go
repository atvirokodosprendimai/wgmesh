package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// parseProduction parses the non-test .go files of this package, by file name.
func parseProduction(t *testing.T) map[string]*ast.File {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	out := make(map[string]*ast.File)
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		out[name] = f
	}
	if len(out) == 0 {
		t.Fatal("no production files parsed")
	}
	return out
}

// selectors calls fn for every `x.Sel` in node.
func selectors(node ast.Node, fn func(x, sel string)) {
	ast.Inspect(node, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok {
				fn(id.Name, s.Sel.Name)
			} else {
				fn("", s.Sel.Name)
			}
		}
		return true
	})
}

// TestKernelWritesOnlyInConverge: no production file of pkg/daemon outside
// converge.go calls wireguard.SetPeer or wireguard.RemovePeer, or the Daemon
// seams that stand for them. Later tasks extend the pattern list.
func TestKernelWritesOnlyInConverge(t *testing.T) {
	banned := []struct{ x, sel string }{
		{"wireguard", "SetPeer"},
		{"wireguard", "RemovePeer"},
		{"", "setPeerFn"},
		{"", "removePeerFn"},
	}
	files := parseProduction(t)
	if _, ok := files["converge.go"]; !ok {
		t.Fatal("converge.go does not exist")
	}
	for name, f := range files {
		if name == "converge.go" {
			continue
		}
		selectors(f, func(x, sel string) {
			for _, b := range banned {
				if sel == b.sel && (b.x == "" || x == b.x) {
					t.Errorf("%s calls %s.%s: kernel peer writes belong in converge.go", name, x, sel)
				}
			}
		})
	}
}
