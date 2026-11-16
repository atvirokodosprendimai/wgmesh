package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/flock"
)

// PeerData represents peer information for RPC
type PeerData struct {
	WGPubKey         string
	Hostname         string
	MeshIP           string
	Endpoint         string
	LastSeen         time.Time
	DiscoveredVia    []string
	RoutableNetworks []string
	LatencyMs        *float64
}

// StatusData represents daemon status for RPC
type StatusData struct {
	MeshIP    string
	PubKey    string
	Uptime    time.Duration
	Interface string
}

// ServerConfig configures the RPC server with callback functions
type ServerConfig struct {
	SocketPath    string
	Version       string
	GetPeers      func() []*PeerData
	GetPeer       func(pubKey string) (*PeerData, bool)
	GetPeerCounts func() (active, total, dead int)
	GetStatus     func() *StatusData
}

// Server implements an RPC server using Unix domain sockets
type Server struct {
	socketPath      string
	listener        net.Listener
	version         string
	ctx             context.Context
	cancel          context.CancelFunc
	getPeersFn      func() []*PeerData
	getPeerFn       func(pubKey string) (*PeerData, bool)
	getPeerCountsFn func() (active, total, dead int)
	getStatusFn     func() *StatusData

	// releaseLock frees <socketPath>.lock; set once in NewServer, never
	// written again, so concurrent Stop calls only read it.
	releaseLock func()
	stopOnce    sync.Once
	stopErr     error // written inside stopOnce.Do, read after it returns
}

// NewServer creates a new RPC server.
//
// Words: it takes the lock <SocketPath>.lock before it removes a stale
// socket, and fails (naming the socket path and the holder pid) when the
// lock is held; Stop releases the lock after the socket file is removed.
// Why: the default socket path has no interface in it, so a second daemon
// unlinked the first daemon's live socket (#860, ADR-0005 M3).
// Invariants: with no other holder the behaviour is as before (stale socket
// removed, directory created); a failed NewServer releases what it took.
func NewServer(config ServerConfig) (*Server, error) {
	// Validate required fields
	if config.SocketPath == "" {
		return nil, fmt.Errorf("socket path is required")
	}
	if config.GetPeers == nil || config.GetPeer == nil || config.GetPeerCounts == nil || config.GetStatus == nil {
		return nil, fmt.Errorf("all callback functions are required")
	}

	// Ensure directory exists (the lock file lives beside the socket)
	dir := filepath.Dir(config.SocketPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Own the path before anything is removed.
	releaseLock, holder, err := flock.TryLock(config.SocketPath + ".lock")
	if err != nil {
		if errors.Is(err, flock.ErrHeld) {
			if holder == "" {
				holder = "unknown"
			}
			return nil, fmt.Errorf("RPC socket %s is in use by another daemon (pid %s): %w", config.SocketPath, holder, err)
		}
		return nil, fmt.Errorf("lock RPC socket %s: %w", config.SocketPath, err)
	}

	// Remove existing socket if it exists (handles race condition by ignoring ENOENT)
	if err := os.Remove(config.SocketPath); err != nil && !os.IsNotExist(err) {
		releaseLock()
		// If removal fails for reasons other than "file doesn't exist", verify it's a socket
		if info, statErr := os.Stat(config.SocketPath); statErr == nil {
			if info.Mode()&os.ModeSocket == 0 {
				return nil, fmt.Errorf("path exists but is not a socket: %s", config.SocketPath)
			}
		}
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		socketPath:      config.SocketPath,
		version:         config.Version,
		ctx:             ctx,
		cancel:          cancel,
		getPeersFn:      config.GetPeers,
		getPeerFn:       config.GetPeer,
		getPeerCountsFn: config.GetPeerCounts,
		getStatusFn:     config.GetStatus,
		releaseLock:     releaseLock,
	}

	return s, nil
}

// Start starts the RPC server
func (s *Server) Start() error {
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions to 0600 (owner only)
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		s.listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	log.Printf("RPC server listening on %s", s.socketPath)

	// Accept connections
	go s.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Printf("RPC accept error: %v", err)
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles a single connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	writer := bufio.NewWriter(conn)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Parse request
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := &Response{
				JSONRPC: "2.0",
				Error: &Error{
					Code:    ErrCodeParseError,
					Message: fmt.Sprintf("failed to parse request: %v", err),
				},
				ID: nil,
			}
			s.writeResponse(writer, resp)
			continue
		}

		// Handle request
		resp := s.handleRequest(&req)
		s.writeResponse(writer, resp)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("RPC connection error: %v", err)
	}
}

// writeResponse writes a response to the connection
func (s *Server) writeResponse(w *bufio.Writer, resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to encode response: %v", err)
		return
	}

	if _, err := w.Write(append(data, '\n')); err != nil {
		log.Printf("Failed to write response: %v", err)
		return
	}

	if err := w.Flush(); err != nil {
		log.Printf("Failed to flush response: %v", err)
	}
}

// handleRequest handles a single RPC request
func (s *Server) handleRequest(req *Request) *Response {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	// Validate JSON-RPC version
	if req.JSONRPC != "2.0" {
		resp.Error = &Error{
			Code:    ErrCodeInvalidRequest,
			Message: "invalid jsonrpc version, must be 2.0",
		}
		return resp
	}

	// Dispatch to handler
	switch req.Method {
	case "peers.list":
		result, err := s.handlePeersList(req.Params)
		if err != nil {
			resp.Error = err
		} else {
			resp.Result = result
		}

	case "peers.get":
		result, err := s.handlePeersGet(req.Params)
		if err != nil {
			resp.Error = err
		} else {
			resp.Result = result
		}

	case "peers.count":
		result, err := s.handlePeersCount(req.Params)
		if err != nil {
			resp.Error = err
		} else {
			resp.Result = result
		}

	case "daemon.status":
		result, err := s.handleDaemonStatus(req.Params)
		if err != nil {
			resp.Error = err
		} else {
			resp.Result = result
		}

	case "daemon.ping":
		result, err := s.handleDaemonPing(req.Params)
		if err != nil {
			resp.Error = err
		} else {
			resp.Result = result
		}

	default:
		resp.Error = &Error{
			Code:    ErrCodeMethodNotFound,
			Message: fmt.Sprintf("method not found: %s", req.Method),
		}
	}

	return resp
}

// handlePeersList implements peers.list
func (s *Server) handlePeersList(params map[string]interface{}) (*PeersListResult, *Error) {
	peers := s.getPeersFn()

	result := &PeersListResult{
		Peers: make([]*PeerInfo, 0, len(peers)),
	}

	for _, peer := range peers {
		result.Peers = append(result.Peers, &PeerInfo{
			PubKey:           peer.WGPubKey,
			Hostname:         peer.Hostname,
			MeshIP:           peer.MeshIP,
			Endpoint:         peer.Endpoint,
			LastSeen:         peer.LastSeen.Format(time.RFC3339),
			DiscoveredVia:    peer.DiscoveredVia,
			RoutableNetworks: peer.RoutableNetworks,
			LatencyMs:        peer.LatencyMs,
		})
	}

	return result, nil
}

// handlePeersGet implements peers.get
func (s *Server) handlePeersGet(params map[string]interface{}) (*PeerInfo, *Error) {
	pubkey, ok := params["pubkey"].(string)
	if !ok || pubkey == "" {
		return nil, &Error{
			Code:    ErrCodeInvalidParams,
			Message: "missing or invalid 'pubkey' parameter",
		}
	}

	peer, exists := s.getPeerFn(pubkey)
	if !exists {
		return nil, &Error{
			Code:    ErrCodeInvalidParams,
			Message: fmt.Sprintf("peer not found: %s", pubkey),
		}
	}

	return &PeerInfo{
		PubKey:           peer.WGPubKey,
		Hostname:         peer.Hostname,
		MeshIP:           peer.MeshIP,
		Endpoint:         peer.Endpoint,
		LastSeen:         peer.LastSeen.Format(time.RFC3339),
		DiscoveredVia:    peer.DiscoveredVia,
		RoutableNetworks: peer.RoutableNetworks,
		LatencyMs:        peer.LatencyMs,
	}, nil
}

// handlePeersCount implements peers.count
func (s *Server) handlePeersCount(params map[string]interface{}) (*PeersCountResult, *Error) {
	active, total, dead := s.getPeerCountsFn()

	return &PeersCountResult{
		Active: active,
		Total:  total,
		Dead:   dead,
	}, nil
}

// handleDaemonStatus implements daemon.status
func (s *Server) handleDaemonStatus(params map[string]interface{}) (*DaemonStatusResult, *Error) {
	status := s.getStatusFn()
	if status == nil {
		return nil, &Error{
			Code:    ErrCodeInternalError,
			Message: "daemon status unavailable",
		}
	}

	return &DaemonStatusResult{
		MeshIP:    status.MeshIP,
		PubKey:    status.PubKey,
		Uptime:    status.Uptime,
		Interface: status.Interface,
		Version:   s.version,
	}, nil
}

// handleDaemonPing implements daemon.ping
func (s *Server) handleDaemonPing(params map[string]interface{}) (*DaemonPingResult, *Error) {
	return &DaemonPingResult{
		Pong:    true,
		Version: s.version,
	}, nil
}

// Stop stops the RPC server and releases the socket lock after the socket
// file is removed. It is safe to call twice and from several goroutines.
func (s *Server) Stop() error {
	s.stopOnce.Do(func() {
		s.cancel()

		if s.listener != nil {
			s.listener.Close()
		}

		// Remove socket file, then free the path for the next owner
		if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
			s.stopErr = fmt.Errorf("failed to remove socket: %w", err)
		}
		s.releaseLock()

		log.Printf("RPC server stopped")
	})
	return s.stopErr
}

// GetSocketPath determines the appropriate socket path
func GetSocketPath() string {
	// Check environment variable first
	if path := os.Getenv("WGMESH_SOCKET"); path != "" {
		return path
	}

	// Try /var/run (requires root)
	if IsWritable("/var/run") {
		return "/var/run/wgmesh.sock"
	}

	// Fallback to XDG_RUNTIME_DIR for non-root
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "wgmesh.sock")
	}

	// Last resort: /tmp
	return "/tmp/wgmesh.sock"
}

// IsWritable checks if a directory is writable
func IsWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !info.IsDir() {
		return false
	}

	// Try to create a temporary file using a randomized name
	f, err := os.CreateTemp(path, ".wgmesh-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return false
	}
	if err := os.Remove(name); err != nil {
		return false
	}

	return true
}

// FormatSocketPath formats a socket path for display, shortening home directory
func FormatSocketPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
