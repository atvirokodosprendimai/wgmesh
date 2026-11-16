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
// `iptables -C`.
func commandAllowed(file, cmd, arg string) bool {
	switch file {
	case "converge.go":
		return true
	case "observe.go":
		return cmd == "iptables" && arg == "-C"
	}
	return false
}

// ipRouteViolation is the text of a violation by an `ip` Command call that
// names `route` among its literal arguments: a route write (del, replace,
// add) belongs in converge.go, a route read in observe.go (or converge.go).
// restoreConnectedRoute builds its command at run time, which the guard does
// not see: it is held by its callers (callViolations).
func ipRouteViolation(name string, call *ast.CallExpr, lits map[string][]string) string {
	words := callWords(call, lits)
	if !words["route"] {
		return ""
	}
	if words["del"] || words["replace"] || words["add"] {
		if name != "converge.go" {
			return fmt.Sprintf("%s runs an ip route write: route writes belong in converge.go", name)
		}
		return ""
	}
	if name != "observe.go" && name != "converge.go" {
		return fmt.Sprintf("%s runs an ip route read: route reads belong in observe.go", name)
	}
	return ""
}

// callWords are the literal words among the arguments of an `ip` Command call.
func callWords(call *ast.CallExpr, lits map[string][]string) map[string]bool {
	words := make(map[string]bool)
	for _, a := range call.Args[1:] {
		if vals, ok := literalValues(firstElement(a), lits); ok {
			for _, v := range vals {
				words[v] = true
			}
		}
	}
	return words
}

// funcAt is the name of the top-level function of f that holds pos, "" outside one.
func funcAt(f *ast.File, pos token.Pos) string {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Pos() <= pos && pos <= fd.End() {
			return fd.Name.Name
		}
	}
	return ""
}

// ipAddrViolation is the text of a violation by an `ip` Command call that
// names `addr` (or `address`) and `add`, `del` or `flush` among its literal
// arguments: an address write belongs in converge.go. The named exceptions are
// the first-setup helper setInterfaceAddress and, for a flush only, the reset
// helper resetInterface of helpers.go (they flush before the interface is up
// or when it is reset; collision.go reaches the first through the variable
// applyInterfaceAddress, so no `ip` call of collision.go is excepted).
func ipAddrViolation(name, fn string, call *ast.CallExpr, lits map[string][]string) string {
	words := callWords(call, lits)
	if !(words["addr"] || words["address"]) || !(words["add"] || words["del"] || words["flush"]) {
		return ""
	}
	if name == "converge.go" || (name == "helpers.go" && (fn == "setInterfaceAddress" || (fn == "resetInterface" && !words["add"] && !words["del"]))) {
		return ""
	}
	return fmt.Sprintf("%s runs an ip addr write in %q: address writes belong in converge.go (first setup: setInterfaceAddress in helpers.go)", name, fn)
}

// routeWriters are the functions that write routes; only converge.go calls them.
var routeWriters = map[string]bool{
	"applyRouteDiff": true, "restoreConnectedRoute": true,
	"applyRouteDiffDarwin": true, "darwinRouteCommand": true, "restoreHostDefault": true,
}

// routeWriteViolation is the text of a violation by a `route` Command call
// (macOS) that names `add`, `delete` or `change` among its literal arguments:
// a route write belongs in converge.go. The named exception is the first-setup
// helper setInterfaceAddress of helpers.go, which installs the mesh route.
// The darwin adapter builds its arguments at run time, which the guard does
// not see: it is held by its callers (callViolations, routeWriters).
func routeWriteViolation(name, fn string, call *ast.CallExpr, lits map[string][]string) string {
	words := callWords(call, lits)
	if !(words["add"] || words["delete"] || words["change"]) {
		return ""
	}
	if name == "converge.go" || (name == "helpers.go" && fn == "setInterfaceAddress") {
		return ""
	}
	return fmt.Sprintf("%s runs a route write in %q: route writes belong in converge.go (first setup: setInterfaceAddress in helpers.go)", name, fn)
}

// callViolations is one text per call of a route writer in file `name`, unless
// the file is converge.go.
func callViolations(name string, f *ast.File) []string {
	var out []string
	if name == "converge.go" {
		return out
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && routeWriters[id.Name] {
				out = append(out, fmt.Sprintf("%s calls %s: route writes belong in converge.go", name, id.Name))
			}
		}
		return true
	})
	return out
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
			if cmd == "ip" {
				if v := ipRouteViolation(name, call, lits); v != "" {
					out = append(out, v)
				}
				if v := ipAddrViolation(name, funcAt(f, call.Pos()), call, lits); v != "" {
					out = append(out, v)
				}
				continue
			}
			if cmd == "route" {
				if v := routeWriteViolation(name, funcAt(f, call.Pos()), call, lits); v != "" {
					out = append(out, v)
				}
				continue
			}
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
// alias), or the Daemon seams that stand for them, calls applyRouteDiff or
// restoreConnectedRoute, runs sysctl or iptables through any Command call
// (whatever the receiver or the os/exec import alias) except observe.go for
// `iptables -C`, runs an `ip route` write or an `ip addr add` / `ip addr del`
// (the named exception: setInterfaceAddress of helpers.go, first setup); an
// `ip route` read belongs in observe.go. Later tasks extend the pattern list.
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
		for _, v := range callViolations(name, f) {
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
		{"observe may read routes", "observe.go", `cmdExecutor.Command("ip", "-6", "route", "show", "dev", "wg0")`, false},
		{"observe may not replace a route", "observe.go", `cmdExecutor.Command("ip", "route", "replace", "x", "via", "y", "dev", "wg0")`, true},
		{"converge may replace a route", "converge.go", `cmdExecutor.Command("ip", "route", "replace", "x", "via", "y", "dev", "wg0")`, false},
		{"routes may not delete a route", "routes.go", `cmdExecutor.Command("ip", "route", "del", "x", "via", "y", "dev", "wg0")`, true},
		{"routes may not read routes", "routes.go", `cmdExecutor.Command("ip", "route", "show", "dev", "wg0")`, true},
		{"route word in a constant", "routes.go", "const r = \"replace\"\n\tcmdExecutor.Command(\"ip\", \"route\", r, \"x\")", true},
		{"ip addr is not a route command", "routes.go", `cmdExecutor.Command("ip", "-4", "-o", "addr", "show")`, false},
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

// The address guard can fail: `ip addr add` and `ip addr del` are flagged
// outside converge.go, except in the first-setup helper setInterfaceAddress of
// helpers.go (also reached through applyInterfaceAddress of collision.go).
func TestAddressGuardForms(t *testing.T) {
	cases := []struct {
		name string
		file string
		fn   string
		body string
		bad  bool
	}{
		{"converge may add", "converge.go", "f", `cmdExecutor.Command("ip", "addr", "add", "x", "dev", "wg0")`, false},
		{"converge may delete", "converge.go", "f", `cmdExecutor.Command("ip", "addr", "del", "x", "dev", "wg0")`, false},
		{"observe may read", "observe.go", "f", `cmdExecutor.Command("ip", "-o", "addr", "show", "dev", "wg0")`, false},
		{"observe may not add", "observe.go", "f", `cmdExecutor.Command("ip", "addr", "add", "x", "dev", "wg0")`, true},
		{"routes may not delete", "routes.go", "f", `cmdExecutor.Command("ip", "addr", "del", "x", "dev", "wg0")`, true},
		{"routes may read", "routes.go", "f", `cmdExecutor.Command("ip", "-4", "-o", "addr", "show", "to", "x")`, false},
		{"first setup helper may add", "helpers.go", "setInterfaceAddress", `cmdExecutor.Command("ip", "addr", "add", "x", "dev", "wg0")`, false},
		{"another helper may not add", "helpers.go", "resetInterface", `cmdExecutor.Command("ip", "addr", "add", "x", "dev", "wg0")`, true},
		{"the helper's name elsewhere is no exception", "routes.go", "setInterfaceAddress", `cmdExecutor.Command("ip", "addr", "add", "x", "dev", "wg0")`, true},
		{"collision may not add", "collision.go", "f", `cmdExecutor.Command("ip", "addr", "add", "x", "dev", "wg0")`, true},
		{"word in a constant", "routes.go", "f", "const w = \"del\"\n\tcmdExecutor.Command(\"ip\", \"addr\", w, \"x\")", true},
		{"ip -6 addr add", "other.go", "f", `cmdExecutor.Command("ip", "-6", "addr", "add", "x", "dev", "wg0")`, true},
		{"ip address add", "routes.go", "f", `cmdExecutor.Command("ip", "address", "add", "x", "dev", "wg0")`, true},
		{"ip addr flush", "routes.go", "f", `cmdExecutor.Command("ip", "addr", "flush", "dev", "wg0")`, true},
		{"ip -4 addr flush in another helper", "helpers.go", "other", `cmdExecutor.Command("ip", "-4", "addr", "flush", "dev", "wg0")`, true},
		{"first setup helper may flush", "helpers.go", "setInterfaceAddress", `cmdExecutor.Command("ip", "-4", "addr", "flush", "dev", "wg0")`, false},
		{"the reset helper may flush", "helpers.go", "resetInterface", `cmdExecutor.Command("ip", "addr", "flush", "dev", "wg0")`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "package daemon\n\nfunc " + c.fn + "(args []string) {\n\t" + c.body + "\n}\n"
			f, err := parser.ParseFile(token.NewFileSet(), c.file, src, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := commandViolations(c.file, f, stringLiterals(map[string]*ast.File{c.file: f}))
			if (len(got) > 0) != c.bad {
				t.Errorf("violations = %q, want violation %v", got, c.bad)
			}
		})
	}
}

// A route writer called outside converge.go is flagged; converge.go itself,
// and another function, are not.
func TestRouteWriterCallsGuardForms(t *testing.T) {
	cases := []struct {
		name string
		file string
		body string
		bad  bool
	}{
		{"routes calls applyRouteDiff", "routes.go", `applyRouteDiff("wg0", nil, nil)`, true},
		{"routes calls restoreConnectedRoute", "routes.go", `restoreConnectedRoute("10.0.0.0/24")`, true},
		{"converge calls applyRouteDiff", "converge.go", `applyRouteDiff("wg0", nil, nil)`, false},
		{"converge calls restoreConnectedRoute", "converge.go", `restoreConnectedRoute("10.0.0.0/24")`, false},
		{"darwin_routes calls applyRouteDiffDarwin", "darwin_routes.go", `applyRouteDiffDarwin("utun4", routePlan{})`, true},
		{"darwin_routes calls darwinRouteCommand", "darwin_routes.go", `darwinRouteCommand("add", "10.0.0.0/24", "utun4")`, true},
		{"converge calls applyRouteDiffDarwin", "converge.go", `applyRouteDiffDarwin("utun4", routePlan{})`, false},
		{"converge calls darwinRouteCommand", "converge.go", `darwinRouteCommand("add", "10.0.0.0/24", "utun4")`, false},
		{"routes calls restoreHostDefault", "routes.go", `restoreHostDefault(hostRoute{}, false)`, true},
		{"converge calls restoreHostDefault", "converge.go", `restoreHostDefault(hostRoute{}, false)`, false},
		{"another function", "routes.go", `applyRoutes("wg0")`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "package daemon\n\nfunc f() {\n\t" + c.body + "\n}\n"
			f, err := parser.ParseFile(token.NewFileSet(), c.file, src, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got := callViolations(c.file, f); (len(got) > 0) != c.bad {
				t.Errorf("violations = %q, want violation %v", got, c.bad)
			}
		})
	}
}

// The macOS route guard can fail: `route -n add|delete|change` is flagged
// outside converge.go, except in the first-setup helper setInterfaceAddress of
// helpers.go; `route -n get` and `route -n monitor` write nothing.
func TestRouteCommandGuardForms(t *testing.T) {
	cases := []struct {
		name string
		file string
		fn   string
		body string
		bad  bool
	}{
		{"converge may add", "converge.go", "f", `cmdExecutor.Command("route", "-n", "add", "-net", "x", "-interface", "utun4")`, false},
		{"converge may delete", "converge.go", "f", `cmdExecutor.Command("route", "-n", "delete", "-net", "x", "-interface", "utun4")`, false},
		{"converge may change", "converge.go", "f", `cmdExecutor.Command("route", "-n", "change", "-net", "x", "-interface", "utun4")`, false},
		{"darwin_routes may not add", "darwin_routes.go", "f", `cmdExecutor.Command("route", "-n", "add", "-net", "x", "-interface", "utun4")`, true},
		{"darwin_routes may not delete", "darwin_routes.go", "f", `cmdExecutor.Command("route", "-n", "delete", "-net", "x", "-interface", "utun4")`, true},
		{"observe may not change", "observe.go", "f", `cmdExecutor.Command("route", "-n", "change", "-net", "x", "-interface", "utun4")`, true},
		{"route add with -inet6", "routes.go", "f", `cmdExecutor.Command("route", "-n", "add", "-inet6", "-net", "x", "-interface", "utun4")`, true},
		{"route add built with a spread", "routes.go", "f", `cmdExecutor.Command("route", "-n", "add", rest...)`, true},
		{"first setup helper may add", "helpers.go", "setInterfaceAddress", `cmdExecutor.Command("route", "-n", "add", "-net", "x", "-interface", "utun4")`, false},
		{"first setup helper may change", "helpers.go", "setInterfaceAddress", `cmdExecutor.Command("route", "-n", "change", "-net", "x", "-interface", "utun4")`, false},
		{"another helper may not add", "helpers.go", "other", `cmdExecutor.Command("route", "-n", "add", "-net", "x", "-interface", "utun4")`, true},
		{"the helper's name elsewhere is no exception", "routes.go", "setInterfaceAddress", `cmdExecutor.Command("route", "-n", "add", "-net", "x", "-interface", "utun4")`, true},
		{"collision may not add", "collision.go", "f", `cmdExecutor.Command("route", "-n", "add", "-net", "x", "-interface", "utun4")`, true},
		{"word in a constant", "routes.go", "f", "const w = \"delete\"\n\tcmdExecutor.Command(\"route\", \"-n\", w, \"x\")", true},
		{"route get reads", "darwin_routes.go", "f", `cmdExecutor.Command("route", "-n", "get", "192.168.50.1")`, false},
		{"netstat reads", "darwin_routes.go", "f", `cmdExecutor.Command("netstat", "-rn", "-f", "inet")`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "package daemon\n\nfunc " + c.fn + "(args, rest []string) {\n\t" + c.body + "\n}\n"
			f, err := parser.ParseFile(token.NewFileSet(), c.file, src, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := commandViolations(c.file, f, stringLiterals(map[string]*ast.File{c.file: f}))
			if (len(got) > 0) != c.bad {
				t.Errorf("violations = %q, want violation %v", got, c.bad)
			}
		})
	}
}
