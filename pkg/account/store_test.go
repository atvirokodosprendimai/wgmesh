package account

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	store := NewStore("")
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
}

func TestCreateAccount(t *testing.T) {
	store := NewStore("")

	account, err := store.CreateAccount("test@example.com")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	if account.ID == "" {
		t.Error("Account ID is empty")
	}
	if account.Email != "test@example.com" {
		t.Errorf("Email = %s, want %s", account.Email, "test@example.com")
	}
	if account.ReferralCode == "" {
		t.Error("ReferralCode is empty")
	}
	if account.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestCreateAccountUnique(t *testing.T) {
	store := NewStore("")

	acc1, _ := store.CreateAccount("user1@example.com")
	acc2, _ := store.CreateAccount("user2@example.com")

	if acc1.ID == acc2.ID {
		t.Error("Account IDs are not unique")
	}
	if acc1.ReferralCode == acc2.ReferralCode {
		t.Error("Referral codes are not unique")
	}
}

func TestCreateAccountWithReferral(t *testing.T) {
	store := NewStore("")

	// Create referrer account
	referrer, err := store.CreateAccount("referrer@example.com")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	// Create account with referral code
	referred, err := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
	if err != nil {
		t.Fatalf("CreateAccountWithReferral() error = %v", err)
	}

	if referred.ReferredBy != referrer.ID {
		t.Errorf("ReferredBy = %s, want %s", referred.ReferredBy, referrer.ID)
	}
}

func TestCreateAccountWithInvalidReferral(t *testing.T) {
	store := NewStore("")

	_, err := store.CreateAccountWithReferral("test@example.com", ReferralCode("INVALIDCODE"))
	if err == nil {
		t.Error("Expected error for invalid referral code")
	}

	_, err = store.CreateAccountWithReferral("test@example.com", ReferralCode("AAAAAAAAAAA1"))
	if err == nil {
		t.Error("Expected error for non-existent referral code")
	}
}

func TestGetByID(t *testing.T) {
	store := NewStore("")

	account, err := store.CreateAccount("test@example.com")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	retrieved, err := store.GetByID(account.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if retrieved.ID != account.ID {
		t.Errorf("Retrieved account ID mismatch")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	store := NewStore("")

	_, err := store.GetByID(AccountID("nonexistent"))
	if err == nil {
		t.Error("Expected error for non-existent account")
	}
}

func TestGetByCode(t *testing.T) {
	store := NewStore("")

	account, err := store.CreateAccount("test@example.com")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	retrieved, err := store.GetByCode(account.ReferralCode)
	if err != nil {
		t.Fatalf("GetByCode() error = %v", err)
	}

	if retrieved.ID != account.ID {
		t.Errorf("Retrieved account ID mismatch")
	}
}

func TestRecordReferral(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")
	referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)

	// Record the referral
	err := store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
	if err != nil {
		t.Fatalf("RecordReferral() error = %v", err)
	}

	// Verify referral was recorded
	referrals, err := store.GetReferrals(referrer.ID)
	if err != nil {
		t.Fatalf("GetReferrals() error = %v", err)
	}

	if len(referrals) != 1 {
		t.Errorf("GetReferrals() count = %d, want 1", len(referrals))
	}

	if referrals[0].ReferredID != referred.ID {
		t.Errorf("ReferredID mismatch")
	}
}

func TestRecordReferralDuplicate(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")
	referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)

	// Record twice
	store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
	err := store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)

	if err != nil {
		t.Error("Duplicate recording should be idempotent")
	}
}

func TestGetTotalReferrals(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// Create 5 referred accounts
	for i := 0; i < 5; i++ {
		referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)
		store.RecordReferral(referrer.ID, referred.ID, referred.ReferralCode)
	}

	count, err := store.GetTotalReferrals(referrer.ID)
	if err != nil {
		t.Fatalf("GetTotalReferrals() error = %v", err)
	}

	if count != 5 {
		t.Errorf("GetTotalReferrals() = %d, want 5", count)
	}
}

func TestGetConvertedReferrals(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")

	// Create 3 referred accounts, but only convert 2
	referred1, _ := store.CreateAccountWithReferral("referred1@example.com", referrer.ReferralCode)
	referred2, _ := store.CreateAccountWithReferral("referred2@example.com", referrer.ReferralCode)
	referred3, _ := store.CreateAccountWithReferral("referred3@example.com", referrer.ReferralCode)

	store.RecordReferral(referrer.ID, referred1.ID, referred1.ReferralCode)
	store.RecordReferral(referrer.ID, referred2.ID, referred2.ReferralCode)
	store.RecordReferral(referrer.ID, referred3.ID, referred3.ReferralCode)

	// Convert only 2
	store.MarkConverted(referred1.ID)
	store.MarkConverted(referred2.ID)

	count, err := store.GetConvertedReferrals(referrer.ID)
	if err != nil {
		t.Fatalf("GetConvertedReferrals() error = %v", err)
	}

	if count != 2 {
		t.Errorf("GetConvertedReferrals() = %d, want 2", count)
	}
}

func TestMarkConverted(t *testing.T) {
	store := NewStore("")

	account, _ := store.CreateAccount("test@example.com")

	err := store.MarkConverted(account.ID)
	if err != nil {
		t.Fatalf("MarkConverted() error = %v", err)
	}

	retrieved, _ := store.GetByID(account.ID)
	if retrieved.ConvertedAt == nil {
		t.Error("ConvertedAt should be set")
	}

	// Mark converted again (should be idempotent)
	store.MarkConverted(account.ID)
	retrieved, _ = store.GetByID(account.ID)
	firstConversion := *retrieved.ConvertedAt

	store.MarkConverted(account.ID)
	retrieved, _ = store.GetByID(account.ID)

	if *retrieved.ConvertedAt != firstConversion {
		t.Error("MarkConverted() should not change existing conversion time")
	}
}

func TestStorePersistence(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "accounts.json")

	// Create account and save
	store1 := NewStore(storePath)
	account1, err := store1.CreateAccount("test@example.com")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	err = store1.Save()
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load into new store
	store2 := NewStore(storePath)
	err = store2.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	account2, err := store2.GetByID(account1.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if account2.ID != account1.ID {
		t.Error("Account ID mismatch after persistence")
	}
	if account2.Email != account1.Email {
		t.Error("Email mismatch after persistence")
	}
	if account2.ReferralCode != account1.ReferralCode {
		t.Error("ReferralCode mismatch after persistence")
	}
}

func TestStoreAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "accounts.json")

	store := NewStore(storePath)
	account, _ := store.CreateAccount("test@example.com")

	// Save multiple times to ensure atomic writes work
	for i := 0; i < 5; i++ {
		err := store.Save()
		if err != nil {
			t.Fatalf("Save() iteration %d error = %v", i, err)
		}

		// Verify file exists
		if _, err := os.Stat(storePath); os.IsNotExist(err) {
			t.Fatalf("Store file does not exist after save iteration %d", i)
		}
	}

	// Load and verify
	store2 := NewStore(storePath)
	store2.Load()

	retrieved, _ := store2.GetByID(account.ID)
	if retrieved.ID != account.ID {
		t.Error("Account ID mismatch after atomic writes")
	}
}

func TestGetCurrentAccount(t *testing.T) {
	store := NewStore("")

	// No accounts
	account := store.GetCurrentAccount()
	if account != nil {
		t.Error("GetCurrentAccount() should return nil when no accounts")
	}

	// Add account
	store.CreateAccount("test@example.com")

	account = store.GetCurrentAccount()
	if account == nil {
		t.Error("GetCurrentAccount() should return account")
	}
}

func TestGetReferrer(t *testing.T) {
	store := NewStore("")

	referrer, _ := store.CreateAccount("referrer@example.com")
	referred, _ := store.CreateAccountWithReferral("referred@example.com", referrer.ReferralCode)

	// Get referrer
	retrievedReferrer, err := store.GetReferrer(referred.ID)
	if err != nil {
		t.Fatalf("GetReferrer() error = %v", err)
	}

	if retrievedReferrer.ID != referrer.ID {
		t.Error("Referrer ID mismatch")
	}

	// Try getting referrer for referrer (should return nil)
	nilReferrer, err := store.GetReferrer(referrer.ID)
	if err != nil {
		t.Fatalf("GetReferrer() error = %v", err)
	}
	if nilReferrer != nil {
		t.Error("GetReferrer() should return nil for account with no referrer")
	}
}
