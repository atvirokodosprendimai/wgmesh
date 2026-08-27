package promo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store manages promo codes, campaigns, and redemptions.
// Uses file-based storage with atomic writes for crash safety.
type Store struct {
	mu          sync.RWMutex
	path        string // Path to storage file
	campaigns   map[string]*Campaign
	promos      map[Code]*Promo
	redemptions map[string]*Redemption // Key: code string
}

// StoreConfig holds configuration for the promo store.
type StoreConfig struct {
	StoragePath string // Path to store file (default: ~/.wgmesh/promos.json)
}

// NewStore creates a new promo store.
func NewStore(cfg StoreConfig) (*Store, error) {
	path := cfg.StoragePath
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		path = filepath.Join(homeDir, ".wgmesh", "promos.json")
	}

	s := &Store{
		path:        path,
		campaigns:   make(map[string]*Campaign),
		promos:      make(map[Code]*Promo),
		redemptions: make(map[string]*Redemption),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// storeData represents the JSON serialization format.
type storeData struct {
	Campaigns   []*Campaign   `json:"campaigns"`
	Promos      []*Promo      `json:"promos"`
	Redemptions []*Redemption `json:"redemptions"`
}

// load reads store from disk.
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // First run, no data yet
		}
		return fmt.Errorf("read promo store: %w", err)
	}

	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("parse promo store: %w", err)
	}

	// Restore maps
	s.campaigns = make(map[string]*Campaign)
	for _, c := range sd.Campaigns {
		s.campaigns[c.ID] = c
	}

	s.promos = make(map[Code]*Promo)
	for _, p := range sd.Promos {
		s.promos[p.Code] = p
	}

	s.redemptions = make(map[string]*Redemption)
	for _, r := range sd.Redemptions {
		s.redemptions[string(r.Code)] = r
	}

	return nil
}

// save writes store to disk atomically.
func (s *Store) save() error {
	// Build slice for serialization
	campaigns := make([]*Campaign, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		campaigns = append(campaigns, c)
	}

	promos := make([]*Promo, 0, len(s.promos))
	for _, p := range s.promos {
		promos = append(promos, p)
	}

	redemptions := make([]*Redemption, 0, len(s.redemptions))
	for _, r := range s.redemptions {
		redemptions = append(redemptions, r)
	}

	sd := storeData{
		Campaigns:   campaigns,
		Promos:      promos,
		Redemptions: redemptions,
	}

	data, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal promo store: %w", err)
	}

	// Write atomically (temp file + rename)
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// CreateCampaign creates a new promotional campaign.
func (s *Store) CreateCampaign(id, name string, source Source, trialDays, nodeLimit int, expiresAt time.Time) (*Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.campaigns[id]; exists {
		return nil, fmt.Errorf("campaign already exists: %s", id)
	}

	if name == "" {
		return nil, fmt.Errorf("campaign name cannot be empty")
	}
	if trialDays <= 0 {
		return nil, fmt.Errorf("trial days must be positive")
	}
	if nodeLimit <= 0 {
		return nil, fmt.Errorf("node limit must be positive")
	}

	campaign := &Campaign{
		ID:        id,
		Name:      name,
		Source:    source,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		TrialDays: trialDays,
		NodeLimit: nodeLimit,
	}

	s.campaigns[id] = campaign
	if err := s.save(); err != nil {
		delete(s.campaigns, id)
		return nil, err
	}

	return campaign, nil
}

// GetCampaign retrieves a campaign by ID.
func (s *Store) GetCampaign(id string) (*Campaign, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	campaign, exists := s.campaigns[id]
	if !exists {
		return nil, fmt.Errorf("campaign not found: %s", id)
	}

	return campaign, nil
}

// GeneratePromo creates a new promo code for a campaign.
func (s *Store) GeneratePromo(campaignID string, seed string) (Code, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	campaign, exists := s.campaigns[campaignID]
	if !exists {
		return "", fmt.Errorf("campaign not found: %s", campaignID)
	}

	// Check campaign expiry
	if !campaign.ExpiresAt.IsZero() && time.Now().After(campaign.ExpiresAt) {
		return "", fmt.Errorf("campaign expired")
	}

	code, err := GenerateCode(campaignID, seed)
	if err != nil {
		return "", err
	}

	// Check for collision (extremely rare)
	if _, exists := s.promos[code]; exists {
		return "", fmt.Errorf("code collision (retry with different seed)")
	}

	promo := &Promo{
		Code:       code,
		CampaignID: campaignID,
		Redeemed:   false,
		CreatedAt:  time.Now(),
	}

	s.promos[code] = promo
	if err := s.save(); err != nil {
		delete(s.promos, code)
		return "", err
	}

	return code, nil
}

// RedeemCode redeems a promo code.
// Returns the campaign details and trial expiry.
func (s *Store) RedeemCode(code Code, accountID string) (*Campaign, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate checksum
	if !code.IsValid() {
		return nil, time.Time{}, fmt.Errorf("invalid promo code")
	}

	promo, exists := s.promos[code]
	if !exists {
		return nil, time.Time{}, fmt.Errorf("promo code not found")
	}

	if promo.Redeemed {
		return nil, time.Time{}, fmt.Errorf("promo code already redeemed")
	}

	campaign, exists := s.campaigns[promo.CampaignID]
	if !exists {
		return nil, time.Time{}, fmt.Errorf("campaign not found")
	}

	// Check campaign expiry
	if !campaign.ExpiresAt.IsZero() && time.Now().After(campaign.ExpiresAt) {
		return nil, time.Time{}, fmt.Errorf("campaign expired")
	}

	// Mark as redeemed
	promo.Redeemed = true
	promo.RedeemedAt = time.Now()

	// Record redemption
	trialEndsAt := time.Now().AddDate(0, 0, campaign.TrialDays)
	redemption := &Redemption{
		Code:        code,
		CampaignID:  promo.CampaignID,
		AccountID:   accountID,
		RedeemedAt:  time.Now(),
		TrialEndsAt: trialEndsAt,
	}
	s.redemptions[string(code)] = redemption

	if err := s.save(); err != nil {
		// Rollback
		promo.Redeemed = false
		promo.RedeemedAt = time.Time{}
		delete(s.redemptions, string(code))
		return nil, time.Time{}, err
	}

	return campaign, trialEndsAt, nil
}

// GetCampaignStats returns statistics for a campaign.
func (s *Store) GetCampaignStats(campaignID string) (generated, redeemed int, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, promo := range s.promos {
		if promo.CampaignID == campaignID {
			generated++
			if promo.Redeemed {
				redeemed++
			}
		}
	}

	return generated, redeemed, nil
}

// ListCampaigns returns all campaigns.
func (s *Store) ListCampaigns() []*Campaign {
	s.mu.RLock()
	defer s.mu.RUnlock()

	campaigns := make([]*Campaign, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		campaigns = append(campaigns, c)
	}

	return campaigns
}

// GetRedemptions returns all redemptions for a campaign.
func (s *Store) GetRedemptions(campaignID string) []*Redemption {
	s.mu.RLock()
	defer s.mu.RUnlock()

	redemptions := make([]*Redemption, 0)
	for _, r := range s.redemptions {
		if r.CampaignID == campaignID {
			redemptions = append(redemptions, r)
		}
	}

	return redemptions
}
