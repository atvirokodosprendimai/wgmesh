package promo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPromoSaveLeavesOnlyTheStoreFile: after a save the directory holds the
// store file and nothing else, so the helper's temp file is not left behind
// (the older test checks only the fixed ".tmp" name).
func TestPromoSaveLeavesOnlyTheStoreFile(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "promos.json")
	store, err := NewStore(StoreConfig{StoragePath: storePath})
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}
	if _, err := store.CreateCampaign("test", "Test Campaign", SourceDiscord, 30, 5, time.Now().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("CreateCampaign() failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "promos.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only promos.json", names)
	}
}
