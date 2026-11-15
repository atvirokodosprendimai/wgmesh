package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// argvSecret matches a --secret flag whose value is a variable, braced or not,
// quoted or not: the secret would be on the container command line.
var argvSecret = regexp.MustCompile(`--secret(=|\s+)["']?\$`)

// The mesh secret must reach the container through the environment
// (WGMESH_SECRET), never on the command line where ps and docker inspect show it.
// Every docker-compose*.yml in the repo and DOCKER-COMPOSE.md are scanned,
// commented-out example services included: people uncomment and copy them.
// Lines with `exec` are skipped: `docker compose exec ... wgmesh status --secret "$X"`
// runs inside an already running container (status reads flags only).
func TestComposeFilesKeepSecretOffCommandLine(t *testing.T) {
	t.Parallel()

	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "DOCKER-COMPOSE.md" || (strings.HasPrefix(d.Name(), "docker-compose") && strings.HasSuffix(d.Name(), ".yml")) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(files) < 3 {
		t.Fatalf("found %d compose files %v, want at least 3", len(files), files)
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "exec") {
				continue
			}
			if argvSecret.MatchString(line) {
				t.Errorf("%s:%d puts the secret on the command line: %s", path, i+1, strings.TrimSpace(line))
			}
		}
	}
}
