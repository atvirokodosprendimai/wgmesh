// Package lighthouse implements the cloudroof.eu CDN control plane.
//
// The control plane is federated: every lighthouse instance can accept API
// writes and syncs state to peers via the WireGuard mesh. There is no single
// point of failure — any lighthouse can serve the full API and push xDS
// snapshots to connected Envoy edges.
//
// The single shared truth is the mesh secret ("one universe"). All lighthouse
// instances in the same mesh see the same route state via CRDT-style
// last-writer-wins replication.
package lighthouse

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Org represents a customer organization. API keys are scoped to orgs.
type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIKey authenticates API requests. Scoped to a single org.
type APIKey struct {
	ID         string    `json:"id"`
	OrgID      string    `json:"org_id"`
	KeyHash    string    `json:"-"`      // bcrypt hash, never exposed
	Prefix     string    `json:"prefix"` // first 8 chars for identification
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
}

// TLSMode controls how TLS is handled for a site.
type TLSMode string

const (
	TLSModeAuto   TLSMode = "auto"   // Automatic Let's Encrypt via edge Caddy
	TLSModeCustom TLSMode = "custom" // Customer provides cert
	TLSModeOff    TLSMode = "off"    // HTTP only (not recommended)
)

// SiteStatus tracks the lifecycle of a site registration.
type SiteStatus string

const (
	SiteStatusPendingDNS    SiteStatus = "pending_dns"
	SiteStatusPendingVerify SiteStatus = "pending_verify"
	SiteStatusActive        SiteStatus = "active"
	SiteStatusSuspended     SiteStatus = "suspended"
	SiteStatusDeleted       SiteStatus = "deleted"
)

// Origin defines where traffic should be proxied to.
type Origin struct {
	MeshIP   string `json:"mesh_ip"`  // WireGuard mesh IP of the origin node
	Port     int    `json:"port"`     // Port on the origin
	Protocol string `json:"protocol"` // "http" or "https" (to origin)
}

// Site represents a customer domain routed through the CDN.
type Site struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"org_id"`
	Domain    string     `json:"domain"`
	Origin    Origin     `json:"origin"`
	TLS       TLSMode    `json:"tls"`
	Status    SiteStatus `json:"status"`
	DNSTarget string     `json:"dns_target"` // Where the customer should point DNS
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Version is a logical clock for CRDT last-writer-wins replication.
	// Each mutation increments this. During sync, highest version wins.
	Version int64 `json:"version"`

	// NodeID identifies which lighthouse instance last wrote this record.
	NodeID string `json:"node_id"`
}

// Edge represents an Envoy edge node connected to the control plane.
type Edge struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Location      string    `json:"location"`  // Hetzner location code
	MeshIP        string    `json:"mesh_ip"`   // WireGuard mesh IP
	PublicIP      string    `json:"public_ip"` // Public-facing IP for DNS
	Status        string    `json:"status"`    // "connected", "disconnected"
	LastHeartbeat time.Time `json:"last_heartbeat"`
	SiteCount     int       `json:"site_count"`
	EnvoyVersion  string    `json:"envoy_version,omitempty"`
}

// SyncMessage is sent between lighthouse instances to replicate state.
type SyncMessage struct {
	Type      string    `json:"type"`      // "site", "org", "key"
	Action    string    `json:"action"`    // "upsert", "delete"
	Payload   []byte    `json:"payload"`   // JSON-encoded Site/Org/APIKey
	Version   int64     `json:"version"`   // Logical clock
	NodeID    string    `json:"node_id"`   // Originating lighthouse
	Timestamp time.Time `json:"timestamp"` // Wall clock (tie-breaker)
}

// --- ID generation ---

// GenerateID creates a prefixed random ID (e.g., "org_a1b2c3d4e5f6").
func GenerateID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(b)
}

// GenerateAPIKey creates a raw API key with "cr_" prefix.
// Returns the full key (to give to the customer) — only the hash is stored.
func GenerateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return "cr_" + hex.EncodeToString(b)
}
