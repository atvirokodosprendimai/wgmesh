package daemon

import (
	"fmt"
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

// commandAllowed says whether production file `file` may run `cmd arg ...`
// (a kernel command): converge.go writes, observe.go only reads, with
// `iptables -C`. routes.go is a known exception: its blind sysctl and
// ensureWGForwardingRule calls stay until ADR-0003 T5 removes them, and T5
// removes this case.
func commandAllowed(file, cmd, arg string) bool {
	switch file {
	case "converge.go", "routes.go":
		return true
	case "observe.go":
		return cmd == "iptables" && arg == "-C"
	}
	return false
}

// stringLiterals maps every identifier of the package that is a const, a var
// or a := target initialised from a string literal to those literals, so a
// command name held in a variable is still read. It goes by name, not by
// scope: a shadowing name can only make the guard stricter.
func stringLiterals(files map[string]*ast.File) map[string][]string {
	out := make(map[string][]string)
	add := func(id ast.Expr, val ast.Expr) {
		name, ok := id.(*ast.Ident)
		lit, isLit := val.(*ast.BasicLit)
		if !ok || !isLit || lit.Kind != token.STRING {
			return
		}
		if s, err := strconv.Unquote(lit.Value); err == nil {
			out[name.Name] = append(out[name.Name], s)
		}
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				for i, id := range n.Names {
					if i < len(n.Values) {
						add(id, n.Values[i])
					}
				}
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i := range n.Lhs {
						add(n.Lhs[i], n.Rhs[i])
					}
				}
			}
			return true
		})
	}
	return out
}

// literalValues is the string a Command argument is, or the strings it may be
// when it is an identifier of stringLiterals; ok is false for anything else.
// The limit of the guard: an argument computed at run time (a parameter, a
// slice element such as args[0], a call, a concatenation) is not seen, so a
// kernel command must be named by a literal or by a constant or variable
// initialised from one.
func literalValues(e ast.Expr, lits map[string][]string) (vals []string, ok bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			if s, err := strconv.Unquote(e.Value); err == nil {
				return []string{s}, true
			}
		}
	case *ast.Ident:
		if v := lits[e.Name]; len(v) > 0 {
			return v, true
		}
	}
	return nil, false
}

// firstElement unwraps append([]string{"-C"}, rest...) to its first element,
// the way the rule helpers build an iptables argument list; any other
// expression is returned as it is.
func firstElement(e ast.Expr) ast.Expr {
	if c, ok := e.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "append" && len(c.Args) > 0 {
			if cl, ok := c.Args[0].(*ast.CompositeLit); ok && len(cl.Elts) > 0 {
				return cl.Elts[0]
			}
		}
	}
	return e
}

// commandViolations is one text per Command call in file `name` that runs a
// kernel command the file may not.
func commandViolations(name string, f *ast.File, lits map[string][]string) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Command" {
			return true
		}
		cmds, ok := literalValues(call.Args[0], lits)
		if !ok {
			return true
		}
		args := []string{""}
		if len(call.Args) > 1 {
			if v, ok := literalValues(firstElement(call.Args[1]), lits); ok {
				args = v
			}
		}
		for _, cmd := range cmds {
			if !kernelCommands[cmd] {
				continue
			}
			for _, arg := range args {
				if !commandAllowed(name, cmd, arg) {
					out = append(out, fmt.Sprintf("%s runs %s %s: forwarding writes belong in converge.go, observe.go may only run iptables -C", name, cmd, arg))
				}
			}
		}
		return true
	})
	return out
}

// TestKernelWritesOnlyInConverge: no production file of pkg/daemon outside
// converge.go calls SetPeer or RemovePeer of pkg/wireguard (under any import
// alias), or the Daemon seams that stand for them, and none runs sysctl or
// iptables through any Command call (whatever the receiver or the os/exec
// import alias) except converge.go, routes.go (the exception above) and
// observe.go for `iptables -C`. Later tasks extend the pattern list.
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
	lits := stringLiterals(files)
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
		for _, v := range commandViolations(name, f, lits) {
			t.Error(v)
		}
	}
}

// The guard can fail: each case is a source file the guard must flag or pass.
func TestCommandGuardForms(t *testing.T) {
	cases := []struct {
		name string
		file string
		body string
		bad  bool
	}{
		{"observe may check", "observe.go", `cmdExecutor.Command("iptables", "-C", "FORWARD")`, false},
		{"observe may not append", "observe.go", `cmdExecutor.Command("iptables", "-A", "FORWARD")`, true},
		{"observe may not sysctl", "observe.go", `cmdExecutor.Command("sysctl", "-w", "x=1")`, true},
		{"observe may not iptables without arg", "observe.go", `cmdExecutor.Command("iptables")`, true},
		{"converge may write", "converge.go", `cmdExecutor.Command("iptables", "-A", "FORWARD")`, false},
		{"other file literal", "other.go", `cmdExecutor.Command("sysctl", "-w", "x=1")`, true},
		{"other file const", "other.go", "const c = \"sysctl\"\n\tcmdExecutor.Command(c, \"-w\")", true},
		{"other file short var", "other.go", "c := \"iptables\"\n\tcmdExecutor.Command(c, \"-A\")", true},
		{"observe const append", "observe.go", "const c = \"iptables\"\n\tconst a = \"-A\"\n\tcmdExecutor.Command(c, a)", true},
		{"observe const check", "observe.go", "const c = \"iptables\"\n\tconst a = \"-C\"\n\tcmdExecutor.Command(c, a)", false},
		{"other command", "other.go", `cmdExecutor.Command("wg", "show")`, false},
		{"observe check built with append", "observe.go", `cmdExecutor.Command("iptables", append([]string{"-C"}, args...)...)`, false},
		{"observe append built with -A", "observe.go", `cmdExecutor.Command("iptables", append([]string{"-A"}, args...)...)`, true},
		{"observe computed second argument", "observe.go", `cmdExecutor.Command("iptables", args[0])`, true},
		// The documented limit: a computed first argument is not seen.
		{"computed argument is not seen", "other.go", `cmdExecutor.Command(args[0], args[1:]...)`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "package daemon\n\nfunc f(args []string) {\n\t" + c.body + "\n}\n"
			f, err := parser.ParseFile(token.NewFileSet(), c.file, src, 0)
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]*ast.File{c.file: f}
			got := commandViolations(c.file, f, stringLiterals(files))
			if (len(got) > 0) != c.bad {
				t.Errorf("violations = %q, want violation %v", got, c.bad)
			}
		})
	}
}
