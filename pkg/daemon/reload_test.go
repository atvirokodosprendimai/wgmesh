package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// --- LoadReloadFile tests ---

func TestLoadReloadFile_AdvertiseRoutes(t *testing.T) {
	t.Parallel()
	f := writeTempReload(t, "advertise-routes=10.0.0.0/8,192.168.1.0/24\n")
	opts, err := LoadReloadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"10.0.0.0/8", "192.168.1.0/24"}
	if !reflect.DeepEqual(opts.AdvertiseRoutes, want) {
		t.Errorf("AdvertiseRoutes = %v, want %v", opts.AdvertiseRoutes, want)
	}
}

func TestLoadReloadFile_AdvertiseRoutesNormalized(t *testing.T) {
	t.Parallel()
	f := writeTempReload(t, "advertise-routes=192.168.1.10/24, 10.0.0.0/8,10.0.0.0/8,\n")
	opts, err := LoadReloadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"192.168.1.0/24", "10.0.0.0/8"}
	if !reflect.DeepEqual(opts.AdvertiseRoutes, want) {
		t.Errorf("AdvertiseRoutes = %v, want %v", opts.AdvertiseRoutes, want)
	}
}

func TestLoadReloadFile_InvalidRouteRejected(t *testing.T) {
	t.Parallel()
	f := writeTempReload(t, "log-level=debug\nadvertise-routes=10.0.0.0/8,192.168.1.0\n")
	if _, err := LoadReloadFile(f); err == nil {
		t.Fatal("expected error for bare IP in advertise-routes")
	}
}

func TestLoadReloadFile_LogLevel(t *testing.T) {
	t.Parallel()
	f := writeTempReload(t, "log-level=debug\n")
	opts, err := LoadReloadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", opts.LogLevel, "debug")
	}
}

func TestLoadReloadFile_EmptyRoutes(t *testing.T) {
	t.Parallel()
	f := writeTempReload(t, "advertise-routes=\n")
	opts, err := LoadReloadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.AdvertiseRoutes) != 0 {
		t.Errorf("expected empty AdvertiseRoutes, got %v", opts.AdvertiseRoutes)
	}
}

func TestLoadReloadFile_CommentsIgnored(t *testing.T) {
	t.Parallel()
	content := "# this is a comment\nlog-level=warn\n# another comment\n"
	f := writeTempReload(t, content)
	opts, err := LoadReloadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want %q", opts.LogLevel, "warn")
	}
}

func TestLoadReloadFile_UnknownKeysIgnored(t *testing.T) {
	t.Parallel()
	f := writeTempReload(t, "unknown-key=value\nlog-level=error\n")
	opts, err := LoadReloadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want %q", opts.LogLevel, "error")
	}
}

func TestLoadReloadFile_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := LoadReloadFile("/nonexistent/path/wg0.reload")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// --- routeSlicesEqual tests ---

func TestRouteSlicesEqual(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"both empty", []string{}, []string{}, true},
		{"equal single", []string{"10.0.0.0/8"}, []string{"10.0.0.0/8"}, true},
		{"equal multi order-independent", []string{"a", "b"}, []string{"b", "a"}, true},
		{"different lengths", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a"}, []string{"b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := routeSlicesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("routeSlicesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// --- reloadConfig / GetAdvertiseRoutes / GetLogLevel tests ---

func TestReloadConfig_UpdatesLogLevel(t *testing.T) {
	t.Parallel()
	d := newMinimalDaemon(t)
	d.config.LogLevel = "info"

	d.reloadConfig(DaemonOpts{LogLevel: "debug", AdvertiseRoutes: []string{}})

	if d.GetLogLevel() != "debug" {
		t.Errorf("LogLevel = %q, want %q", d.GetLogLevel(), "debug")
	}
}

func TestReloadConfig_UpdatesAdvertiseRoutes(t *testing.T) {
	t.Parallel()
	d := newMinimalDaemon(t)
	d.config.AdvertiseRoutes = []string{"10.0.0.0/8"}
	d.localNode = &LocalNode{RoutableNetworks: []string{"10.0.0.0/8"}}

	newRoutes := []string{"192.168.1.0/24", "172.16.0.0/12"}
	d.reloadConfig(DaemonOpts{LogLevel: "info", AdvertiseRoutes: newRoutes})

	got := d.GetAdvertiseRoutes()
	if !reflect.DeepEqual(got, newRoutes) {
		t.Errorf("AdvertiseRoutes = %v, want %v", got, newRoutes)
	}
	if !reflect.DeepEqual(d.localNode.RoutableNetworks, newRoutes) {
		t.Errorf("localNode.RoutableNetworks = %v, want %v", d.localNode.RoutableNetworks, newRoutes)
	}
}

func TestReloadConfig_NoChangeIsNoop(t *testing.T) {
	t.Parallel()
	d := newMinimalDaemon(t)
	d.config.LogLevel = "info"
	d.config.AdvertiseRoutes = []string{"10.0.0.0/8"}
	d.localNode = &LocalNode{RoutableNetworks: []string{"10.0.0.0/8"}}

	// Reload with same values — should not panic or error
	d.reloadConfig(DaemonOpts{LogLevel: "info", AdvertiseRoutes: []string{"10.0.0.0/8"}})

	if d.GetLogLevel() != "info" {
		t.Errorf("LogLevel changed unexpectedly to %q", d.GetLogLevel())
	}
}

func TestReloadConfig_EmptyLogLevelPreservesExisting(t *testing.T) {
	t.Parallel()
	d := newMinimalDaemon(t)
	d.config.LogLevel = "warn"

	// Empty LogLevel in opts should not overwrite
	d.reloadConfig(DaemonOpts{LogLevel: "", AdvertiseRoutes: nil})

	if d.GetLogLevel() != "warn" {
		t.Errorf("LogLevel = %q, want %q", d.GetLogLevel(), "warn")
	}
}

func TestReloadConfig_NilAdvertiseRoutesPreservesExisting(t *testing.T) {
	t.Parallel()
	d := newMinimalDaemon(t)
	existing := []string{"10.0.0.0/8", "172.16.0.0/12"}
	d.config.AdvertiseRoutes = existing

	// Reload file contains only log-level — AdvertiseRoutes should be nil and
	// must NOT overwrite existing routes.
	path := writeTempReload(t, "log-level=debug\n")
	opts, err := LoadReloadFile(path)
	if err != nil {
		t.Fatalf("LoadReloadFile: %v", err)
	}
	if opts.AdvertiseRoutes != nil {
		t.Fatalf("expected nil AdvertiseRoutes from log-level-only file, got %v", opts.AdvertiseRoutes)
	}
	d.reloadConfig(opts)

	got := d.GetAdvertiseRoutes()
	if len(got) != len(existing) {
		t.Errorf("AdvertiseRoutes = %v, want %v (nil in opts must not clobber existing)", got, existing)
	}
}

// --- handleSIGHUP tests ---

func TestHandleSIGHUP_MissingFileIsNoop(t *testing.T) {
	t.Parallel()
	d := newMinimalDaemon(t)
	d.config.InterfaceName = "wg-test-nonexistent"
	d.config.LogLevel = "info"

	// Should not panic; missing reload file is a no-op
	d.handleSIGHUP()

	if d.GetLogLevel() != "info" {
		t.Errorf("LogLevel changed unexpectedly after missing-file SIGHUP: %q", d.GetLogLevel())
	}
}

func TestHandleSIGHUP_LoadsReloadFile(t *testing.T) {
	d := newMinimalDaemon(t) // no t.Parallel() — uses global cmdExecutor via reconcile path
	d.config.LogLevel = "info"
	d.config.AdvertiseRoutes = []string{}
	d.localNode = &LocalNode{RoutableNetworks: []string{}}

	// Write reload file to /tmp so we don't need /var/lib/wgmesh
	dir := t.TempDir()
	iface := "wg-sighup-test"
	reloadPath := filepath.Join(dir, iface+".reload")
	if err := os.WriteFile(reloadPath, []byte("log-level=debug\nadvertise-routes=10.0.0.0/8\n"), 0o600); err != nil {
		t.Fatalf("write reload file: %v", err)
	}

	// Monkey-patch ReloadConfigPath by overriding config interface name to
	// match the file we created in a temp dir, but since ReloadConfigPath is a
	// package-level function we write the file where it will look.
	// Instead, call LoadReloadFile + reloadConfig directly (same code path as
	// handleSIGHUP minus the path resolution) to avoid needing /var/lib/wgmesh.
	opts, err := LoadReloadFile(reloadPath)
	if err != nil {
		t.Fatalf("LoadReloadFile: %v", err)
	}
	d.reloadConfig(opts)

	if d.GetLogLevel() != "debug" {
		t.Errorf("LogLevel = %q, want debug", d.GetLogLevel())
	}
	got := d.GetAdvertiseRoutes()
	if len(got) != 1 || got[0] != "10.0.0.0/8" {
		t.Errorf("AdvertiseRoutes = %v, want [10.0.0.0/8]", got)
	}
}

// --- helpers ---

// writeTempReload writes content to a temp file and returns its path.
func writeTempReload(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "reload-*.conf")
	if err != nil {
		t.Fatalf("create temp reload file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp reload file: %v", err)
	}
	_ = f.Close()
	return f.Name()
}

// newMinimalDaemon creates a daemon with just enough state for reload tests.
func newMinimalDaemon(t *testing.T) *Daemon {
	t.Helper()
	cfg, err := NewConfig(DaemonOpts{Secret: testConfigSecret})
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	d := &Daemon{
		config:                 cfg,
		peerStore:              NewPeerStore(),
		relayRoutes:            make(map[string]string),
		temporaryOffline:       make(map[string]time.Time),
		lastAppliedPeerConfigs: make(map[string]string),
	}
	return d
}

// TestAcceptRoutesReloadApplies: the reload file sets accept-routes and a later
// offer is judged by the new list; an empty value opens the node again.
func TestAcceptRoutesReloadApplies(t *testing.T) {
	old := node.EnforceRouteBinding
	node.EnforceRouteBinding = true
	t.Cleanup(func() { node.EnforceRouteBinding = old })

	d := newMinimalDaemon(t)
	d.peerStore.SetPolicy(d.admitPolicy())
	offer := func(key string) Decision {
		return d.peerStore.Offer(Candidate{
			Info:   &PeerInfo{WGPubKey: key, MeshIP: "10.99.1.1", RoutableNetworks: []string{"192.168.5.0/24", "172.16.0.0/12"}},
			Source: "dht",
			Hand:   FirstHand,
		})
	}
	reload := func(content string) {
		opts, err := LoadReloadFile(writeTempReload(t, content))
		if err != nil {
			t.Fatalf("LoadReloadFile: %v", err)
		}
		d.reloadConfig(opts)
	}

	if dec := offer("before"); dec.Verdict != node.Admit || len(dec.Dropped) != 0 {
		t.Fatalf("with no list: decision = %+v, want a plain Admit", dec)
	}
	reload("accept-routes=192.168.0.0/16\n")
	if dec := offer("during"); dec.Verdict != node.AdmitPartial || !reflect.DeepEqual(dec.Dropped, []string{"172.16.0.0/12"}) {
		t.Fatalf("with the list: decision = %+v, want 172.16.0.0/12 dropped", dec)
	}
	reload("accept-routes=\n")
	if dec := offer("after"); dec.Verdict != node.Admit || len(dec.Dropped) != 0 {
		t.Fatalf("after clearing the list: decision = %+v, want a plain Admit", dec)
	}
}

func TestLoadReloadFile_AcceptRoutes(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"accept-routes=10.0.0.5/8,bogus\n", "accept-routes=,\n", "accept-routes= ,  \n"} {
		if _, err := LoadReloadFile(writeTempReload(t, bad)); err == nil || !strings.Contains(err.Error(), "invalid accept-routes") {
			t.Errorf("LoadReloadFile(%q) err = %v, want an invalid accept-routes error", bad, err)
		}
	}
	// an empty value clears the list: non-nil and empty, unlike an absent key
	opts, err := LoadReloadFile(writeTempReload(t, "accept-routes=\n"))
	if err != nil || opts.AcceptRoutes == nil || len(opts.AcceptRoutes) != 0 {
		t.Errorf("accept-routes= : AcceptRoutes = %#v, err = %v, want non-nil empty", opts.AcceptRoutes, err)
	}
	opts, err = LoadReloadFile(writeTempReload(t, "log-level=debug\n"))
	if err != nil || opts.AcceptRoutes != nil {
		t.Errorf("absent key: AcceptRoutes = %#v, err = %v, want nil", opts.AcceptRoutes, err)
	}
}
