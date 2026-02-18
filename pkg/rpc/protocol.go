package rpc

import (
	"time"
)

// JSON-RPC 2.0 protocol structures

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string                 `json:"jsonrpc"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
	ID      interface{}            `json:"id"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

// Error represents a JSON-RPC 2.0 error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC 2.0 error codes
const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternalError  = -32603
)

// PeerInfo represents peer information in RPC responses
type PeerInfo struct {
	PubKey           string   `json:"pubkey"`
	MeshIP           string   `json:"mesh_ip"`
	Endpoint         string   `json:"endpoint"`
	LastSeen         string   `json:"last_seen"` // ISO 8601 format
	DiscoveredVia    []string `json:"discovered_via"`
	RoutableNetworks []string `json:"routable_networks,omitempty"`
}

// PeersListResult represents the result of peers.list
type PeersListResult struct {
	Peers []*PeerInfo `json:"peers"`
}

// PeersCountResult represents the result of peers.count
type PeersCountResult struct {
	Active int `json:"active"`
	Total  int `json:"total"`
	Dead   int `json:"dead"`
}

// DaemonStatusResult represents the result of daemon.status
type DaemonStatusResult struct {
	MeshIP    string        `json:"mesh_ip"`
	PubKey    string        `json:"pubkey"`
	Uptime    time.Duration `json:"uptime"`
	Interface string        `json:"interface"`
	Version   string        `json:"version"`
}

// DaemonPingResult represents the result of daemon.ping
type DaemonPingResult struct {
	Pong    bool   `json:"pong"`
	Version string `json:"version"`
}
