package lighthouse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// HealthStatus represents the current health of an origin.
type HealthStatus string

const (
	HealthStatusUnknown   HealthStatus = "unknown"
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// defaultUnhealthyThreshold is the number of consecutive failures before marking an origin down.
const defaultUnhealthyThreshold = 2

// defaultHealthyThreshold is the number of consecutive successes before marking an origin up.
const defaultHealthyThreshold = 2

// defaultCheckInterval is the default interval between health checks.
const defaultCheckInterval = 10 * time.Second

// defaultCheckTimeout is the default per-probe HTTP timeout.
const defaultCheckTimeout = 5 * time.Second

// OriginHealth holds the live health state for a single origin.
type OriginHealth struct {
	SiteID       string       `json:"site_id"`
	Status       HealthStatus `json:"status"`
	ConsecFails  int          `json:"consec_fails"`
	ConsecPasses int          `json:"consec_passes"`
	LastChecked  time.Time    `json:"last_checked,omitempty"`
	LastError    string       `json:"last_error,omitempty"`
}

// Checker runs periodic HTTP health probes for origin endpoints.
// It manages per-site health state and transitions between healthy/unhealthy
// based on configurable consecutive pass/fail thresholds.
type Checker struct {
	mu     sync.Mutex
	states map[string]*OriginHealth // keyed by site ID
	client *http.Client
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewChecker creates a Checker with the provided HTTP client.
// If client is nil, a default client is used.
func NewChecker(client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{}
	}
	return &Checker{
		states: make(map[string]*OriginHealth),
		client: client,
		stopCh: make(chan struct{}),
	}
}

// Run starts background health probe loops for the given sites.
// Each site with a non-empty HealthCheck.Path gets its own goroutine.
// Call Stop to shut down all probes.
func (c *Checker) Run(sites []Site) {
	for _, site := range sites {
		site := site // capture loop var
		if site.Origin.HealthCheck.Path == "" {
			continue
		}
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.probeLoop(site)
		}()
	}
}

// Stop halts all running probe goroutines and waits for them to finish.
func (c *Checker) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// Status returns the current OriginHealth for the given site ID.
// If no state has been recorded, Status returns an unknown entry.
func (c *Checker) Status(siteID string) OriginHealth {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.states[siteID]; ok {
		return *s
	}
	return OriginHealth{SiteID: siteID, Status: HealthStatusUnknown}
}

// probeLoop continuously probes the origin for a single site until stopped.
func (c *Checker) probeLoop(site Site) {
	hc := site.Origin.HealthCheck
	interval := hc.Interval
	if interval <= 0 {
		interval = defaultCheckInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.probe(site)
		}
	}
}

// probe performs a single HTTP health check and updates state.
func (c *Checker) probe(site Site) {
	hc := site.Origin.HealthCheck
	if hc.Path == "" {
		return
	}

	timeout := hc.Timeout
	if timeout <= 0 {
		timeout = defaultCheckTimeout
	}

	url := fmt.Sprintf("%s://%s:%d%s", site.Origin.Protocol, site.Origin.MeshIP, site.Origin.Port, hc.Path)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		c.recordFail(site, fmt.Sprintf("build request: %v", err))
		return
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.recordFail(site, err.Error())
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		c.recordPass(site)
	} else {
		c.recordFail(site, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}
}

// recordPass updates state after a successful probe.
func (c *Checker) recordPass(site Site) {
	hc := site.Origin.HealthCheck
	healthyThreshold := hc.Healthy
	if healthyThreshold <= 0 {
		healthyThreshold = defaultHealthyThreshold
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	s := c.getOrCreate(site.ID)
	s.LastChecked = time.Now().UTC()
	s.LastError = ""
	s.ConsecFails = 0
	s.ConsecPasses++

	if s.Status != HealthStatusHealthy && s.ConsecPasses >= healthyThreshold {
		prev := s.Status
		s.Status = HealthStatusHealthy
		log.Printf("[health] site %s: %s → healthy (after %d consecutive passes)", site.ID, prev, s.ConsecPasses)
	}
}

// recordFail updates state after a failed probe.
func (c *Checker) recordFail(site Site, reason string) {
	hc := site.Origin.HealthCheck
	unhealthyThreshold := hc.Unhealthy
	if unhealthyThreshold <= 0 {
		unhealthyThreshold = defaultUnhealthyThreshold
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	s := c.getOrCreate(site.ID)
	s.LastChecked = time.Now().UTC()
	s.LastError = reason
	s.ConsecPasses = 0
	s.ConsecFails++

	if s.Status != HealthStatusUnhealthy && s.ConsecFails >= unhealthyThreshold {
		prev := s.Status
		s.Status = HealthStatusUnhealthy
		log.Printf("[health] site %s: %s → unhealthy (after %d consecutive failures): %s", site.ID, prev, s.ConsecFails, reason)
	}
}

// getOrCreate returns the OriginHealth for the siteID, creating it if necessary.
// Caller must hold c.mu.
func (c *Checker) getOrCreate(siteID string) *OriginHealth {
	if s, ok := c.states[siteID]; ok {
		return s
	}
	s := &OriginHealth{SiteID: siteID, Status: HealthStatusUnknown}
	c.states[siteID] = s
	return s
}

// HealthReporter stores origin health reports submitted by edges
// and exposes aggregate status to the lighthouse API.
// It is safe for concurrent use and holds only in-memory state
// (health status is ephemeral — not persisted to Redis).
type HealthReporter struct {
	mu      sync.RWMutex
	reports map[string]map[string]OriginHealth // siteID → edgeID → status
}

// NewHealthReporter creates a HealthReporter.
func NewHealthReporter() *HealthReporter {
	return &HealthReporter{
		reports: make(map[string]map[string]OriginHealth),
	}
}

// Report records a health status update for a site as seen from a specific edge.
func (r *HealthReporter) Report(edgeID string, h OriginHealth) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reports[h.SiteID] == nil {
		r.reports[h.SiteID] = make(map[string]OriginHealth)
	}
	r.reports[h.SiteID][edgeID] = h
}

// StatusForSite returns the latest health report for the given site, keyed by edge ID.
func (r *HealthReporter) StatusForSite(siteID string) map[string]OriginHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	edgeMap := r.reports[siteID]
	if len(edgeMap) == 0 {
		return nil
	}
	// Return a copy to avoid data races.
	out := make(map[string]OriginHealth, len(edgeMap))
	for k, v := range edgeMap {
		out[k] = v
	}
	return out
}
