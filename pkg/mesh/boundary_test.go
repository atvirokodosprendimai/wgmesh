package mesh

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The centralized deploy pipeline (ADR-0006) never touches the PeerStore side:
// no non-test file of pkg/mesh, pkg/ssh or pkg/wireguard may import the
// decentralized packages.
func TestCentralizedPackagesDoNotImportPeerStore(t *testing.T) {
	const module = "github.com/atvirokodosprendimai/wgmesh/pkg/"
	forbidden := []string{"node", "daemon", "discovery"}
	for _, pkg := range []string{"mesh", "ssh", "wireguard"} {
		files, err := filepath.Glob(filepath.Join("..", pkg, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no files for pkg/%s: %v", pkg, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				for _, bad := range forbidden {
					if path == module+bad || strings.HasPrefix(path, module+bad+"/") {
						t.Errorf("%s imports %s", file, path)
					}
				}
			}
		}
	}
}
