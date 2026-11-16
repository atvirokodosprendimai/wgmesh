package node

// Words: the local node's mutable fields (MeshIP, MeshIPv6, NATType,
// Introducer) are read and written only through accessors that hold stateMu;
// the routes and the endpoint keep their own locks; Snapshot() copies each
// group under its own lock, in turn, never nested.
//
// Why: reconcile writes MeshIP, STUN goroutines write NATType, the SIGHUP
// handler writes the routes, and exchange, LAN, gossip, probe and admit
// goroutines read them; an announcement was assembled from fields read at
// different moments (ADR-0005 T8, M 2026-11-16: every access of a shared field
// is atomic or locked, reads too).
//
// Invariant: these tests must be clean under -race.

import (
	"fmt"
	"sync"
	"testing"
)

func TestLocalNodeAccessorsConcurrent(t *testing.T) {
	n := &LocalNode{WGPubKey: "pub", Hostname: "h"}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				v := fmt.Sprintf("%d-%d", g, i)
				n.SetMeshIP("10.0." + v)
				n.SetMeshIPv6("fd00::" + v)
				n.SetNATType("cone")
				n.SetIntroducer(i%2 == 0)
				n.SetRoutableNetworks([]string{"192.168." + v + ".0/24"})
				n.SetEndpoint("198.51.100.1:" + v)
				_ = n.GetMeshIP()
				_ = n.GetMeshIPv6()
				_ = n.GetNATType()
				_ = n.IsIntroducer()
				_ = n.GetRoutableNetworks()
				_ = n.GetEndpoint()
				if key, _ := n.Identity(); key != "pub" {
					t.Errorf("Identity key = %q", key)
				}
				if s := n.Snapshot(); s.PubKey != "pub" || s.Hostname != "h" {
					t.Errorf("Snapshot write-once fields = %q %q", s.PubKey, s.Hostname)
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestRoutableNetworksAreCopied(t *testing.T) {
	n := &LocalNode{}
	in := []string{"10.1.0.0/16", "10.2.0.0/16"}
	n.SetRoutableNetworks(in)
	in[0] = "changed"
	got := n.GetRoutableNetworks()
	if got[0] != "10.1.0.0/16" {
		t.Fatalf("setter kept the caller's slice: %v", got)
	}
	got[1] = "changed"
	if again := n.GetRoutableNetworks(); again[1] != "10.2.0.0/16" {
		t.Fatalf("getter returned the internal slice: %v", again)
	}
}

func TestSnapshotMatchesAccessors(t *testing.T) {
	n := &LocalNode{WGPubKey: "pub", Hostname: "host"}
	n.SetMeshIP("10.0.0.1")
	n.SetMeshIPv6("fd00::1")
	n.SetNATType("symmetric")
	n.SetIntroducer(true)
	n.SetRoutableNetworks([]string{"10.9.0.0/16"})
	n.SetEndpoint("198.51.100.1:51820")

	s := n.Snapshot()
	if s.MeshIP != n.GetMeshIP() || s.MeshIPv6 != n.GetMeshIPv6() || s.NATType != n.GetNATType() ||
		s.Introducer != n.IsIntroducer() || s.Endpoint != n.GetEndpoint() ||
		s.PubKey != "pub" || s.Hostname != "host" {
		t.Fatalf("Snapshot %+v differs from the accessors", s)
	}
	if len(s.Routes) != 1 || s.Routes[0] != "10.9.0.0/16" {
		t.Fatalf("Snapshot routes = %v", s.Routes)
	}
	s.Routes[0] = "changed"
	n.SetRoutableNetworks([]string{"10.8.0.0/16"})
	if got := n.GetRoutableNetworks(); got[0] != "10.8.0.0/16" {
		t.Fatalf("snapshot routes alias the node: %v", got)
	}
	if s2 := n.Snapshot(); s2.Routes[0] != "10.8.0.0/16" {
		t.Fatalf("second snapshot = %v", s2.Routes)
	}
}

func TestIdentityReadsKeyAndIPTogether(t *testing.T) {
	n := &LocalNode{WGPubKey: "pub"}
	n.SetMeshIP("10.0.0.7")
	key, ip := n.Identity()
	if key != "pub" || ip != "10.0.0.7" {
		t.Fatalf("Identity = %q %q", key, ip)
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				ip := fmt.Sprintf("10.0.%d.%d", g, i%250)
				n.SetMeshIP(ip)
				if k, got := n.Identity(); k != "pub" || got == "" {
					t.Errorf("Identity = %q %q", k, got)
				}
			}
		}(g)
	}
	wg.Wait()
}
