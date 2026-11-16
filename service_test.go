package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	lighthouse "github.com/atvirokodosprendimai/lighthouse-go"
	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/mesh"
)

func TestParseLocalAddr(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{":11434", 11434, false},
		{":8080", 8080, false},
		{"127.0.0.1:8080", 8080, false},
		{"0.0.0.0:3000", 3000, false},
		{"[::1]:8080", 8080, false},
		{":65535", 65535, false},
		{":1", 1, false},
		{"abc", 0, true},
		{":", 0, true},
		{":0", 0, true},
		{":70000", 0, true},
		{":99999", 0, true},
	}

	for _, tt := range tests {
		port, err := parseLocalAddr(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseLocalAddr(%q): expected error, got %d", tt.input, port)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseLocalAddr(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if port != tt.expected {
			t.Errorf("parseLocalAddr(%q): expected %d, got %d", tt.input, tt.expected, port)
		}
	}
}

func TestValidServiceName(t *testing.T) {
	valid := []string{"ollama", "my-api", "a", "web-server-1", "a1"}
	invalid := []string{"", "-bad", "bad-", "BAD", "has space", "has.dot", "has_underscore"}

	for _, name := range valid {
		if !validServiceName.MatchString(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}
	for _, name := range invalid {
		if validServiceName.MatchString(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		domain   string
		meshID   string
		expected string
	}{
		{"ollama.abcdef123456.wgmesh.dev", "abcdef123456", "ollama"},
		{"my-api.abcdef123456.wgmesh.dev", "abcdef123456", "my-api"},
		{"unknown.domain.com", "abcdef123456", "unknown.domain.com"},
	}

	for _, tt := range tests {
		got := extractServiceName(tt.domain, tt.meshID)
		if got != tt.expected {
			t.Errorf("extractServiceName(%q, %q): expected %q, got %q", tt.domain, tt.meshID, tt.expected, got)
		}
	}
}

func TestResolveSecret(t *testing.T) {
	// Flag takes precedence
	got := resolveSecret("flag-value")
	if got != "flag-value" {
		t.Errorf("expected flag-value, got %q", got)
	}

	// Falls back to env
	t.Setenv("WGMESH_SECRET", "env-value")
	got = resolveSecret("")
	if got != "env-value" {
		t.Errorf("expected env-value, got %q", got)
	}
}

func TestResolveSecretReadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("wgmesh://v1/file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WGMESH_SECRET", "")
	t.Setenv("WGMESH_SECRET_FILE", path)
	if got := resolveSecret(""); got != "file-value" {
		t.Errorf("expected file-value, got %q", got)
	}
}

func TestResolveAccount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "account.json")

	// Providing flag saves and returns
	acct, err := resolveAccount(path, "cr_test123")
	if err != nil {
		t.Fatalf("resolveAccount with flag: %v", err)
	}
	if acct.APIKey != "cr_test123" {
		t.Errorf("expected cr_test123, got %q", acct.APIKey)
	}

	// Subsequent call without flag loads from disk
	acct, err = resolveAccount(path, "")
	if err != nil {
		t.Fatalf("resolveAccount from disk: %v", err)
	}
	if acct.APIKey != "cr_test123" {
		t.Errorf("expected cr_test123 from disk, got %q", acct.APIKey)
	}

	// Updating API key preserves LighthouseURL
	acctWithURL := mesh.AccountConfig{
		APIKey:        "cr_test123",
		LighthouseURL: "https://lighthouse.example.com",
	}
	pathWithURL := filepath.Join(dir, "account_url.json")
	if err := mesh.SaveAccount(pathWithURL, acctWithURL); err != nil {
		t.Fatalf("save account with URL: %v", err)
	}
	acct, err = resolveAccount(pathWithURL, "cr_newkey")
	if err != nil {
		t.Fatalf("resolveAccount updating key: %v", err)
	}
	if acct.APIKey != "cr_newkey" {
		t.Errorf("expected cr_newkey, got %q", acct.APIKey)
	}
	if acct.LighthouseURL != "https://lighthouse.example.com" {
		t.Errorf("expected LighthouseURL preserved, got %q", acct.LighthouseURL)
	}

	// Missing file without flag returns error
	missingPath := filepath.Join(dir, "missing.json")
	_, err = resolveAccount(missingPath, "")
	if err == nil {
		t.Fatal("expected error for missing account without flag")
	}
}

// TestServiceEndToEnd simulates the add → list → remove cycle with a mock Lighthouse.
func TestServiceEndToEnd(t *testing.T) {
	// Track sites in memory
	sites := make(map[string]lighthouse.Site)
	siteCounter := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth
		if r.Header.Get("Authorization") != "Bearer cr_e2etest" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"detail": "unauthorized"})
			return
		}

		switch {
		case r.Method == "POST" && r.URL.Path == "/v1/sites":
			var req lighthouse.CreateSiteRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"detail":"invalid request body: %s"}`, err.Error())
				return
			}

			siteCounter++
			site := lighthouse.Site{
				ID:     fmt.Sprintf("site_%d", siteCounter),
				Domain: req.Domain,
				Origin: req.Origin,
				Status: lighthouse.SiteStatusPendingDNS,
				TLS:    lighthouse.TLSMode(req.TLS),
			}
			sites[site.ID] = site

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(site)

		case r.Method == "GET" && r.URL.Path == "/v1/sites":
			var siteList []lighthouse.Site
			for _, s := range sites {
				siteList = append(siteList, s)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"sites": siteList})

		case r.Method == "DELETE":
			siteID := r.URL.Path[len("/v1/sites/"):]
			delete(sites, siteID)
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Set up state directory
	stateDir := t.TempDir()
	servicesPath := filepath.Join(stateDir, servicesFileName)
	accountPath := filepath.Join(stateDir, accountFileName)

	// Save account with mock server URL
	acct := mesh.AccountConfig{
		APIKey:        "cr_e2etest",
		LighthouseURL: server.URL,
	}
	if err := mesh.SaveAccount(accountPath, acct); err != nil {
		t.Fatalf("save account: %v", err)
	}

	// --- ADD ---
	client := lighthouse.NewClient(server.URL, "cr_e2etest")
	site, err := client.CreateSite(lighthouse.CreateSiteRequest{
		Domain: "ollama.testmesh123.wgmesh.dev",
		Origin: lighthouse.Origin{
			MeshIP:   "10.42.100.1",
			Port:     11434,
			Protocol: "http",
		},
		TLS: "auto",
	})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	if site.ID == "" {
		t.Fatal("expected non-empty site ID")
	}
	if site.Status != lighthouse.SiteStatusPendingDNS {
		t.Errorf("expected pending_dns, got %s", site.Status)
	}

	// Save local state (as service add would)
	state := mesh.ServiceState{
		Services: map[string]mesh.ServiceEntry{
			"ollama": {
				SiteID:    site.ID,
				Name:      "ollama",
				Domain:    "ollama.testmesh123.wgmesh.dev",
				LocalAddr: ":11434",
				Protocol:  "http",
			},
		},
	}
	if err := mesh.SaveServices(servicesPath, state); err != nil {
		t.Fatalf("save services: %v", err)
	}

	// --- LIST ---
	listedSites, err := client.ListSites()
	if err != nil {
		t.Fatalf("ListSites failed: %v", err)
	}
	if len(listedSites) != 1 {
		t.Fatalf("expected 1 site, got %d", len(listedSites))
	}
	if listedSites[0].Domain != "ollama.testmesh123.wgmesh.dev" {
		t.Errorf("expected domain ollama.testmesh123.wgmesh.dev, got %s", listedSites[0].Domain)
	}

	// Verify local state
	loadedState, err := mesh.LoadServices(servicesPath)
	if err != nil {
		t.Fatalf("load services: %v", err)
	}
	if _, ok := loadedState.Services["ollama"]; !ok {
		t.Fatal("expected ollama in local state")
	}

	// --- REMOVE ---
	if err := client.DeleteSite(site.ID); err != nil {
		t.Fatalf("DeleteSite failed: %v", err)
	}

	// Clean local state
	delete(state.Services, "ollama")
	if err := mesh.SaveServices(servicesPath, state); err != nil {
		t.Fatalf("save services after remove: %v", err)
	}

	// Verify removed from Lighthouse
	listedSites, err = client.ListSites()
	if err != nil {
		t.Fatalf("ListSites after remove failed: %v", err)
	}
	if len(listedSites) != 0 {
		t.Errorf("expected 0 sites after remove, got %d", len(listedSites))
	}

	// Verify removed from local state
	loadedState, err = mesh.LoadServices(servicesPath)
	if err != nil {
		t.Fatalf("load services after remove: %v", err)
	}
	if len(loadedState.Services) != 0 {
		t.Errorf("expected empty local state, got %d entries", len(loadedState.Services))
	}
}

func TestDeriveMeshIPForServiceUsesPersistedState(t *testing.T) {
	secret := "wgmesh://v1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	_, custom, _ := net.ParseCIDR("192.168.50.0/24")

	// Rows "no mesh_ip" and "invalid mesh_ip" cover the
	// fallback derivation from wg_pubkey.
	tests := []struct {
		name   string
		state  string
		subnet *net.IPNet
		want   string // "" means: derived from wg_pubkey in the custom subnet
	}{
		{"persisted mesh_ip kept", `{"wg_pubkey":"PUBKEY","mesh_ip":"10.200.3.4"}`, nil, "10.200.3.4"},
		{"non-10/8 mesh_ip without custom subnet is kept", `{"wg_pubkey":"PUBKEY","mesh_ip":"192.168.100.7"}`, nil, "192.168.100.7"},
		{"IPv4-mapped mesh_ip is normalized", `{"wg_pubkey":"PUBKEY","mesh_ip":"::ffff:10.0.0.5"}`, nil, "10.0.0.5"},
		{"mesh_ip outside custom subnet is re-derived", `{"wg_pubkey":"PUBKEY","mesh_ip":"10.200.3.4"}`, custom, ""},
		{"mesh_ip inside custom subnet kept", `{"wg_pubkey":"PUBKEY","mesh_ip":"192.168.50.9"}`, custom, "192.168.50.9"},
		{"no mesh_ip derives from wg_pubkey", `{"wg_pubkey":"PUBKEY"}`, nil, crypto.DeriveMeshIP(keys.MeshSubnet, "PUBKEY", secret)},
		{"invalid mesh_ip derives from wg_pubkey", `{"wg_pubkey":"PUBKEY","mesh_ip":"nonsense"}`, nil, crypto.DeriveMeshIP(keys.MeshSubnet, "PUBKEY", secret)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "wg0.json"), []byte(tt.state), 0o600); err != nil {
				t.Fatal(err)
			}
			got := deriveMeshIPForService(keys, secret, tt.subnet, dir)
			want := tt.want
			if want == "" {
				w, err := crypto.DeriveMeshIPInSubnet(tt.subnet, "PUBKEY", secret)
				if err != nil {
					t.Fatal(err)
				}
				want = w
			}
			if got != want {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
}

// TestServiceAddReadsResolvedIP: after the daemon lost a collision it wrote
// the nonce-derived address into the state file (T5); service registration
// reports that address, not the natural one.
func TestServiceAddReadsResolvedIP(t *testing.T) {
	secret := "wgmesh://v1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	natural := crypto.DeriveMeshIP(keys.MeshSubnet, "PUBKEY", secret)
	resolved := daemon.DeriveMeshIPWithNonce(keys.MeshSubnet, "PUBKEY", secret, 1)
	if resolved == natural {
		t.Fatalf("test needs distinct addresses, both %s", natural)
	}

	dir := t.TempDir()
	state := fmt.Sprintf(`{"wg_pubkey":"PUBKEY","wg_private_key":"x","mesh_ip":%q}`, resolved)
	if err := os.WriteFile(filepath.Join(dir, "wg0.json"), []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := deriveMeshIPForService(keys, secret, nil, dir); got != resolved {
		t.Errorf("got %s, want the resolved address %s (natural %s)", got, resolved, natural)
	}
}

func TestDeriveMeshIPForServiceNoStateIsPlaceholder(t *testing.T) {
	secret := "wgmesh://v1/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}
	got := deriveMeshIPForService(keys, secret, nil, t.TempDir())
	if want := crypto.DeriveMeshIP(keys.MeshSubnet, "unjoined", secret); got != want {
		t.Errorf("got %s, want placeholder %s", got, want)
	}
}
