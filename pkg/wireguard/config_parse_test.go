package wireguard

import "testing"

// TestParseWGConfig covers the real wg dump format. The interface line is
// "private_key listen_port [fwmark]"; each peer row is
// "public_key preshared_key endpoint allowed_ips latest_handshake transfer_rx
// transfer_tx persistent_keepalive". Field 5 (index 4) is latest_handshake, a
// unix timestamp — it must never leak into PersistentKeepalive.
func TestParseWGConfig(t *testing.T) {
	dump := "privkey1234\t51820\n" +
		"peer1pubkey\t(none)\t192.168.1.10:51820\t10.99.0.2/32\t1724928000\t123456\t654321\t25\n" +
		"peer2pubkey\tpsk2\t(none)\t10.99.0.3/32\t0\t0\t0\t0\n"

	cfg, err := parseWGConfig(dump)
	if err != nil {
		t.Fatalf("parseWGConfig() error = %v", err)
	}
	if cfg.Interface.PrivateKey != "privkey1234" {
		t.Errorf("PrivateKey = %q, want privkey1234", cfg.Interface.PrivateKey)
	}
	if cfg.Interface.ListenPort != 51820 {
		t.Errorf("ListenPort = %d, want 51820", cfg.Interface.ListenPort)
	}
	if len(cfg.Peers) != 2 {
		t.Fatalf("len(Peers) = %d, want 2", len(cfg.Peers))
	}

	p1 := cfg.Peers["peer1pubkey"]
	if p1.PersistentKeepalive != 25 {
		t.Errorf("peer1 PersistentKeepalive = %d, want 25 (latest_handshake must not leak in)", p1.PersistentKeepalive)
	}
	if p1.Endpoint != "192.168.1.10:51820" {
		t.Errorf("peer1 Endpoint = %q, want 192.168.1.10:51820", p1.Endpoint)
	}
	if len(p1.AllowedIPs) != 1 || p1.AllowedIPs[0] != "10.99.0.2/32" {
		t.Errorf("peer1 AllowedIPs = %v, want [10.99.0.2/32]", p1.AllowedIPs)
	}

	p2 := cfg.Peers["peer2pubkey"]
	if p2.PersistentKeepalive != 0 {
		t.Errorf("peer2 PersistentKeepalive = %d, want 0", p2.PersistentKeepalive)
	}
	if p2.PresharedKey != "psk2" {
		t.Errorf("peer2 PresharedKey = %q, want psk2", p2.PresharedKey)
	}
}

func TestParseWGConfig_EmptyOutput(t *testing.T) {
	if _, err := parseWGConfig(""); err == nil {
		t.Error("parseWGConfig(\"\") = nil error, want error")
	}
}
