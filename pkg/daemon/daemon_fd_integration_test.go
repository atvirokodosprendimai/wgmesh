package daemon

import (
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

func TestSetupWireGuardWithVPNFD(t *testing.T) {
	privateKey, _, err := wireguard.GenerateKeyPair()
	if err != nil {
		t.Skipf("wg binary not available: %v", err)
	}

	privKeyBytes, err := wireguard.ParseKey(privateKey)
	if err != nil {
		t.Fatalf("ParseKey() failed: %v", err)
	}

	cfg, err := NewConfig(DaemonOpts{
		Secret:        testConfigSecret,
		VPNFD:         100,
		TunPrivateKey: privKeyBytes,
	})
	if err != nil {
		t.Fatalf("NewConfig() failed: %v", err)
	}

	d, err := NewDaemon(cfg)
	if err != nil {
		t.Fatalf("NewDaemon() failed: %v", err)
	}

	d.localNode = &LocalNode{WGPrivateKey: privateKey}

	if err := d.setupWireGuard(); err != nil {
		t.Fatalf("setupWireGuard() failed: %v", err)
	}
	defer d.teardownWireGuard()

	fdDevice, ok := d.wgDevice.(*wireguard.FDDevice)
	if !ok {
		t.Fatalf("expected FDDevice, got %T", d.wgDevice)
	}

	if peers, err := fdDevice.GetPeers(); err != nil {
		t.Fatalf("GetPeers() failed: %v", err)
	} else if len(peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(peers))
	}
}
