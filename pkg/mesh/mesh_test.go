package mesh

import (
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"testing"
)

func TestListSimple(t *testing.T) {
	tests := []struct {
		name     string
		nodes    map[string]*Node
		expected []string
	}{
		{
			name: "nodes with actual hostnames",
			nodes: map[string]*Node{
				"node1": {
					Hostname:       "node1",
					MeshIP:         net.ParseIP("10.99.0.1"),
					ActualHostname: "server01",
				},
				"node2": {
					Hostname:       "node2",
					MeshIP:         net.ParseIP("10.99.0.2"),
					ActualHostname: "webserver",
				},
			},
			expected: []string{
				"server01 10.99.0.1",
				"webserver 10.99.0.2",
			},
		},
		{
			name: "nodes without actual hostnames",
			nodes: map[string]*Node{
				"node1": {
					Hostname: "node1",
					MeshIP:   net.ParseIP("10.99.0.1"),
				},
				"node2": {
					Hostname: "node2",
					MeshIP:   net.ParseIP("10.99.0.2"),
				},
			},
			expected: []string{
				"node1 10.99.0.1",
				"node2 10.99.0.2",
			},
		},
		{
			name: "mixed actual and configured hostnames",
			nodes: map[string]*Node{
				"node1": {
					Hostname:       "node1",
					MeshIP:         net.ParseIP("10.99.0.1"),
					ActualHostname: "server01",
				},
				"node2": {
					Hostname: "node2",
					MeshIP:   net.ParseIP("10.99.0.2"),
				},
			},
			expected: []string{
				"server01 10.99.0.1",
				"node2 10.99.0.2",
			},
		},
		{
			name:     "empty nodes",
			nodes:    map[string]*Node{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Mesh{
				Nodes: tt.nodes,
			}

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			m.ListSimple()

			w.Close()
			os.Stdout = old

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := strings.TrimSpace(buf.String())

			if len(tt.expected) == 0 {
				if output != "" {
					t.Errorf("Expected empty output, got: %s", output)
				}
				return
			}

			lines := strings.Split(output, "\n")
			if len(lines) != len(tt.expected) {
				t.Errorf("Expected %d lines, got %d: %v", len(tt.expected), len(lines), lines)
				return
			}

			// Check that all expected lines are present (order may vary due to map iteration)
			expectedSet := make(map[string]bool)
			for _, exp := range tt.expected {
				expectedSet[exp] = true
			}

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if !expectedSet[line] {
					t.Errorf("Unexpected output line: %s", line)
				}
				delete(expectedSet, line)
			}

			if len(expectedSet) > 0 {
				t.Errorf("Missing expected lines: %v", expectedSet)
			}
		})
	}
}

func TestNode_ActualHostnameFields(t *testing.T) {
	// Verify that the Node struct has the new fields
	node := &Node{
		Hostname:       "test",
		MeshIP:         net.ParseIP("10.0.0.1"),
		ActualHostname: "actual-test",
		FQDN:           "test.example.com",
	}

	if node.ActualHostname != "actual-test" {
		t.Errorf("Expected ActualHostname to be 'actual-test', got %s", node.ActualHostname)
	}

	if node.FQDN != "test.example.com" {
		t.Errorf("Expected FQDN to be 'test.example.com', got %s", node.FQDN)
	}
}

func TestListSimple_OutputFormat(t *testing.T) {
	// Test that each line follows the format "hostname ip"
	m := &Mesh{
		Nodes: map[string]*Node{
			"node1": {
				Hostname:       "node1",
				MeshIP:         net.ParseIP("10.99.0.1"),
				ActualHostname: "server01",
			},
		},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	m.ListSimple()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := strings.TrimSpace(buf.String())

	// Verify format: hostname<space>ip
	parts := strings.Fields(output)
	if len(parts) != 2 {
		t.Errorf("Expected 2 fields (hostname ip), got %d: %v", len(parts), parts)
	}

	if parts[0] != "server01" {
		t.Errorf("Expected hostname 'server01', got %s", parts[0])
	}

	ip := net.ParseIP(parts[1])
	if ip == nil {
		t.Errorf("Expected valid IP address, got %s", parts[1])
	}
}

func ExampleMesh_ListSimple() {
	m := &Mesh{
		Nodes: map[string]*Node{
			"node1": {
				Hostname:       "node1",
				MeshIP:         net.ParseIP("10.99.0.1"),
				ActualHostname: "server01",
			},
			"node2": {
				Hostname: "node2",
				MeshIP:   net.ParseIP("10.99.0.2"),
			},
		},
	}

	// Output will vary due to map iteration order, but format is:
	// hostname ip
	// hostname ip
	m.ListSimple()
}

func TestMesh_SaveLoad_WithActualHostname(t *testing.T) {
	// Test that ActualHostname and FQDN fields are properly saved and loaded
	tmpFile := "/tmp/test-mesh-state.json"
	defer os.Remove(tmpFile)

	original := &Mesh{
		InterfaceName: "wg0",
		Network:       "10.99.0.0/16",
		ListenPort:    51820,
		LocalHostname: "localhost",
		Nodes: map[string]*Node{
			"node1": {
				Hostname:       "node1",
				MeshIP:         net.ParseIP("10.99.0.1"),
				PublicKey:      "test-key",
				ActualHostname: "server01",
				FQDN:           "server01.example.com",
				SSHHost:        "192.168.1.1",
				SSHPort:        22,
				ListenPort:     51820,
			},
		},
	}

	// Save
	err := original.Save(tmpFile)
	if err != nil {
		t.Fatalf("Failed to save mesh: %v", err)
	}

	// Load
	loaded, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load mesh: %v", err)
	}

	// Verify ActualHostname and FQDN are preserved
	node := loaded.Nodes["node1"]
	if node == nil {
		t.Fatal("Node not found after load")
	}

	if node.ActualHostname != "server01" {
		t.Errorf("Expected ActualHostname 'server01', got %s", node.ActualHostname)
	}

	if node.FQDN != "server01.example.com" {
		t.Errorf("Expected FQDN 'server01.example.com', got %s", node.FQDN)
	}
}
