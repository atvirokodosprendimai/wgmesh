package lighthouse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- HealthCheck type tests ---

func TestHealthCheckDefaults(t *testing.T) {
	t.Parallel()

	hc := HealthCheck{}
	if hc.Path != "" {
		t.Errorf("default Path = %q, want empty", hc.Path)
	}
	if hc.Interval != 0 {
		t.Errorf("default Interval = %v, want 0 (sentinel for defaultCheckInterval)", hc.Interval)
	}
}

func TestHealthCheckJSONRoundtrip(t *testing.T) {
	t.Parallel()

	hc := HealthCheck{
		Path:      "/healthz",
		Interval:  10 * time.Second,
		Timeout:   3 * time.Second,
		Unhealthy: 2,
		Healthy:   3,
	}

	data, err := json.Marshal(hc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded HealthCheck
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Path != hc.Path {
		t.Errorf("Path = %q, want %q", decoded.Path, hc.Path)
	}
	if decoded.Interval != hc.Interval {
		t.Errorf("Interval = %v, want %v", decoded.Interval, hc.Interval)
	}
	if decoded.Unhealthy != hc.Unhealthy {
		t.Errorf("Unhealthy = %d, want %d", decoded.Unhealthy, hc.Unhealthy)
	}
	if decoded.Healthy != hc.Healthy {
		t.Errorf("Healthy = %d, want %d", decoded.Healthy, hc.Healthy)
	}
}

func TestOriginWithHealthCheck_JSONRoundtrip(t *testing.T) {
	t.Parallel()

	o := Origin{
		MeshIP:   "10.0.0.1",
		Port:     8080,
		Protocol: "http",
		HealthCheck: HealthCheck{
			Path:      "/healthz",
			Interval:  15 * time.Second,
			Unhealthy: 2,
			Healthy:   2,
		},
	}

	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Origin
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.HealthCheck.Path != "/healthz" {
		t.Errorf("HealthCheck.Path = %q, want /healthz", decoded.HealthCheck.Path)
	}
}

// --- Checker helper ---

// makeSiteAtServer creates a Site that points at the given test server.
func makeSiteAtServer(id string, srv *httptest.Server, path string, unhealthy, healthy int) Site {
	addr := strings.TrimPrefix(srv.URL, "http://")
	var host string
	var port int
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		host = addr[:i]
		fmt.Sscanf(addr[i+1:], "%d", &port)
	}
	return Site{
		ID:    id,
		OrgID: "org_test",
		Origin: Origin{
			MeshIP:   host,
			Port:     port,
			Protocol: "http",
			HealthCheck: HealthCheck{
				Path:      path,
				Timeout:   200 * time.Millisecond,
				Unhealthy: unhealthy,
				Healthy:   healthy,
			},
		},
	}
}

// --- Checker state machine tests ---

func TestChecker_InitialStatusUnknown(t *testing.T) {
	t.Parallel()

	c := NewChecker(nil)
	s := c.Status("site_nonexistent")
	if s.Status != HealthStatusUnknown {
		t.Errorf("Status = %q, want %q", s.Status, HealthStatusUnknown)
	}
}

func TestChecker_HealthyTransition(t *testing.T) {
	t.Parallel()

	// Server always returns 200.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(srv.Client())
	site := makeSiteAtServer("site_1", srv, "/healthz", 2, 2)

	// Unknown → healthy requires 2 consecutive passes.
	if got := c.Status(site.ID).Status; got != HealthStatusUnknown {
		t.Fatalf("initial Status = %q, want unknown", got)
	}

	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusUnknown {
		t.Errorf("after 1 pass, Status = %q, want unknown (threshold=2)", got)
	}

	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusHealthy {
		t.Errorf("after 2 passes, Status = %q, want healthy", got)
	}
}

func TestChecker_UnhealthyTransition(t *testing.T) {
	t.Parallel()

	// Server always returns 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewChecker(srv.Client())
	site := makeSiteAtServer("site_2", srv, "/healthz", 2, 2)

	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusUnknown {
		t.Errorf("after 1 fail, Status = %q, want unknown (threshold=2)", got)
	}

	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusUnhealthy {
		t.Errorf("after 2 fails, Status = %q, want unhealthy", got)
	}
}

func TestChecker_HealthyToUnhealthyToHealthy(t *testing.T) {
	t.Parallel()

	// Controllable response code.
	code := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	defer srv.Close()

	c := NewChecker(srv.Client())
	site := makeSiteAtServer("site_3", srv, "/healthz", 2, 2)

	// Become healthy.
	c.probe(site)
	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusHealthy {
		t.Fatalf("want healthy after 2 passes, got %q", got)
	}

	// One failure should NOT yet flip to unhealthy.
	code = http.StatusInternalServerError
	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusHealthy {
		t.Errorf("after 1 fail, Status = %q, want still healthy", got)
	}

	// Second failure flips to unhealthy.
	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusUnhealthy {
		t.Fatalf("after 2 fails, Status = %q, want unhealthy", got)
	}

	// Recover: one success should NOT flip back yet.
	code = http.StatusOK
	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusUnhealthy {
		t.Errorf("after 1 pass from unhealthy, Status = %q, want still unhealthy", got)
	}

	// Second success flips back to healthy.
	c.probe(site)
	if got := c.Status(site.ID).Status; got != HealthStatusHealthy {
		t.Fatalf("after 2 passes recovery, Status = %q, want healthy", got)
	}
}

func TestChecker_FailOnConnectionError(t *testing.T) {
	t.Parallel()

	c := NewChecker(nil)
	site := Site{
		ID:    "site_err",
		OrgID: "org_test",
		Origin: Origin{
			MeshIP:   "127.0.0.1",
			Port:     1, // nothing listening
			Protocol: "http",
			HealthCheck: HealthCheck{
				Path:      "/healthz",
				Timeout:   100 * time.Millisecond,
				Unhealthy: 1,
			},
		},
	}

	c.probe(site)
	s := c.Status(site.ID)
	if s.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want unhealthy on connection error", s.Status)
	}
	if s.LastError == "" {
		t.Error("expected LastError to be set on connection error")
	}
}

func TestChecker_NoProbeWhenPathEmpty(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewChecker(srv.Client())
	site := makeSiteAtServer("site_nohc", srv, "", 2, 2) // empty path

	c.probe(site)

	if called {
		t.Error("expected no HTTP probe when HealthCheck.Path is empty")
	}
}

// --- HealthReporter tests ---

func TestHealthReporter_EmptyStatus(t *testing.T) {
	t.Parallel()

	r := NewHealthReporter()
	m := r.StatusForSite("site_missing")
	if m != nil {
		t.Errorf("expected nil for unknown site, got %v", m)
	}
}

func TestHealthReporter_ReportAndRetrieve(t *testing.T) {
	t.Parallel()

	r := NewHealthReporter()
	r.Report("edge_1", OriginHealth{SiteID: "site_a", Status: HealthStatusHealthy})
	r.Report("edge_2", OriginHealth{SiteID: "site_a", Status: HealthStatusUnhealthy})

	m := r.StatusForSite("site_a")
	if len(m) != 2 {
		t.Fatalf("expected 2 edge reports, got %d", len(m))
	}
	if m["edge_1"].Status != HealthStatusHealthy {
		t.Errorf("edge_1 status = %q, want healthy", m["edge_1"].Status)
	}
	if m["edge_2"].Status != HealthStatusUnhealthy {
		t.Errorf("edge_2 status = %q, want unhealthy", m["edge_2"].Status)
	}
}

func TestHealthReporter_UpdateOverwrites(t *testing.T) {
	t.Parallel()

	r := NewHealthReporter()
	r.Report("edge_1", OriginHealth{SiteID: "site_b", Status: HealthStatusUnknown})
	r.Report("edge_1", OriginHealth{SiteID: "site_b", Status: HealthStatusHealthy})

	m := r.StatusForSite("site_b")
	if m["edge_1"].Status != HealthStatusHealthy {
		t.Errorf("expected updated status healthy, got %q", m["edge_1"].Status)
	}
}

// --- API health endpoint tests ---

func TestHandleReportHealth_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       `{bad json}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing edge_id",
			body:       `{"status":"healthy"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid status",
			body:       `{"edge_id":"edge_1","status":"maybe"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	a := testAPI()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(a.handleReportHealth)
			req := httptest.NewRequest("POST", "/v1/sites/site_abc/health", strings.NewReader(tt.body))
			req.SetPathValue("site_id", "site_abc")
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestHandleReportHealth_Accepted(t *testing.T) {
	t.Parallel()

	a := testAPI()
	handler := http.HandlerFunc(a.handleReportHealth)

	body := `{"edge_id":"edge_1","status":"unhealthy","error":"connection refused"}`
	req := httptest.NewRequest("POST", "/v1/sites/site_xyz/health", strings.NewReader(body))
	req.SetPathValue("site_id", "site_xyz")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	// Verify the state was recorded.
	m := a.health.StatusForSite("site_xyz")
	if m == nil {
		t.Fatal("expected health status to be recorded")
	}
	if m["edge_1"].Status != HealthStatusUnhealthy {
		t.Errorf("recorded status = %q, want unhealthy", m["edge_1"].Status)
	}
	if m["edge_1"].LastError != "connection refused" {
		t.Errorf("recorded error = %q, want 'connection refused'", m["edge_1"].LastError)
	}
}

func TestHandleReportHealth_RouteRegistered(t *testing.T) {
	t.Parallel()

	// Verify the POST /v1/sites/{site_id}/health route exists and requires auth.
	api := testAPIWithRoutes()
	req := httptest.NewRequest("POST", "/v1/sites/site_abc/health", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	api.ServeHTTP(w, req)
	// Without auth, should get 401 not 404.
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (auth required)", w.Code, http.StatusUnauthorized)
	}
}

// --- Caddyfile generation tests ---

func TestRenderCaddyBlock_WithHealthCheck(t *testing.T) {
	t.Parallel()

	site := Site{
		ID:     "site_hc1",
		Domain: "example.com",
		Status: SiteStatusActive,
		TLS:    TLSModeAuto,
		Origin: Origin{
			MeshIP:   "10.0.0.5",
			Port:     8080,
			Protocol: "http",
			HealthCheck: HealthCheck{
				Path:      "/healthz",
				Interval:  15 * time.Second,
				Timeout:   3 * time.Second,
				Unhealthy: 2,
				Healthy:   2,
			},
		},
	}

	var sb strings.Builder
	renderCaddyBlock(&sb, site)
	got := sb.String()

	if !strings.Contains(got, "health_uri /healthz") {
		t.Errorf("Caddyfile missing health_uri, got:\n%s", got)
	}
	if !strings.Contains(got, "health_interval 15s") {
		t.Errorf("Caddyfile missing health_interval, got:\n%s", got)
	}
	if !strings.Contains(got, "health_timeout 3s") {
		t.Errorf("Caddyfile missing health_timeout, got:\n%s", got)
	}
}

func TestRenderCaddyBlock_NoHealthCheck(t *testing.T) {
	t.Parallel()

	site := Site{
		ID:     "site_nohc",
		Domain: "plain.example.com",
		Status: SiteStatusActive,
		TLS:    TLSModeOff,
		Origin: Origin{
			MeshIP:   "10.0.0.6",
			Port:     80,
			Protocol: "http",
		},
	}

	var sb strings.Builder
	renderCaddyBlock(&sb, site)
	got := sb.String()

	if strings.Contains(got, "health_uri") {
		t.Errorf("Caddyfile should not contain health_uri when HealthCheck.Path is empty, got:\n%s", got)
	}
}

func TestRenderCaddyBlock_DefaultIntervals(t *testing.T) {
	t.Parallel()

	site := Site{
		ID:     "site_defaults",
		Domain: "defaults.example.com",
		Status: SiteStatusActive,
		TLS:    TLSModeAuto,
		Origin: Origin{
			MeshIP:   "10.0.0.7",
			Port:     8080,
			Protocol: "http",
			HealthCheck: HealthCheck{
				Path: "/ping",
				// Interval and Timeout are zero → should use defaults (10s and 5s).
			},
		},
	}

	var sb strings.Builder
	renderCaddyBlock(&sb, site)
	got := sb.String()

	if !strings.Contains(got, "health_uri /ping") {
		t.Errorf("Caddyfile missing health_uri, got:\n%s", got)
	}
	// Should contain the default interval (10s).
	if !strings.Contains(got, "health_interval 10s") {
		t.Errorf("Caddyfile should use default health_interval 10s, got:\n%s", got)
	}
	if !strings.Contains(got, "health_timeout 5s") {
		t.Errorf("Caddyfile should use default health_timeout 5s, got:\n%s", got)
	}
}
