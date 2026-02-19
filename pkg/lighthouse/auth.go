package lighthouse

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Auth handles API key authentication and org-scoped authorization.
type Auth struct {
	store *Store
}

// NewAuth creates a new Auth service.
func NewAuth(store *Store) *Auth {
	return &Auth{store: store}
}

// HashKey produces a SHA-256 hash of the raw API key for storage.
// We use SHA-256 (not bcrypt) because API keys are high-entropy random
// strings, not user-chosen passwords. SHA-256 is sufficient and fast.
func HashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

// PrefixFromKey extracts the first 8 chars after "cr_" for lookup.
func PrefixFromKey(rawKey string) string {
	stripped := strings.TrimPrefix(rawKey, "cr_")
	if len(stripped) < 8 {
		return stripped
	}
	return stripped[:8]
}

// Authenticate validates a Bearer token and returns the associated org ID.
// Returns the org ID and nil error on success.
func (a *Auth) Authenticate(ctx context.Context, r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("invalid Authorization scheme (expected Bearer)")
	}

	rawKey := strings.TrimPrefix(authHeader, "Bearer ")
	if !strings.HasPrefix(rawKey, "cr_") {
		return "", fmt.Errorf("invalid API key format")
	}

	prefix := PrefixFromKey(rawKey)
	key, err := a.store.LookupAPIKey(ctx, prefix)
	if err != nil {
		return "", fmt.Errorf("invalid API key")
	}

	// Constant-time comparison of key hashes
	providedHash := HashKey(rawKey)
	if subtle.ConstantTimeCompare([]byte(providedHash), []byte(key.KeyHash)) != 1 {
		return "", fmt.Errorf("invalid API key")
	}

	// Update last-used timestamp asynchronously (best-effort)
	go func() {
		touchCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = a.store.TouchAPIKey(touchCtx, prefix)
	}()

	return key.OrgID, nil
}

// CreateOrgWithKey creates a new org and its first API key.
// Returns the org and the raw API key (only shown once).
func (a *Auth) CreateOrgWithKey(ctx context.Context, name string) (*Org, string, error) {
	org := &Org{Name: name}
	if err := a.store.CreateOrg(ctx, org); err != nil {
		return nil, "", fmt.Errorf("create org: %w", err)
	}

	rawKey := GenerateAPIKey()
	apiKey := &APIKey{
		ID:        GenerateID("key"),
		OrgID:     org.ID,
		KeyHash:   HashKey(rawKey),
		Prefix:    PrefixFromKey(rawKey),
		CreatedAt: time.Now().UTC(),
	}

	if err := a.store.StoreAPIKey(ctx, apiKey); err != nil {
		return nil, "", fmt.Errorf("store key: %w", err)
	}

	return org, rawKey, nil
}

// CreateAdditionalKey generates a new API key for an existing org.
func (a *Auth) CreateAdditionalKey(ctx context.Context, orgID string) (string, *APIKey, error) {
	// Verify org exists
	if _, err := a.store.GetOrg(ctx, orgID); err != nil {
		return "", nil, err
	}

	rawKey := GenerateAPIKey()
	apiKey := &APIKey{
		ID:        GenerateID("key"),
		OrgID:     orgID,
		KeyHash:   HashKey(rawKey),
		Prefix:    PrefixFromKey(rawKey),
		CreatedAt: time.Now().UTC(),
	}

	if err := a.store.StoreAPIKey(ctx, apiKey); err != nil {
		return "", nil, fmt.Errorf("store key: %w", err)
	}

	return rawKey, apiKey, nil
}
