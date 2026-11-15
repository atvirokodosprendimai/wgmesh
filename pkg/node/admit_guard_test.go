package node

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectPeerStoreMutation fails when a non-test file outside pkg/node
// calls Update or SetPeerDirectly on a receiver whose last identifier is
// peerStore, PeerStore, ps or store: production claims go through Offer.
//
// Limit: the match is by receiver name. A store held in a variable of another
// name is not seen, nor is a call through a function value or an interface.
func TestNoDirectPeerStoreMutation(t *testing.T) {
	root := filepath.Join("..", "..")
	self := filepath.Join(root, "pkg", "node")
	methods := map[string]bool{"Update": true, "SetPeerDirectly": true}
	receivers := map[string]bool{"peerStore": true, "PeerStore": true, "ps": true, "store": true}

	var found []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if path == self || e.Name() == ".git" || e.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !methods[sel.Sel.Name] {
				return true
			}
			if receivers[lastIdent(sel.X)] {
				found = append(found, fset.Position(call.Pos()).String()+" "+sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("direct peer store mutation, use Offer: %s", f)
	}
}

// lastIdent returns the last identifier of x or a.b.c: c.
func lastIdent(x ast.Expr) string {
	switch v := x.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}
