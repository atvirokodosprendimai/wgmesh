package atomicfile

// Words: a state file is replaced by atomicfile.WriteFile and by nothing else.
// These tests parse every production .go file of the module and fail when the
// number of os.WriteFile / ioutil.WriteFile calls in a file differs from the
// table below, or when os.Rename appears outside this package's atomicfile.go.
//
// Why: the private temp-and-rename copies never fsynced and shared one fixed
// ".tmp" name (issue #870 class). One helper plus a guard that fails on a new
// bypass keeps the rule in one place (ADR-0005, M6).
//
// Limits, stated so nobody trusts the guard more than it is:
//   - it counts calls per file, so a second call that replaces an allowed one
//     in the same file is not seen;
//   - a call through an alias import (import o "os"; o.WriteFile) is not seen,
//     and neither is a call made through a function value (w := os.WriteFile;
//     w(...)): only the selector expression is counted;
//   - identifiers named os and ioutil are matched by name, not by import;
//   - os.Create, os.OpenFile (with O_TRUNC) and syscall.Rename are not counted
//     either;
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
	"sort"
	"strings"
	"testing"
)

// allowedWriteFile is the table of production files that may call
// os.WriteFile or ioutil.WriteFile, with the exact count. A file not in the
// table must have 0. Add a row only with a reason.
var allowedWriteFile = map[string]int{
	"main.go":                1, // --output report, not state read again
	"pkg/pilot/report.go":    1, // report output
	"pkg/pilot/config.go":    1, // pilot config template
	"pkg/daemon/systemd.go":  1, // unit at install time; BACKLOG: install-service writing the unit atomically (re-running repairs a short file)
	"cmd/status-gen/main.go": 1, // generated STATUS.md
}

// allowedRename is the only file that may call os.Rename.
const allowedRename = "pkg/atomicfile/atomicfile.go"

// moduleRoot returns the directory of the module under test.
func moduleRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("go list -m -f {{.Dir}}: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// countCalls parses every production .go file under root and returns, per
// slash-separated relative path, the number of calls to pkg.fn for each
// "pkg.fn" in names.
func countCalls(t *testing.T, root string, names ...string) map[string]map[string]int {
	t.Helper()
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	found := map[string]map[string]int{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata":
				return filepath.SkipDir
			}
			if path != root {
				// A hidden directory (.git, .worktrees, .claude) or a nested
				// module holds files that are not this module's.
				if strings.HasPrefix(d.Name(), ".") {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			name := id.Name + "." + sel.Sel.Name
			if want[name] {
				if found[rel] == nil {
					found[rel] = map[string]int{}
				}
				found[rel][name]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
}

func TestNoInPlaceStateWrite(t *testing.T) {
	found := countCalls(t, moduleRoot(t), "os.WriteFile", "ioutil.WriteFile")
	files := map[string]bool{}
	for f := range found {
		files[f] = true
	}
	for f := range allowedWriteFile {
		files[f] = true
	}
	var names []string
	for f := range files {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		got := found[f]["os.WriteFile"] + found[f]["ioutil.WriteFile"]
		allowed := allowedWriteFile[f]
		if got > allowed {
			t.Errorf("%s: found %d os.WriteFile/ioutil.WriteFile calls, allowed %d; "+
				"add a justified row to allowedWriteFile or use atomicfile.WriteFile",
				f, got, allowed)
		} else if got < allowed {
			t.Errorf("%s: found %d os.WriteFile/ioutil.WriteFile calls, allowed %d; "+
				"the table row is stale: lower or remove it",
				f, got, allowed)
		}
	}
}

func TestNoPrivateRenameCopy(t *testing.T) {
	found := countCalls(t, moduleRoot(t), "os.Rename")
	var names []string
	for f := range found {
		names = append(names, f)
	}
	sort.Strings(names)
	for _, f := range names {
		if f == allowedRename {
			continue
		}
		t.Errorf("%s: found %d os.Rename calls, allowed 0; "+
			"a state file must be replaced with atomicfile.WriteFile, not a private temp-and-rename copy",
			f, found[f]["os.Rename"])
	}
	if found[allowedRename]["os.Rename"] != 1 {
		t.Errorf("%s: found %d os.Rename calls, allowed 1 (the helper's own commit step)",
			allowedRename, found[allowedRename]["os.Rename"])
	}
}

// TestGuardSkipsHiddenAndNestedModuleDirs: a call in a hidden directory or in
// a nested module is not this module's and is not counted; a normal directory
// is.
func TestGuardSkipsHiddenAndNestedModuleDirs(t *testing.T) {
	root := t.TempDir()
	const src = "package x\n\nimport \"os\"\n\nfunc f() { _ = os.WriteFile(\"a\", nil, 0o600) }\n"
	files := map[string]string{
		"normal/a.go":       src,
		".worktrees/w/a.go": src,
		".claude/a.go":      src,
		"nested/go.mod":     "module example.com/nested\n",
		"nested/a.go":       src,
		"nested/sub/a.go":   src,
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	found := countCalls(t, root, "os.WriteFile")
	if len(found) != 1 || found["normal/a.go"]["os.WriteFile"] != 1 {
		t.Errorf("countCalls = %v, want only normal/a.go with 1 call", found)
	}
}
