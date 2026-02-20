// lighthouse is the cloudroof.eu CDN control plane.
//
// It provides a REST API for managing CDN routes (sites, orgs, API keys)
// and generates Envoy/Caddy configuration for edge proxy nodes.
//
// Lighthouse is federated: every instance stores state in local Dragonfly
// and replicates via the WireGuard mesh. No single point of failure.
// The mesh network IS the distributed database.
//
// Usage:
//
//	lighthouse -addr :9090 -redis 127.0.0.1:6379 -mesh-ip 10.77.1.1
//	lighthouse -addr :9090 -redis 127.0.0.1:6379 -mesh-ip 10.77.1.1 -peer 10.77.1.2 -peer 10.77.1.3
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/atvirokodosprendimai/wgmesh/pkg/lighthouse"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ratelimit"
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	addr := flag.String("addr", ":9090", "API listen address")
	redisAddr := flag.String("redis", "127.0.0.1:6379", "Dragonfly/Redis address")
	meshIP := flag.String("mesh-ip", "", "This node's WireGuard mesh IP")
	nodeID := flag.String("node-id", "", "Unique node ID (auto-generated if empty)")
	dnsTarget := flag.String("dns-target", "edge.cloudroof.eu", "DNS target for customer CNAME")
	rateLimitRPS := flag.Float64("rate-limit-rps", 100, "Rate limit: requests per second per API key (0 to disable)")
	rateLimitBurst := flag.Int("rate-limit-burst", 200, "Rate limit: burst size per API key")

	var peers stringSlice
	flag.Var(&peers, "peer", "Mesh IP of another lighthouse instance (repeatable)")
	flag.Parse()

	// Node ID defaults to hostname
	nid := *nodeID
	if nid == "" {
		h, err := os.Hostname()
		if err != nil {
			h = "lighthouse-unknown"
		}
		nid = h
	}

	// Connect to Dragonfly
	store, err := lighthouse.NewStore(*redisAddr, nid)
	if err != nil {
		log.Fatalf("Failed to connect to Dragonfly: %v", err)
	}
	defer store.Close()

	auth := lighthouse.NewAuth(store)
	xds := lighthouse.NewXDS(store)

	var limiter *ratelimit.IPRateLimiter
	if *rateLimitRPS > 0 {
		limiter = ratelimit.New(*rateLimitRPS, float64(*rateLimitBurst), ratelimit.DefaultMaxIPs)
	}
	api := lighthouse.NewAPI(store, auth, *dnsTarget, limiter)

	// Start mesh sync if we have a mesh IP
	var sync *lighthouse.Sync
	if *meshIP != "" {
		sync = lighthouse.NewSync(store, nid, *meshIP)
		for _, p := range peers {
			sync.AddPeer(p)
		}
		if err := sync.Start(); err != nil {
			log.Printf("WARNING: mesh sync failed to start: %v", err)
		} else {
			defer sync.Stop()
		}
	}

	// HTTP routes
	mux := http.NewServeMux()

	// API routes (mounted at /)
	mux.Handle("/", api)

	// xDS config endpoints (for edge nodes)
	mux.HandleFunc("GET /v1/xds/config", xds.HandleConfig)
	mux.HandleFunc("GET /v1/xds/caddyfile", xds.HandleCaddyConfig)

	log.Printf("lighthouse starting on %s (node=%s, redis=%s, mesh=%s, peers=%v)",
		*addr, nid, *redisAddr, *meshIP, peers)

	if sync != nil {
		log.Printf("Mesh sync active — federated replication enabled")
	} else {
		log.Printf("WARNING: No mesh IP configured — running standalone (no federation)")
	}

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
