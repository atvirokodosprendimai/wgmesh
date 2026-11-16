package wireguard

import (
	"os"
	"reflect"
	"testing"
)

func TestParseDump(t *testing.T) {
	raw, err := os.ReadFile("testdata/dump.txt")
	if err != nil {
		t.Fatal(err)
	}
	peers, err := ParseDump(string(raw))
	if err != nil {
		t.Fatalf("ParseDump(fixture): %v", err)
	}
	if len(peers) != 2 {
		t.Fatalf("got %d peers, want 2 (the interface line is not a peer): %v", len(peers), peers)
	}

	a, ok := peers["PRkiAlzVWkUp1JfrtygDbjkXjGMJ/sxESxQt+rYuAkM="]
	if !ok {
		t.Fatalf("peer with endpoint missing: %v", peers)
	}
	if a.Endpoint != "203.0.113.7:51820" {
		t.Errorf("endpoint = %q", a.Endpoint)
	}
	if want := []string{"10.99.0.2/32", "192.168.50.0/24"}; !reflect.DeepEqual(a.AllowedIPs, want) {
		t.Errorf("allowed = %v, want %v", a.AllowedIPs, want)
	}
	if a.PresharedKey != "" || a.Keepalive != 25 || a.LatestHandshake != 0 {
		t.Errorf("psk %q keepalive %d handshake %d, want empty, 25, 0", a.PresharedKey, a.Keepalive, a.LatestHandshake)
	}

	b, ok := peers["NfLtlc9KgzZVeNg6mkON8vFbfs+zkJEY6Junb0+o70Q="]
	if !ok {
		t.Fatalf("peer with (none) fields missing: %v", peers)
	}
	if b.Endpoint != "" || len(b.AllowedIPs) != 0 || b.PresharedKey != "" || b.Keepalive != 0 {
		t.Errorf("(none) fields not empty: %+v", b)
	}

	t.Run("numbers", func(t *testing.T) {
		line := "K1=\t(none)\t1.2.3.4:5\t10.0.0.1/32\t1700000000\t11\t22\toff\n"
		got, err := ParseDump("(none)\t(none)\t51820\toff\n" + line)
		if err != nil {
			t.Fatal(err)
		}
		p := got["K1="]
		if p.LatestHandshake != 1700000000 || p.RxBytes != 11 || p.TxBytes != 22 || p.Keepalive != 0 {
			t.Errorf("%+v", p)
		}
	})

	for name, in := range map[string]string{
		"empty":            "",
		"short peer line":  "(none)\t(none)\t51820\toff\nK1=\t(none)\t1.2.3.4:5\n",
		"bad handshake":    "(none)\t(none)\t51820\toff\nK1=\t(none)\t(none)\t(none)\tsoon\t0\t0\toff\n",
		"3-field iface":    "(none)\t(none)\t51820\n",
		"7-field peer":     "(none)\t(none)\t51820\toff\nK1=\t(none)\t(none)\t(none)\t0\t0\t0\n",
		"rx not a number":  "(none)\t(none)\t51820\toff\nK1=\t(none)\t(none)\t(none)\t0\tabc\t0\toff\n",
		"tx not a number":  "(none)\t(none)\t51820\toff\nK1=\t(none)\t(none)\t(none)\t0\t0\tabc\toff\n",
		"keepalive soon":   "(none)\t(none)\t51820\toff\nK1=\t(none)\t(none)\t(none)\t0\t0\t0\tsoon\n",
		"short iface line": "(none)\n",
	} {
		if _, err := ParseDump(in); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if got, err := ParseDump("(none)\t(none)\t51820\toff\n"); err != nil || len(got) != 0 {
		t.Errorf("interface line only: %v, %v; want no peers and no error", got, err)
	}
}
