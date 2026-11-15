package node

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// guardMethods and guardReceivers name what TestNoDirectPeerStoreMutation
// flags: a call of one of the methods on a receiver of one of the names.
var (
	guardMethods   = map[string]bool{"Update": true, "SetPeerDirectly": true}
	guardReceivers = map[string]bool{"peerStore": true, "PeerStore": true, "ps": true, "store": true}
)

// TestNoDirectPeerStoreMutation fails when a non-test file outside pkg/node
// calls Update or SetPeerDirectly on a receiver whose last identifier is
// peerStore, PeerStore, ps or store: production claims go through Offer.
// pkg/node itself is not scanned.
//
// Limit: the match is by receiver name. A store held in a variable of another
// name is not seen, nor is a call through a function value or an interface.
func TestNoDirectPeerStoreMutation(t *testing.T) {
	root := filepath.Join("..", "..")
	found, err := directMutations(root, filepath.Join(root, "pkg", "node"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Errorf("direct peer store mutation, use Offer: %s", f)
	}
}

// directMutations returns "position method" for every guarded call in the
// non-test Go files under root, leaving out skipDir, .git and vendor.
func directMutations(root, skipDir string) ([]string, error) {
	var found []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if path == skipDir || e.Name() == ".git" || e.Name() == "vendor" {
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
			if !ok || !guardMethods[sel.Sel.Name] {
				return true
			}
			if guardReceivers[lastIdent(sel.X)] {
				found = append(found, fset.Position(call.Pos()).String()+" "+sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	return found, err
}

// TestGuardFlagsEveryMutatingMethod proves the guard on bait files: each
// guarded method on each guarded receiver name is reported, a call it must
// not flag is not, and a file under the skipped directory is not scanned.
func TestGuardFlagsEveryMutatingMethod(t *testing.T) {
	cases := []struct {
		name string
		call string
		want string // method reported; empty means not flagged
	}{
		{"update_field", `peerStore.Update(x, "m")`, "Update"},
		{"set_field", `d.peerStore.SetPeerDirectly(k, p)`, "SetPeerDirectly"},
		{"update_store", `store.Update(x, "m")`, "Update"},
		{"set_ps", `ps.SetPeerDirectly(k, p)`, "SetPeerDirectly"},
		{"update_upper", `d.PeerStore.Update(x, "m")`, "Update"},
		{"other_receiver", `other.Update(x, "m")`, ""},
		{"offer", `peerStore.Offer(c)`, ""},
	}
	root := t.TempDir()
	skip := filepath.Join(root, "skipped")
	if err := os.Mkdir(skip, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		src := "package bait\n\nfunc f() {\n\t" + c.call + "\n}\n"
		if err := os.WriteFile(filepath.Join(root, c.name+".go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skip, "bait.go"), []byte("package bait\n\nfunc f() {\n\tpeerStore.Update(x, \"m\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := directMutations(root, skip)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		var got []string
		for _, f := range found {
			if strings.Contains(f, c.name+".go:") {
				got = append(got, f)
			}
		}
		switch {
		case c.want == "" && len(got) != 0:
			t.Errorf("%s: %s was flagged: %v", c.name, c.call, got)
		case c.want != "" && (len(got) != 1 || !strings.HasSuffix(got[0], " "+c.want)):
			t.Errorf("%s: %s: got %v, want one %s", c.name, c.call, got, c.want)
		}
	}
	for _, f := range found {
		if strings.Contains(f, "skipped") {
			t.Errorf("file under the skipped dir was scanned: %s", f)
		}
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
