package rpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
)

// Client is an RPC client that connects to the daemon via Unix socket
type Client struct {
	socketPath string
	conn       net.Conn
	nextID     atomic.Int64
}

// NewClient creates a new RPC client connected to the given socket path
func NewClient(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}

	client := &Client{
		socketPath: socketPath,
		conn:       conn,
	}
	client.nextID.Store(1)

	return client, nil
}

// Call makes an RPC call to the daemon
func (c *Client) Call(method string, params map[string]interface{}) (interface{}, error) {
	// Build request
	req := &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      c.nextID.Add(1),
	}

	// Encode request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Send request (line-delimited JSON)
	if _, err := c.conn.Write(append(reqData, '\n')); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(c.conn)
	respData, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Decode response
	var resp Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for errors
	if resp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// Close closes the connection to the daemon
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
