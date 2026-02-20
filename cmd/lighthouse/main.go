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
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/lighthouse"
	"github.com/atvirokodosprendimai/wgmesh/pkg/proxy"
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
	proxyAddr := flag.String("proxy-addr", "", "address to listen for proxy traffic (e.g. :8081)")
	proxyOrigins := flag.String("proxy-origins", "", "comma-separated domain=url pairs for reverse proxy (e.g. example.com=http://10.0.0.2:3000)")

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

	// Start DNS verification loop
	dnsCtx, dnsCancel := context.WithCancel(context.Background())
	defer dnsCancel()
	go runDNSVerificationLoop(dnsCtx, store, lighthouse.NetResolver{})

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

	// Start reverse proxy listener if configured.
	if *proxyAddr != "" {
		origins := make(map[string]string)
		for _, pair := range strings.Split(*proxyOrigins, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			idx := strings.IndexByte(pair, '=')
			if idx <= 0 {
				log.Fatalf("proxy: invalid origin pair %q (expected domain=url)", pair)
			}
			origins[strings.TrimSpace(pair[:idx])] = strings.TrimSpace(pair[idx+1:])
		}
		p, err := proxy.New(origins)
		if err != nil {
			log.Fatalf("proxy: %v", err)
		}
		log.Printf("proxy starting on %s (%d origins)", *proxyAddr, len(origins))
		go func() {
			if err := http.ListenAndServe(*proxyAddr, p); err != nil {
				log.Printf("proxy error: %v", err)
			}
		}()
	}

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

// runDNSVerificationLoop checks sites in pending_dns state every 60 seconds.
// Sites that pass DNS verification are transitioned to active.
// Sites that have been pending for more than 24 hours are transitioned to dns_failed.
func runDNSVerificationLoop(ctx context.Context, store *lighthouse.Store, resolver lighthouse.Resolver) {
	const (
		dnsVerificationInterval = 60 * time.Second
		dnsVerificationTimeout  = 24 * time.Hour
	)
	ticker := time.NewTicker(dnsVerificationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkPendingDNSSites(ctx, store, resolver, dnsVerificationTimeout)
		}
	}
}

func checkPendingDNSSites(ctx context.Context, store *lighthouse.Store, resolver lighthouse.Resolver, timeout time.Duration) {
	sites, err := store.ListSitesByStatus(ctx, lighthouse.SiteStatusPendingDNS)
	if err != nil {
		log.Printf("dns-verify: list pending sites: %v", err)
		return
	}

	for i := range sites {
		site := &sites[i]
		ok, err := lighthouse.VerifyDNS(resolver, site.Domain, site.DNSTarget, nil)
		if err != nil {
			log.Printf("dns-verify: domain=%s lookup error: %v", site.Domain, err)
		}
		if ok {
			site.Status = lighthouse.SiteStatusActive
			if err := store.UpdateSite(ctx, site); err != nil {
				log.Printf("dns-verify: activate site %s: %v", site.ID, err)
			} else {
				log.Printf("dns-verify: site %s (%s) → active", site.ID, site.Domain)
			}
			continue
		}
		// Mark as failed after timeout
		if time.Since(site.CreatedAt) > timeout {
			site.Status = lighthouse.SiteStatusDNSFailed
			if err := store.UpdateSite(ctx, site); err != nil {
				log.Printf("dns-verify: fail site %s: %v", site.ID, err)
			} else {
				log.Printf("dns-verify: site %s (%s) → dns_failed (timeout)", site.ID, site.Domain)
			}
		}
	}
}
