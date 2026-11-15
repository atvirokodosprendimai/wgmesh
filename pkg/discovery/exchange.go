package discovery

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
	"github.com/atvirokodosprendimai/wgmesh/pkg/ratelimit"
	"github.com/atvirokodosprendimai/wgmesh/pkg/wireguard"
)

const (
	ExchangeTimeout         = 4 * time.Second
	MaxExchangeSize         = 65536 // 64KB max message size
	ExchangePort            = 51821 // Default exchange port (matches crypto.GossipPortBase)
	PunchInterval           = 100 * time.Millisecond
	RendezvousSessionTTL    = 20 * time.Second
	RendezvousStartLeadTime = 1800 * time.Millisecond
	RendezvousPunchCooldown = 6 * time.Second
	RendezvousStartCooldown = 8 * time.Second
	RendezvousPortSpread    = 2
	HandshakeWaitTimeout    = 10 * time.Second // Increased from 3s - WG handshake needs more time for cross-DC
	HandshakePollInterval   = 250 * time.Millisecond
	ExchangeLogCooldown     = 30 * time.Second
	maxLogThrottleEntries   = 4096 // bound on lastPacketLog: a flood of spoofed sources cannot grow it further
)

type rendezvousOffer struct {
	Protocol      string   `json:"protocol"`
	Timestamp     int64    `json:"timestamp"`
	FromPubKey    string   `json:"from_pubkey"`
	TargetPubKey  string   `json:"target_pubkey"`
	PairID        string   `json:"pair_id"`
	Candidates    []string `json:"candidates,omitempty"`
	ObservedAddr  string   `json:"observed_addr,omitempty"`
	IntroducerKey string   `json:"introducer_key,omitempty"`
}

type rendezvousStart struct {
	Protocol       string   `json:"protocol"`
	Timestamp      int64    `json:"timestamp"`
	PairID         string   `json:"pair_id"`
	PeerPubKey     string   `json:"peer_pubkey"`
	PeerCandidates []string `json:"peer_candidates,omitempty"`
	StartAtUnixMs  int64    `json:"start_at_unix_ms"`
	IntroducerKey  string   `json:"introducer_key,omitempty"`
}

type goodbyeMessage struct {
	Protocol  string `json:"protocol"`
	Timestamp int64  `json:"timestamp"`
	WGPubKey  string `json:"wg_pubkey"`
}

type rendezvousState struct {
	offers    map[string]*rendezvousOffer
	endpoints map[string]string
	createdAt time.Time
}

// PeerExchange handles the encrypted peer exchange protocol
type PeerExchange struct {
	config    *daemon.Config
	localNode *daemon.LocalNode
	peerStore *daemon.PeerStore

	conn    *net.UDPConn
	port    int
	limiter *ratelimit.IPRateLimiter

	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}

	pendingMu      sync.Mutex
	pendingReplies map[string]chan *daemon.PeerInfo

	announceHandler func(*crypto.PeerAnnouncement, *net.UDPAddr)

	rendezvousMu       sync.Mutex
	rendezvousSessions map[string]*rendezvousState
	activePunches      map[string]time.Time
	rendezvousStarts   map[string]time.Time

	logMu          sync.Mutex
	lastPacketLog  map[string]time.Time
	lastLogCleanup time.Time

	// nonces remembers recently opened datagrams so a replay inside the
	// acceptance window is dropped (ADR-0001); HELLO is exempt.
	nonces *nonceCache

	// legacyOpens counts received envelopes sealed without type authentication
	// (ADR-0001), per packet and not per sender. It measures old traffic once
	// writers seal with additional data; until then it counts most traffic.
	legacyOpens atomic.Uint64

	// replayDrops counts datagrams dropped by the nonce cache.
	replayDrops atomic.Uint64
}

// NewPeerExchange creates a new peer exchange handler
func NewPeerExchange(config *daemon.Config, localNode *daemon.LocalNode, peerStore *daemon.PeerStore) *PeerExchange {
	return &PeerExchange{
		config:             config,
		localNode:          localNode,
		peerStore:          peerStore,
		limiter:            ratelimit.NewDefault(),
		stopCh:             make(chan struct{}),
		pendingReplies:     make(map[string]chan *daemon.PeerInfo),
		rendezvousSessions: make(map[string]*rendezvousState),
		activePunches:      make(map[string]time.Time),
		rendezvousStarts:   make(map[string]time.Time),
		lastPacketLog:      make(map[string]time.Time),
		nonces:             newNonceCache(nonceCacheTTL, nonceCacheCapacity, wallClock),
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
	pe.stopCh = make(chan struct{}) // Re-create for restart safety (B8)

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

		// Rate-limit per source IP before dispatching
		if !pe.limiter.Allow(remoteAddr.IP.String()) {
			continue
		}

		// Handle message in goroutine
		data := make([]byte, n)
		copy(data, buf[:n])
		go pe.handleMessage(data, remoteAddr)
	}
}

// LegacyEnvelopeOpens returns how many received envelopes were sealed without
// type authentication.
func (pe *PeerExchange) LegacyEnvelopeOpens() uint64 { return pe.legacyOpens.Load() }

// handleMessage processes an incoming peer exchange message
func (pe *PeerExchange) handleMessage(data []byte, remoteAddr *net.UDPAddr) {
	// Try to decrypt the message
	envelope, plaintext, err := crypto.OpenEnvelopeRaw(data, pe.config.Keys.GossipKey)
	if err != nil {
		// Could be a DHT message or wrong key - log for debugging
		log.Printf("[Exchange] Received non-wgmesh packet from %s (len=%d, possibly DHT or wrong secret)", remoteAddr.String(), len(data))
		return
	}

	// HELLO, REPLY and ANNOUNCE are logged as valid after their payload passes
	// validation below. A legacy GOODBYE is ignored below, so it is not
	// announced as valid.
	switch envelope.MessageType {
	case crypto.MessageTypeHello, crypto.MessageTypeReply, crypto.MessageTypeAnnounce:
	default:
		if !(envelope.LegacyAAD && envelope.MessageType == crypto.MessageTypeGoodbye) {
			pe.logIncomingPacket(envelope.MessageType, remoteAddr)
		}
	}

	if envelope.LegacyAAD {
		pe.legacyOpens.Add(1)
		if pe.allowLog("legacy "+envelope.MessageType, remoteAddr) {
			log.Printf("[Exchange] %s from %s was sealed without type authentication (sender runs an older release)", envelope.MessageType, remoteAddr.String())
		}
	}

	// A datagram whose nonce was already seen inside the acceptance window is a
	// replay. The window is twice MaxMessageAge because OpenEnvelopeRaw accepts a
	// timestamp that far in the future. HELLO is exempt: the retry loop in
	// ExchangeWithPeer resends one sealed buffer on purpose, so a lost reply can
	// be recovered, and senders from before this check do the same.
	if envelope.MessageType != crypto.MessageTypeHello {
		var nonce [12]byte
		copy(nonce[:], envelope.Nonce)
		if pe.nonces.Seen(nonce, envelope.MessageType) {
			pe.replayDrops.Add(1)
			if pe.allowLog("replayed "+envelope.MessageType, remoteAddr) {
				log.Printf("[Exchange] Dropped replayed %s from %s", envelope.MessageType, remoteAddr.String())
			}
			return
		}
	}

	switch envelope.MessageType {
	case crypto.MessageTypeHello:
		var announcement crypto.PeerAnnouncement
		if err := json.Unmarshal(plaintext, &announcement); err != nil {
			log.Printf("[Exchange] Invalid HELLO payload from %s: %v", remoteAddr.String(), err)
			return
		}
		if err := validateExchangeAnnouncement(&announcement); err != nil {
			log.Printf("[Exchange] Dropping invalid HELLO from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.logIncomingPacket(envelope.MessageType, remoteAddr)
		pe.handleHello(&announcement, remoteAddr)
	case crypto.MessageTypeReply:
		var reply crypto.PeerAnnouncement
		if err := json.Unmarshal(plaintext, &reply); err != nil {
			log.Printf("[Exchange] Invalid REPLY payload from %s: %v", remoteAddr.String(), err)
			return
		}
		if err := validateExchangeAnnouncement(&reply); err != nil {
			log.Printf("[Exchange] Dropping invalid REPLY from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.logIncomingPacket(envelope.MessageType, remoteAddr)
		pe.handleReply(&reply, remoteAddr)
	case crypto.MessageTypeAnnounce:
		var announcement crypto.PeerAnnouncement
		if err := json.Unmarshal(plaintext, &announcement); err != nil {
			log.Printf("[Exchange] Invalid ANNOUNCE payload from %s: %v", remoteAddr.String(), err)
			return
		}
		if err := validateExchangeAnnouncement(&announcement); err != nil {
			log.Printf("[Exchange] Dropping invalid ANNOUNCE from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.logIncomingPacket(envelope.MessageType, remoteAddr)
		pe.mu.RLock()
		handler := pe.announceHandler
		pe.mu.RUnlock()
		if handler != nil {
			handler(&announcement, remoteAddr)
		}
	case crypto.MessageTypeRendezvousOffer:
		var offer rendezvousOffer
		if err := json.Unmarshal(plaintext, &offer); err != nil {
			log.Printf("[NAT] Invalid RENDEZVOUS_OFFER from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.handleRendezvousOffer(&offer, remoteAddr)
	case crypto.MessageTypeRendezvousStart:
		var start rendezvousStart
		if err := json.Unmarshal(plaintext, &start); err != nil {
			log.Printf("[NAT] Invalid RENDEZVOUS_START from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.handleRendezvousStart(&start, remoteAddr)
	case crypto.MessageTypeGoodbye:
		// An envelope sealed without additional data does not authenticate its
		// type, so anyone who captured a HELLO could have re-sent it as GOODBYE.
		// Only a GOODBYE whose type is bound to the tag may remove a peer.
		if envelope.LegacyAAD {
			if pe.allowLog("ignored legacy GOODBYE", remoteAddr) {
				log.Printf("[Exchange] Ignored GOODBYE from %s: sent without type authentication, the sender must be upgraded", remoteAddr.String())
			}
			return
		}
		var bye goodbyeMessage
		if err := json.Unmarshal(plaintext, &bye); err != nil {
			log.Printf("[Exchange] Invalid GOODBYE payload from %s: %v", remoteAddr.String(), err)
			return
		}
		if bye.WGPubKey == "" || bye.WGPubKey == pe.localNode.WGPubKey {
			return
		}
		// Validate timestamp to prevent replay attacks
		msgTime := time.Unix(bye.Timestamp, 0)
		if time.Since(msgTime) > 60*time.Second {
			log.Printf("[Exchange] Rejected stale GOODBYE from %s (age: %v)", remoteAddr.String(), time.Since(msgTime))
			return
		}
		if msgTime.After(time.Now().Add(60 * time.Second)) {
			log.Printf("[Exchange] Rejected GOODBYE with future timestamp from %s", remoteAddr.String())
			return
		}
		pe.peerStore.Withdraw(bye.WGPubKey, "goodbye")
		name := bye.WGPubKey
		if len(name) > 8 {
			name = name[:8] + "..."
		}
		log.Printf("[Exchange] Peer %s reported shutdown, removed from active set", name)
	default:
		log.Printf("[Exchange] Unknown message type: %s", envelope.MessageType)
	}
}

// validateExchangeAnnouncement exists because a locked test calls it by this
// name; the validation itself is crypto.FilterAnnouncement.
func validateExchangeAnnouncement(a *crypto.PeerAnnouncement) error {
	return crypto.FilterAnnouncement(a)
}

// handleHello responds to a peer's HELLO message
func (pe *PeerExchange) handleHello(announcement *crypto.PeerAnnouncement, remoteAddr *net.UDPAddr) {
	// Skip if this is from ourselves
	if announcement.WGPubKey == pe.localNode.WGPubKey {
		return
	}

	// Update peer store with the sender's info
	endpoint, guessed := peerEndpoint(announcement.WGEndpoint, remoteAddr, pe.config.DisableIPv6)
	peerInfo := &daemon.PeerInfo{
		WGPubKey:         announcement.WGPubKey,
		Hostname:         announcement.Hostname,
		MeshIP:           announcement.MeshIP,
		MeshIPv6:         announcement.MeshIPv6,
		Endpoint:         endpoint,
		EndpointGuessed:  guessed,
		Introducer:       announcement.Introducer,
		RoutableNetworks: announcement.RoutableNetworks,
		NATType:          announcement.NATType,
	}

	pe.peerStore.Offer(daemon.Candidate{Info: peerInfo, Source: DHTMethod, Hand: daemon.FirstHand})

	pe.updateTransitivePeers(announcement.KnownPeers)

	// Send reply
	if err := pe.sendReply(remoteAddr); err != nil {
		log.Printf("[Exchange] Failed to send reply to %s: %v", remoteAddr.String(), err)
	}
}

// handleReply routes a REPLY back to an in-flight exchange request.
// If the reply contains ObservedEndpoint (peer-as-STUN reflector), we use
// the reflected public IP to update our own localNode.WGEndpoint.
func (pe *PeerExchange) handleReply(reply *crypto.PeerAnnouncement, remoteAddr *net.UDPAddr) {
	// Peer-as-STUN reflector: the responder tells us what our public
	// IP:port looks like. Use the reflected IP combined with our WG port.
	pe.applyObservedEndpoint(reply.ObservedEndpoint)

	endpoint, guessed := peerEndpoint(reply.WGEndpoint, remoteAddr, pe.config.DisableIPv6)
	peerInfo := &daemon.PeerInfo{
		WGPubKey:         reply.WGPubKey,
		Hostname:         reply.Hostname,
		MeshIP:           reply.MeshIP,
		MeshIPv6:         reply.MeshIPv6,
		Endpoint:         endpoint,
		EndpointGuessed:  guessed,
		Introducer:       reply.Introducer,
		RoutableNetworks: reply.RoutableNetworks,
		NATType:          reply.NATType,
	}

	pe.updateTransitivePeers(reply.KnownPeers)

	// Always update the peer store so reconcile can configure WG promptly,
	// even when we also route this reply to a pending ExchangeWithPeer caller.
	pe.peerStore.Offer(daemon.Candidate{Info: peerInfo, Source: DHTMethod, Hand: daemon.FirstHand})

	if ch, ok := pe.getPendingReplyChannel(remoteAddr.String()); ok {
		select {
		case ch <- peerInfo:
		default:
		}
		return
	}

	log.Printf("[Exchange] Received unsolicited REPLY from %s", remoteAddr.String())
}

// applyObservedEndpoint updates localNode.WGEndpoint if a peer reflected
// a usable public IP back to us. Only updates the IP component — the WG
// port comes from our own config, not from the observed NAT port.
func (pe *PeerExchange) applyObservedEndpoint(observed string) {
	if observed == "" {
		return
	}

	host, _, err := net.SplitHostPort(observed)
	if err != nil {
		return
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return
	}
	if pe.config.DisableIPv6 && ip.To4() == nil {
		return
	}

	// Ignore private/loopback — both peers are on the same LAN,
	// so the observed address isn't useful for NAT traversal.
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
		return
	}

	// Extract our WG listen port from the current endpoint
	currentEP := pe.localNode.GetEndpoint()
	currentHost, wgPort, err := net.SplitHostPort(currentEP)
	if err != nil {
		return
	}

	// If we already have a usable public IPv6 endpoint, don't downgrade it
	// to IPv4 due to peer reflection from an IPv4-only path.
	if currentIP := net.ParseIP(currentHost); currentIP != nil {
		if currentIP.To4() == nil && currentIP.IsGlobalUnicast() && !currentIP.IsPrivate() && ip.To4() != nil {
			return
		}
	}

	newEndpoint := net.JoinHostPort(ip.String(), wgPort)
	if pe.localNode.ObserveEndpoint(daemon.EndpointReflection, newEndpoint) {
		log.Printf("[Exchange] Peer reflected our address: %s (was %s)", newEndpoint, currentEP)
	}
}

// sendReply sends a REPLY message to a peer.
// The reply includes ObservedEndpoint — the HELLO sender's public IP:port
// as seen by us, enabling peer-as-STUN-reflector (zero infrastructure).
func (pe *PeerExchange) sendReply(remoteAddr *net.UDPAddr) error {
	// Build list of known peers for transitive discovery
	knownPeers := pe.getKnownPeers()

	announcement := crypto.CreateAnnouncement(
		pe.localNode.WGPubKey,
		pe.localNode.MeshIP,
		pe.localNode.GetEndpoint(),
		pe.localNode.Introducer,
		pe.localNode.RoutableNetworks,
		knownPeers,
		pe.localNode.Hostname,
		pe.localNode.MeshIPv6,
		string(pe.localNode.NATType),
	)
	announcement.ObservedEndpoint = remoteAddr.String()

	data, err := crypto.SealEnvelope(crypto.MessageTypeReply, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal reply: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}
	return nil
}

// ExchangeWithPeer initiates a peer exchange with a remote address
func (pe *PeerExchange) ExchangeWithPeer(addrStr string) (*daemon.PeerInfo, error) {
	remoteAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	// Warn when target IP matches own public endpoint — likely a self-connection
	// attempt via gossip loop. Not a hard block because peers behind the same
	// NAT/CGNAT share a public IP (different ports, different WGPubKeys).
	// Hard self-filtering happens upstream by WGPubKey in controlEndpointForPeer
	// and getKnownPeers.
	ownEndpoint := pe.localNode.GetEndpoint()
	if ownEndpoint != "" {
		ownHost, _, splitErr := net.SplitHostPort(ownEndpoint)
		if splitErr == nil && ownHost == remoteAddr.IP.String() {
			log.Printf("[NAT] Warning: punch target %s matches own public IP (possible self-connection)", addrStr)
		}
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
		pe.localNode.GetEndpoint(),
		pe.localNode.Introducer,
		pe.localNode.RoutableNetworks,
		knownPeers,
		pe.localNode.Hostname,
		pe.localNode.MeshIPv6,
		string(pe.localNode.NATType),
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeHello, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return nil, fmt.Errorf("failed to seal hello: %w", err)
	}

	log.Printf("[Exchange] Sending HELLO to %s (exchange port: %d)", remoteAddr.String(), pe.port)
	if !pe.config.DisablePunching {
		log.Printf("[NAT] Punch attempt started with %s (timeout=%v interval=%v local_port=%d)", remoteAddr.String(), ExchangeTimeout, PunchInterval, pe.port)
	}

	attempts := 0
	sendHello := func() error {
		attempts++
		_, sendErr := pe.conn.WriteToUDP(data, remoteAddr)
		if sendErr != nil {
			return fmt.Errorf("failed to send hello: %w", sendErr)
		}
		return nil
	}

	if err := sendHello(); err != nil {
		return nil, err
	}

	if pe.config.DisablePunching {
		timeout := time.NewTimer(ExchangeTimeout)
		defer timeout.Stop()
		select {
		case peerInfo := <-replyCh:
			log.Printf("[Exchange] Peer exchange succeeded with %s", remoteAddr.String())
			return peerInfo, nil
		case <-timeout.C:
			return nil, fmt.Errorf("exchange timeout")
		}
	}

	timeout := time.NewTimer(ExchangeTimeout)
	defer timeout.Stop()
	punchTicker := time.NewTicker(PunchInterval)
	defer punchTicker.Stop()

	for {
		select {
		case peerInfo := <-replyCh:
			if attempts > 1 {
				log.Printf("[NAT] Punch success with %s after %d HELLO attempts", remoteAddr.String(), attempts)
			} else {
				log.Printf("[NAT] Peer exchange succeeded with %s on first attempt", remoteAddr.String())
			}
			return peerInfo, nil
		case <-punchTicker.C:
			if err := sendHello(); err != nil {
				log.Printf("[Exchange] HELLO resend to %s failed: %v", remoteAddr.String(), err)
			}
		case <-timeout.C:
			log.Printf("[NAT] Punch timeout with %s after %d HELLO attempts", remoteAddr.String(), attempts)
			return nil, fmt.Errorf("exchange timeout")
		}
	}
}

// RequestRendezvous asks an introducer to coordinate synchronized NAT punching
// between this node and the target peer.
func (pe *PeerExchange) RequestRendezvous(introducerAddr, targetPubKey string, candidates []string) error {
	if pe.config.DisablePunching {
		return nil
	}
	if introducerAddr == "" || targetPubKey == "" {
		return fmt.Errorf("introducer and target pubkey are required")
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", introducerAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve introducer address: %w", err)
	}

	offer := &rendezvousOffer{
		Protocol:      crypto.ProtocolVersion,
		Timestamp:     time.Now().Unix(),
		FromPubKey:    pe.localNode.WGPubKey,
		TargetPubKey:  targetPubKey,
		PairID:        pairIDForPeers(pe.localNode.WGPubKey, targetPubKey),
		Candidates:    filterCandidatesForConfig(normalizeCandidates(candidates), pe.config.DisableIPv6),
		IntroducerKey: "",
	}

	data, err := crypto.SealEnvelope(crypto.MessageTypeRendezvousOffer, offer, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal rendezvous offer: %w", err)
	}

	if _, err := pe.conn.WriteToUDP(data, remoteAddr); err != nil {
		return fmt.Errorf("failed to send rendezvous offer: %w", err)
	}

	log.Printf("[NAT] Sent rendezvous offer for pair %s via introducer %s", shortKey(offer.PairID), remoteAddr.String())
	return nil
}

func (pe *PeerExchange) handleRendezvousOffer(offer *rendezvousOffer, remoteAddr *net.UDPAddr) {
	if pe.config.DisablePunching {
		return
	}
	if offer == nil || remoteAddr == nil {
		return
	}
	if offer.FromPubKey == "" || offer.TargetPubKey == "" {
		return
	}
	if offer.FromPubKey == pe.localNode.WGPubKey || offer.TargetPubKey == pe.localNode.WGPubKey {
		// We are a participant, not an introducer for this pair.
		return
	}
	if !pe.localNode.Introducer {
		// Only designated introducers relay offers between other peers.
		return
	}

	pairID := pairIDForPeers(offer.FromPubKey, offer.TargetPubKey)
	if offer.PairID != "" && offer.PairID != pairID {
		log.Printf("[NAT] Dropping rendezvous offer with invalid pair id from %s", remoteAddr.String())
		return
	}

	observed := remoteAddr.String()
	candidates := append([]string{}, offer.Candidates...)
	candidates = append(candidates, observed)
	candidates = expandCandidatePorts(normalizeCandidates(candidates), RendezvousPortSpread)
	candidates = filterCandidatesForConfig(candidates, pe.config.DisableIPv6)
	log.Printf("[NAT] Introducer %s received offer pair=%s from=%s target=%s observed=%s candidates=%v", shortKey(pe.localNode.WGPubKey), shortKey(pairID), shortKey(offer.FromPubKey), shortKey(offer.TargetPubKey), observed, candidates)

	pe.rendezvousMu.Lock()
	defer pe.rendezvousMu.Unlock()

	now := time.Now()
	for id, st := range pe.rendezvousSessions {
		if now.Sub(st.createdAt) > RendezvousSessionTTL {
			delete(pe.rendezvousSessions, id)
		}
	}
	for id, t := range pe.rendezvousStarts {
		if now.Sub(t) > RendezvousStartCooldown*2 {
			delete(pe.rendezvousStarts, id)
		}
	}
	for id, t := range pe.activePunches {
		if now.Sub(t) > RendezvousPunchCooldown*2 {
			delete(pe.activePunches, id)
		}
	}

	st, ok := pe.rendezvousSessions[pairID]
	if !ok {
		st = &rendezvousState{
			offers:    make(map[string]*rendezvousOffer),
			endpoints: make(map[string]string),
			createdAt: now,
		}
		pe.rendezvousSessions[pairID] = st
	}

	offerCopy := *offer
	offerCopy.PairID = pairID
	offerCopy.IntroducerKey = pe.localNode.WGPubKey
	offerCopy.ObservedAddr = observed
	offerCopy.Candidates = candidates
	st.offers[offer.FromPubKey] = &offerCopy
	st.endpoints[offer.FromPubKey] = observed

	a := st.offers[offer.FromPubKey]
	b := st.offers[offer.TargetPubKey]
	if b == nil {
		if target, ok := pe.peerStore.Get(offer.TargetPubKey); ok {
			targetControl := controlEndpointFromPeerEndpoint(target.Endpoint, int(pe.config.Keys.GossipPort))
			if targetControl != "" {
				b = &rendezvousOffer{
					Protocol:      crypto.ProtocolVersion,
					Timestamp:     time.Now().Unix(),
					FromPubKey:    offer.TargetPubKey,
					TargetPubKey:  offer.FromPubKey,
					PairID:        pairID,
					Candidates:    expandCandidatePorts(normalizeCandidates([]string{targetControl}), RendezvousPortSpread),
					IntroducerKey: pe.localNode.WGPubKey,
				}
				st.offers[offer.TargetPubKey] = b
				st.endpoints[offer.TargetPubKey] = targetControl
				log.Printf("[NAT] Introducer %s synthesized target offer for pair %s target=%s endpoint=%s", shortKey(pe.localNode.WGPubKey), shortKey(pairID), shortKey(offer.TargetPubKey), targetControl)
			}
		}
	}
	if a == nil || b == nil {
		log.Printf("[NAT] Introducer %s waiting pair %s: got %s, waiting for %s", shortKey(pe.localNode.WGPubKey), shortKey(pairID), shortKey(offer.FromPubKey), shortKey(offer.TargetPubKey))
		return
	}

	if lastStart, ok := pe.rendezvousStarts[pairID]; ok && time.Since(lastStart) < RendezvousStartCooldown {
		log.Printf("[NAT] Introducer %s throttling START for pair %s (last start %v ago)", shortKey(pe.localNode.WGPubKey), shortKey(pairID), time.Since(lastStart).Round(time.Millisecond))
		return
	}
	pe.rendezvousStarts[pairID] = time.Now()

	startAt := time.Now().Add(RendezvousStartLeadTime)
	go pe.sendRendezvousStart(pairID, a.FromPubKey, st.endpoints[a.FromPubKey], b.FromPubKey, b.Candidates, startAt)
	go pe.sendRendezvousStart(pairID, b.FromPubKey, st.endpoints[b.FromPubKey], a.FromPubKey, a.Candidates, startAt)

	delete(pe.rendezvousSessions, pairID)
	log.Printf("[NAT] Introducer %s started synchronized rendezvous pair %s (%s <-> %s) at %s", shortKey(pe.localNode.WGPubKey), shortKey(pairID), shortKey(a.FromPubKey), shortKey(b.FromPubKey), startAt.UTC().Format(time.RFC3339Nano))
}

func (pe *PeerExchange) sendRendezvousStart(pairID, targetPubKey, targetEndpoint, peerPubKey string, peerCandidates []string, startAt time.Time) {
	if targetEndpoint == "" || peerPubKey == "" {
		return
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", targetEndpoint)
	if err != nil {
		log.Printf("[NAT] Failed to resolve rendezvous START destination %s: %v", targetEndpoint, err)
		return
	}

	msg := &rendezvousStart{
		Protocol:       crypto.ProtocolVersion,
		Timestamp:      time.Now().Unix(),
		PairID:         pairID,
		PeerPubKey:     peerPubKey,
		PeerCandidates: filterCandidatesForConfig(normalizeCandidates(peerCandidates), pe.config.DisableIPv6),
		StartAtUnixMs:  startAt.UnixMilli(),
		IntroducerKey:  pe.localNode.WGPubKey,
	}

	data, err := crypto.SealEnvelope(crypto.MessageTypeRendezvousStart, msg, pe.config.Keys.GossipKey)
	if err != nil {
		log.Printf("[NAT] Failed to seal rendezvous START: %v", err)
		return
	}

	if _, err := pe.conn.WriteToUDP(data, remoteAddr); err != nil {
		log.Printf("[NAT] Failed to send rendezvous START to %s: %v", remoteAddr.String(), err)
		return
	}

	log.Printf("[NAT] Introducer %s sent START pair=%s to=%s for_peer=%s candidates=%v start_at=%s", shortKey(pe.localNode.WGPubKey), shortKey(pairID), remoteAddr.String(), shortKey(peerPubKey), msg.PeerCandidates, startAt.UTC().Format(time.RFC3339Nano))
}

func (pe *PeerExchange) handleRendezvousStart(start *rendezvousStart, remoteAddr *net.UDPAddr) {
	if pe.config.DisablePunching {
		return
	}
	if start == nil {
		return
	}
	if start.PeerPubKey == "" || start.PairID == "" {
		return
	}

	pairID := pairIDForPeers(pe.localNode.WGPubKey, start.PeerPubKey)
	if pairID != start.PairID {
		log.Printf("[NAT] Ignoring rendezvous START for mismatched pair %s from %s", start.PairID, remoteAddr.String())
		return
	}

	var startAt time.Time
	if start.StartAtUnixMs > 0 {
		startAt = time.UnixMilli(start.StartAtUnixMs)
	} else {
		startAt = time.Now().Add(100 * time.Millisecond)
	}

	candidates := append([]string{}, start.PeerCandidates...)
	candidates = normalizeCandidates(candidates)
	candidates = filterCandidatesForConfig(candidates, pe.config.DisableIPv6)
	if len(candidates) == 0 {
		log.Printf("[NAT] Rendezvous START for pair %s had no peer candidates", shortKey(start.PairID))
		return
	}

	if !pe.beginPunchJob(pairID) {
		log.Printf("[NAT] Rendezvous START ignored for pair %s (cooldown active)", shortKey(start.PairID))
		return
	}

	log.Printf("[NAT] Rendezvous START received for pair %s via introducer %s; peer=%s candidates=%v start_at=%s", shortKey(start.PairID), shortKey(start.IntroducerKey), shortKey(start.PeerPubKey), candidates, startAt.UTC().Format(time.RFC3339Nano))

	go pe.runRendezvousPunch(pairID, start.PeerPubKey, candidates, startAt)
}

func (pe *PeerExchange) runRendezvousPunch(pairID, peerPubKey string, candidates []string, startAt time.Time) {
	defer pe.endPunchJob(pairID)

	wait := time.Until(startAt)
	if wait > 0 {
		time.Sleep(wait)
	}

	baselineHandshake := pe.getLatestHandshake(peerPubKey)

	for _, candidate := range candidates {
		log.Printf("[NAT] Rendezvous punching peer %s via candidate %s", shortKey(peerPubKey), candidate)
		peerInfo, err := pe.ExchangeWithPeer(candidate)
		if err != nil {
			continue
		}
		if peerInfo == nil {
			continue
		}
		if peerPubKey != "" && peerInfo.WGPubKey != "" && peerInfo.WGPubKey != peerPubKey {
			log.Printf("[NAT] Candidate %s replied with unexpected peer %s (wanted %s)", candidate, shortKey(peerInfo.WGPubKey), shortKey(peerPubKey))
			continue
		}

		newHandshake := pe.waitForHandshake(peerPubKey, baselineHandshake, HandshakeWaitTimeout)
		if newHandshake <= baselineHandshake {
			log.Printf("[NAT] Control path reached %s via %s but WG handshake not established", shortKey(peerPubKey), candidate)
			continue
		}

		if isIPv6Endpoint(peerInfo.Endpoint) {
			log.Printf("[Path] Direct IPv6 established for pair %s with %s at %s", shortKey(pairID), shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
		} else {
			log.Printf("[NAT] Rendezvous punch succeeded for pair %s with %s at %s", shortKey(pairID), shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
		}
		pe.peerStore.Offer(daemon.Candidate{Info: peerInfo, Source: DHTMethod + "-rendezvous", Hand: daemon.FirstHand})
		return
	}

	log.Printf("[NAT] Rendezvous punch failed for pair %s peer %s", shortKey(pairID), shortKey(peerPubKey))
}

func (pe *PeerExchange) getLatestHandshake(peerPubKey string) int64 {
	if peerPubKey == "" {
		return 0
	}
	hs, err := wireguard.GetLatestHandshakes(pe.config.InterfaceName)
	if err != nil {
		return 0
	}
	return hs[peerPubKey]
}

func (pe *PeerExchange) waitForHandshake(peerPubKey string, baseline int64, timeout time.Duration) int64 {
	if peerPubKey == "" {
		return 0
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current := pe.getLatestHandshake(peerPubKey)
		if current > baseline {
			return current
		}
		time.Sleep(HandshakePollInterval)
	}
	return pe.getLatestHandshake(peerPubKey)
}

func (pe *PeerExchange) beginPunchJob(pairID string) bool {
	pe.rendezvousMu.Lock()
	defer pe.rendezvousMu.Unlock()

	last, exists := pe.activePunches[pairID]
	if exists && time.Since(last) < RendezvousPunchCooldown {
		return false
	}

	pe.activePunches[pairID] = time.Now()
	return true
}

func (pe *PeerExchange) endPunchJob(pairID string) {
	pe.rendezvousMu.Lock()
	defer pe.rendezvousMu.Unlock()
	pe.activePunches[pairID] = time.Now()
}

func pairIDForPeers(a, b string) string {
	return fmt.Sprintf("%016x", pairSeed(a, b))
}

func normalizeCandidates(candidates []string) []string {
	if len(candidates) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		norm := normalizeKnownPeerEndpoint(c)
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}

	return out
}

func isIPv6Endpoint(endpoint string) bool {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.To4() == nil
}

func expandCandidatePorts(candidates []string, spread int) []string {
	if len(candidates) == 0 || spread <= 0 {
		return candidates
	}

	seen := make(map[string]struct{}, len(candidates)*(spread*2+1))
	out := make([]string, 0, len(candidates)*(spread*2+1))

	appendCandidate := func(c string) {
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}

	for _, candidate := range candidates {
		host, portStr, err := net.SplitHostPort(candidate)
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			continue
		}

		appendCandidate(net.JoinHostPort(host, strconv.Itoa(port)))
		for delta := 1; delta <= spread; delta++ {
			if p := port - delta; p > 0 {
				appendCandidate(net.JoinHostPort(host, strconv.Itoa(p)))
			}
			if p := port + delta; p <= 65535 {
				appendCandidate(net.JoinHostPort(host, strconv.Itoa(p)))
			}
		}
	}

	return out
}

func (pe *PeerExchange) updateTransitivePeers(knownPeers []crypto.KnownPeer) {
	for _, kp := range knownPeers {
		if kp.WGPubKey == pe.localNode.WGPubKey {
			continue
		}
		transitivePeer := &daemon.PeerInfo{
			WGPubKey:   kp.WGPubKey,
			Hostname:   kp.Hostname,
			MeshIP:     kp.MeshIP,
			MeshIPv6:   kp.MeshIPv6,
			Endpoint:   filterEndpointForConfig(normalizeKnownPeerEndpoint(kp.WGEndpoint), pe.config.DisableIPv6),
			Introducer: kp.Introducer,
			NATType:    kp.NATType,
		}
		pe.peerStore.Offer(daemon.Candidate{Info: transitivePeer, Source: DHTMethod + "-transitive", Hand: daemon.SecondHand})
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

// resolvePeerEndpoint applies the exchange endpoint rules (node.ResolveEndpoint)
// to a UDP sender; guessed is true when the endpoint is the sender IP with the
// default WireGuard port and nothing advertised.
func resolvePeerEndpoint(advertised string, sender *net.UDPAddr) (endpoint string, guessed bool) {
	var ip net.IP
	if sender != nil {
		ip = sender.IP
	}
	return node.ResolveEndpoint(advertised, ip, daemon.DefaultWGPort, false)
}

// peerEndpoint is resolvePeerEndpoint followed by the IPv6 filter; a filtered
// endpoint is empty and so no longer guessed.
func peerEndpoint(advertised string, sender *net.UDPAddr, disableIPv6 bool) (endpoint string, guessed bool) {
	endpoint, guessed = resolvePeerEndpoint(advertised, sender)
	endpoint = filterEndpointForConfig(endpoint, disableIPv6)
	return endpoint, guessed && endpoint != ""
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

func filterEndpointForConfig(endpoint string, disableIPv6 bool) string {
	if endpoint == "" {
		return ""
	}
	if disableIPv6 && isIPv6Endpoint(endpoint) {
		return ""
	}
	return endpoint
}

func filterCandidatesForConfig(candidates []string, disableIPv6 bool) []string {
	if len(candidates) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		filtered := filterEndpointForConfig(candidate, disableIPv6)
		if filtered == "" {
			continue
		}
		if _, ok := seen[filtered]; ok {
			continue
		}
		seen[filtered] = struct{}{}
		out = append(out, filtered)
	}

	return out
}

func controlEndpointFromPeerEndpoint(endpoint string, controlPort int) string {
	if endpoint == "" || controlPort <= 0 {
		return ""
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(controlPort))
}

// getKnownPeers returns a list of known peers for sharing with other nodes.
// Filters out the local node to prevent self-advertisement via gossip.
func (pe *PeerExchange) getKnownPeers() []crypto.KnownPeer {
	peers := pe.peerStore.GetActive()
	knownPeers := make([]crypto.KnownPeer, 0, len(peers))

	for _, p := range peers {
		// Skip empty pubkeys (useless/poisoned entries) and self (prevents
		// gossip loop where own endpoint is sent back and triggers self-connection)
		if p.WGPubKey == "" || p.WGPubKey == pe.localNode.WGPubKey {
			continue
		}
		// A guessed endpoint is not announced: a wrong address is worse than
		// none, and the peer keeps its key in the list (#839).
		endpoint := p.Endpoint
		if p.EndpointGuessed {
			endpoint = ""
		}
		knownPeers = append(knownPeers, crypto.KnownPeer{
			WGPubKey:   p.WGPubKey,
			Hostname:   p.Hostname,
			MeshIP:     p.MeshIP,
			MeshIPv6:   p.MeshIPv6,
			WGEndpoint: endpoint,
			Introducer: p.Introducer,
			NATType:    p.NATType,
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
		pe.localNode.GetEndpoint(),
		pe.localNode.Introducer,
		pe.localNode.RoutableNetworks,
		knownPeers,
		pe.localNode.Hostname,
		pe.localNode.MeshIPv6,
		string(pe.localNode.NATType),
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal announce: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send announce: %w", err)
	}
	return nil
}

// SendGoodbye sends a shutdown notification to a specific peer exchange endpoint.
func (pe *PeerExchange) SendGoodbye(addr string) error {
	remoteAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to resolve goodbye target %s: %w", addr, err)
	}

	msg := goodbyeMessage{
		Protocol:  crypto.ProtocolVersion,
		Timestamp: time.Now().Unix(),
		WGPubKey:  pe.localNode.WGPubKey,
	}

	data, err := crypto.SealEnvelopeAAD(crypto.MessageTypeGoodbye, msg, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal goodbye: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send goodbye: %w", err)
	}
	return nil
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

func (pe *PeerExchange) logIncomingPacket(messageType string, remoteAddr *net.UDPAddr) {
	if pe.allowLog(messageType, remoteAddr) {
		log.Printf("[Exchange] Received valid %s from %s", messageType, remoteAddr.String())
	}
}

// allowLog reports whether a log line of this kind for this address is due: at
// most one per ExchangeLogCooldown, so a flood of packets cannot fill the log.
func (pe *PeerExchange) allowLog(kind string, remoteAddr *net.UDPAddr) bool {
	if remoteAddr == nil {
		return false
	}
	key := kind + "|" + remoteAddr.String()
	now := time.Now()

	pe.logMu.Lock()
	defer pe.logMu.Unlock()
	last, exists := pe.lastPacketLog[key]
	if exists && now.Sub(last) < ExchangeLogCooldown {
		return false
	}
	// Drop stale entries at most once per cooldown, so a flood of distinct
	// sources cannot make every call scan the whole map under the lock.
	if len(pe.lastPacketLog) > 100 && now.Sub(pe.lastLogCleanup) >= ExchangeLogCooldown {
		pe.lastLogCleanup = now
		for k, t := range pe.lastPacketLog {
			if now.Sub(t) > ExchangeLogCooldown*2 {
				delete(pe.lastPacketLog, k)
			}
		}
	}
	// Hard bound: past it a new kind or source is not logged rather than remembered.
	if len(pe.lastPacketLog) >= maxLogThrottleEntries {
		return false
	}
	pe.lastPacketLog[key] = now
	return true
}
