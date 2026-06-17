package onboarding

import (
	"fmt"
	"time"
)

// ChecklistItem represents a single onboarding step
type ChecklistItem struct {
	ID          string
	Name        string
	Description string
	Validate    func() error
	Remediate   string // Fallback guidance when validation fails
}

// ChecklistStep represents the ordered sequence of onboarding items
var ChecklistSteps = []ChecklistItem{
	{
		ID:          "secret_generation",
		Name:        "Secret Generation",
		Description: "Generate a cryptographically secure shared secret",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Generate a secret using: wgmesh init --secret",
	},
	{
		ID:          "interface_config",
		Name:        "Interface Configuration",
		Description: "Configure WireGuard interface with valid keys",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Ensure WireGuard is installed and you have permissions to configure interfaces",
	},
	{
		ID:          "github_registry",
		Name:        "GitHub Registry Connectivity",
		Description: "Verify GitHub Issues registry is reachable",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Check network connectivity and firewall rules. GitHub API must be reachable (api.github.com). Try: --skip-registry",
	},
	{
		ID:          "lan_multicast",
		Name:        "LAN Multicast Discovery",
		Description: "Bind UDP multicast socket for local peer discovery",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Verify you have permissions to bind multicast sockets. Try: --discovery=dht-only",
	},
	{
		ID:          "dht_bootstrap",
		Name:        "DHT Bootstrap",
		Description: "Contact BitTorrent DHT bootstrap nodes",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Check outbound UDP connectivity to router.bittorrent.com:6881. Verify firewall allows UDP traffic.",
	},
	{
		ID:          "first_peer_contact",
		Name:        "First Peer Discovery",
		Description: "Discover at least one peer via any discovery layer",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Ensure at least one other node is running with the same secret. Check discovery logs for errors.",
	},
	{
		ID:          "bidirectional_ping",
		Name:        "Bidirectional Handshake",
		Description: "Successful WireGuard handshake with discovered peer",
		Validate:    nil, // Set in NewChecklist
		Remediate:   "Check peer is online and WireGuard interface is up. Run: wg show wg0 latest-handshakes",
	},
}

// Checklist represents the onboarding checklist state
type Checklist struct {
	items        []*ChecklistItemState
	startedAt    time.Time
	lastUpdated  time.Time
	currentIndex int
}

// ChecklistItemState tracks the state of a single checklist item
type ChecklistItemState struct {
	Item      ChecklistItem
	Status    string // "pending", "running", "complete", "failed"
	Error     string
	Completed *time.Time
}

// NewChecklist creates a new checklist with validator functions
func NewChecklist(secret string, opts WizardOptions) *Checklist {
	c := &Checklist{
		items:        make([]*ChecklistItemState, len(ChecklistSteps)),
		startedAt:    time.Now(),
		currentIndex: 0,
	}

	// Copy steps and attach validators
	for i, step := range ChecklistSteps {
		item := &ChecklistItemState{
			Item:   step,
			Status: "pending",
		}

		// Attach validator function based on step ID
		switch step.ID {
		case "secret_generation":
			item.Item.Validate = validateSecretGeneration(secret)
		case "interface_config":
			item.Item.Validate = validateInterfaceConfig(opts.InterfaceName)
		case "github_registry":
			item.Item.Validate = validateGitHubRegistry(opts.SkipRegistry)
		case "lan_multicast":
			item.Item.Validate = validateLANMulticast(opts.DisableLANDiscovery)
		case "dht_bootstrap":
			item.Item.Validate = validateDHTBootstrap()
		case "first_peer_contact":
			item.Item.Validate = validateFirstPeerContact(opts.PeerTimeout)
		case "bidirectional_ping":
			item.Item.Validate = validateBidirectionalPing(opts.InterfaceName)
		}

		c.items[i] = item
	}

	return c
}

// CurrentItem returns the current pending item
func (c *Checklist) CurrentItem() *ChecklistItemState {
	if c.currentIndex >= len(c.items) {
		return nil
	}
	return c.items[c.currentIndex]
}

// Advance moves to the next item
func (c *Checklist) Advance() {
	c.currentIndex++
	c.lastUpdated = time.Now()
}

// MarkComplete marks the current item as complete
func (c *Checklist) MarkComplete() {
	if c.CurrentItem() != nil {
		now := time.Now()
		c.CurrentItem().Status = "complete"
		c.CurrentItem().Completed = &now
		c.lastUpdated = now
	}
}

// MarkFailed marks the current item as failed
func (c *Checklist) MarkFailed(err error) {
	if c.CurrentItem() != nil {
		c.CurrentItem().Status = "failed"
		c.CurrentItem().Error = err.Error()
		c.lastUpdated = time.Now()
	}
}

// IsComplete returns true if all items are complete
func (c *Checklist) IsComplete() bool {
	for _, item := range c.items {
		if item.Status != "complete" {
			return false
		}
	}
	return true
}

// CompletedCount returns the number of completed items
func (c *Checklist) CompletedCount() int {
	count := 0
	for _, item := range c.items {
		if item.Status == "complete" {
			count++
		}
	}
	return count
}

// TotalCount returns the total number of items
func (c *Checklist) TotalCount() int {
	return len(c.items)
}

// Progress returns a visual progress bar string
func (c *Checklist) Progress() string {
	completed := c.CompletedCount()
	total := c.TotalCount()

	barWidth := 20
	filled := (completed * barWidth) / total

	bar := "["
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "="
		} else {
			bar += "-"
		}
	}
	bar += "]"

	return fmt.Sprintf("%s %d/%d steps complete", bar, completed, total)
}

// ToStore converts checklist to Store format
func (c *Checklist) ToStore() *Store {
	completed := make([]string, 0)
	currentStep := ""

	for i, item := range c.items {
		if item.Status == "complete" {
			completed = append(completed, item.Item.ID)
		}
		if i == c.currentIndex && item.Status != "complete" {
			currentStep = item.Item.ID
		}
	}

	return &Store{
		CompletedItems: completed,
		CurrentStep:    currentStep,
		StartedAt:      c.startedAt,
		LastUpdated:    c.lastUpdated,
	}
}

// ValidateSecretGeneration returns a validator for secret generation
func validateSecretGeneration(secret string) func() error {
	return func() error {
		if secret == "" {
			return fmt.Errorf("secret is empty")
		}
		// Check minimum entropy (base64 encoding, 128 bytes -> 172 chars)
		if len(secret) < 32 {
			return fmt.Errorf("secret too short (must be at least 32 characters)")
		}
		return nil
	}
}

// ValidateInterfaceConfig returns a validator for interface configuration
func validateInterfaceConfig(interfaceName string) func() error {
	return func() error {
		// This will be implemented in validation.go
		return validateWGInterface(interfaceName)
	}
}

// ValidateGitHubRegistry returns a validator for GitHub registry
func validateGitHubRegistry(skipRegistry bool) func() error {
	return func() error {
		if skipRegistry {
			return nil // Skip validation
		}
		// This will be implemented in validation.go
		return validateGitHubConnectivity()
	}
}

// ValidateLANMulticast returns a validator for LAN multicast
func validateLANMulticast(disableLAN bool) func() error {
	return func() error {
		if disableLAN {
			return nil // Skip validation
		}
		// This will be implemented in validation.go
		return validateLANMulticastBind()
	}
}

// ValidateDHTBootstrap returns a validator for DHT bootstrap
func validateDHTBootstrap() func() error {
	return func() error {
		// This will be implemented in validation.go
		return validateDHTConnectivity()
	}
}

// ValidateFirstPeerContact returns a validator for first peer discovery
func validateFirstPeerContact(timeout time.Duration) func() error {
	return func() error {
		// This will be implemented in validation.go
		return validatePeerDiscovery(timeout)
	}
}

// ValidateBidirectionalPing returns a validator for handshake validation
func validateBidirectionalPing(interfaceName string) func() error {
	return func() error {
		// This will be implemented in validation.go
		return validateWireGuardHandshake(interfaceName)
	}
}
