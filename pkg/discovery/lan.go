package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
)

const (
	LANMulticastBase    = "239.192.0.0"
	LANMulticastPort    = 51830
	LANAnnounceInterval = 5 * time.Second
	LANMaxMessageSize   = 4096
	LANMethod           = "lan"
)

// LANDiscovery handles peer discovery via UDP multicast on the local network
type LANDiscovery struct {
	config    *daemon.Config
	localNode *LocalNode
	peerStore *daemon.PeerStore
	gossipKey [32]byte

	multicastAddr *net.UDPAddr
	conn          *net.UDPConn

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
}

// NewLANDiscovery creates a new LAN multicast discovery instance
func NewLANDiscovery(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) (*LANDiscovery, error) {
	// Derive multicast address from the multicast ID
	// Use 239.192.X.Y where X.Y come from MulticastID
	multicastIP := net.IPv4(239, 192,
		config.Keys.MulticastID[0],
		config.Keys.MulticastID[1])

	multicastAddr := &net.UDPAddr{
		IP:   multicastIP,
		Port: LANMulticastPort,
	}

	return &LANDiscovery{
		config:        config,
		localNode:     localNode,
		peerStore:     peerStore,
		gossipKey:     config.Keys.GossipKey,
		multicastAddr: multicastAddr,
		stopCh:        make(chan struct{}),
	}, nil
}

// Start begins LAN multicast discovery
func (l *LANDiscovery) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.running {
		return fmt.Errorf("LAN discovery already running")
	}

	// Listen on the multicast address
	conn, err := net.ListenMulticastUDP("udp4", nil, l.multicastAddr)
	if err != nil {
		return fmt.Errorf("failed to join multicast group %s: %w", l.multicastAddr.String(), err)
	}

	// Set read buffer size
	conn.SetReadBuffer(LANMaxMessageSize)

	l.conn = conn
	l.running = true

	// Start listener and announcer
	go l.listenLoop()
	go l.announceLoop()

	log.Printf("[LAN] Multicast discovery started on %s", l.multicastAddr.String())
	return nil
}

// Stop stops LAN multicast discovery
func (l *LANDiscovery) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.running {
		return nil
	}

	l.running = false
	close(l.stopCh)

	if l.conn != nil {
		l.conn.Close()
	}

	log.Printf("[LAN] Multicast discovery stopped")
	return nil
}

// announceLoop periodically sends multicast announcements
func (l *LANDiscovery) announceLoop() {
	// Initial announce immediately
	l.announce()

	ticker := time.NewTicker(LANAnnounceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.announce()
		}
	}
}

// announce sends a multicast announcement
func (l *LANDiscovery) announce() {
	// Create announcement
	announcement := crypto.CreateAnnouncement(
		l.localNode.WGPubKey,
		l.localNode.MeshIP,
		l.localNode.WGEndpoint,
		l.localNode.RoutableNetworks,
		nil, // No known peers in LAN announce (keep small)
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcement, l.gossipKey)
	if err != nil {
		log.Printf("[LAN] Failed to create announcement: %v", err)
		return
	}

	// Send multicast via a new UDP connection (send socket)
	sendConn, err := net.DialUDP("udp4", nil, l.multicastAddr)
	if err != nil {
		log.Printf("[LAN] Failed to create send socket: %v", err)
		return
	}
	defer sendConn.Close()

	if _, err := sendConn.Write(data); err != nil {
		log.Printf("[LAN] Failed to send announcement: %v", err)
	}
}

// listenLoop listens for multicast announcements
func (l *LANDiscovery) listenLoop() {
	buf := make([]byte, LANMaxMessageSize)

	for {
		select {
		case <-l.stopCh:
			return
		default:
		}

		l.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			l.mu.RLock()
			running := l.running
			l.mu.RUnlock()
			if running {
				log.Printf("[LAN] Read error: %v", err)
			}
			continue
		}

		// Try to decrypt
		_, announcement, err := crypto.OpenEnvelope(buf[:n], l.gossipKey)
		if err != nil {
			// Not a wgmesh packet or wrong secret - silently ignore
			continue
		}

		// Skip our own announcements
		if announcement.WGPubKey == l.localNode.WGPubKey {
			continue
		}

		// Resolve endpoint from the sender's address if the announced one is 0.0.0.0
		endpoint := resolveEndpoint(announcement.WGEndpoint, remoteAddr)

		peer := &daemon.PeerInfo{
			WGPubKey:         announcement.WGPubKey,
			MeshIP:           announcement.MeshIP,
			Endpoint:         endpoint,
			RoutableNetworks: announcement.RoutableNetworks,
		}

		log.Printf("[LAN] Discovered peer %s (%s) at %s", safeTruncate(peer.WGPubKey, 8), peer.MeshIP, peer.Endpoint)
		l.peerStore.Update(peer, LANMethod)
	}
}

// resolveEndpoint resolves the peer endpoint from the announcement and sender address
func resolveEndpoint(advertised string, sender *net.UDPAddr) string {
	if host, port, err := net.SplitHostPort(advertised); err == nil {
		if host == "" || host == "0.0.0.0" || host == "::" {
			if sender != nil && sender.IP != nil {
				return net.JoinHostPort(sender.IP.String(), port)
			}
		}
		return advertised
	}
	if sender != nil && sender.IP != nil {
		return net.JoinHostPort(sender.IP.String(), fmt.Sprintf("%d", daemon.DefaultWGPort))
	}
	return ""
}

// MarshalJSON implements json.Marshaler for debugging
func (l *LANDiscovery) MarshalJSON() ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"multicast_addr": l.multicastAddr.String(),
		"running":        l.running,
	})
}

// safeTruncate safely truncates a string for logging
func safeTruncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
