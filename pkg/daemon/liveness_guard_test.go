package daemon

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

// lastSeenAges returns "position what" for every age or order taken from a
// LastSeen selector in the non-test Go files under root, leaving out skipDir,
// .git and vendor: time.Since(x.LastSeen), and Sub, Before, After or Compare
// called on x.LastSeen or with it as an argument. visited is the number of
// files parsed.
func lastSeenAges(root, skipDir string) ([]string, int, error) {
	visited := 0
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
		visited++
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "Since", "Sub", "Before", "After", "Compare":
			default:
				return true
			}
			if sel.Sel.Name != "Since" && isLastSeen(sel.X) {
				found = append(found, fset.Position(call.Pos()).String()+" LastSeen."+sel.Sel.Name)
			}
			for _, arg := range call.Args {
				if isLastSeen(arg) {
					found = append(found, fset.Position(call.Pos()).String()+" "+sel.Sel.Name+"(LastSeen)")
				}
			}
			return true
		})
		return nil
	})
	return found, visited, err
}

// isLastSeen reports whether x is a selector named LastSeen: p.LastSeen,
// d.peer.LastSeen.
func isLastSeen(x ast.Expr) bool {
	sel, ok := x.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "LastSeen"
}

// TestNoDirectLastSeenAge fails when a production file outside pkg/node
// computes the age or the order of a PeerInfo.LastSeen itself, with
// time.Since(x.LastSeen), or Sub, Before, After or Compare called on
// x.LastSeen or with it as an argument: a liveness reader asks
// PeerInfo.Alive, PeerInfo.LastAlive or PeerInfo.DecideByAge instead. pkg/node
// is not scanned.
//
// Limit: an age computed through a local variable (t := p.LastSeen; now.Sub(t))
// is not seen.
func TestNoDirectLastSeenAge(t *testing.T) {
	root := filepath.Join("..", "..")
	found, visited, err := lastSeenAges(root, filepath.Join(root, "pkg", "node"))
	if err != nil {
		t.Fatal(err)
	}
	if visited == 0 {
		t.Fatal("the real-tree scan parsed no files")
	}
	for _, f := range found {
		t.Errorf("direct LastSeen age, ask Alive or LastAlive: %s", f)
	}

	// The guard sees the forms it names, and not the ones it must leave alone.
	cases := []struct {
		name string
		expr string
		want bool
	}{
		{"since", `time.Since(p.LastSeen)`, true},
		{"since_bare", `time.Since(peer.LastSeen) < time.Minute`, true},
		{"sub_arg", `now.Sub(p.LastSeen)`, true},
		{"sub_receiver", `p.LastSeen.Sub(now)`, true},
		{"since_alive", `time.Since(p.LastAlive(now))`, false},
		{"sub_alive", `now.Sub(p.LastAlive(now))`, false},
		{"copy_for_display", `use(p.LastSeen)`, false},
		{"other_since", `time.Since(p.Started)`, false},
		{"before_receiver", `p.LastSeen.Before(now)`, true},
		{"after_receiver", `p.LastSeen.After(now)`, true},
		{"after_arg", `now.After(p.LastSeen)`, true},
		{"before_arg", `now.Before(p.LastSeen)`, true},
		{"compare_arg", `now.Compare(p.LastSeen)`, true},
		{"compare_receiver", `p.LastSeen.Compare(now)`, true},
		{"before_alive", `p.LastAlive(now).Before(now)`, false},
	}
	dir := t.TempDir()
	skip := filepath.Join(dir, "skipped")
	if err := os.Mkdir(skip, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		src := "package bait\n\nfunc f() {\n\t_ = " + c.expr + "\n}\n"
		if err := os.WriteFile(filepath.Join(dir, c.name+".go"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skip, "bait.go"), []byte("package bait\n\nfunc f() {\n\t_ = time.Since(p.LastSeen)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bait_test.go"), []byte("package bait\n\nfunc g() {\n\t_ = time.Since(p.LastSeen)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baitFound, baitVisited, err := lastSeenAges(dir, skip)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(cases) + 0; baitVisited != want {
		t.Errorf("the bait scan parsed %d files, want %d: the counter must count only the files outside the skipped directory and not the test file", baitVisited, want)
	}
	for _, c := range cases {
		var got []string
		for _, f := range baitFound {
			if strings.Contains(f, c.name+".go:") {
				got = append(got, f)
			}
		}
		if c.want && len(got) != 1 {
			t.Errorf("%s: %s: got %v, want one finding", c.name, c.expr, got)
		}
		if !c.want && len(got) != 0 {
			t.Errorf("%s: %s was flagged: %v", c.name, c.expr, got)
		}
	}
	for _, f := range baitFound {
		if strings.Contains(f, "skipped") || strings.Contains(f, "bait_test.go") {
			t.Errorf("a skipped or test file was scanned: %s", f)
		}
	}
}
