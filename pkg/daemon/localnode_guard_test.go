package daemon

// Words: the local node's mutable fields are written through its setters and
// by nothing else. This test parses every production .go file of the module
// outside pkg/node and fails on an assignment (=, op=, ++, --) to a selector
// named MeshIP, MeshIPv6, NATType, Introducer or RoutableNetworks whose
// receiver is a local-node expression (an identifier or a selector ending in
// localNode, or a GetLocalNode() call), and on an assignment to WGPubKey,
// WGPrivateKey or Hostname of such a receiver in any function except
// initLocalNode and loadLocalNode (write-once before any goroutine starts).
//
// Why: a field with later writes is shared by reconcile, STUN, SIGHUP and the
// announcement goroutines; a direct write bypasses stateMu (ADR-0005 T8, M
// 2026-11-16). The unexported fields make the compiler the real guard; this
// test catches an exported field returning.
//
// Limits, stated so nobody trusts the guard more than it is:
//   - the receiver is matched by name (localNode, GetLocalNode()), so a write
//     through another variable that holds the node is not seen;
//   - composite literals (&LocalNode{MeshIP: x}) and PeerInfo fields are not
//     flagged;
//   - an alias import or a function value hides a selector from it;
//   - the module root comes from `go list -m -f {{.Dir}}`, which prints one
//     line per module under a go.work file: the guard assumes no go.work.
//
// Invariant: the module root comes from `go list -m -f {{.Dir}}`, never from a
// typed path.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var localNodeStateFields = map[string]bool{
	"MeshIP": true, "MeshIPv6": true, "NATType": true, "Introducer": true, "RoutableNetworks": true,
}

var localNodeWriteOnceFields = map[string]bool{
	"WGPubKey": true, "WGPrivateKey": true, "Hostname": true,
}

// localNodeWriteOnceWriters may assign the write-once fields.
var localNodeWriteOnceWriters = map[string]bool{"initLocalNode": true, "loadLocalNode": true}

// isLocalNodeExpr reports whether e names the local node by the conventions
// above.
func isLocalNodeExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name == "localNode"
	case *ast.SelectorExpr:
		return x.Sel.Name == "localNode"
	case *ast.CallExpr:
		sel, ok := x.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "GetLocalNode"
	}
	return false
}

// localNodeWrites returns "file:line field" for every flagged assignment in
// the file.
func localNodeWrites(fset *token.FileSet, f *ast.File, rel string) []string {
	var found []string
	check := func(lhs ast.Expr, fn string) {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok || !isLocalNodeExpr(sel.X) {
			return
		}
		name := sel.Sel.Name
		bad := localNodeStateFields[name] || (localNodeWriteOnceFields[name] && !localNodeWriteOnceWriters[fn])
		if bad {
			found = append(found, rel+":"+strconv.Itoa(fset.Position(lhs.Pos()).Line)+" "+name+" in "+fn)
		}
	}
	walk := func(root ast.Node, fn string) {
		ast.Inspect(root, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				for _, l := range s.Lhs {
					check(l, fn)
				}
			case *ast.IncDecStmt:
				check(s.X, fn)
			}
			return true
		})
	}
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			walk(fd, fd.Name.Name)
		} else {
			walk(decl, "")
		}
	}
	return found
}

func TestLocalNodeFieldsNotWrittenDirectly(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("go list -m -f {{.Dir}}: %v", err)
	}
	root := strings.TrimSpace(string(out))
	fset := token.NewFileSet()
	var bad []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata":
				return filepath.SkipDir
			}
			if path != root {
				if strings.HasPrefix(d.Name(), ".") || rel == "pkg/node" {
					return filepath.SkipDir
				}
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		bad = append(bad, localNodeWrites(fset, f, rel)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bad {
		t.Errorf("direct write to a local node field (use its setter): %s", b)
	}
}
