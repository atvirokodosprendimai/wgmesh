package lighthouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = fmt.Errorf("not found")

const (
	keyPrefixOrg  = "lh:org:"
	keyPrefixSite = "lh:site:"
	keyPrefixKey  = "lh:key:" // by key hash prefix
	keyPrefixEdge = "lh:edge:"
	keyIndexOrg   = "lh:idx:orgs"     // SET of org IDs
	keyIndexSites = "lh:idx:sites:"   // SET of site IDs per org (lh:idx:sites:<org_id>)
	keyIndexAll   = "lh:idx:allsites" // SET of all site IDs (for xDS snapshot)
	keyIndexKeys  = "lh:idx:keys:"    // SET of key IDs per org
	keyIndexEdges = "lh:idx:edges"    // SET of edge IDs
	keyDomainMap  = "lh:domain:"      // domain -> site_id lookup
)

// Store provides CRUD operations backed by Dragonfly/Redis.
// Each lighthouse node has its own local Dragonfly. The mesh gossip layer
// replicates mutations between nodes using SyncMessage with LWW semantics.
type Store struct {
	rdb    *redis.Client
	nodeID string // This lighthouse instance's unique ID

	mu        sync.RWMutex
	listeners []func(SyncMessage) // Called on local writes for gossip propagation
}

// NewStore connects to the local Dragonfly instance.
func NewStore(redisAddr, nodeID string) (*Store, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		DB:           1, // DB 1 for lighthouse (DB 0 used by chimney)
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		DialTimeout:  2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("dragonfly connection failed: %w", err)
	}

	return &Store{rdb: rdb, nodeID: nodeID}, nil
}

// OnWrite registers a callback for state changes (used by sync layer).
func (s *Store) OnWrite(fn func(SyncMessage)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *Store) notify(msg SyncMessage) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, fn := range s.listeners {
		fn(msg)
	}
}

// --- Orgs ---

func (s *Store) CreateOrg(ctx context.Context, org *Org) error {
	org.ID = GenerateID("org")
	org.CreatedAt = time.Now().UTC()
	org.UpdatedAt = org.CreatedAt

	data, err := json.Marshal(org)
	if err != nil {
		return fmt.Errorf("marshal org: %w", err)
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefixOrg+org.ID, data, 0)
	pipe.SAdd(ctx, keyIndexOrg, org.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store org: %w", err)
	}

	s.notify(SyncMessage{
		Type:      "org",
		Action:    "upsert",
		Payload:   data,
		Version:   1,
		NodeID:    s.nodeID,
		Timestamp: org.CreatedAt,
	})
	return nil
}

func (s *Store) GetOrg(ctx context.Context, orgID string) (*Org, error) {
	data, err := s.rdb.Get(ctx, keyPrefixOrg+orgID).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("org not found: %s", orgID)
	}
	if err != nil {
		return nil, fmt.Errorf("get org: %w", err)
	}
	var org Org
	if err := json.Unmarshal(data, &org); err != nil {
		return nil, fmt.Errorf("unmarshal org: %w", err)
	}
	return &org, nil
}

func (s *Store) ListOrgs(ctx context.Context) ([]Org, error) {
	ids, err := s.rdb.SMembers(ctx, keyIndexOrg).Result()
	if err != nil {
		return nil, fmt.Errorf("list org IDs: %w", err)
	}
	orgs := make([]Org, 0, len(ids))
	for _, id := range ids {
		org, err := s.GetOrg(ctx, id)
		if err != nil {
			continue
		}
		orgs = append(orgs, *org)
	}
	return orgs, nil
}

// --- API Keys ---

func (s *Store) StoreAPIKey(ctx context.Context, key *APIKey) error {
	data, err := json.Marshal(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefixKey+key.Prefix, data, 0)
	pipe.SAdd(ctx, keyIndexKeys+key.OrgID, key.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store key: %w", err)
	}

	s.notify(SyncMessage{
		Type:      "key",
		Action:    "upsert",
		Payload:   data,
		Version:   1,
		NodeID:    s.nodeID,
		Timestamp: key.CreatedAt,
	})
	return nil
}

// LookupAPIKey finds a key by its prefix (first 8 chars of the raw key).
func (s *Store) LookupAPIKey(ctx context.Context, prefix string) (*APIKey, error) {
	data, err := s.rdb.Get(ctx, keyPrefixKey+prefix).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("key %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get key: %w", err)
	}
	var key APIKey
	if err := json.Unmarshal(data, &key); err != nil {
		return nil, fmt.Errorf("unmarshal key: %w", err)
	}
	return &key, nil
}

func (s *Store) TouchAPIKey(ctx context.Context, prefix string) error {
	key, err := s.LookupAPIKey(ctx, prefix)
	if err != nil {
		return err
	}
	key.LastUsedAt = time.Now().UTC()
	data, err := json.Marshal(key)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, keyPrefixKey+prefix, data, 0).Err()
}

func (s *Store) DeleteAPIKey(ctx context.Context, prefix, orgID string) error {
	// Look up the key first to get the ID (index stores key.ID, not prefix)
	key, err := s.LookupAPIKey(ctx, prefix)
	if err != nil {
		return err
	}
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, keyPrefixKey+prefix)
	pipe.SRem(ctx, keyIndexKeys+orgID, key.ID)
	_, err = pipe.Exec(ctx)
	return err
}

// --- Sites ---

func (s *Store) CreateSite(ctx context.Context, site *Site) error {
	site.ID = GenerateID("site")
	site.CreatedAt = time.Now().UTC()
	site.UpdatedAt = site.CreatedAt
	site.Version = 1
	site.NodeID = s.nodeID

	data, err := json.Marshal(site)
	if err != nil {
		return fmt.Errorf("marshal site: %w", err)
	}

	// Atomic domain uniqueness check using SetNX (set-if-not-exists)
	set, err := s.rdb.SetNX(ctx, keyDomainMap+site.Domain, site.ID, 0).Result()
	if err != nil {
		return fmt.Errorf("check domain: %w", err)
	}
	if !set {
		return fmt.Errorf("domain already registered: %s", site.Domain)
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefixSite+site.ID, data, 0)
	pipe.SAdd(ctx, keyIndexSites+site.OrgID, site.ID)
	pipe.SAdd(ctx, keyIndexAll, site.ID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("store site: %w", err)
	}

	s.notify(SyncMessage{
		Type:      "site",
		Action:    "upsert",
		Payload:   data,
		Version:   site.Version,
		NodeID:    s.nodeID,
		Timestamp: site.UpdatedAt,
	})
	return nil
}

func (s *Store) GetSite(ctx context.Context, siteID string) (*Site, error) {
	data, err := s.rdb.Get(ctx, keyPrefixSite+siteID).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("site not found: %s", siteID)
	}
	if err != nil {
		return nil, fmt.Errorf("get site: %w", err)
	}
	var site Site
	if err := json.Unmarshal(data, &site); err != nil {
		return nil, fmt.Errorf("unmarshal site: %w", err)
	}
	return &site, nil
}

func (s *Store) GetSiteByDomain(ctx context.Context, domain string) (*Site, error) {
	siteID, err := s.rdb.Get(ctx, keyDomainMap+domain).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup domain: %w", err)
	}
	return s.GetSite(ctx, siteID)
}

func (s *Store) ListSites(ctx context.Context, orgID string) ([]Site, error) {
	ids, err := s.rdb.SMembers(ctx, keyIndexSites+orgID).Result()
	if err != nil {
		return nil, fmt.Errorf("list site IDs: %w", err)
	}
	sites := make([]Site, 0, len(ids))
	for _, id := range ids {
		site, err := s.GetSite(ctx, id)
		if err != nil {
			continue
		}
		sites = append(sites, *site)
	}
	return sites, nil
}

func (s *Store) ListSitesByStatus(ctx context.Context, status SiteStatus) ([]Site, error) {
	ids, err := s.rdb.SMembers(ctx, keyIndexAll).Result()
	if err != nil {
		return nil, fmt.Errorf("list all site IDs: %w", err)
	}
	sites := make([]Site, 0)
	for _, id := range ids {
		site, err := s.GetSite(ctx, id)
		if err != nil {
			continue
		}
		if site.Status == status {
			sites = append(sites, *site)
		}
	}
	return sites, nil
}

func (s *Store) ListAllSites(ctx context.Context) ([]Site, error) {
	ids, err := s.rdb.SMembers(ctx, keyIndexAll).Result()
	if err != nil {
		return nil, fmt.Errorf("list all site IDs: %w", err)
	}
	sites := make([]Site, 0, len(ids))
	for _, id := range ids {
		site, err := s.GetSite(ctx, id)
		if err != nil {
			continue
		}
		if site.Status != SiteStatusDeleted {
			sites = append(sites, *site)
		}
	}
	return sites, nil
}

func (s *Store) UpdateSite(ctx context.Context, site *Site) error {
	site.UpdatedAt = time.Now().UTC()
	site.Version++
	site.NodeID = s.nodeID

	data, err := json.Marshal(site)
	if err != nil {
		return fmt.Errorf("marshal site: %w", err)
	}

	if err := s.rdb.Set(ctx, keyPrefixSite+site.ID, data, 0).Err(); err != nil {
		return fmt.Errorf("update site: %w", err)
	}

	s.notify(SyncMessage{
		Type:      "site",
		Action:    "upsert",
		Payload:   data,
		Version:   site.Version,
		NodeID:    s.nodeID,
		Timestamp: site.UpdatedAt,
	})
	return nil
}

func (s *Store) DeleteSite(ctx context.Context, siteID, orgID string) error {
	site, err := s.GetSite(ctx, siteID)
	if err != nil {
		return err
	}

	site.Status = SiteStatusDeleted
	site.UpdatedAt = time.Now().UTC()
	site.Version++
	site.NodeID = s.nodeID

	data, err := json.Marshal(site)
	if err != nil {
		return fmt.Errorf("marshal site: %w", err)
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefixSite+siteID, data, 0) // Tombstone — keep for sync
	pipe.SRem(ctx, keyIndexSites+orgID, siteID)
	pipe.SRem(ctx, keyIndexAll, siteID)
	pipe.Del(ctx, keyDomainMap+site.Domain)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("delete site: %w", err)
	}

	s.notify(SyncMessage{
		Type:      "site",
		Action:    "delete",
		Payload:   data,
		Version:   site.Version,
		NodeID:    s.nodeID,
		Timestamp: site.UpdatedAt,
	})
	return nil
}

// --- Edges ---

func (s *Store) UpsertEdge(ctx context.Context, edge *Edge) error {
	data, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("marshal edge: %w", err)
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefixEdge+edge.ID, data, 0)
	pipe.SAdd(ctx, keyIndexEdges, edge.ID)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) GetEdge(ctx context.Context, edgeID string) (*Edge, error) {
	data, err := s.rdb.Get(ctx, keyPrefixEdge+edgeID).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("edge not found: %s", edgeID)
	}
	if err != nil {
		return nil, fmt.Errorf("get edge: %w", err)
	}
	var edge Edge
	if err := json.Unmarshal(data, &edge); err != nil {
		return nil, fmt.Errorf("unmarshal edge: %w", err)
	}
	return &edge, nil
}

func (s *Store) ListEdges(ctx context.Context) ([]Edge, error) {
	ids, err := s.rdb.SMembers(ctx, keyIndexEdges).Result()
	if err != nil {
		return nil, fmt.Errorf("list edge IDs: %w", err)
	}
	edges := make([]Edge, 0, len(ids))
	for _, id := range ids {
		edge, err := s.GetEdge(ctx, id)
		if err != nil {
			continue
		}
		edges = append(edges, *edge)
	}
	return edges, nil
}

// --- Sync (LWW merge) ---

// ApplySync merges a remote SyncMessage using last-writer-wins semantics.
// Returns true if the remote version was applied (newer), false if local wins.
func (s *Store) ApplySync(ctx context.Context, msg SyncMessage) (bool, error) {
	switch msg.Type {
	case "site":
		return s.applySyncSite(ctx, msg)
	case "org":
		return s.applySyncOrg(ctx, msg)
	case "key":
		return s.applySyncKey(ctx, msg)
	default:
		return false, fmt.Errorf("unknown sync type: %s", msg.Type)
	}
}

func (s *Store) applySyncSite(ctx context.Context, msg SyncMessage) (bool, error) {
	var remote Site
	if err := json.Unmarshal(msg.Payload, &remote); err != nil {
		return false, fmt.Errorf("unmarshal sync site: %w", err)
	}

	// Check if we have a local version
	local, err := s.GetSite(ctx, remote.ID)
	if err != nil {
		// No local copy — accept remote
		return true, s.applySiteUpsert(ctx, &remote)
	}

	// LWW: higher version wins. Tie-break on timestamp, then nodeID.
	if remote.Version > local.Version ||
		(remote.Version == local.Version && remote.UpdatedAt.After(local.UpdatedAt)) ||
		(remote.Version == local.Version && remote.UpdatedAt.Equal(local.UpdatedAt) && remote.NodeID > local.NodeID) {
		return true, s.applySiteUpsert(ctx, &remote)
	}

	return false, nil // Local wins
}

func (s *Store) applySiteUpsert(ctx context.Context, site *Site) error {
	data, err := json.Marshal(site)
	if err != nil {
		return err
	}

	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, keyPrefixSite+site.ID, data, 0)

	if site.Status == SiteStatusDeleted {
		pipe.SRem(ctx, keyIndexSites+site.OrgID, site.ID)
		pipe.SRem(ctx, keyIndexAll, site.ID)
		pipe.Del(ctx, keyDomainMap+site.Domain)
	} else {
		pipe.SAdd(ctx, keyIndexSites+site.OrgID, site.ID)
		pipe.SAdd(ctx, keyIndexAll, site.ID)
		pipe.Set(ctx, keyDomainMap+site.Domain, site.ID, 0)
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (s *Store) applySyncOrg(ctx context.Context, msg SyncMessage) (bool, error) {
	var remote Org
	if err := json.Unmarshal(msg.Payload, &remote); err != nil {
		return false, err
	}

	// Orgs are append-only (no versioning needed — latest timestamp wins)
	local, err := s.GetOrg(ctx, remote.ID)
	if err != nil || remote.UpdatedAt.After(local.UpdatedAt) {
		data, _ := json.Marshal(remote)
		pipe := s.rdb.Pipeline()
		pipe.Set(ctx, keyPrefixOrg+remote.ID, data, 0)
		pipe.SAdd(ctx, keyIndexOrg, remote.ID)
		_, err := pipe.Exec(ctx)
		return err == nil, err
	}
	return false, nil
}

func (s *Store) applySyncKey(ctx context.Context, msg SyncMessage) (bool, error) {
	var remote APIKey
	if err := json.Unmarshal(msg.Payload, &remote); err != nil {
		return false, err
	}

	// Keys are append-only — if we don't have it, accept it
	_, err := s.LookupAPIKey(ctx, remote.Prefix)
	if err != nil && errors.Is(err, ErrNotFound) {
		data, _ := json.Marshal(remote)
		pipe := s.rdb.Pipeline()
		pipe.Set(ctx, keyPrefixKey+remote.Prefix, data, 0)
		pipe.SAdd(ctx, keyIndexKeys+remote.OrgID, remote.ID)
		_, err := pipe.Exec(ctx)
		return err == nil, err
	}
	return false, nil
}

// Close shuts down the store connection.
func (s *Store) Close() error {
	return s.rdb.Close()
}
