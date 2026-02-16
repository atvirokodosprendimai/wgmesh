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
	ExchangeTimeout         = 10 * time.Second
	MaxExchangeSize         = 65536 // 64KB max message size
	ExchangePort            = 51821 // Default exchange port (can be derived from secret)
	PunchInterval           = 300 * time.Millisecond
	RendezvousSessionTTL    = 20 * time.Second
	RendezvousStartLeadTime = 1200 * time.Millisecond
	RendezvousPunchCooldown = 15 * time.Second
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

type rendezvousState struct {
	offers    map[string]*rendezvousOffer
	endpoints map[string]string
	createdAt time.Time
}

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

	rendezvousMu       sync.Mutex
	rendezvousSessions map[string]*rendezvousState
	activePunches      map[string]time.Time
}

// NewPeerExchange creates a new peer exchange handler
func NewPeerExchange(config *daemon.Config, localNode *LocalNode, peerStore *daemon.PeerStore) *PeerExchange {
	return &PeerExchange{
		config:             config,
		localNode:          localNode,
		peerStore:          peerStore,
		stopCh:             make(chan struct{}),
		pendingReplies:     make(map[string]chan *daemon.PeerInfo),
		rendezvousSessions: make(map[string]*rendezvousState),
		activePunches:      make(map[string]time.Time),
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
	envelope, plaintext, err := crypto.OpenEnvelopeRaw(data, pe.config.Keys.GossipKey)
	if err != nil {
		// Could be a DHT message or wrong key - log for debugging
		log.Printf("[Exchange] Received non-wgmesh packet from %s (len=%d, possibly DHT or wrong secret)", remoteAddr.String(), len(data))
		return
	}

	log.Printf("[Exchange] SUCCESS! Received valid %s from wgmesh peer at %s", envelope.MessageType, remoteAddr.String())

	switch envelope.MessageType {
	case crypto.MessageTypeHello:
		var announcement crypto.PeerAnnouncement
		if err := json.Unmarshal(plaintext, &announcement); err != nil {
			log.Printf("[Exchange] Invalid HELLO payload from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.handleHello(&announcement, remoteAddr)
	case crypto.MessageTypeReply:
		var reply crypto.PeerAnnouncement
		if err := json.Unmarshal(plaintext, &reply); err != nil {
			log.Printf("[Exchange] Invalid REPLY payload from %s: %v", remoteAddr.String(), err)
			return
		}
		pe.handleReply(&reply, remoteAddr)
	case crypto.MessageTypeAnnounce:
		var announcement crypto.PeerAnnouncement
		if err := json.Unmarshal(plaintext, &announcement); err != nil {
			log.Printf("[Exchange] Invalid ANNOUNCE payload from %s: %v", remoteAddr.String(), err)
			return
		}
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
		Introducer:       announcement.Introducer,
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
		Introducer:       reply.Introducer,
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
		pe.localNode.Introducer,
		pe.localNode.RoutableNetworks,
		knownPeers,
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeReply, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal reply: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	return err
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
		pe.localNode.Introducer,
		pe.localNode.RoutableNetworks,
		knownPeers,
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeHello, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return nil, fmt.Errorf("failed to seal hello: %w", err)
	}

	log.Printf("[Exchange] Sending HELLO to %s (our exchange port: %d)", remoteAddr.String(), pe.port)
	log.Printf("[NAT] Punch attempt started with %s (timeout=%v interval=%v)", remoteAddr.String(), ExchangeTimeout, PunchInterval)

	attempts := 0

	sendHello := func() error {
		attempts++
		_, sendErr := pe.conn.WriteToUDP(data, remoteAddr)
		if sendErr != nil {
			return fmt.Errorf("failed to send hello: %w", sendErr)
		}
		return nil
	}

	// Send initial HELLO immediately.
	if err := sendHello(); err != nil {
		return nil, err
	}

	// During the exchange window, keep sending HELLO packets.
	// This improves NAT hole punching success when both peers are behind NAT.
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
		Candidates:    normalizeCandidates(candidates),
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
	if offer == nil || remoteAddr == nil {
		return
	}
	if offer.FromPubKey == "" || offer.TargetPubKey == "" {
		return
	}
	if offer.FromPubKey == pe.localNode.WGPubKey || offer.TargetPubKey == pe.localNode.WGPubKey {
		// We only act as introducer here.
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
	candidates = normalizeCandidates(candidates)
	log.Printf("[NAT] Introducer %s received offer pair=%s from=%s target=%s observed=%s candidates=%v", shortKey(pe.localNode.WGPubKey), shortKey(pairID), shortKey(offer.FromPubKey), shortKey(offer.TargetPubKey), observed, candidates)

	pe.rendezvousMu.Lock()
	defer pe.rendezvousMu.Unlock()

	now := time.Now()
	for id, st := range pe.rendezvousSessions {
		if now.Sub(st.createdAt) > RendezvousSessionTTL {
			delete(pe.rendezvousSessions, id)
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
					Candidates:    normalizeCandidates([]string{targetControl}),
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
		PeerCandidates: normalizeCandidates(peerCandidates),
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

	startAt := time.UnixMilli(start.StartAtUnixMs)
	if startAt.IsZero() {
		startAt = time.Now().Add(100 * time.Millisecond)
	}

	candidates := append([]string{}, start.PeerCandidates...)
	candidates = normalizeCandidates(candidates)
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

		log.Printf("[NAT] Rendezvous punch succeeded for pair %s with %s at %s", shortKey(pairID), shortKey(peerInfo.WGPubKey), peerInfo.Endpoint)
		pe.peerStore.Update(peerInfo, DHTMethod+"-rendezvous")
		return
	}

	log.Printf("[NAT] Rendezvous punch failed for pair %s peer %s", shortKey(pairID), shortKey(peerPubKey))
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

func (pe *PeerExchange) updateTransitivePeers(knownPeers []crypto.KnownPeer) {
	for _, kp := range knownPeers {
		if kp.WGPubKey == pe.localNode.WGPubKey {
			continue
		}
		transitivePeer := &daemon.PeerInfo{
			WGPubKey:   kp.WGPubKey,
			MeshIP:     kp.MeshIP,
			Endpoint:   normalizeKnownPeerEndpoint(kp.WGEndpoint),
			Introducer: kp.Introducer,
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

// getKnownPeers returns a list of known peers for sharing with other nodes
func (pe *PeerExchange) getKnownPeers() []crypto.KnownPeer {
	peers := pe.peerStore.GetActive()
	knownPeers := make([]crypto.KnownPeer, 0, len(peers))

	for _, p := range peers {
		knownPeers = append(knownPeers, crypto.KnownPeer{
			WGPubKey:   p.WGPubKey,
			MeshIP:     p.MeshIP,
			WGEndpoint: p.Endpoint,
			Introducer: p.Introducer,
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
		pe.localNode.Introducer,
		pe.localNode.RoutableNetworks,
		knownPeers,
	)

	data, err := crypto.SealEnvelope(crypto.MessageTypeAnnounce, announcement, pe.config.Keys.GossipKey)
	if err != nil {
		return fmt.Errorf("failed to seal announce: %w", err)
	}

	_, err = pe.conn.WriteToUDP(data, remoteAddr)
	return err
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
