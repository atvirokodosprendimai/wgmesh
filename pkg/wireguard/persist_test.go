package wireguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

// wgQuickGolden is the wg-quick text the unchanged generator produced for
// validWgQuickFixture. WGMESH_UPDATE_GOLDEN=1 rewrites it.
const wgQuickGolden = "../../testdata/central/wgquick-valid.golden"

func validWgQuickFixture() (*FullConfig, []ssh.RouteEntry) {
	config := &FullConfig{
		Interface: WGInterface{PrivateKey: "PRIVATEKEYPRIVATEKEYPRIVATEKEYPRIVATEKEY0=", Address: "10.99.0.2/16", ListenPort: 51820},
		Peers: []WGPeer{
			{PublicKey: "PEERONEPEERONEPEERONEPEERONEPEERONEPEER00=", Endpoint: "198.51.100.1:51820", AllowedIPs: []string{"10.99.0.1/32", "192.168.10.0/24"}, PersistentKeepalive: 5},
			{PublicKey: "PEERTWOPEERTWOPEERTWOPEERTWOPEERTWOPEER00=", AllowedIPs: []string{"10.99.0.3/32"}},
		},
	}
	routes := []ssh.RouteEntry{
		{Network: "192.168.10.0/24", Gateway: "10.99.0.1"},
		{Network: "fd00:20::/64", Gateway: "fd00::3"},
	}
	return config, routes
}

func TestWgQuickValidConfigUnchanged(t *testing.T) {
	config, routes := validWgQuickFixture()
	got, err := GenerateWgQuickConfig(config, routes)
	if err != nil {
		t.Fatalf("GenerateWgQuickConfig: %v", err)
	}
	if os.Getenv("WGMESH_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(wgQuickGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(wgQuickGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(wgQuickGolden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Errorf("wg-quick text for the valid fixture differs from %s.\n--- got ---\n%s\n--- want ---\n%s", wgQuickGolden, got, want)
	}
}

func TestWgQuickQuotesRouteValues(t *testing.T) {
	tests := []struct {
		name        string
		route       ssh.RouteEntry
		wantPostUp  string
		wantPreDown string
	}{
		{
			name:        "network with a command",
			route:       ssh.RouteEntry{Network: "10.0.0.0/8; id", Gateway: "10.99.0.1"},
			wantPostUp:  "PostUp = ip route add '10.0.0.0/8; id' via 10.99.0.1 dev %i || true\n",
			wantPreDown: "PreDown = ip route del '10.0.0.0/8; id' via 10.99.0.1 dev %i || true\n",
		},
		{
			name:        "gateway with a quote",
			route:       ssh.RouteEntry{Network: "10.0.0.0/8", Gateway: "x'; id; '"},
			wantPostUp:  `PostUp = ip route add 10.0.0.0/8 via 'x'\''; id; '\''' dev %i || true` + "\n",
			wantPreDown: `PreDown = ip route del 10.0.0.0/8 via 'x'\''; id; '\''' dev %i || true` + "\n",
		},
		{
			name:        "ipv6 network and gateway stay bare",
			route:       ssh.RouteEntry{Network: "fd00:20::/64", Gateway: "fd00::3"},
			wantPostUp:  "PostUp = ip route add fd00:20::/64 via fd00::3 dev %i || true\n",
			wantPreDown: "PreDown = ip route del fd00:20::/64 via fd00::3 dev %i || true\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, _ := validWgQuickFixture()
			got, err := GenerateWgQuickConfig(config, []ssh.RouteEntry{tt.route})
			if err != nil {
				t.Fatalf("GenerateWgQuickConfig: %v", err)
			}
			if !strings.Contains(got, tt.wantPostUp) {
				t.Errorf("PostUp line missing or not quoted, want %q in:\n%s", tt.wantPostUp, got)
			}
			if !strings.Contains(got, tt.wantPreDown) {
				t.Errorf("PreDown line missing or not quoted, want %q in:\n%s", tt.wantPreDown, got)
			}
		})
	}
}

func TestWgQuickRefusesControlCharacters(t *testing.T) {
	fields := []struct {
		name   string
		inject func(config *FullConfig, routes []ssh.RouteEntry, v string)
	}{
		{"address", func(c *FullConfig, _ []ssh.RouteEntry, v string) { c.Interface.Address = v }},
		{"private key", func(c *FullConfig, _ []ssh.RouteEntry, v string) { c.Interface.PrivateKey = v }},
		{"public key", func(c *FullConfig, _ []ssh.RouteEntry, v string) { c.Peers[0].PublicKey = v }},
		{"endpoint", func(c *FullConfig, _ []ssh.RouteEntry, v string) { c.Peers[0].Endpoint = v }},
		{"allowed IP", func(c *FullConfig, _ []ssh.RouteEntry, v string) { c.Peers[1].AllowedIPs[0] = v }},
		{"route network", func(_ *FullConfig, r []ssh.RouteEntry, v string) { r[0].Network = v }},
		{"route gateway", func(_ *FullConfig, r []ssh.RouteEntry, v string) { r[1].Gateway = v }},
	}
	chars := map[string]string{"newline": "\n", "carriage return": "\r", "NUL": "\x00"}

	for _, f := range fields {
		for cname, c := range chars {
			t.Run(f.name+"/"+cname, func(t *testing.T) {
				config, routes := validWgQuickFixture()
				f.inject(config, routes, "a"+c+"PostUp = touch /x")
				got, err := GenerateWgQuickConfig(config, routes)
				if err == nil {
					t.Fatalf("no error for a %s in the %s; text:\n%s", cname, f.name, got)
				}
				if !strings.Contains(err.Error(), f.name) {
					t.Errorf("error %q does not name the field %q", err, f.name)
				}
				if got != "" {
					t.Errorf("text returned with the error: %q", got)
				}
			})
		}
	}
}
