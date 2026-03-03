package lighthouse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateSite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/sites" {
			t.Errorf("expected /v1/sites, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer cr_testkey" {
			t.Errorf("expected Bearer cr_testkey, got %s", r.Header.Get("Authorization"))
		}

		var req CreateSiteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Domain != "ollama.abcdef123456.wgmesh.dev" {
			t.Errorf("expected domain 'ollama.abcdef123456.wgmesh.dev', got %q", req.Domain)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Site{
			ID:     "site_abc123",
			Domain: req.Domain,
			Origin: req.Origin,
			Status: SiteStatusPendingDNS,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "cr_testkey")
	site, err := client.CreateSite(CreateSiteRequest{
		Domain: "ollama.abcdef123456.wgmesh.dev",
		Origin: Origin{
			MeshIP:   "10.42.100.1",
			Port:     11434,
			Protocol: "http",
		},
	})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if site.ID != "site_abc123" {
		t.Errorf("expected site ID 'site_abc123', got %q", site.ID)
	}
	if site.Status != SiteStatusPendingDNS {
		t.Errorf("expected status pending_dns, got %q", site.Status)
	}
}

func TestListSites(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sites": []Site{
				{ID: "site_1", Domain: "a.test.wgmesh.dev", Status: SiteStatusActive},
				{ID: "site_2", Domain: "b.test.wgmesh.dev", Status: SiteStatusPendingDNS},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "cr_testkey")
	sites, err := client.ListSites()
	if err != nil {
		t.Fatalf("ListSites failed: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
	if sites[0].ID != "site_1" {
		t.Errorf("expected first site ID 'site_1', got %q", sites[0].ID)
	}
}

func TestDeleteSite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/v1/sites/site_abc123" {
			t.Errorf("expected /v1/sites/site_abc123, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "cr_testkey")
	if err := client.DeleteSite("site_abc123"); err != nil {
		t.Fatalf("DeleteSite failed: %v", err)
	}
}

func TestCreateSiteUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"title":  "Unauthorized",
			"detail": "invalid API key",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "cr_badkey")
	_, err := client.CreateSite(CreateSiteRequest{Domain: "test.wgmesh.dev"})
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

func TestCreateSiteUnreachable(t *testing.T) {
	client := NewClient("http://localhost:1", "cr_testkey")
	_, err := client.CreateSite(CreateSiteRequest{Domain: "test.wgmesh.dev"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
