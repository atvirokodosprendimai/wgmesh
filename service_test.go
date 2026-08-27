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

// Issue #827: the persisted mesh IP is authoritative for service registration.
// deriveMeshIPForService must return the IP saved by the daemon — including
// one adjusted by collision-nonce re-derivation — instead of always re-deriving.

const serviceIPTestSecretURI = "wgmesh://v1/SGVsbG8gV29ybGQhIFRoaXMgaXMgYSB0ZXN0IHNlY3JldCB0aGF0IGlzIGxvbmcgZW5vdWdoIGZvciB0aGUga2V5IGRlcml2YXRpb24u"

const serviceIPTestPubKey = "service-ip-test-pubkey"

// writeServiceIPTestState writes a daemon-format local node state file.
func writeServiceIPTestState(t *testing.T, path, pubKey, meshIP string) {
	t.Helper()

	state := struct {
		WGPubKey     string `json:"wg_pubkey"`
		WGPrivateKey string `json:"wg_private_key"`
		MeshIP       string `json:"mesh_ip,omitempty"`
	}{
		WGPubKey:     pubKey,
		WGPrivateKey: "service-ip-test-private-key",
		MeshIP:       meshIP,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func TestDeriveMeshIPForService_PersistedIPAuthoritative(t *testing.T) {
	secret := resolveSecret(serviceIPTestSecretURI)
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	plainIP := crypto.DeriveMeshIP(keys.MeshSubnet, serviceIPTestPubKey, secret)
	nonceIP := daemon.DeriveMeshIPWithNonce(keys.MeshSubnet, serviceIPTestPubKey, secret, 1)
	if nonceIP == plainIP {
		t.Fatalf("test setup: nonce-derived IP equals plain derivation (%s)", plainIP)
	}

	_, customNet, err := net.ParseCIDR("192.168.100.0/24")
	if err != nil {
		t.Fatalf("parse custom subnet: %v", err)
	}
	customPlainIP, err := crypto.DeriveMeshIPInSubnet(customNet, serviceIPTestPubKey, secret)
	if err != nil {
		t.Fatalf("DeriveMeshIPInSubnet: %v", err)
	}

	tests := []struct {
		name         string
		persistedIP  string
		customSubnet *net.IPNet
		want         string
	}{
		{
			name:        "persisted plain IP returned",
			persistedIP: plainIP,
			want:        plainIP,
		},
		{
			name:        "persisted collision-nonce IP returned (differs from plain derivation)",
			persistedIP: nonceIP,
			want:        nonceIP,
		},
		{
			name:         "persisted IP inside custom subnet returned",
			persistedIP:  "192.168.100.55",
			customSubnet: customNet,
			want:         "192.168.100.55",
		},
		{
			name:         "persisted legacy IP outside custom subnet re-derived",
			persistedIP:  plainIP,
			customSubnet: customNet,
			want:         customPlainIP,
		},
		{
			name: "persisted IP outside changed legacy subnet re-derived",
			// 172.16/12 can never fall inside the legacy 10.x/16 derivation space.
			persistedIP: "172.16.5.5",
			want:        plainIP,
		},
		{
			name:        "state without mesh_ip falls back to pubkey derivation",
			persistedIP: "",
			want:        plainIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statePath := filepath.Join(t.TempDir(), "wg0.json")
			writeServiceIPTestState(t, statePath, serviceIPTestPubKey, tt.persistedIP)

			got := deriveMeshIPForService(keys, secret, tt.customSubnet, statePath)
			if got != tt.want {
				t.Errorf("deriveMeshIPForService = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveMeshIPForService_MissingStateFallsBackToPlaceholder(t *testing.T) {
	secret := resolveSecret(serviceIPTestSecretURI)
	keys, err := crypto.DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	statePath := filepath.Join(t.TempDir(), "does-not-exist.json")
	got := deriveMeshIPForService(keys, secret, nil, statePath)
	if want := crypto.DeriveMeshIP(keys.MeshSubnet, "unjoined", secret); got != want {
		t.Errorf("deriveMeshIPForService = %q, want placeholder %q", got, want)
	}
}
