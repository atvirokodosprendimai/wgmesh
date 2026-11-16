package daemon

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
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

// wireguardImportPath is the package whose SetPeer and RemovePeer write peers.
const wireguardImportPath = "github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"

// importNames is every local name file f gives to the package at path, found
// by the import path, so an alias does not hide a call.
func importNames(f *ast.File, path string) map[string]bool {
	names := make(map[string]bool)
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil || p != path {
			continue
		}
		name := filepath.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = true
	}
	return names
}

// kernelCommands are the commands that write forwarding state of the kernel.
var kernelCommands = map[string]bool{"sysctl": true, "iptables": true}

// commandReaders may run them: converge.go writes, observe.go reads (iptables -C).
// routes.go is a known exception: its blind sysctl and ensureWGForwardingRule
// calls stay until ADR-0003 T5 removes them, and T5 removes this line.
var commandReaders = map[string]bool{"converge.go": true, "observe.go": true, "routes.go": true}

// TestKernelWritesOnlyInConverge: no production file of pkg/daemon outside
// converge.go calls SetPeer or RemovePeer of pkg/wireguard (under any import
// alias), or the Daemon seams that stand for them, and none outside
// converge.go, observe.go (and the routes.go exception above) runs sysctl or
// iptables through any Command call, whatever the receiver or the os/exec
// import alias. Later tasks extend the pattern list.
func TestKernelWritesOnlyInConverge(t *testing.T) {
	banned := []struct {
		sel     string
		inWgPkg bool // the selector must be on the wireguard package, else any receiver
	}{
		{"SetPeer", true},
		{"RemovePeer", true},
		{"setPeerFn", false},
		{"removePeerFn", false},
		{"setSeam", false},
		{"removeSeam", false},
	}
	files := parseProduction(t)
	if _, ok := files["converge.go"]; !ok {
		t.Fatal("converge.go does not exist")
	}
	for name, f := range files {
		if name != "converge.go" {
			wg := importNames(f, wireguardImportPath)
			selectors(f, func(x, sel string) {
				for _, b := range banned {
					if sel == b.sel && (!b.inWgPkg || wg[x]) {
						t.Errorf("%s calls %s.%s: kernel peer writes belong in converge.go", name, x, sel)
					}
				}
			})
		}
		if commandReaders[name] {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Command" {
				return true
			}
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if cmd, err := strconv.Unquote(lit.Value); err == nil && kernelCommands[cmd] {
					t.Errorf("%s runs %s: forwarding commands belong in converge.go and observe.go", name, cmd)
				}
			}
			return true
		})
	}
}
