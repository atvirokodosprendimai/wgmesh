package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultStorePath is the default path for the account store
	DefaultStorePath = ".wgmesh/accounts.json"
)

// Store manages account and referral persistence
type Store struct {
	mu        sync.RWMutex
	path      string
	accounts  map[AccountID]*Account
	referrals []*Referral
}

// storeData represents the JSON structure for persistence
type storeData struct {
	Accounts  map[string]*Account `json:"accounts"`
	Referrals []*Referral         `json:"referrals"`
}

// NewStore creates a new account store
func NewStore(path string) *Store {
	if path == "" {
		path = DefaultStorePath
	}
	return &Store{
		path:      path,
		accounts:  make(map[AccountID]*Account),
		referrals: make([]*Referral, 0),
	}
}

// Load loads the store from disk
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			// First run, no data yet
			return nil
		}
		return fmt.Errorf("reading store: %w", err)
	}

	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("unmarshaling store: %w", err)
	}

	// Convert map[string]*Account to map[AccountID]*Account
	s.accounts = make(map[AccountID]*Account)
	for id, acc := range sd.Accounts {
		s.accounts[AccountID(id)] = acc
	}
	s.referrals = sd.Referrals

	return nil
}

// Save writes the store to disk atomically
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Convert to JSON-serializable format
	sd := storeData{
		Accounts:  make(map[string]*Account),
		Referrals: s.referrals,
	}
	for id, acc := range s.accounts {
		sd.Accounts[string(id)] = acc
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling store: %w", err)
	}

	// Write to temp file
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath) // Clean up
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

// CreateAccount generates a new account with referral code
func (s *Store) CreateAccount(email string) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate unique account ID
	accountID := AccountID(uuid.New().String())

	// Generate referral code
	code, err := GenerateCode(accountID)
	if err != nil {
		return nil, fmt.Errorf("generating referral code: %w", err)
	}

	account := &Account{
		ID:           accountID,
		Email:        email,
		ReferralCode: code,
		CreatedAt:    time.Now().UTC(),
	}

	s.accounts[accountID] = account

	return account, nil
}

// CreateAccountWithReferral creates a new account using a referral code
func (s *Store) CreateAccountWithReferral(email string, referralCode ReferralCode) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate referral code format
	if _, err := ValidateCode(referralCode); err != nil {
		return nil, fmt.Errorf("invalid referral code: %w", err)
	}

	// Find referrer by code
	var referrerID AccountID
	for _, acc := range s.accounts {
		if acc.ReferralCode == referralCode {
			referrerID = acc.ID
			break
		}
	}

	if referrerID == "" {
		return nil, fmt.Errorf("referral code not found")
	}

	// Generate unique account ID
	accountID := AccountID(uuid.New().String())

	// Generate referral code for the new account
	code, err := GenerateCode(accountID)
	if err != nil {
		return nil, fmt.Errorf("generating referral code: %w", err)
	}

	account := &Account{
		ID:           accountID,
		Email:        email,
		ReferralCode: code,
		CreatedAt:    time.Now().UTC(),
		ReferredBy:   referrerID,
	}

	s.accounts[accountID] = account

	return account, nil
}

// GetByID retrieves an account by ID
func (s *Store) GetByID(id AccountID) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, ok := s.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	return account, nil
}

// GetByCode retrieves an account by referral code
func (s *Store) GetByCode(code ReferralCode) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, acc := range s.accounts {
		if acc.ReferralCode == code {
			return acc, nil
		}
	}
	return nil, fmt.Errorf("referral code not found")
}

// GetReferrer retrieves the referrer account for a given account
func (s *Store) GetReferrer(accountID AccountID) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}

	if account.ReferredBy == "" {
		return nil, nil // No referrer
	}

	referrer, ok := s.accounts[account.ReferredBy]
	if !ok {
		return nil, fmt.Errorf("referrer account not found")
	}

	return referrer, nil
}

// RecordReferral records a successful referral conversion
func (s *Store) RecordReferral(referrerID, referredID AccountID, code ReferralCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify both accounts exist
	_, ok := s.accounts[referrerID]
	if !ok {
		return fmt.Errorf("referrer account not found")
	}
	_, ok = s.accounts[referredID]
	if !ok {
		return fmt.Errorf("referred account not found")
	}

	// Check if already recorded
	for _, r := range s.referrals {
		if r.ReferrerID == referrerID && r.ReferredID == referredID {
			return nil // Already recorded
		}
	}

	// Record the referral
	referral := &Referral{
		ReferrerID:  referrerID,
		ReferredID:  referredID,
		Code:        code,
		ConvertedAt: time.Now().UTC(),
	}
	s.referrals = append(s.referrals, referral)

	// Note: The referred account's conversion status is managed separately
	// via MarkConverted() when they complete their first mesh setup

	return nil
}

// GetReferrals retrieves all referrals for a given referrer
func (s *Store) GetReferrals(referrerID AccountID) ([]*Referral, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Referral
	for _, r := range s.referrals {
		if r.ReferrerID == referrerID {
			result = append(result, r)
		}
	}
	return result, nil
}

// GetTotalReferrals returns the total number of referrals for an account
func (s *Store) GetTotalReferrals(referrerID AccountID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, r := range s.referrals {
		if r.ReferrerID == referrerID {
			count++
		}
	}
	return count, nil
}

// GetConvertedReferrals returns the number of referrals that completed mesh setup
func (s *Store) GetConvertedReferrals(referrerID AccountID) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, r := range s.referrals {
		if r.ReferrerID == referrerID {
			// Check if referred account has converted
			if referred, ok := s.accounts[r.ReferredID]; ok && referred.ConvertedAt != nil {
				count++
			}
		}
	}
	return count, nil
}

// GetAllAccounts returns all accounts
func (s *Store) GetAllAccounts() []*Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accounts := make([]*Account, 0, len(s.accounts))
	for _, acc := range s.accounts {
		accounts = append(accounts, acc)
	}
	return accounts
}

// GetCurrentAccount returns the first account (for single-account systems)
func (s *Store) GetCurrentAccount() *Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, acc := range s.accounts {
		return acc
	}
	return nil
}

// MarkConverted marks an account as having completed first mesh setup
func (s *Store) MarkConverted(accountID AccountID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return fmt.Errorf("account not found")
	}

	if account.ConvertedAt == nil {
		now := time.Now().UTC()
		account.ConvertedAt = &now
	}

	return nil
}
