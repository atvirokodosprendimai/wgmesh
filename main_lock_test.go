package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
		"stop it first (systemctl stop wgmesh, or brew services stop wgmesh)",
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
	// The release must be a top-level `defer releaseLock()` of joinCmd that
	// follows the acquire: a plain call would drop the lock at once. Like the
	// order check above, this only sees calls written in joinCmd itself.
	var deferPos token.Pos
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "joinCmd" {
			continue
		}
		for _, stmt := range fn.Body.List {
			d, ok := stmt.(*ast.DeferStmt)
			if !ok {
				continue
			}
			if id, ok := d.Call.Fun.(*ast.Ident); ok && id.Name == "releaseLock" {
				deferPos = d.Pos()
			}
		}
	}
	if deferPos == token.NoPos {
		t.Error("joinCmd has no top-level `defer releaseLock()`")
	} else if deferPos <= lock {
		t.Errorf("defer releaseLock (%v) does not follow acquireInterfaceLock (%v)", fset.Position(deferPos), fset.Position(lock))
	}
}

func TestAcquireInterfaceLockPermissionHint(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root is not denied by directory modes")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	_, err := acquireInterfaceLock(filepath.Join(dir, "wg0.lock"))
	if err == nil {
		t.Fatal("acquire succeeded in a read-only directory")
	}
	for _, want := range []string{"permission denied", "run wgmesh join as root"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestAcquireInterfaceLockCreatesDirectory0700(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	release, err := acquireInterfaceLock(filepath.Join(dir, "wg0.lock"))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer release()
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() || fi.Mode().Perm() != 0o700 {
		t.Errorf("lock directory mode = %v, want a directory with 0700", fi.Mode())
	}
}

// A holder that locked but has not written its pid yet is reported as unknown.
func TestAcquireInterfaceLockHolderWithoutPid(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "wg0.lock")
	f, err := os.OpenFile(lock, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, err = acquireInterfaceLock(lock)
	if err == nil || !strings.Contains(err.Error(), "pid unknown") {
		t.Errorf("error = %v, want it to say pid unknown", err)
	}
}
