package rpc

import (
	"encoding/json"
	"testing"
)

func TestRequestSerialization(t *testing.T) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "peers.list",
		Params:  map[string]interface{}{"test": "value"},
		ID:      1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC 2.0, got %s", decoded.JSONRPC)
	}
	if decoded.Method != "peers.list" {
		t.Errorf("expected method peers.list, got %s", decoded.Method)
	}
}

func TestResponseSerialization(t *testing.T) {
	resp := &Response{
		JSONRPC: "2.0",
		Result:  map[string]interface{}{"peers": []interface{}{}},
		ID:      1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.JSONRPC != "2.0" {
		t.Errorf("expected JSONRPC 2.0, got %s", decoded.JSONRPC)
	}
}

func TestErrorResponse(t *testing.T) {
	resp := &Response{
		JSONRPC: "2.0",
		Error: &Error{
			Code:    ErrCodeMethodNotFound,
			Message: "method not found",
		},
		ID: 1,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal error response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected error to be present")
	}
	if decoded.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("expected error code %d, got %d", ErrCodeMethodNotFound, decoded.Error.Code)
	}
}

func TestPeersListResult(t *testing.T) {
	result := &PeersListResult{
		Peers: []*PeerInfo{
			{
				PubKey:        "test-key",
				MeshIP:        "10.0.0.1",
				Endpoint:      "1.2.3.4:51820",
				LastSeen:      "2024-01-01T00:00:00Z",
				DiscoveredVia: []string{"dht"},
			},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded PeersListResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(decoded.Peers) != 1 {
		t.Errorf("expected 1 peer, got %d", len(decoded.Peers))
	}
	if decoded.Peers[0].PubKey != "test-key" {
		t.Errorf("expected pubkey test-key, got %s", decoded.Peers[0].PubKey)
	}
}

func TestPeersCountResult(t *testing.T) {
	result := &PeersCountResult{
		Active: 5,
		Total:  10,
		Dead:   5,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded PeersCountResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.Active != 5 {
		t.Errorf("expected 5 active peers, got %d", decoded.Active)
	}
	if decoded.Total != 10 {
		t.Errorf("expected 10 total peers, got %d", decoded.Total)
	}
}
