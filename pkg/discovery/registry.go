package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

const (
	RegistryRepo        = "wgmesh-registry/public"
	RegistryAPI         = "https://api.github.com"
	RegistryMethod      = "registry"
	RegistryMaxPeers    = 50
	RegistryUpdateAge   = 24 * time.Hour
	RegistryRetryDelay  = 5 * time.Second
	RegistryMaxRetries  = 3
	RegistryHTTPTimeout = 15 * time.Second
)

// RegistryPeerEntry represents a peer entry stored in the registry
type RegistryPeerEntry struct {
	WGPubKey         string   `json:"wg_pubkey"`
	MeshIP           string   `json:"mesh_ip"`
	MeshIPv6         string   `json:"mesh_ipv6,omitempty"`
	Endpoint         string   `json:"endpoint"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
	Timestamp        int64    `json:"timestamp"`
}

// RendezvousRegistry implements GitHub Issue-based peer discovery
type RendezvousRegistry struct {
	SearchTerm string
	GossipKey  [32]byte
	IssueURL   string // Cached after first find/create
	issueNum   int

	client *http.Client
	mu     sync.Mutex
}

// NewRendezvousRegistry creates a new registry discovery instance
func NewRendezvousRegistry(keys *crypto.DerivedKeys) *RendezvousRegistry {
	return &RendezvousRegistry{
		SearchTerm: fmt.Sprintf("wgmesh-%x", keys.RendezvousID),
		GossipKey:  keys.GossipKey,
		client: &http.Client{
			Timeout: RegistryHTTPTimeout,
		},
	}
}

// FindOrCreate searches for an existing registry entry or creates one
func (r *RendezvousRegistry) FindOrCreate(myInfo *daemon.PeerInfo) ([]*daemon.PeerInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Search for existing meeting point (no auth required for search)
	peers, err := r.searchRegistry()
	if err != nil {
		log.Printf("[Registry] Search failed: %v", err)
	}

	// If no existing entry found, try to create one
	if r.issueNum == 0 {
		token := os.Getenv("GITHUB_TOKEN")
		if token != "" {
			if err := r.createIssue(myInfo, token); err != nil {
				log.Printf("[Registry] Failed to create registry entry: %v", err)
			}
		} else {
			log.Printf("[Registry] No GITHUB_TOKEN set, cannot create registry entry (search-only mode)")
		}
	} else {
		// Update existing entry — merge our info with existing peers
		token := os.Getenv("GITHUB_TOKEN")
		if token != "" {
			if err := r.updatePeerListMerged(myInfo, peers, token); err != nil {
				log.Printf("[Registry] Failed to update registry entry: %v", err)
			}
		}
	}

	return peers, nil
}

// searchRegistry searches GitHub Issues for the rendezvous point
func (r *RendezvousRegistry) searchRegistry() ([]*daemon.PeerInfo, error) {
	searchURL := fmt.Sprintf("%s/search/issues?q=%s+repo:%s+in:title",
		RegistryAPI, r.SearchTerm, RegistryRepo)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "wgmesh")

	// Use token if available for higher rate limits
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, nil
	}

	// Use the first matching issue
	issue := result.Items[0]
	r.issueNum = issue.Number
	r.IssueURL = fmt.Sprintf("%s/repos/%s/issues/%d", RegistryAPI, RegistryRepo, issue.Number)

	log.Printf("[Registry] Found registry entry: issue #%d", issue.Number)

	// Decrypt the peer list from the issue body
	return r.decryptPeerList(issue.Body), nil
}

// decryptPeerList decrypts the peer list from the issue body
func (r *RendezvousRegistry) decryptPeerList(body string) []*daemon.PeerInfo {
	// The body contains the encrypted peer list between markers
	const startMarker = "<!-- PEERS:"
	const endMarker = ":PEERS -->"

	startIdx := strings.Index(body, startMarker)
	endIdx := strings.LastIndex(body, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return nil
	}

	encryptedData := strings.TrimSpace(body[startIdx+len(startMarker) : endIdx])
	if encryptedData == "" {
		return nil
	}

	// Decrypt using gossip key
	_, announcement, err := crypto.OpenEnvelope([]byte(encryptedData), r.GossipKey)
	if err != nil {
		log.Printf("[Registry] Failed to decrypt peer list: %v", err)
		return nil
	}

	var peers []*daemon.PeerInfo

	// The announcement itself is a peer
	if announcement.WGPubKey != "" {
		peers = append(peers, &daemon.PeerInfo{
			WGPubKey:         announcement.WGPubKey,
			Hostname:         announcement.Hostname,
			MeshIP:           announcement.MeshIP,
			MeshIPv6:         announcement.MeshIPv6,
			Endpoint:         announcement.WGEndpoint,
			RoutableNetworks: announcement.RoutableNetworks,
			NATType:          announcement.NATType,
		})
	}

	// Known peers from the announcement
	for _, kp := range announcement.KnownPeers {
		peers = append(peers, &daemon.PeerInfo{
			WGPubKey: kp.WGPubKey,
			Hostname: kp.Hostname,
			MeshIP:   kp.MeshIP,
			MeshIPv6: kp.MeshIPv6,
			Endpoint: kp.WGEndpoint,
			NATType:  kp.NATType,
		})
	}

	log.Printf("[Registry] Decrypted %d peers from registry", len(peers))
	return peers
}

// createIssue creates a new registry issue
func (r *RendezvousRegistry) createIssue(myInfo *daemon.PeerInfo, token string) error {
	// Create encrypted body
	body, err := r.buildIssueBody([]*daemon.PeerInfo{myInfo})
	if err != nil {
		return fmt.Errorf("failed to build issue body: %w", err)
	}

	issue := map[string]string{
		"title": r.SearchTerm,
		"body":  body,
	}

	jsonData, err := json.Marshal(issue)
	if err != nil {
		return fmt.Errorf("failed to marshal issue: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/issues", RegistryAPI, RegistryRepo)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wgmesh")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("create issue request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create issue returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode create response: %w", err)
	}

	r.issueNum = result.Number
	r.IssueURL = fmt.Sprintf("%s/repos/%s/issues/%d", RegistryAPI, RegistryRepo, result.Number)

	log.Printf("[Registry] Created registry entry: issue #%d", result.Number)
	return nil
}

// updatePeerListMerged updates the registry, merging myInfo with existing peers
func (r *RendezvousRegistry) updatePeerListMerged(myInfo *daemon.PeerInfo, existingPeers []*daemon.PeerInfo, token string) error {
	if r.issueNum == 0 {
		return fmt.Errorf("no issue number set")
	}

	// Merge: start with myInfo, then add existing peers (deduplicate by pubkey)
	merged := []*daemon.PeerInfo{myInfo}
	seen := map[string]bool{myInfo.WGPubKey: true}
	for _, p := range existingPeers {
		if p.WGPubKey == "" || seen[p.WGPubKey] {
			continue
		}
		seen[p.WGPubKey] = true
		merged = append(merged, p)
		if len(merged) >= RegistryMaxPeers {
			break
		}
	}

	body, err := r.buildIssueBody(merged)
	if err != nil {
		return fmt.Errorf("failed to build issue body: %w", err)
	}

	update := map[string]string{
		"body": body,
	}

	jsonData, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/issues/%d", RegistryAPI, RegistryRepo, r.issueNum)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wgmesh")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("update request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update returned status %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[Registry] Updated registry entry: issue #%d", r.issueNum)
	return nil
}

// buildIssueBody creates the encrypted issue body
func (r *RendezvousRegistry) buildIssueBody(peers []*daemon.PeerInfo) (string, error) {
	if len(peers) == 0 {
		return "", fmt.Errorf("no peers to publish")
	}

	// Build known peers list (all but first)
	var knownPeers []crypto.KnownPeer
	for _, p := range peers[1:] {
		knownPeers = append(knownPeers, crypto.KnownPeer{
			WGPubKey:   p.WGPubKey,
			Hostname:   p.Hostname,
			MeshIP:     p.MeshIP,
			MeshIPv6:   p.MeshIPv6,
			WGEndpoint: p.Endpoint,
			Introducer: p.Introducer,
			NATType:    p.NATType,
		})
	}

	// Create announcement from the first peer
	first := peers[0]
	announcement := crypto.CreateAnnouncement(
		first.WGPubKey,
		first.MeshIP,
		first.Endpoint,
		first.Introducer,
		first.RoutableNetworks,
		knownPeers,
		first.Hostname,
		first.MeshIPv6,
		first.NATType,
	)

	encrypted, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcement, r.GossipKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt peer list: %w", err)
	}

	body := fmt.Sprintf("wgmesh registry rendezvous point\n\n<!-- PEERS:\n%s\n:PEERS -->", string(encrypted))
	return body, nil
}

// UpdatePeerListWithAll updates the registry with all known peers
func (r *RendezvousRegistry) UpdatePeerListWithAll(peers []*daemon.PeerInfo) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN not set")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.issueNum == 0 {
		return fmt.Errorf("no registry issue found")
	}

	body, err := r.buildIssueBody(peers)
	if err != nil {
		return fmt.Errorf("failed to build issue body: %w", err)
	}

	update := map[string]string{
		"body": body,
	}

	jsonData, err := json.Marshal(update)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/repos/%s/issues/%d", RegistryAPI, RegistryRepo, r.issueNum)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "wgmesh")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
