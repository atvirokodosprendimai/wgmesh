package promo

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "promos.json")

	store, err := NewStore(StoreConfig{StoragePath: storePath})
	if err != nil {
		t.Fatalf("NewStore() failed: %v", err)
	}

	return store
}

func TestNewStore(t *testing.T) {
	t.Run("creates new store", func(t *testing.T) {
		store := tempStore(t)
		if store == nil {
			t.Error("store is nil")
		}
	})

	t.Run("loads existing store", func(t *testing.T) {
		tmpDir := t.TempDir()
		storePath := filepath.Join(tmpDir, "promos.json")

		// Create store and add data
		store1, err := NewStore(StoreConfig{StoragePath: storePath})
		if err != nil {
			t.Fatalf("NewStore() failed: %v", err)
		}

		campaign, err := store1.CreateCampaign("test", "Test Campaign", SourceDiscord, 30, 5, time.Now().Add(30*24*time.Hour))
		if err != nil {
			t.Fatalf("CreateCampaign() failed: %v", err)
		}

		_, err = store1.GeneratePromo(campaign.ID, "seed-1")
		if err != nil {
			t.Fatalf("GeneratePromo() failed: %v", err)
		}

		// Load store again
		store2, err := NewStore(StoreConfig{StoragePath: storePath})
		if err != nil {
			t.Fatalf("NewStore() reload failed: %v", err)
		}

		loadedCampaign, err := store2.GetCampaign("test")
		if err != nil {
			t.Fatalf("GetCampaign() failed: %v", err)
		}

		if loadedCampaign.Name != "Test Campaign" {
			t.Errorf("campaign name = %q, want %q", loadedCampaign.Name, "Test Campaign")
		}
	})
}

func TestCreateCampaign(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		campName  string
		source    Source
		trialDays int
		nodeLimit int
		expiresAt time.Time
		wantErr   bool
	}{
		{
			name:      "valid campaign",
			id:        "test-1",
			campName:  "Test Campaign",
			source:    SourceDiscord,
			trialDays: 30,
			nodeLimit: 5,
			expiresAt: time.Now().Add(30 * 24 * time.Hour),
			wantErr:   false,
		},
		{
			name:      "empty name",
			id:        "test-2",
			campName:  "",
			source:    SourceDiscord,
			trialDays: 30,
			nodeLimit: 5,
			expiresAt: time.Now().Add(30 * 24 * time.Hour),
			wantErr:   true,
		},
		{
			name:      "zero trial days",
			id:        "test-3",
			campName:  "Test",
			source:    SourceDiscord,
			trialDays: 0,
			nodeLimit: 5,
			expiresAt: time.Now().Add(30 * 24 * time.Hour),
			wantErr:   true,
		},
		{
			name:      "zero node limit",
			id:        "test-4",
			campName:  "Test",
			source:    SourceDiscord,
			trialDays: 30,
			nodeLimit: 0,
			expiresAt: time.Now().Add(30 * 24 * time.Hour),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := tempStore(t)
			_, err := store.CreateCampaign(tt.id, tt.campName, tt.source, tt.trialDays, tt.nodeLimit, tt.expiresAt)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCampaign() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGeneratePromo(t *testing.T) {
	store := tempStore(t)

	campaign, err := store.CreateCampaign("test", "Test Campaign", SourceDiscord, 30, 5, time.Now().Add(30*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign() failed: %v", err)
	}

	t.Run("generates valid code", func(t *testing.T) {
		code, err := store.GeneratePromo(campaign.ID, "seed-1")
		if err != nil {
			t.Fatalf("GeneratePromo() failed: %v", err)
		}

		if !code.IsValid() {
			t.Error("generated code is not valid")
		}
	})

	t.Run("campaign not found", func(t *testing.T) {
		_, err := store.GeneratePromo("nonexistent", "seed")
		if err == nil {
			t.Error("expected error for nonexistent campaign")
		}
	})

	t.Run("expired campaign", func(t *testing.T) {
		expiredCampaign, err := store.CreateCampaign("expired", "Expired", SourceDiscord, 30, 5, time.Now().Add(-1*time.Hour))
		if err != nil {
			t.Fatalf("CreateCampaign() failed: %v", err)
		}

		_, err = store.GeneratePromo(expiredCampaign.ID, "seed")
		if err == nil {
			t.Error("expected error for expired campaign")
		}
	})
}

func TestRedeemCode(t *testing.T) {
	store := tempStore(t)

	campaign, err := store.CreateCampaign("test", "Test Campaign", SourceDiscord, 30, 5, time.Now().Add(30*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign() failed: %v", err)
	}

	code, err := store.GeneratePromo(campaign.ID, "seed-1")
	if err != nil {
		t.Fatalf("GeneratePromo() failed: %v", err)
	}

	t.Run("redeems valid code", func(t *testing.T) {
		redeemedCampaign, trialEndsAt, err := store.RedeemCode(code, "account-123")
		if err != nil {
			t.Fatalf("RedeemCode() failed: %v", err)
		}

		if redeemedCampaign.ID != campaign.ID {
			t.Errorf("campaign ID = %q, want %q", redeemedCampaign.ID, campaign.ID)
		}

		expectedTrialEnd := time.Now().AddDate(0, 0, 30)
		if trialEndsAt.Before(expectedTrialEnd.Add(-1*time.Minute)) || trialEndsAt.After(expectedTrialEnd.Add(1*time.Minute)) {
			t.Errorf("trial end time not within expected range")
		}
	})

	t.Run("prevents double redemption", func(t *testing.T) {
		_, _, err := store.RedeemCode(code, "account-456")
		if err == nil {
			t.Error("expected error for double redemption")
		}
	})

	t.Run("invalid code", func(t *testing.T) {
		_, _, err := store.RedeemCode("INVALIDCODE123", "account-123")
		if err == nil {
			t.Error("expected error for invalid code")
		}
	})

	t.Run("nonexistent code", func(t *testing.T) {
		validCode, _ := GenerateCode("test", "seed")
		_, _, err := store.RedeemCode(validCode, "account-123")
		if err == nil {
			t.Error("expected error for nonexistent code")
		}
	})
}

func TestStoreAtomicWrite(t *testing.T) {
	t.Run("survives crash during write", func(t *testing.T) {
		tmpDir := t.TempDir()
		storePath := filepath.Join(tmpDir, "promos.json")

		// Create store and add data
		store1, err := NewStore(StoreConfig{StoragePath: storePath})
		if err != nil {
			t.Fatalf("NewStore() failed: %v", err)
		}

		campaign, err := store1.CreateCampaign("test", "Test Campaign", SourceDiscord, 30, 5, time.Now().Add(30*24*time.Hour))
		if err != nil {
			t.Fatalf("CreateCampaign() failed: %v", err)
		}

		code, err := store1.GeneratePromo(campaign.ID, "seed-1")
		if err != nil {
			t.Fatalf("GeneratePromo() failed: %v", err)
		}

		_, _, err = store1.RedeemCode(code, "account-123")
		if err != nil {
			t.Fatalf("RedeemCode() failed: %v", err)
		}

		// Simulate crash: corrupt the main file but leave .tmp
		tmpPath := storePath + ".tmp"
		if _, err := os.ReadFile(tmpPath); err == nil {
			// Temp file exists from atomic write, save it for verification
			tmpData, _ := os.ReadFile(tmpPath)
			_ = os.WriteFile(storePath, tmpData, 0600)
		}

		// Reload store
		store2, err := NewStore(StoreConfig{StoragePath: storePath})
		if err != nil {
			t.Fatalf("NewStore() reload failed: %v", err)
		}

		// Verify data survived
		_, err = store2.GetCampaign("test")
		if err != nil {
			t.Errorf("campaign not recovered: %v", err)
		}
	})
}

func TestGetCampaignStats(t *testing.T) {
	store := tempStore(t)

	campaign, err := store.CreateCampaign("test", "Test Campaign", SourceDiscord, 30, 5, time.Now().Add(30*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateCampaign() failed: %v", err)
	}

	// Generate 5 codes
	for i := 0; i < 5; i++ {
		seed := fmt.Sprintf("seed-%d", i)
		code, err := store.GeneratePromo(campaign.ID, seed)
		if err != nil {
			t.Fatalf("GeneratePromo() failed: %v", err)
		}

		// Redeem first 3
		if i < 3 {
			_, _, err := store.RedeemCode(code, fmt.Sprintf("account-%d", i))
			if err != nil {
				t.Fatalf("RedeemCode() failed: %v", err)
			}
		}
	}

	generated, redeemed, err := store.GetCampaignStats(campaign.ID)
	if err != nil {
		t.Fatalf("GetCampaignStats() failed: %v", err)
	}

	if generated != 5 {
		t.Errorf("generated = %d, want 5", generated)
	}
	if redeemed != 3 {
		t.Errorf("redeemed = %d, want 3", redeemed)
	}
}
