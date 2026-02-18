package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ratelimit"
)

const (
	GossipInterval       = 10 * time.Second
	GossipMethod         = "gossip"
	GossipMaxMessageSize = 65536
)

// MeshGossip handles in-mesh gossip for peer discovery over WireGuard tunnels
type MeshGossip struct {
	config    *daemon.Config
	localNode *LocalNode
	peerStore *daemon.PeerStore
	gossipKey [32]byte
	port      uint16

	conn     *net.UDPConn
	exchange *PeerExchange
	limiter  *ratelimit.IPRateLimiter

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// NewMeshGossip creates a new in-mesh gossip instance
func NewMeshGossip(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) (*MeshGossip, error) {
	return &MeshGossip{
		config:    config,
		localNode: localNode,
		peerStore: peerStore,
		gossipKey: config.Keys.GossipKey,
		port:      config.Keys.GossipPort,
		limiter:   ratelimit.NewDefault(),
		stopCh:    make(chan struct{}),
	}, nil
}

// NewMeshGossipWithExchange creates a new in-mesh gossip instance that reuses the peer exchange socket.
func NewMeshGossipWithExchange(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore, exchange *PeerExchange) (*MeshGossip, error) {
	return &MeshGossip{
		config:    config,
		localNode: localNode,
		peerStore: peerStore,
		gossipKey: config.Keys.GossipKey,
		port:      config.Keys.GossipPort,
		exchange:  exchange,
		limiter:   ratelimit.NewDefault(),
		stopCh:    make(chan struct{}),
	}, nil
}

// Start begins in-mesh gossip
func (g *MeshGossip) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.running {
		return fmt.Errorf("gossip already running")
	}

	if g.exchange != nil {
		g.running = true
		go g.gossipLoop()
		log.Printf("[Gossip] In-mesh gossip started via exchange on port %d", g.port)
		return nil
	}

	// Bind to mesh IP on gossip port
	addr := &net.UDPAddr{
		IP:   net.ParseIP(g.localNode.MeshIP),
		Port: int(g.port),
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		// Fall back to binding on all interfaces if mesh IP binding fails
		addr = &net.UDPAddr{Port: int(g.port)}
		conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return fmt.Errorf("failed to bind gossip port: %w", err)
		}
	}

	// Update port to match actual bound port
	g.port = uint16(conn.LocalAddr().(*net.UDPAddr).Port)

	g.conn = conn
	g.running = true

	go g.listenLoop()
	go g.gossipLoop()

	log.Printf("[Gossip] In-mesh gossip started on %s", addr.String())
	return nil
}

// Stop stops in-mesh gossip
func (g *MeshGossip) Stop() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.running {
		return nil
	}

	g.running = false
	close(g.stopCh)

	if g.conn != nil {
		g.conn.Close()
	}

	log.Printf("[Gossip] In-mesh gossip stopped")
	return nil
}

// gossipLoop periodically exchanges peer information with random peers
func (g *MeshGossip) gossipLoop() {
	ticker := time.NewTicker(GossipInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			g.exchangeWithRandomPeer()
		}
	}
}

// exchangeWithRandomPeer sends our peer list to a random known peer
func (g *MeshGossip) exchangeWithRandomPeer() {
	peers := g.peerStore.GetActive()
	if len(peers) == 0 {
		return
	}

	// Filter to only peers with mesh IPs (exclude ourselves)
	var candidates []*daemon.PeerInfo
	for _, p := range peers {
		if p.WGPubKey != g.localNode.WGPubKey && p.MeshIP != "" {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		return
	}

	// Pick a random peer
	target := candidates[rand.Intn(len(candidates))]

	// Send to the peer's mesh IP on the gossip port
	ip := net.ParseIP(target.MeshIP)
	if ip == nil {
		log.Printf("[Gossip] Invalid mesh IP for peer %s: %s", target.WGPubKey, target.MeshIP)
		return
	}
	targetAddr := &net.UDPAddr{
		IP:   ip,
		Port: int(g.port),
	}

	// When using the exchange socket, delegate sending (exchange builds its own peer list)
	if g.exchange != nil {
		if err := g.exchange.SendAnnounce(targetAddr); err != nil {
			log.Printf("[Gossip] Failed to send to %s: %v", target.MeshIP, err)
		}
		return
	}

	// Standalone mode: build known peers list and send directly
	var knownPeers []crypto.KnownPeer
	for _, p := range peers {
		if p.WGPubKey != target.WGPubKey {
			knownPeers = append(knownPeers, crypto.KnownPeer{
				WGPubKey:   p.WGPubKey,
				Hostname:   p.Hostname,
				MeshIP:     p.MeshIP,
				MeshIPv6:   p.MeshIPv6,
				WGEndpoint: p.Endpoint,
				Introducer: p.Introducer,
				NATType:    p.NATType,
			})
		}
	}

	announcement := crypto.CreateAnnouncement(
		g.localNode.WGPubKey,
		g.localNode.MeshIP,
		g.localNode.GetEndpoint(),
		g.localNode.Introducer,
		g.localNode.RoutableNetworks,
		knownPeers,
		g.localNode.Hostname,
		g.localNode.MeshIPv6,
		string(g.localNode.NATType),
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcement, g.gossipKey)
	if err != nil {
		log.Printf("[Gossip] Failed to seal gossip message: %v", err)
		return
	}

	if _, err := g.conn.WriteToUDP(data, targetAddr); err != nil {
		log.Printf("[Gossip] Failed to send to %s: %v", target.MeshIP, err)
	}
}

// listenLoop listens for gossip messages
func (g *MeshGossip) listenLoop() {
	buf := make([]byte, GossipMaxMessageSize)

	for {
		select {
		case <-g.stopCh:
			return
		default:
		}

		g.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := g.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			g.mu.RLock()
			running := g.running
			g.mu.RUnlock()
			if running {
				log.Printf("[Gossip] Read error: %v", err)
			}
			continue
		}

		// Rate-limit per source IP before decrypting
		if !g.limiter.Allow(remoteAddr.IP.String()) {
			continue
		}

		_, announcement, err := crypto.OpenEnvelope(buf[:n], g.gossipKey)
		if err != nil {
			continue
		}

		// In standalone gossip mode, remoteAddr is the mesh IP + gossip port,
		// not the WireGuard underlay endpoint, so treat this as having no sender.
		_ = remoteAddr
		g.handleAnnouncement(announcement, nil)
	}
}

// HandleAnnounceFrom processes an incoming gossip announcement and source address.
func (g *MeshGossip) HandleAnnounceFrom(announcement *crypto.PeerAnnouncement, sender *net.UDPAddr) {
	g.handleAnnouncement(announcement, sender)
}

func (g *MeshGossip) handleAnnouncement(announcement *crypto.PeerAnnouncement, sender *net.UDPAddr) {
	if announcement == nil {
		return
	}
	if announcement.WGPubKey == g.localNode.WGPubKey {
		return
	}

	endpoint := resolvePeerEndpoint(announcement.WGEndpoint, sender)
	if sender == nil {
		endpoint = normalizeKnownPeerEndpoint(announcement.WGEndpoint)
	}
	endpoint = filterEndpointForConfig(endpoint, g.config.DisableIPv6)

	// Update the sender's info
	peer := &daemon.PeerInfo{
		WGPubKey:         announcement.WGPubKey,
		Hostname:         announcement.Hostname,
		MeshIP:           announcement.MeshIP,
		MeshIPv6:         announcement.MeshIPv6,
		Endpoint:         endpoint,
		Introducer:       announcement.Introducer,
		RoutableNetworks: announcement.RoutableNetworks,
		NATType:          announcement.NATType,
	}
	g.peerStore.Update(peer, GossipMethod)

	// Process transitive peers
	for _, kp := range announcement.KnownPeers {
		if kp.WGPubKey == g.localNode.WGPubKey {
			continue
		}
		transitivePeer := &daemon.PeerInfo{
			WGPubKey:   kp.WGPubKey,
			Hostname:   kp.Hostname,
			MeshIP:     kp.MeshIP,
			MeshIPv6:   kp.MeshIPv6,
			Endpoint:   filterEndpointForConfig(normalizeKnownPeerEndpoint(kp.WGEndpoint), g.config.DisableIPv6),
			Introducer: kp.Introducer,
			NATType:    kp.NATType,
		}
		g.peerStore.Update(transitivePeer, GossipMethod+"-transitive")
	}
}

// MarshalJSON implements json.Marshaler for debugging
func (g *MeshGossip) MarshalJSON() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"port":    g.port,
		"running": g.running,
	})
}
