package wireguard

import (
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/ssh"
)

func TestGenerateWgQuickConfigRoutes(t *testing.T) {
	cfg := &FullConfig{
		Interface: WGInterface{PrivateKey: "PRIV", Address: "10.0.0.1/24", ListenPort: 51820},
		Peers: []WGPeer{{
			PublicKey:           "keyA",
			Endpoint:            "203.0.113.5:51820",
			AllowedIPs:          []string{"10.0.0.2/32", "10.0.0.3/32"},
			PersistentKeepalive: 25,
		}},
	}
	const peerBlock = "[Peer]\n" +
		"PublicKey = keyA\n" +
		"Endpoint = 203.0.113.5:51820\n" +
		"AllowedIPs = 10.0.0.2/32, 10.0.0.3/32\n" +
		"PersistentKeepalive = 25\n" +
		"\n"

	t.Run("without routes", func(t *testing.T) {
		want := "[Interface]\n" +
			"Address = 10.0.0.1/24\n" +
			"ListenPort = 51820\n" +
			"PrivateKey = PRIV\n" +
			"PostUp = sysctl -w net.ipv4.ip_forward=1\n" +
			"\n" + peerBlock
		if got := GenerateWgQuickConfig(cfg, nil); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("two routes", func(t *testing.T) {
		routes := []ssh.RouteEntry{
			{Network: "192.168.1.0/24", Gateway: "10.0.0.2"},
			{Network: "172.16.0.0/16", Gateway: "10.0.0.3"},
		}
		want := "[Interface]\n" +
			"Address = 10.0.0.1/24\n" +
			"ListenPort = 51820\n" +
			"PrivateKey = PRIV\n" +
			"PostUp = ip route add 192.168.1.0/24 via 10.0.0.2 dev %i || true\n" +
			"PostUp = ip route add 172.16.0.0/16 via 10.0.0.3 dev %i || true\n" +
			"PreDown = ip route del 192.168.1.0/24 via 10.0.0.2 dev %i || true\n" +
			"PreDown = ip route del 172.16.0.0/16 via 10.0.0.3 dev %i || true\n" +
			"PostUp = sysctl -w net.ipv4.ip_forward=1\n" +
			"\n" + peerBlock
		if got := GenerateWgQuickConfig(cfg, routes); got != want {
			t.Fatalf("got:\n%s\nwant:\n%s", got, want)
		}
	})
}
