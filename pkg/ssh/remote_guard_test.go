package ssh

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// remoteGuardDirs are the packages that send commands to a node. They are
// named here, so deleting a Cmd call, or adding a fourth package that runs
// commands, is a visible edit of this list.
var remoteGuardDirs = []string{".", "../wireguard", "../mesh"}

// commandSinks are the method names through which a command line reaches a
// shell: the four Client methods that take one (WriteFile builds its own line
// from a path it quotes, so its inner session.Start is the sink) and
// session.Start. The first argument is the command line.
var commandSinks = map[string]bool{"Run": true, "RunQuiet": true, "RunWithStdin": true, "Start": true}

// clientPassThrough are the Client methods that hand their parameter to the
// session unchanged; their own calls to a sink are the primitive, not a user.
var clientPassThrough = map[string]bool{"Run": true, "RunQuiet": true, "RunWithStdin": true}

// TestNoUnquotedRemoteCommand fails when a non-test file of pkg/ssh,
// pkg/wireguard or pkg/mesh passes a command line to a sink that is not made
// of string constants and calls of Cmd (ADR-0006 T2).
//
// What it sees, and what it does not: it parses the files and judges the
// first argument of every call whose method name is Run, RunQuiet,
// RunWithStdin or Start and that has at least one argument, whatever the
// receiver is. The argument may be a string constant (a literal, or a name
// declared const in the package or in the enclosing function), a call of Cmd
// whose command name is itself constant, or a sum of those. Quote may appear
// in a sum after its first operand (`"cat > " + Quote(path)`), never first,
// because the first word is the program that runs. Anything else fails:
// fmt.Sprintf, a variable, a function result, a concatenation with a
// variable. The one exception is routes.ConnectedRouteCmd as the name of a
// Cmd call: it builds a finished command from a parsed network, an interface
// name checked by pkg/routes and a parsed address (ADR-0006, Out of Scope).
//
// It does not follow a command through a function value, an interface, a
// method value or a differently named wrapper, and it does not resolve a name
// that a local variable shadows. Two limits of matching by name: the
// pass-through exemption is given to methods named Run, RunQuiet and
// RunWithStdin whose receiver type is named Client, so another type that
// is also called Client (in one of the three packages) would get it too; and
// Cmd and Quote are recognised unqualified or with the qualifier `ssh`, so a
// package imported under another name, or a different package called ssh,
// would be taken for this one. The files of pkg/ssh/sshtest and every
// _test.go file are not parsed.
func TestNoUnquotedRemoteCommand(t *testing.T) {
	for _, dir := range remoteGuardDirs {
		files := parseNonTestFiles(t, dir)
		raw, checked := rawCommands(files.fset, files.files)
		if checked == 0 {
			t.Errorf("%s: no command call found; the guard would pass without looking at anything", dir)
		}
		for _, r := range raw {
			t.Errorf("%s", r)
		}
	}
}

type parsedDir struct {
	fset  *token.FileSet
	files []*ast.File
}

func parseNonTestFiles(t *testing.T, dir string) parsedDir {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := parsedDir{fset: token.NewFileSet()}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(p.fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		p.files = append(p.files, f)
	}
	return p
}

// rawCommands returns one line per sink call in files whose command argument
// is not made of constants and Cmd calls, and the number of sink calls it
// judged.
func rawCommands(fset *token.FileSet, files []*ast.File) (raw []string, checked int) {
	pkgConsts := map[string]bool{}
	for _, f := range files {
		for _, d := range f.Decls {
			if gd, ok := d.(*ast.GenDecl); ok {
				addConsts(gd, pkgConsts)
			}
		}
	}
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			consts := map[string]bool{}
			for n := range pkgConsts {
				consts[n] = true
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if gd, ok := n.(*ast.GenDecl); ok {
					addConsts(gd, consts)
				}
				return true
			})
			passThrough := recvIsClient(fd) && clientPassThrough[fd.Name.Name]
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !commandSinks[sel.Sel.Name] || passThrough {
					return true
				}
				checked++
				if !isCommandExpr(call.Args[0], consts, true) {
					raw = append(raw, fmt.Sprintf("%s: %s(%s) is not made of constants and Cmd calls",
						fset.Position(call.Pos()), sel.Sel.Name, render(fset, call.Args[0])))
				}
				return true
			})
		}
	}
	return raw, checked
}

func addConsts(gd *ast.GenDecl, into map[string]bool) {
	if gd.Tok != token.CONST {
		return
	}
	for _, s := range gd.Specs {
		if vs, ok := s.(*ast.ValueSpec); ok {
			for _, name := range vs.Names {
				into[name.Name] = true
			}
		}
	}
}

// isCommandExpr reports whether e is a command line built safely. first is
// true while e is the leftmost operand of the line, where Quote is refused.
func isCommandExpr(e ast.Expr, consts map[string]bool, first bool) bool {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return isCommandExpr(e.X, consts, first)
	case *ast.BinaryExpr:
		return e.Op == token.ADD && isCommandExpr(e.X, consts, first) && isCommandExpr(e.Y, consts, false)
	case *ast.CallExpr:
		switch calleeName(e.Fun) {
		case "Cmd":
			return len(e.Args) > 0 && isCmdName(e.Args[0], consts)
		case "Quote":
			return !first
		}
		return false
	}
	return isConstString(e, consts)
}

// isCmdName reports whether e is acceptable as the command name of Cmd, which
// uses it as given: a constant, or routes.ConnectedRouteCmd(...).
func isCmdName(e ast.Expr, consts map[string]bool) bool {
	if call, ok := e.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "routes" && sel.Sel.Name == "ConnectedRouteCmd" {
				return true
			}
		}
	}
	return isConstString(e, consts)
}

// isConstString reports whether e is a string literal, a name declared const,
// or a sum of those.
func isConstString(e ast.Expr, consts map[string]bool) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.Ident:
		return consts[e.Name]
	case *ast.ParenExpr:
		return isConstString(e.X, consts)
	case *ast.BinaryExpr:
		return e.Op == token.ADD && isConstString(e.X, consts) && isConstString(e.Y, consts)
	}
	return false
}

// calleeName is the function name of a call to this repository's helpers:
// `Cmd` for Cmd(...) and ssh.Cmd(...). A call qualified by any other package
// (strconv.Quote) has no name here.
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if x, ok := f.X.(*ast.Ident); ok && x.Name == "ssh" {
			return f.Sel.Name
		}
	}
	return ""
}

// recvIsClient reports whether fd is a method of Client or *Client.
func recvIsClient(fd *ast.FuncDecl) bool {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return false
	}
	t := fd.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	id, ok := t.(*ast.Ident)
	return ok && id.Name == "Client"
}

func render(fset *token.FileSet, e ast.Expr) string {
	var b bytes.Buffer
	if err := printer.Fprint(&b, fset, e); err != nil {
		return "?"
	}
	return b.String()
}

// TestRemoteGuardJudgesShapes feeds the guard source snippets, so each way a
// raw command can come back (a Sprintf, a variable, a concatenation, a Cmd
// whose name is built, a leading Quote) is known to fail it, and each safe
// shape is known to pass.
func TestRemoteGuardJudgesShapes(t *testing.T) {
	const prelude = "package p\nconst detect = \"curl -s\"\nfunc f(client *Client, iface string, a, b, c int) {\n"
	tests := []struct {
		name string
		body string
		bad  bool
	}{
		{"constant", `client.Run("which wg")`, false},
		{"constant name", `client.Run(detect)`, false},
		{"Cmd", `client.Run(ssh.Cmd("ip", "route", "show", "dev", iface))`, false},
		{"Cmd with a suffix", `client.RunQuiet(Cmd("ip", iface) + " 2>/dev/null || true")`, false},
		{"two Cmd", `client.RunQuiet(Cmd("ip", iface) + " || " + Cmd("ip", iface))`, false},
		{"constant then Quote", `client.RunQuiet("cat > " + Quote(iface))`, false},
		{"pass-through Cmd of the routes builder", `client.Run(ssh.Cmd(routes.ConnectedRouteCmd(a, b, c)))`, false},
		{"stdin variant", `client.RunWithStdin(Cmd("wg", "set", iface), "psk")`, false},
		{"Sprintf", `client.Run(fmt.Sprintf("ip route show dev %s", iface))`, true},
		{"variable", `cmd := "ip route show"; client.Run(cmd)`, true},
		{"concatenation with a variable", `client.Run("ip route show dev " + iface)`, true},
		{"Cmd then a variable", `client.RunQuiet(Cmd("ip") + iface)`, true},
		{"Cmd name with a variable", `client.Run(Cmd("ip " + iface))`, true},
		{"Cmd name from Sprintf", `client.Run(Cmd(fmt.Sprintf("ip %s", iface)))`, true},
		{"Quote first", `client.Run(Quote(iface) + " --version")`, true},
		{"Quote alone", `client.Run(Quote(iface))`, true},
		{"function result", `client.Run(build(iface))`, true},
		{"RunWithStdin with a variable", `cmd := "wg"; client.RunWithStdin(cmd, "psk")`, true},
		{"session Start with a variable", `session.Start(iface)`, true},
		{"Cmd with no arguments", `client.Run(Cmd())`, true},
		{"Quote of another package", `client.RunQuiet("cat > " + strconv.Quote(iface))`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "x.go", prelude+tt.body+"\n}\n", 0)
			if err != nil {
				t.Fatal(err)
			}
			raw, checked := rawCommands(fset, []*ast.File{f})
			if checked != 1 {
				t.Fatalf("judged %d calls, want 1", checked)
			}
			if got := len(raw) > 0; got != tt.bad {
				t.Errorf("rejected = %v, want %v (%v)", got, tt.bad, raw)
			}
		})
	}

	t.Run("a Client method that passes its parameter on is the primitive", func(t *testing.T) {
		src := "package p\nfunc (c *Client) Run(cmd string) { session.Run(cmd) }\nfunc (c *Client) WriteFile(p string) { session.Start(p) }\n"
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := rawCommands(fset, []*ast.File{f})
		if len(raw) != 1 || !strings.Contains(raw[0], "Start(p)") {
			t.Errorf("want only the Start in WriteFile rejected, got %v", raw)
		}
	})
	t.Run("a method of another type is not the primitive", func(t *testing.T) {
		src := "package p\nfunc (x *other) Run(p string) { session.Run(fmt.Sprintf(\"x %s\", p)) }\n"
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x.go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := rawCommands(fset, []*ast.File{f})
		if len(raw) != 1 {
			t.Errorf("want the Sprintf in other.Run rejected, got %v", raw)
		}
	})
}
