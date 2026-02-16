package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

const (
	ExchangeTimeout = 10 * time.Second
	MaxExchangeSize = 65536 // 64KB max message size
	ExchangePort    = 51821 // Default exchange port (can be derived from secret)
)

// PeerExchange handles the encrypted peer exchange protocol
type PeerExchange struct {
	config    *daemon.Config
	localNode *LocalNode
	peerStore *daemon.PeerStore

	conn *net.UDPConn
	port int

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}

	pendingMu      sync.Mutex
	pendingReplies map[string]chan *daemon.PeerInfo

	announceHandler func(*crypto.PeerAnnouncement, *net.UDPAddr)
}

// NewPeerExchange creates a new peer exchange handler
func NewPeerExchange(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) *PeerExchange {
	return &PeerExchange{
		config:         config,
		localNode:      localNode,
		peerStore:      peerStore,
		stopCh:         make(chan struct{}),
		pendingReplies: make(map[string]chan *daemon.PeerInfo),
	}
}

// Start starts the peer exchange server
func (pe *PeerExchange) Start() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.running {
		return fmt.Errorf("peer exchange already running")
	}

	// Use gossip port derived from secret
	port := int(pe.config.Keys.GossipPort)

	// Bind UDP socket
	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind UDP port %d: %w", port, err)
	}

	pe.conn = conn
	pe.port = port
	pe.running = true

	// Start listener
	go pe.listenLoop()

	log.Printf("[Exchange] Listening on UDP port %d", port)
	return nil
}

// Stop stops the peer exchange server
func (pe *PeerExchange) Stop() {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if !pe.running {
		return
	}

	pe.running = false
	close(pe.stopCh)

	if pe.conn != nil {
		pe.conn.Close()
	}
}

// Port returns the listening port
func (pe *PeerExchange) Port() int {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.port
}

// UDPConn returns the UDP connection for DHT multiplexing
func (pe *PeerExchange) UDPConn() net.PacketConn {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.conn
}

// listenLoop handles incoming peer exchange requests
func (pe *PeerExchange) listenLoop() {
	buf := make([]byte, MaxExchangeSize)

	for {
		select {
		case <-pe.stopCh:
			return
		default:
		}

		pe.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := pe.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if pe.running {
				log.Printf("[Exchange] Read error: %v", err)
			}
			continue
		}

		// Handle message in goroutine
		data := make([]byte, n)
		copy(data, buf[:n])
		go pe.handleMessage(data, remoteAddr)
	}
}

// handleMessage processes an incoming peer exchange message
func (pe *PeerExchange) handleMessage(data []byte, remoteAddr *net.UDPAddr) {
	// Try to decrypt the message
	envelope, announcement, err := crypto.OpenEnvelope(data, pe.config.Keys.GossipKey)
	if err != nil {
		// Could be a DHT message or wrong key - log for debugging
		log.Printf("[Exchange] Received non-wgmesh packet from %s (len=%d, possibly DHT or wrong secret)", remoteAddr.String(), len(data))
		return
	}

	log.Printf("[Exchange] SUCCESS! Received valid %s from wgmesh peer at %s", envelope.MessageType, remoteAddr.String())

	switch envelope.MessageType {
	case crypto.MessageTypeHello:
		pe.handleHello(announcement, remoteAddr)
	case crypto.MessageTypeReply:
		pe.handleReply(announcement, remoteAddr)
	case crypto.MessageTypeAnnounce:
		pe.mu.RLock()
		handler := pe.announceHandler
		pe.mu.RUnlock()
		if handler != nil {
			handler(announcement, remoteAddr)
		}
	default:
		log.Printf("[Exchange] Unknown message type: %s", envelope.MessageType)
	}
}

// handleHello responds to a peer's HELLO message
func (pe *PeerExchange) handleHello(announcement *crypto.PeerAnnouncement, remoteAddr *net.UDPAddr) {
	// Skip if this is from ourselves
	if announcement.WGPubKey == pe.localNode.WGPubKey {
		return
	}

	// Update peer store with the sender's info
	peerInfo := &daemon.PeerInfo{
		WGPubKey:         announcement.WGPubKey,
		MeshIP:           announcement.MeshIP,
		Endpoint:         resolvePeerEndpoint(announcement.WGEndpoint, remoteAddr),
		RoutableNetworks: announcement.RoutableNetworks,
	}

	pe.peerStore.Update(peerInfo, DHTMethod)

	pe.updateTransitivePeers(announcement.KnownPeers)

	// Send reply
	if err := pe.sendReply(remoteAddr); err != nil {
		log.Printf("[Exchange] Failed to send reply to %s: %v", remoteAddr.String(), err)
	}
}

// handleReply routes a REPLY back to an in-flight exchange request.
func (pe *PeerExchange) handleReply(reply *crypto.PeerAnnouncement, remoteAddr *net.UDPAddr) {
	peerInfo := &daemon.PeerInfo{
		WGPubKey:         reply.WGPubKey,
		MeshIP:           reply.MeshIP,
		Endpoint:         resolvePeerEndpoint(reply.WGEndpoint, remoteAddr),
		RoutableNetworks: reply.RoutableNetworks,
	}

	pe.updateTransitivePeers(reply.KnownPeers)

	if ch, ok := pe.getPendingReplyChannel(remoteAddr.String()); ok {
		select {
		case ch <- peerInfo:
		default:
		}
		return
	}

	log.Printf("[Exchange] Received unsolicited REPLY from %s", remoteAddr.String())
	pe.peerStore.Update(peerInfo, DHTMethod)
}

// sendReply sends a REPLY message to a peer
func (pe *PeerExchange) sendReply(remoteAddr *net.UDPAddr) error {
	// Build list of known peers for transitive discovery
	knownPeers := pe.getKnownPeers()

	announcement := crypto.CreateAnnouncement(
		pe.localNode.WGPubKey,
		pe.localNode.MeshIP,
		pe.localNode.WGEndpoint,
		pe.localNode.RoutableNetworks,
		knownPeers,
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeReply, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal reply: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	return fmt.Errorf("failed to send reply: %w", err)
}

// ExchangeWithPeer initiates a peer exchange with a remote address
func (pe *PeerExchange) ExchangeWithPeer(addrStr string) (*daemon.PeerInfo, error) {
	remoteAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	replyCh := make(chan *daemon.PeerInfo, 1)
	pe.setPendingReplyChannel(remoteAddr.String(), replyCh)
	defer pe.clearPendingReplyChannel(remoteAddr.String())

	// Build list of known peers for transitive discovery
	knownPeers := pe.getKnownPeers()

	// Create HELLO message
	announcement := crypto.CreateAnnouncement(
		pe.localNode.WGPubKey,
		pe.localNode.MeshIP,
		pe.localNode.WGEndpoint,
		pe.localNode.RoutableNetworks,
		knownPeers,
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeHello, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return nil, fmt.Errorf("failed to seal hello: %w", err)
	}

	log.Printf("[Exchange] Sending HELLO to %s (our exchange port: %d)", remoteAddr.String(), pe.port)

	// Send HELLO
	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send hello: %w", err)
	}

	select {
	case peerInfo := <-replyCh:
		return peerInfo, nil
	case <-time.After(ExchangeTimeout):
		return nil, fmt.Errorf("exchange timeout")
	}
}

func (pe *PeerExchange) updateTransitivePeers(knownPeers []crypto.KnownPeer) {
	for _, kp := range knownPeers {
		if kp.WGPubKey == pe.localNode.WGPubKey {
			continue
		}
		transitivePeer := &daemon.PeerInfo{
			WGPubKey: kp.WGPubKey,
			MeshIP:   kp.MeshIP,
			Endpoint: normalizeKnownPeerEndpoint(kp.WGEndpoint),
		}
		pe.peerStore.Update(transitivePeer, DHTMethod+"-transitive")
	}
}

func (pe *PeerExchange) setPendingReplyChannel(remote string, ch chan *daemon.PeerInfo) {
	pe.pendingMu.Lock()
	defer pe.pendingMu.Unlock()
	pe.pendingReplies[remote] = ch
}

func (pe *PeerExchange) clearPendingReplyChannel(remote string) {
	pe.pendingMu.Lock()
	defer pe.pendingMu.Unlock()
	delete(pe.pendingReplies, remote)
}

func (pe *PeerExchange) getPendingReplyChannel(remote string) (chan *daemon.PeerInfo, bool) {
	pe.pendingMu.Lock()
	defer pe.pendingMu.Unlock()
	ch, ok := pe.pendingReplies[remote]
	return ch, ok
}

func resolvePeerEndpoint(advertised string, sender *net.UDPAddr) string {
	if host, port, err := net.SplitHostPort(advertised); err == nil {
		resolvedHost := host
		if resolvedHost == "" || resolvedHost == "0.0.0.0" || resolvedHost == "::" {
			if sender != nil && sender.IP != nil {
				resolvedHost = sender.IP.String()
			}
		}
		if resolvedHost != "" {
			return net.JoinHostPort(resolvedHost, port)
		}
	}

	if sender != nil && sender.IP != nil {
		return net.JoinHostPort(sender.IP.String(), strconv.Itoa(daemon.DefaultWGPort))
	}

	return ""
}

func normalizeKnownPeerEndpoint(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return ""
	}
	return endpoint
}

// getKnownPeers returns a list of known peers for sharing with other nodes
func (pe *PeerExchange) getKnownPeers() []crypto.KnownPeer {
	peers := pe.peerStore.GetActive()
	knownPeers := make([]crypto.KnownPeer, 0, len(peers))

	for _, p := range peers {
		knownPeers = append(knownPeers, crypto.KnownPeer{
			WGPubKey:   p.WGPubKey,
			MeshIP:     p.MeshIP,
			WGEndpoint: p.Endpoint,
		})
	}

	return knownPeers
}

// SendAnnounce sends an announce message to a specific peer (used for gossip)
func (pe *PeerExchange) SendAnnounce(remoteAddr *net.UDPAddr) error {
	knownPeers := pe.getKnownPeers()

	announcement := crypto.CreateAnnouncement(
		pe.localNode.WGPubKey,
		pe.localNode.MeshIP,
		pe.localNode.WGEndpoint,
		pe.localNode.RoutableNetworks,
		knownPeers,
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal announce: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	return fmt.Errorf("failed to send announce: %w", err)
}

// SetAnnounceHandler sets a handler for gossip announcements.
func (pe *PeerExchange) SetAnnounceHandler(handler func(*crypto.PeerAnnouncement, *net.UDPAddr)) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.announceHandler = handler
}

// MarshalJSON implements json.Marshaler for debugging
func (pe *PeerExchange) MarshalJSON() ([]byte, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"port":    pe.port,
		"running": pe.running,
	})
}
