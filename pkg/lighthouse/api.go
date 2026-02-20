package lighthouse

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ratelimit"
)

// API implements the cloudroof.eu REST API.
// All endpoints return JSON. Errors follow RFC 7807 Problem Details.
type API struct {
	store     *Store
	auth      *Auth
	dnsTarget string // Edge DNS target (e.g., "edge.cloudroof.eu")
	limiter   *ratelimit.IPRateLimiter
	health    *HealthReporter
	mux       *http.ServeMux
}

// NewAPI creates the API handler. limiter may be nil to disable rate limiting.
func NewAPI(store *Store, auth *Auth, dnsTarget string, limiter *ratelimit.IPRateLimiter) *API {
	a := &API{
		store:     store,
		auth:      auth,
		dnsTarget: dnsTarget,
		limiter:   limiter,
		health:    NewHealthReporter(),
		mux:       http.NewServeMux(),
	}
	a.registerRoutes()
	return a
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *API) registerRoutes() {
	// Public (no auth)
	a.mux.HandleFunc("GET /healthz", a.handleHealthz)
	a.mux.HandleFunc("GET /v1/openapi.json", a.handleOpenAPI)

	// Org creation (no auth — this is how you bootstrap)
	a.mux.HandleFunc("POST /v1/orgs", a.handleCreateOrg)

	// Authenticated
	a.mux.HandleFunc("GET /v1/orgs/{org_id}", a.requireAuth(a.rateLimit(a.handleGetOrg)))
	a.mux.HandleFunc("POST /v1/orgs/{org_id}/keys", a.requireAuth(a.rateLimit(a.handleCreateKey)))

	// Sites
	a.mux.HandleFunc("POST /v1/sites", a.requireAuth(a.rateLimit(a.handleCreateSite)))
	a.mux.HandleFunc("GET /v1/sites", a.requireAuth(a.rateLimit(a.handleListSites)))
	a.mux.HandleFunc("GET /v1/sites/{site_id}", a.requireAuth(a.rateLimit(a.handleGetSite)))
	a.mux.HandleFunc("PATCH /v1/sites/{site_id}", a.requireAuth(a.rateLimit(a.handleUpdateSite)))
	a.mux.HandleFunc("DELETE /v1/sites/{site_id}", a.requireAuth(a.rateLimit(a.handleDeleteSite)))
	a.mux.HandleFunc("GET /v1/sites/{site_id}/dns-status", a.requireAuth(a.rateLimit(a.handleGetSiteDNSStatus)))

	// Edges (read-only, authenticated)
	a.mux.HandleFunc("GET /v1/edges", a.requireAuth(a.rateLimit(a.handleListEdges)))

	// Edge health reporting (internal — edges POST their probe results here)
	a.mux.HandleFunc("POST /v1/sites/{site_id}/health", a.requireAuth(a.handleReportHealth))
}

// --- Middleware ---

type contextKey string

const orgIDKey contextKey = "org_id"

func (a *API) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := a.auth.Authenticate(r.Context(), r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		// Store org_id in a request header (internal, stripped from upstream)
		r.Header.Set("X-Org-ID", orgID)
		next(w, r)
	}
}

func getOrgID(r *http.Request) string {
	return r.Header.Get("X-Org-ID")
}

// rateLimit is a middleware that enforces per-org-ID rate limiting.
// It must be chained after requireAuth so that X-Org-ID is already set.
// If no limiter is configured, the middleware is a no-op.
func (a *API) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.limiter != nil {
			key := getOrgID(r)
			allowed, remaining, retryAfter := a.limiter.Reserve(key)
			// X-RateLimit-Reset: when the next token will be available.
			// When allowed, this is 1/rate seconds from now; when denied,
			// it is retryAfter from now.
			var resetIn time.Duration
			if allowed {
				resetIn = time.Duration(float64(time.Second) / a.limiter.Rate())
			} else {
				resetIn = retryAfter
			}
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(a.limiter.Burst()))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(resetIn).Unix(), 10))
			if !allowed {
				// Retry-After in whole seconds, rounded up via integer arithmetic.
				retrySecs := (int(retryAfter.Milliseconds()) + 999) / 1000
				w.Header().Set("Retry-After", strconv.Itoa(retrySecs))
				writeError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "Rate limit exceeded. Please retry later.")
				return
			}
		}
		next(w, r)
	}
}

// --- Handlers ---

func (a *API) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "lighthouse",
	})
}

func (a *API) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	// Minimal OpenAPI 3.1 spec for LLM consumption
	spec := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       "cloudroof.eu CDN API",
			"version":     "1.0.0",
			"description": "Programmable edge proxy API. Register domains, configure origins, manage routes. Designed for LLM agents.",
		},
		"servers": []map[string]string{
			{"url": "https://api.cloudroof.eu", "description": "Production"},
		},
		"paths": map[string]interface{}{
			"/v1/orgs": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Create organization (returns first API key)",
					"operationId": "createOrg",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"name": map[string]string{"type": "string", "description": "Organization name"},
									},
									"required": []string{"name"},
								},
							},
						},
					},
				},
			},
			"/v1/sites": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Register a domain for CDN routing",
					"operationId": "createSite",
					"security":    []map[string][]string{{"bearerAuth": {}}},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"domain": map[string]string{"type": "string", "description": "Domain name (e.g. example.com)"},
										"origin": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"mesh_ip":  map[string]string{"type": "string", "description": "WireGuard mesh IP of origin"},
												"port":     map[string]string{"type": "integer", "description": "Origin port"},
												"protocol": map[string]string{"type": "string", "description": "http or https"},
											},
										},
										"tls": map[string]string{"type": "string", "description": "auto, custom, or off"},
									},
									"required": []string{"domain", "origin"},
								},
							},
						},
					},
				},
				"get": map[string]interface{}{
					"summary":     "List sites for your organization",
					"operationId": "listSites",
					"security":    []map[string][]string{{"bearerAuth": {}}},
				},
			},
			"/v1/edges": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "List edge nodes and their status",
					"operationId": "listEdges",
					"security":    []map[string][]string{{"bearerAuth": {}}},
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":        "http",
					"scheme":      "bearer",
					"description": "API key (starts with cr_). Obtain via POST /v1/orgs.",
				},
			},
		},
	}
	writeJSON(w, http.StatusOK, spec)
}

func (a *API) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}

	org, rawKey, err := a.auth.CreateOrgWithKey(r.Context(), req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"org":     org,
		"api_key": rawKey,
		"message": "Store this API key securely — it will not be shown again.",
	})
}

func (a *API) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	callerOrg := getOrgID(r)

	if orgID != callerOrg {
		writeError(w, http.StatusForbidden, "forbidden", "Cannot access another organization")
		return
	}

	org, err := a.store.GetOrg(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, org)
}

func (a *API) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("org_id")
	callerOrg := getOrgID(r)

	if orgID != callerOrg {
		writeError(w, http.StatusForbidden, "forbidden", "Cannot create keys for another organization")
		return
	}

	rawKey, apiKey, err := a.auth.CreateAdditionalKey(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"api_key": rawKey,
		"key_id":  apiKey.ID,
		"prefix":  apiKey.Prefix,
		"message": "Store this API key securely — it will not be shown again.",
	})
}

func (a *API) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)

	var req struct {
		Domain string `json:"domain"`
		Origin Origin `json:"origin"`
		TLS    string `json:"tls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	// Validate
	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "domain is required")
		return
	}
	if req.Origin.MeshIP == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "origin.mesh_ip is required")
		return
	}
	if net.ParseIP(req.Origin.MeshIP) == nil {
		writeError(w, http.StatusBadRequest, "validation_error", "origin.mesh_ip is not a valid IP")
		return
	}
	if req.Origin.Port <= 0 || req.Origin.Port > 65535 {
		writeError(w, http.StatusBadRequest, "validation_error", "origin.port must be 1-65535")
		return
	}
	if req.Origin.Protocol == "" {
		req.Origin.Protocol = "http"
	}
	if req.Origin.Protocol != "http" && req.Origin.Protocol != "https" {
		writeError(w, http.StatusBadRequest, "validation_error", "origin.protocol must be http or https")
		return
	}

	tlsMode := TLSModeAuto
	if req.TLS != "" {
		switch TLSMode(req.TLS) {
		case TLSModeAuto, TLSModeCustom, TLSModeOff:
			tlsMode = TLSMode(req.TLS)
		default:
			writeError(w, http.StatusBadRequest, "validation_error", "tls must be auto, custom, or off")
			return
		}
	}

	// Normalize domain
	domain := strings.ToLower(strings.TrimSpace(req.Domain))

	site := &Site{
		OrgID:     orgID,
		Domain:    domain,
		Origin:    req.Origin,
		TLS:       tlsMode,
		Status:    SiteStatusPendingDNS,
		DNSTarget: a.dnsTarget,
	}

	if err := a.store.CreateSite(r.Context(), site); err != nil {
		if strings.Contains(err.Error(), "already registered") {
			writeError(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, site)
}

func (a *API) handleListSites(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)

	sites, err := a.store.ListSites(r.Context(), orgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sites": sites,
		"count": len(sites),
	})
}

func (a *API) handleGetSite(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	siteID := r.PathValue("site_id")

	site, err := a.store.GetSite(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}

	if site.OrgID != orgID {
		writeError(w, http.StatusForbidden, "forbidden", "Site belongs to another organization")
		return
	}

	resp := map[string]interface{}{
		"site":          site,
		"origin_health": a.health.StatusForSite(siteID),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) handleUpdateSite(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	siteID := r.PathValue("site_id")

	site, err := a.store.GetSite(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if site.OrgID != orgID {
		writeError(w, http.StatusForbidden, "forbidden", "Site belongs to another organization")
		return
	}

	var patch struct {
		Origin *Origin `json:"origin,omitempty"`
		TLS    *string `json:"tls,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if patch.Origin != nil {
		if patch.Origin.MeshIP != "" {
			if net.ParseIP(patch.Origin.MeshIP) == nil {
				writeError(w, http.StatusBadRequest, "validation_error", "origin.mesh_ip is not a valid IP")
				return
			}
			site.Origin.MeshIP = patch.Origin.MeshIP
		}
		if patch.Origin.Port != 0 {
			if patch.Origin.Port < 0 || patch.Origin.Port > 65535 {
				writeError(w, http.StatusBadRequest, "validation_error", "origin.port must be 1-65535")
				return
			}
			site.Origin.Port = patch.Origin.Port
		}
		if patch.Origin.Protocol != "" {
			if patch.Origin.Protocol != "http" && patch.Origin.Protocol != "https" {
				writeError(w, http.StatusBadRequest, "validation_error", "origin.protocol must be http or https")
				return
			}
			site.Origin.Protocol = patch.Origin.Protocol
		}
	}
	if patch.TLS != nil {
		switch TLSMode(*patch.TLS) {
		case TLSModeAuto, TLSModeCustom, TLSModeOff:
			site.TLS = TLSMode(*patch.TLS)
		default:
			writeError(w, http.StatusBadRequest, "validation_error", "tls must be auto, custom, or off")
			return
		}
	}

	if err := a.store.UpdateSite(r.Context(), site); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, site)
}

func (a *API) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	siteID := r.PathValue("site_id")

	site, err := a.store.GetSite(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if site.OrgID != orgID {
		writeError(w, http.StatusForbidden, "forbidden", "Site belongs to another organization")
		return
	}

	if err := a.store.DeleteSite(r.Context(), siteID, orgID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"id":     siteID,
	})
}

func (a *API) handleListEdges(w http.ResponseWriter, r *http.Request) {
	edges, err := a.store.ListEdges(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"edges": edges,
		"count": len(edges),
	})
}

func (a *API) handleGetSiteDNSStatus(w http.ResponseWriter, r *http.Request) {
	orgID := getOrgID(r)
	siteID := r.PathValue("site_id")

	site, err := a.store.GetSite(r.Context(), siteID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if site.OrgID != orgID {
		writeError(w, http.StatusForbidden, "forbidden", "Site belongs to another organization")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"site_id":    site.ID,
		"domain":     site.Domain,
		"dns_target": site.DNSTarget,
		"status":     site.Status,
	})
}

// handleReportHealth accepts a health status report from an edge for a specific site.
// Edges call this endpoint to report the result of their HTTP origin health probes.
func (a *API) handleReportHealth(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("site_id")

	var report struct {
		EdgeID string       `json:"edge_id"`
		Status HealthStatus `json:"status"`
		Error  string       `json:"error,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if report.EdgeID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "edge_id is required")
		return
	}
	switch report.Status {
	case HealthStatusHealthy, HealthStatusUnhealthy, HealthStatusUnknown:
	default:
		writeError(w, http.StatusBadRequest, "validation_error", "status must be healthy, unhealthy, or unknown")
		return
	}

	a.health.Report(report.EdgeID, OriginHealth{
		SiteID:      siteID,
		Status:      report.Status,
		LastChecked: time.Now().UTC(),
		LastError:   report.Error,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, errType, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	body := map[string]interface{}{
		"type":   fmt.Sprintf("https://api.cloudroof.eu/errors/%s", errType),
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write error response: %v", err)
	}
}
