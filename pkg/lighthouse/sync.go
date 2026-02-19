package lighthouse

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// SyncPort is the UDP port for lighthouse state replication.
	// Separate from the mesh gossip port to avoid interference.
	SyncPort = 51821

	// SyncInterval is how often we push full state to a random peer.
	SyncInterval = 15 * time.Second

	// SyncMaxMessageSize is the max UDP datagram for sync messages.
	SyncMaxMessageSize = 65536
)

// Sync handles federated state replication between lighthouse instances.
// Uses the same mesh network as gossip — messages travel over WireGuard tunnels.
//
// Replication model: push-based with LWW (last-writer-wins) merge.
// Every mutation triggers a push to all known lighthouse peers.
// Periodic full-state sync catches anything missed.
type Sync struct {
	store  *Store
	nodeID string
	meshIP string // This node's mesh IP (for binding)

	mu      sync.RWMutex
	peers   []string // Mesh IPs of other lighthouse instances
	conn    *net.UDPConn
	running bool
	stopCh  chan struct{}
}

// NewSync creates a sync instance.
func NewSync(store *Store, nodeID, meshIP string) *Sync {
	s := &Sync{
		store:  store,
		nodeID: nodeID,
		meshIP: meshIP,
		stopCh: make(chan struct{}),
	}

	// Register as write listener to push mutations immediately
	store.OnWrite(s.onWrite)

	return s
}

// AddPeer registers another lighthouse instance for replication.
func (s *Sync) AddPeer(meshIP string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.peers {
		if p == meshIP {
			return
		}
	}
	s.peers = append(s.peers, meshIP)
	log.Printf("[Sync] Added peer: %s (total: %d)", meshIP, len(s.peers))
}

// Start begins the sync listener and periodic push loop.
func (s *Sync) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	addr := &net.UDPAddr{
		IP:   net.ParseIP(s.meshIP),
		Port: SyncPort,
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		// Fall back to binding on all interfaces
		addr = &net.UDPAddr{Port: SyncPort}
		conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return err
		}
	}

	s.conn = conn
	s.running = true

	go s.listenLoop()
	go s.pushLoop()

	log.Printf("[Sync] Listening on %s", addr.String())
	return nil
}

// Stop halts the sync service.
func (s *Sync) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	if s.conn != nil {
		s.conn.Close()
	}
	log.Printf("[Sync] Stopped")
}

// onWrite is called by the store on every local mutation.
// It immediately pushes the change to all known peers.
func (s *Sync) onWrite(msg SyncMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Sync] marshal error: %v", err)
		return
	}
	if len(data) > SyncMaxMessageSize {
		log.Printf("[Sync] message too large (%d bytes), skipping push", len(data))
		return
	}

	s.mu.RLock()
	peers := make([]string, len(s.peers))
	copy(peers, s.peers)
	s.mu.RUnlock()

	for _, peerIP := range peers {
		addr := &net.UDPAddr{
			IP:   net.ParseIP(peerIP),
			Port: SyncPort,
		}
		s.mu.RLock()
		conn := s.conn
		s.mu.RUnlock()
		if conn == nil {
			continue
		}
		if _, err := conn.WriteToUDP(data, addr); err != nil {
			log.Printf("[Sync] push to %s failed: %v", peerIP, err)
		}
	}
}

// listenLoop receives sync messages from peers and applies them.
func (s *Sync) listenLoop() {
	buf := make([]byte, SyncMaxMessageSize)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		s.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			s.mu.RLock()
			running := s.running
			s.mu.RUnlock()
			if running {
				log.Printf("[Sync] read error: %v", err)
			}
			continue
		}

		var msg SyncMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}

		// Don't apply our own messages
		if msg.NodeID == s.nodeID {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		applied, err := s.store.ApplySync(ctx, msg)
		cancel()
		if err != nil {
			log.Printf("[Sync] apply error: %v", err)
			continue
		}
		if applied {
			log.Printf("[Sync] Applied %s/%s from %s (v%d)", msg.Type, msg.Action, msg.NodeID, msg.Version)
		}
	}
}

// pushLoop periodically pushes full state to a random peer.
// This ensures convergence even if individual push messages were lost.
func (s *Sync) pushLoop() {
	ticker := time.NewTicker(SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pushFullState()
		}
	}
}

func (s *Sync) pushFullState() {
	s.mu.RLock()
	if len(s.peers) == 0 {
		s.mu.RUnlock()
		return
	}
	// Pick a random peer
	peers := make([]string, len(s.peers))
	copy(peers, s.peers)
	s.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sites, err := s.store.ListAllSites(ctx)
	if err != nil {
		return
	}

	for _, site := range sites {
		data, err := json.Marshal(site)
		if err != nil {
			continue
		}
		msg := SyncMessage{
			Type:      "site",
			Action:    "upsert",
			Payload:   data,
			Version:   site.Version,
			NodeID:    site.NodeID,
			Timestamp: site.UpdatedAt,
		}
		s.onWrite(msg)
	}
}
