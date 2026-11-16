package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Words: acquireInterfaceLock takes the per-interface lock before joinCmd
// builds the daemon or the RPC server.
// Why: a second `wgmesh join` unlinked the live RPC socket, reset the live
// interface and its deferred teardown deleted it (#860).
// Invariants: the error names the lock file and the holder's pid; the lock is
// taken before daemon.NewDaemon and createRPCServer.

func TestAcquireInterfaceLockMessage(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "wgtest0.lock")

	release, err := acquireInterfaceLock(lock)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()
	_, err = acquireInterfaceLock(lock)
	if err == nil {
		t.Fatal("second acquire succeeded while the first is held")
	}
	for _, want := range []string{
		"another wgmesh join holds the lock " + lock,
		"pid " + strconv.Itoa(os.Getpid()),
		"stop it first",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestJoinTakesLockBeforeDaemonAndSocket(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]token.Pos{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "joinCmd" {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				name = fun.Name
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok {
					name = x.Name + "." + fun.Sel.Name
				}
			}
			switch name {
			case "acquireInterfaceLock", "daemon.NewDaemon", "createRPCServer", "handleAccountFlag", "recordReferralCode":
				if _, seen := pos[name]; !seen {
					pos[name] = call.Pos()
				}
			}
			return true
		})
	}
	lock, ok := pos["acquireInterfaceLock"]
	if !ok {
		t.Fatal("joinCmd does not call acquireInterfaceLock")
	}
	for _, later := range []string{"daemon.NewDaemon", "createRPCServer", "handleAccountFlag", "recordReferralCode"} {
		p, ok := pos[later]
		if !ok {
			t.Fatalf("joinCmd no longer calls %s; the test needs updating", later)
		}
		if lock >= p {
			t.Errorf("acquireInterfaceLock (%v) does not precede %s (%v)", fset.Position(lock), later, fset.Position(p))
		}
	}
}
