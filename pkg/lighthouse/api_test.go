package lighthouse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ratelimit"
)

// testAPI creates an API that will fail on store access but lets us test
// HTTP routing, content-type headers, validation, and public endpoints.
// Tests requiring store operations would need a real Redis/Dragonfly.
// Auth is wired with a nil-store Auth (rejects all keys but won't panic on
// missing Authorization header — the middleware checks the header first).
func testAPI() *API {
	return &API{
		store:     nil, // No store — tests must not hit Redis
		auth:      &Auth{store: nil},
		dnsTarget: "edge.test.cloudroof.eu",
		health:    NewHealthReporter(),
		mux:       http.NewServeMux(),
	}
}

// testAPIWithRoutes creates a fully-wired API for route/handler testing.
func testAPIWithRoutes() *API {
	a := testAPI()
	a.registerRoutes()
	return a
}

func TestHandleHealthz(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthz status = %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("healthz content-type = %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("healthz body decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("healthz status = %q, want %q", body["status"], "ok")
	}
	if body["service"] != "lighthouse" {
		t.Errorf("healthz service = %q, want %q", body["service"], "lighthouse")
	}
}

func TestHandleOpenAPI(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()
	req := httptest.NewRequest("GET", "/v1/openapi.json", nil)
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("openapi status = %d, want %d", w.Code, http.StatusOK)
	}

	var spec map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("openapi decode: %v", err)
	}

	if spec["openapi"] != "3.1.0" {
		t.Errorf("openapi version = %v, want 3.1.0", spec["openapi"])
	}

	info, ok := spec["info"].(map[string]interface{})
	if !ok {
		t.Fatal("openapi info not a map")
	}
	if info["title"] != "cloudroof.eu CDN API" {
		t.Errorf("openapi title = %v", info["title"])
	}

	// Check paths exist
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("openapi paths not a map")
	}
	expectedPaths := []string{"/v1/orgs", "/v1/sites", "/v1/edges"}
	for _, p := range expectedPaths {
		if _, exists := paths[p]; !exists {
			t.Errorf("openapi missing path: %s", p)
		}
	}
}

func TestHandleCreateOrg_ValidationErrors(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantType   string
	}{
		{
			name:       "invalid json",
			body:       `{not json}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "invalid_request",
		},
		{
			name:       "empty name",
			body:       `{"name": ""}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "validation_error",
		},
		{
			name:       "missing name field",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
			wantType:   "validation_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/v1/orgs", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/problem+json" {
				t.Errorf("content-type = %q, want application/problem+json", ct)
			}

			var problem map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
				t.Fatalf("decode error body: %v", err)
			}

			typeStr, _ := problem["type"].(string)
			if !strings.Contains(typeStr, tt.wantType) {
				t.Errorf("error type = %q, want to contain %q", typeStr, tt.wantType)
			}
		})
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()

	// Authenticated endpoints should reject requests without auth
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/v1/sites"},
		{"POST", "/v1/sites"},
		{"GET", "/v1/edges"},
		{"GET", "/v1/orgs/org_test123"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var body *strings.Reader
			if ep.method == "POST" {
				body = strings.NewReader(`{"domain":"test.com","origin":{"mesh_ip":"10.0.0.1","port":8080}}`)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(ep.method, ep.path, body)
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: status = %d, want %d", ep.method, ep.path, w.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestAuthMiddleware_InvalidScheme(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()

	req := httptest.NewRequest("GET", "/v1/sites", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var problem map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatalf("decode: %v", err)
	}
	detail, _ := problem["detail"].(string)
	if !strings.Contains(detail, "Bearer") {
		t.Errorf("error detail = %q, want mention of Bearer", detail)
	}
}

func TestAuthMiddleware_InvalidKeyFormat(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()

	req := httptest.NewRequest("GET", "/v1/sites", nil)
	req.Header.Set("Authorization", "Bearer not_a_cr_key")
	w := httptest.NewRecorder()

	api.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"id": "test_123"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["id"] != "test_123" {
		t.Errorf("id = %q, want test_123", body["id"])
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "validation_error", "name is required")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q, want application/problem+json", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	typeStr, _ := body["type"].(string)
	if !strings.Contains(typeStr, "validation_error") {
		t.Errorf("type = %q, want validation_error", typeStr)
	}
	if body["detail"] != "name is required" {
		t.Errorf("detail = %v, want 'name is required'", body["detail"])
	}
	if body["status"] != float64(400) {
		t.Errorf("status in body = %v, want 400", body["status"])
	}
}

func TestPublicEndpointsDontRequireAuth(t *testing.T) {
	t.Parallel()

	api := testAPIWithRoutes()

	publicEndpoints := []struct {
		method     string
		path       string
		wantStatus int
	}{
		{"GET", "/healthz", http.StatusOK},
		{"GET", "/v1/openapi.json", http.StatusOK},
	}

	for _, ep := range publicEndpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()

			api.ServeHTTP(w, req)

			if w.Code != ep.wantStatus {
				t.Errorf("%s status = %d, want %d", ep.path, w.Code, ep.wantStatus)
			}
		})
	}
}

func TestSiteValidation(t *testing.T) {
	t.Parallel()

	// These tests verify request parsing and validation
	// They'll get past auth check if we set X-Org-ID directly
	// but will fail at store operations. We check the validation errors.

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantDetail string
	}{
		{
			name:       "missing domain",
			body:       `{"origin":{"mesh_ip":"10.0.0.1","port":8080}}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "domain is required",
		},
		{
			name:       "missing origin mesh_ip",
			body:       `{"domain":"example.com","origin":{"port":8080}}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "origin.mesh_ip is required",
		},
		{
			name:       "invalid mesh_ip",
			body:       `{"domain":"example.com","origin":{"mesh_ip":"not-an-ip","port":8080}}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "origin.mesh_ip is not a valid IP",
		},
		{
			name:       "port zero",
			body:       `{"domain":"example.com","origin":{"mesh_ip":"10.0.0.1","port":0}}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "origin.port must be 1-65535",
		},
		{
			name:       "port too high",
			body:       `{"domain":"example.com","origin":{"mesh_ip":"10.0.0.1","port":70000}}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "origin.port must be 1-65535",
		},
		{
			name:       "invalid protocol",
			body:       `{"domain":"example.com","origin":{"mesh_ip":"10.0.0.1","port":8080,"protocol":"ftp"}}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "origin.protocol must be http or https",
		},
		{
			name:       "invalid tls mode",
			body:       `{"domain":"example.com","origin":{"mesh_ip":"10.0.0.1","port":8080},"tls":"invalid"}`,
			wantStatus: http.StatusBadRequest,
			wantDetail: "tls must be auto, custom, or off",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a handler that skips auth (simulates authenticated request)
			api := testAPI()
			handler := http.HandlerFunc(api.handleCreateSite)

			req := httptest.NewRequest("POST", "/v1/sites", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Org-ID", "org_test123") // Simulate authenticated
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			var problem map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
				t.Fatalf("decode: %v", err)
			}
			detail, _ := problem["detail"].(string)
			if detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", detail, tt.wantDetail)
			}
		})
	}
}

func TestEnvoySnapshotStructure(t *testing.T) {
	t.Parallel()

	snap := &EnvoySnapshot{
		Version:   "v1",
		Timestamp: "2026-01-01T00:00:00Z",
		Clusters: []EnvoyCluster{
			{Name: "origin_site_abc", Address: "10.0.0.1", Port: 8080, Protocol: "http"},
		},
		Routes: []EnvoyRoute{
			{Domain: "example.com", Cluster: "origin_site_abc", TLS: "auto", SiteID: "site_abc"},
		},
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var decoded EnvoySnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if decoded.Version != "v1" {
		t.Errorf("version = %q, want v1", decoded.Version)
	}
	if len(decoded.Clusters) != 1 {
		t.Errorf("clusters count = %d, want 1", len(decoded.Clusters))
	}
	if len(decoded.Routes) != 1 {
		t.Errorf("routes count = %d, want 1", len(decoded.Routes))
	}
	if decoded.Clusters[0].Name != "origin_site_abc" {
		t.Errorf("cluster name = %q", decoded.Clusters[0].Name)
	}
	if decoded.Routes[0].Domain != "example.com" {
		t.Errorf("route domain = %q", decoded.Routes[0].Domain)
	}
}

func TestSyncMessageSerialization(t *testing.T) {
	t.Parallel()

	msg := SyncMessage{
		Type:    "site",
		Action:  "upsert",
		Payload: []byte(`{"id":"site_test"}`),
		Version: 42,
		NodeID:  "lh-node-1",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded SyncMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Type != "site" {
		t.Errorf("type = %q, want site", decoded.Type)
	}
	if decoded.Version != 42 {
		t.Errorf("version = %d, want 42", decoded.Version)
	}
	if decoded.NodeID != "lh-node-1" {
		t.Errorf("nodeID = %q, want lh-node-1", decoded.NodeID)
	}
}

// testAPIWithLimiter creates an API with a rate limiter for rate limit testing.
func testAPIWithLimiter(limiter *ratelimit.IPRateLimiter) *API {
	a := testAPI()
	a.limiter = limiter
	a.registerRoutes()
	return a
}

// rateLimitRequest builds a request that has X-Org-ID already set (simulating
// a request that has passed requireAuth). Used to exercise rateLimit directly.
func rateLimitRequest(orgID string) *http.Request {
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("X-Org-ID", orgID)
	return req
}

func TestRateLimit_AllowsUnderBurst(t *testing.T) {
	t.Parallel()

	// burst=3: first 3 requests must be allowed
	limiter := ratelimit.New(10, 3, 100)
	a := testAPI()
	a.limiter = limiter

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.rateLimit(inner)

	for i := 0; i < 3; i++ {
		req := rateLimitRequest("org_test")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}
}

func TestRateLimit_Returns429WhenExhausted(t *testing.T) {
	t.Parallel()

	// burst=2: third request must be rate limited
	limiter := ratelimit.New(10, 2, 100)
	a := testAPI()
	a.limiter = limiter

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.rateLimit(inner)

	// Exhaust the burst
	for i := 0; i < 2; i++ {
		req := rateLimitRequest("org_test")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Next request should be rate limited
	req := rateLimitRequest("org_test")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("content-type = %q, want application/problem+json", ct)
	}

	var problem map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&problem); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	typeStr, _ := problem["type"].(string)
	if !strings.Contains(typeStr, "rate_limit_exceeded") {
		t.Errorf("error type = %q, want rate_limit_exceeded", typeStr)
	}
}

func TestRateLimit_Headers(t *testing.T) {
	t.Parallel()

	limiter := ratelimit.New(10, 5, 100)
	a := testAPI()
	a.limiter = limiter

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.rateLimit(inner)

	req := rateLimitRequest("org_abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	for _, hdr := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if w.Header().Get(hdr) == "" {
			t.Errorf("missing header %s", hdr)
		}
	}

	limit, err := strconv.Atoi(w.Header().Get("X-RateLimit-Limit"))
	if err != nil || limit != 5 {
		t.Errorf("X-RateLimit-Limit = %q, want 5", w.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRateLimit_RetryAfterOnDenied(t *testing.T) {
	t.Parallel()

	// burst=1: second request must be denied with Retry-After
	limiter := ratelimit.New(10, 1, 100)
	a := testAPI()
	a.limiter = limiter

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.rateLimit(inner)

	// First request — allowed
	handler.ServeHTTP(httptest.NewRecorder(), rateLimitRequest("org_ra"))

	// Second request — denied
	req := rateLimitRequest("org_ra")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}

	ra := w.Header().Get("Retry-After")
	if ra == "" {
		t.Error("missing Retry-After header on 429")
	}
	secs, err := strconv.Atoi(ra)
	if err != nil || secs < 1 {
		t.Errorf("Retry-After = %q, want a positive integer", ra)
	}
}

func TestRateLimit_NilLimiterIsNoop(t *testing.T) {
	t.Parallel()

	// nil limiter — all requests pass through
	a := testAPI()
	a.limiter = nil

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.rateLimit(inner)

	for i := 0; i < 10; i++ {
		req := rateLimitRequest("org_nolimit")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimit_PerKeyIsolation(t *testing.T) {
	t.Parallel()

	// burst=1: exhausting org_A must not affect org_B
	limiter := ratelimit.New(10, 1, 100)
	a := testAPI()
	a.limiter = limiter

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := a.rateLimit(inner)

	// Exhaust org_A
	handler.ServeHTTP(httptest.NewRecorder(), rateLimitRequest("org_A"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, rateLimitRequest("org_A"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("org_A second request: status = %d, want 429", w.Code)
	}

	// org_B should still be allowed
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, rateLimitRequest("org_B"))
	if w2.Code != http.StatusOK {
		t.Errorf("org_B: status = %d, want 200 (should not be affected by org_A's bucket)", w2.Code)
	}
}
