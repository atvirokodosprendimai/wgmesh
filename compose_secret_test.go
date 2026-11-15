package main

import (
	"os"
	"strings"
	"testing"
)

// The mesh secret must reach the container through the environment
// (WGMESH_SECRET), never on the command line where ps and docker inspect show it.
func TestComposeFilesKeepSecretOffCommandLine(t *testing.T) {
	t.Parallel()

	const argvSecret = `--secret "${MESH_SECRET}"`

	for _, path := range []string{"docker-compose.yml", "DOCKER-COMPOSE.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, argvSecret) {
				t.Errorf("%s:%d puts the secret on the command line: %s", path, i+1, strings.TrimSpace(line))
			}
		}
	}
}
