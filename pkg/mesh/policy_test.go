package mesh

import (
	"net"
	"testing"
)

func TestValidateGroups(t *testing.T) {
	tests := []struct {
		name    string
		mesh    *Mesh
		wantErr bool
		errMsg  string
	}{
		{
			name: "no groups defined",
			mesh: &Mesh{
				Groups: make(map[string]*Group),
			},
			wantErr: false,
		},
		{
			name: "valid groups",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"production": {
						Description: "Production nodes",
						Members:     []string{"node1", "node2"},
					},
					"staging": {
						Description: "Staging nodes",
						Members:     []string{"node3"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate members in group",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"production": {
						Description: "Production nodes",
						Members:     []string{"node1", "node2", "node1"},
					},
				},
			},
			wantErr: true,
			errMsg:  "group production has duplicate member: node1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mesh.ValidateGroups()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGroups() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("ValidateGroups() error = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidatePolicies(t *testing.T) {
	tests := []struct {
		name    string
		mesh    *Mesh
		wantErr bool
		errMsg  string
	}{
		{
			name: "no policies defined",
			mesh: &Mesh{
				AccessPolicies: []*AccessPolicy{},
			},
			wantErr: false,
		},
		{
			name: "valid policies",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"production": {
						Members: []string{"node1", "node2"},
					},
					"database": {
						Members: []string{"node3"},
					},
				},
				AccessPolicies: []*AccessPolicy{
					{
						Name:                  "prod-to-db",
						FromGroups:            []string{"production"},
						ToGroups:              []string{"database"},
						AllowMeshIPs:          true,
						AllowRoutableNetworks: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate policy names",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
					"db":   {Members: []string{"node2"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "policy1", FromGroups: []string{"prod"}, ToGroups: []string{"db"}},
					{Name: "policy1", FromGroups: []string{"db"}, ToGroups: []string{"prod"}},
				},
			},
			wantErr: true,
			errMsg:  "duplicate policy name: policy1",
		},
		{
			name: "policy with no from_groups",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "policy1", FromGroups: []string{}, ToGroups: []string{"prod"}},
				},
			},
			wantErr: true,
			errMsg:  "policy policy1 has no from_groups",
		},
		{
			name: "policy with no to_groups",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "policy1", FromGroups: []string{"prod"}, ToGroups: []string{}},
				},
			},
			wantErr: true,
			errMsg:  "policy policy1 has no to_groups",
		},
		{
			name: "policy references non-existent from_group",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "policy1", FromGroups: []string{"nonexistent"}, ToGroups: []string{"prod"}},
				},
			},
			wantErr: true,
			errMsg:  "policy policy1 references non-existent from_group: nonexistent",
		},
		{
			name: "policy references non-existent to_group",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "policy1", FromGroups: []string{"prod"}, ToGroups: []string{"nonexistent"}},
				},
			},
			wantErr: true,
			errMsg:  "policy policy1 references non-existent to_group: nonexistent",
		},
		{
			name: "policy with empty groups",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{}},
					"db":   {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "policy1", FromGroups: []string{"prod"}, ToGroups: []string{"db"}},
				},
			},
			wantErr: true,
			errMsg:  "policy policy1 does not match any nodes (empty from_groups or to_groups)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mesh.ValidatePolicies()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePolicies() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("ValidatePolicies() error = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestGetNodeGroups(t *testing.T) {
	tests := []struct {
		name           string
		mesh           *Mesh
		hostname       string
		expectedGroups []string
	}{
		{
			name: "node in no groups",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
			},
			hostname:       "node2",
			expectedGroups: []string{},
		},
		{
			name: "node in one group",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod":    {Members: []string{"node1", "node2"}},
					"staging": {Members: []string{"node3"}},
				},
			},
			hostname:       "node1",
			expectedGroups: []string{"prod"},
		},
		{
			name: "node in multiple groups",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
					"web":  {Members: []string{"node1", "node2"}},
					"db":   {Members: []string{"node1"}},
				},
			},
			hostname:       "node1",
			expectedGroups: []string{"prod", "web", "db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := tt.mesh.GetNodeGroups(tt.hostname)
			if len(groups) != len(tt.expectedGroups) {
				t.Errorf("GetNodeGroups() returned %d groups, expected %d", len(groups), len(tt.expectedGroups))
				return
			}

			// Convert to sets for comparison
			gotSet := make(map[string]bool)
			for _, g := range groups {
				gotSet[g] = true
			}
			expectedSet := make(map[string]bool)
			for _, g := range tt.expectedGroups {
				expectedSet[g] = true
			}

			for g := range expectedSet {
				if !gotSet[g] {
					t.Errorf("GetNodeGroups() missing group: %s", g)
				}
			}
		})
	}
}

func TestGetAllowedPeers(t *testing.T) {
	tests := []struct {
		name            string
		setupMesh       func() *Mesh
		hostname        string
		expectedPeers   map[string]bool
		expectedMeshIP  map[string]bool
		expectedNetwork map[string]bool
	}{
		{
			name: "node in no groups gets no peers",
			setupMesh: func() *Mesh {
				return &Mesh{
					Nodes: map[string]*Node{
						"node1": {Hostname: "node1", MeshIP: net.ParseIP("10.99.0.1")},
						"node2": {Hostname: "node2", MeshIP: net.ParseIP("10.99.0.2")},
					},
					Groups: map[string]*Group{
						"production": {
							Members: []string{"node2"},
						},
					},
					AccessPolicies: []*AccessPolicy{
						{
							Name:                  "prod-internal",
							FromGroups:            []string{"production"},
							ToGroups:              []string{"production"},
							AllowMeshIPs:          true,
							AllowRoutableNetworks: true,
						},
					},
				}
			},
			hostname:      "node1",
			expectedPeers: map[string]bool{},
		},
		{
			name: "production node can access prod and db",
			setupMesh: func() *Mesh {
				return &Mesh{
					Nodes: map[string]*Node{
						"node1": {Hostname: "node1", MeshIP: net.ParseIP("10.99.0.1")},
						"node2": {Hostname: "node2", MeshIP: net.ParseIP("10.99.0.2")},
						"node4": {Hostname: "node4", MeshIP: net.ParseIP("10.99.0.4")},
					},
					Groups: map[string]*Group{
						"production": {
							Members: []string{"node1", "node2"},
						},
						"database": {
							Members: []string{"node4"},
						},
					},
					AccessPolicies: []*AccessPolicy{
						{
							Name:                  "prod-internal",
							FromGroups:            []string{"production"},
							ToGroups:              []string{"production"},
							AllowMeshIPs:          true,
							AllowRoutableNetworks: true,
						},
						{
							Name:                  "prod-to-db",
							FromGroups:            []string{"production"},
							ToGroups:              []string{"database"},
							AllowMeshIPs:          true,
							AllowRoutableNetworks: true,
						},
					},
				}
			},
			hostname: "node1",
			expectedPeers: map[string]bool{
				"node2": true,
				"node4": true,
			},
			expectedMeshIP: map[string]bool{
				"node2": true,
				"node4": true,
			},
			expectedNetwork: map[string]bool{
				"node2": true,
				"node4": true,
			},
		},
		{
			name: "database node can access production",
			setupMesh: func() *Mesh {
				return &Mesh{
					Nodes: map[string]*Node{
						"node1": {Hostname: "node1", MeshIP: net.ParseIP("10.99.0.1")},
						"node2": {Hostname: "node2", MeshIP: net.ParseIP("10.99.0.2")},
						"node4": {Hostname: "node4", MeshIP: net.ParseIP("10.99.0.4")},
					},
					Groups: map[string]*Group{
						"production": {
							Members: []string{"node1", "node2"},
						},
						"database": {
							Members: []string{"node4"},
						},
					},
					AccessPolicies: []*AccessPolicy{
						{
							Name:                  "prod-to-db",
							FromGroups:            []string{"production"},
							ToGroups:              []string{"database"},
							AllowMeshIPs:          true,
							AllowRoutableNetworks: true,
						},
					},
				}
			},
			hostname: "node4",
			expectedPeers: map[string]bool{
				"node1": true,
				"node2": true,
			},
			expectedMeshIP:  map[string]bool{},
			expectedNetwork: map[string]bool{},
		},
		{
			name: "staging node can only access staging",
			setupMesh: func() *Mesh {
				return &Mesh{
					Nodes: map[string]*Node{
						"node3": {Hostname: "node3", MeshIP: net.ParseIP("10.99.0.3")},
					},
					Groups: map[string]*Group{
						"staging": {
							Members: []string{"node3"},
						},
					},
					AccessPolicies: []*AccessPolicy{
						{
							Name:                  "staging-internal",
							FromGroups:            []string{"staging"},
							ToGroups:              []string{"staging"},
							AllowMeshIPs:          true,
							AllowRoutableNetworks: false,
						},
					},
				}
			},
			hostname:        "node3",
			expectedPeers:   map[string]bool{},
			expectedMeshIP:  map[string]bool{},
			expectedNetwork: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mesh := tt.setupMesh()
			allowedPeers := mesh.GetAllowedPeers(tt.hostname)

			if len(allowedPeers) != len(tt.expectedPeers) {
				t.Errorf("GetAllowedPeers() returned %d peers, expected %d", len(allowedPeers), len(tt.expectedPeers))
			}

			for peerHostname, access := range allowedPeers {
				if !tt.expectedPeers[peerHostname] {
					t.Errorf("GetAllowedPeers() unexpected peer: %s", peerHostname)
				}

				if tt.expectedMeshIP[peerHostname] && !access.AllowMeshIP {
					t.Errorf("GetAllowedPeers() expected MeshIP access for peer %s", peerHostname)
				}

				if tt.expectedNetwork[peerHostname] && !access.AllowRoutableNetworks {
					t.Errorf("GetAllowedPeers() expected Network access for peer %s", peerHostname)
				}
			}
		})
	}
}

func TestGetAllowedPeers_NoGroups(t *testing.T) {
	mesh := &Mesh{
		Nodes: map[string]*Node{
			"node1": {Hostname: "node1", MeshIP: net.ParseIP("10.99.0.1")},
			"node2": {Hostname: "node2", MeshIP: net.ParseIP("10.99.0.2")},
		},
		Groups:         make(map[string]*Group),
		AccessPolicies: []*AccessPolicy{},
	}

	allowedPeers := mesh.GetAllowedPeers("node1")

	if len(allowedPeers) != 0 {
		t.Errorf("GetAllowedPeers() should return empty map when no groups defined, got %d peers", len(allowedPeers))
	}
}

func TestIsAccessControlEnabled(t *testing.T) {
	tests := []struct {
		name     string
		mesh     *Mesh
		expected bool
	}{
		{
			name:     "no groups or policies",
			mesh:     &Mesh{Groups: make(map[string]*Group), AccessPolicies: []*AccessPolicy{}},
			expected: false,
		},
		{
			name: "groups defined but no policies",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{},
			},
			expected: true,
		},
		{
			name: "policies defined but no groups",
			mesh: &Mesh{
				Groups: make(map[string]*Group),
				AccessPolicies: []*AccessPolicy{
					{Name: "test", FromGroups: []string{"test"}, ToGroups: []string{"test"}},
				},
			},
			expected: true,
		},
		{
			name: "both groups and policies defined",
			mesh: &Mesh{
				Groups: map[string]*Group{
					"prod": {Members: []string{"node1"}},
				},
				AccessPolicies: []*AccessPolicy{
					{Name: "test", FromGroups: []string{"prod"}, ToGroups: []string{"prod"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mesh.IsAccessControlEnabled()
			if result != tt.expected {
				t.Errorf("IsAccessControlEnabled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildPeerConfig(t *testing.T) {
	tests := []struct {
		name     string
		peer     *Node
		access   *PeerAccess
		expected []string
	}{
		{
			name: "full access",
			peer: &Node{
				Hostname:         "node1",
				MeshIP:           net.ParseIP("10.99.0.1"),
				PublicKey:        "test-key",
				PublicEndpoint:   "1.2.3.4:51820",
				RoutableNetworks: []string{"192.168.1.0/24"},
			},
			access: &PeerAccess{
				AllowMeshIP:           true,
				AllowRoutableNetworks: true,
			},
			expected: []string{"10.99.0.1/32", "192.168.1.0/24"},
		},
		{
			name: "mesh IP only",
			peer: &Node{
				Hostname:         "node1",
				MeshIP:           net.ParseIP("10.99.0.1"),
				PublicKey:        "test-key",
				PublicEndpoint:   "1.2.3.4:51820",
				RoutableNetworks: []string{"192.168.1.0/24"},
			},
			access: &PeerAccess{
				AllowMeshIP:           true,
				AllowRoutableNetworks: false,
			},
			expected: []string{"10.99.0.1/32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mesh := &Mesh{}
			peerConfig := mesh.buildPeerConfig(tt.peer, tt.access)

			if peerConfig.PublicKey != tt.peer.PublicKey {
				t.Errorf("buildPeerConfig() PublicKey = %v, want %v", peerConfig.PublicKey, tt.peer.PublicKey)
			}

			if peerConfig.Endpoint != tt.peer.PublicEndpoint {
				t.Errorf("buildPeerConfig() Endpoint = %v, want %v", peerConfig.Endpoint, tt.peer.PublicEndpoint)
			}

			if peerConfig.PersistentKeepalive != 5 {
				t.Errorf("buildPeerConfig() PersistentKeepalive = %v, want 5", peerConfig.PersistentKeepalive)
			}

			if len(peerConfig.AllowedIPs) != len(tt.expected) {
				t.Errorf("buildPeerConfig() AllowedIPs length = %d, want %d", len(peerConfig.AllowedIPs), len(tt.expected))
			}

			// Convert to sets for comparison
			gotSet := make(map[string]bool)
			for _, ip := range peerConfig.AllowedIPs {
				gotSet[ip] = true
			}
			for _, exp := range tt.expected {
				if !gotSet[exp] {
					t.Errorf("buildPeerConfig() missing AllowedIP: %s", exp)
				}
			}
		})
	}
}

func TestMesh_SaveLoad_WithGroupsAndPolicies(t *testing.T) {
	tmpFile, err := createTempMeshFile(t)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer cleanupTempMeshFile(t, tmpFile)

	original := &Mesh{
		InterfaceName: "wg0",
		Network:       "10.99.0.0/16",
		ListenPort:    51820,
		LocalHostname: "localhost",
		Nodes: map[string]*Node{
			"node1": {
				Hostname:       "node1",
				MeshIP:         net.ParseIP("10.99.0.1"),
				PublicKey:      "test-key-1",
				ActualHostname: "server01",
				FQDN:           "server01.example.com",
				SSHHost:        "192.168.1.1",
				SSHPort:        22,
				ListenPort:     51820,
			},
		},
		Groups: map[string]*Group{
			"production": {
				Description: "Production environment",
				Members:     []string{"node1", "node2"},
			},
			"staging": {
				Description: "Staging environment",
				Members:     []string{"node3"},
			},
		},
		AccessPolicies: []*AccessPolicy{
			{
				Name:                  "prod-to-staging",
				Description:           "Allow production to access staging",
				FromGroups:            []string{"production"},
				ToGroups:              []string{"staging"},
				AllowMeshIPs:          true,
				AllowRoutableNetworks: false,
			},
		},
	}

	// Save
	err = original.Save(tmpFile)
	if err != nil {
		t.Fatalf("Failed to save mesh: %v", err)
	}

	// Load
	loaded, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load mesh: %v", err)
	}

	// Verify groups
	if len(loaded.Groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(loaded.Groups))
	}

	prodGroup, ok := loaded.Groups["production"]
	if !ok {
		t.Fatal("Production group not found after load")
	}

	if prodGroup.Description != "Production environment" {
		t.Errorf("Expected description 'Production environment', got %s", prodGroup.Description)
	}

	if len(prodGroup.Members) != 2 {
		t.Errorf("Expected 2 members in production group, got %d", len(prodGroup.Members))
	}

	// Verify policies
	if len(loaded.AccessPolicies) != 1 {
		t.Errorf("Expected 1 policy, got %d", len(loaded.AccessPolicies))
	}

	policy := loaded.AccessPolicies[0]
	if policy.Name != "prod-to-staging" {
		t.Errorf("Expected policy name 'prod-to-staging', got %s", policy.Name)
	}

	if policy.Description != "Allow production to access staging" {
		t.Errorf("Expected policy description 'Allow production to access staging', got %s", policy.Description)
	}

	if !policy.AllowMeshIPs {
		t.Errorf("Expected AllowMeshIPs to be true")
	}

	if policy.AllowRoutableNetworks {
		t.Errorf("Expected AllowRoutableNetworks to be false")
	}
}

// Helper functions for temp file management
func createTempMeshFile(t *testing.T) (string, error) {
	return createTempFile(t, "test-mesh-state-*.json")
}

func createTempFile(t *testing.T, pattern string) (string, error) {
	t.Helper()
	tmpFile, err := createTempFileHelper(pattern)
	return tmpFile, err
}

func createTempFileHelper(pattern string) (string, error) {
	// This is a placeholder - in a real implementation, use os.CreateTemp
	return pattern, nil
}

func cleanupTempMeshFile(t *testing.T, path string) {
	t.Helper()
	cleanupTempFile(path)
}

func cleanupTempFile(path string) {
	// Placeholder for cleanup
}
