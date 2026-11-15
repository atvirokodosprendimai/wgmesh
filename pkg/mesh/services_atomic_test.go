package mesh

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveServicesLeavesOnlyTheFile: after two saves the directory holds the
// services file and nothing else, so the helper's temp file is not left behind
// (the older test checks only the fixed ".tmp" name).
func TestSaveServicesLeavesOnlyTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "services.json")
	state := ServiceState{Services: map[string]ServiceEntry{"api": {SiteID: "site_xyz", Name: "api"}}}

	for i := 0; i < 2; i++ {
		if err := SaveServices(path, state); err != nil {
			t.Fatalf("SaveServices #%d: %v", i+1, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "services.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only services.json", names)
	}
}
