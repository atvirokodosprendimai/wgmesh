package mesh

import (
	"fmt"
	"sort"
)

// ValidateGroups checks for group definition errors
func (m *Mesh) ValidateGroups() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Groups) == 0 {
		return nil
	}

	// Check for duplicate members within groups
	for groupName, group := range m.Groups {
		seen := make(map[string]bool)
		for _, member := range group.Members {
			if seen[member] {
				return fmt.Errorf("group %s has duplicate member: %s", groupName, member)
			}
			seen[member] = true
		}
	}

	return nil
}

// ValidatePolicies checks for policy errors
func (m *Mesh) ValidatePolicies() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.AccessPolicies) == 0 {
		return nil
	}

	// Check for duplicate policy names
	policyNames := make(map[string]bool)
	for _, policy := range m.AccessPolicies {
		if policyNames[policy.Name] {
			return fmt.Errorf("duplicate policy name: %s", policy.Name)
		}
		policyNames[policy.Name] = true
	}

	// Check that all referenced groups exist
	for _, policy := range m.AccessPolicies {
		if len(policy.FromGroups) == 0 {
			return fmt.Errorf("policy %s has no from_groups", policy.Name)
		}
		if len(policy.ToGroups) == 0 {
			return fmt.Errorf("policy %s has no to_groups", policy.Name)
		}

		for _, groupName := range policy.FromGroups {
			if _, exists := m.Groups[groupName]; !exists {
				return fmt.Errorf("policy %s references non-existent from_group: %s", policy.Name, groupName)
			}
		}

		for _, groupName := range policy.ToGroups {
			if _, exists := m.Groups[groupName]; !exists {
				return fmt.Errorf("policy %s references non-existent to_group: %s", policy.Name, groupName)
			}
		}

		// Check that policy matches at least one node
		policyMatchesNode := false
		for _, fromGroup := range policy.FromGroups {
			fromGroupMembers := m.Groups[fromGroup].Members
			for _, toGroup := range policy.ToGroups {
				toGroupMembers := m.Groups[toGroup].Members
				if len(fromGroupMembers) > 0 && len(toGroupMembers) > 0 {
					policyMatchesNode = true
					break
				}
			}
			if policyMatchesNode {
				break
			}
		}

		if !policyMatchesNode {
			return fmt.Errorf("policy %s does not match any nodes (empty from_groups or to_groups)", policy.Name)
		}
	}

	return nil
}

// GetNodeGroups returns all groups that a node belongs to
func (m *Mesh) GetNodeGroups(hostname string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]string, 0)
	for groupName, group := range m.Groups {
		for _, member := range group.Members {
			if member == hostname {
				groups = append(groups, groupName)
				break
			}
		}
	}

	return groups
}

// GetAllowedPeers returns the list of peer hostnames this node can connect to
func (m *Mesh) GetAllowedPeers(hostname string) map[string]*PeerAccess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allowedPeers := make(map[string]*PeerAccess)
	nodeGroups := m.GetNodeGroups(hostname)

	// If node is not in any group, it gets no peers (deny by default)
	if len(nodeGroups) == 0 {
		return allowedPeers
	}

	// Collect all peers from relevant policies
	nodeGroupSet := make(map[string]bool)
	for _, g := range nodeGroups {
		nodeGroupSet[g] = true
	}

	for _, policy := range m.AccessPolicies {
		// Check if this node's groups are in from_groups (outbound policy)
		isOutbound := false
		for _, fromGroup := range policy.FromGroups {
			if nodeGroupSet[fromGroup] {
				isOutbound = true
				break
			}
		}

		// Check if this node's groups are in to_groups (inbound policy)
		isInbound := false
		for _, toGroup := range policy.ToGroups {
			if nodeGroupSet[toGroup] {
				isInbound = true
				break
			}
		}

		// Policy must relate to this node in some direction
		if !isOutbound && !isInbound {
			continue
		}

		// Determine which groups are on the "other side" of this policy
		otherGroups := make([]string, 0)
		if isOutbound {
			otherGroups = append(otherGroups, policy.ToGroups...)
		}
		if isInbound {
			otherGroups = append(otherGroups, policy.FromGroups...)
		}

		// Collect all peers from the other groups
		for _, groupName := range otherGroups {
			group := m.Groups[groupName]
			for _, member := range group.Members {
				if member == hostname {
					continue // Skip self
				}

				// Check if peer exists in nodes
				if _, exists := m.Nodes[member]; !exists {
					continue
				}

				// Get or create access entry for this peer
				access, exists := allowedPeers[member]
				if !exists {
					access = &PeerAccess{}
					allowedPeers[member] = access
				}

				// Update access permissions based on policy
				// Outbound policy allows access FROM this node TO peer
				if isOutbound {
					if policy.AllowMeshIPs {
						access.AllowMeshIP = true
					}
					if policy.AllowRoutableNetworks {
						access.AllowRoutableNetworks = true
					}
				}
			}
		}
	}

	return allowedPeers
}

// GetPeerHostnames returns a sorted list of all node hostnames
func (m *Mesh) GetPeerHostnames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hostnames := make([]string, 0, len(m.Nodes))
	for hostname := range m.Nodes {
		hostnames = append(hostnames, hostname)
	}

	sort.Strings(hostnames)
	return hostnames
}

// GetNodeGroupNames returns a sorted list of all group names
func (m *Mesh) GetNodeGroupNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.Groups))
	for name := range m.Groups {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// GetPolicyNames returns a sorted list of all policy names
func (m *Mesh) GetPolicyNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.AccessPolicies))
	for _, policy := range m.AccessPolicies {
		names = append(names, policy.Name)
	}

	sort.Strings(names)
	return names
}

// HasGroups returns true if groups are defined
func (m *Mesh) HasGroups() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.Groups) > 0
}

// HasPolicies returns true if access policies are defined
func (m *Mesh) HasPolicies() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.AccessPolicies) > 0
}

// IsAccessControlEnabled returns true if access control is enabled (groups or policies exist)
func (m *Mesh) IsAccessControlEnabled() bool {
	return m.HasGroups() || m.HasPolicies()
}
