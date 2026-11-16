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
// the file. The target of an assignment is looked through parentheses and
// index expressions (localNode.RoutableNetworks[0] = x writes the field's
// backing array); a store through a pointer to the node (*d.localNode = ...)
// replaces every field at once and is flagged as "*".
func localNodeWrites(fset *token.FileSet, f *ast.File, rel string) []string {
	var found []string
	flag := func(lhs ast.Expr, name, fn string) {
		found = append(found, rel+":"+strconv.Itoa(fset.Position(lhs.Pos()).Line)+" "+name+" in "+fn)
	}
	check := func(lhs ast.Expr, fn string) {
		target := lhs
		for {
			switch x := target.(type) {
			case *ast.ParenExpr:
				target = x.X
				continue
			case *ast.IndexExpr:
				target = x.X
				continue
			}
			break
		}
		if star, ok := target.(*ast.StarExpr); ok {
			if isLocalNodeExpr(star.X) {
				flag(lhs, "*", fn)
			}
			return
		}
		sel, ok := target.(*ast.SelectorExpr)
		if !ok || !isLocalNodeExpr(sel.X) {
			return
		}
		name := sel.Sel.Name
		if localNodeStateFields[name] || (localNodeWriteOnceFields[name] && !localNodeWriteOnceWriters[fn]) {
			flag(lhs, name, fn)
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

// TestLocalNodeWriteScannerFlagsViolations runs the scanner over inline
// source: a guard that flags nothing would pass the walk below for the wrong
// reason.
func TestLocalNodeWriteScannerFlagsViolations(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // "field in func", empty = not flagged
	}{
		{"state field assigned", "func f() { localNode.MeshIP = x }", []string{"MeshIP in f"}},
		{"counter incremented", "func f() { d.localNode.NATType++ }", []string{"NATType in f"}},
		{"write-once field outside its writers", "func f() { d.localNode.Hostname = x }", []string{"Hostname in f"}},
		{"element of the routes", "func f() { localNode.RoutableNetworks[0] = x }", []string{"RoutableNetworks in f"}},
		{"whole node through a pointer", "func f() { *d.localNode = LocalNode{} }", []string{"* in f"}},
		{"a peer is not the local node", "func f() { peer.MeshIP = x }", nil},
		{"write-once field in initLocalNode", "func initLocalNode() { d.localNode.Hostname = x }", nil},
		{"write-once field in loadLocalNode", "func loadLocalNode() { localNode.WGPubKey = x }", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "x.go", "package p\n"+tc.src, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := localNodeWrites(fset, f, "x.go")
			if len(got) != len(tc.want) {
				t.Fatalf("flagged %q, want %d finding(s) %q", got, len(tc.want), tc.want)
			}
			for i, w := range tc.want {
				if !strings.HasSuffix(got[i], " "+w) {
					t.Errorf("flagged %q, want it to end with %q", got[i], w)
				}
			}
		})
	}
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
