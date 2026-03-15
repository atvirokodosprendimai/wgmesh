package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
	lighthouse "github.com/atvirokodosprendimai/lighthouse-go"
	"github.com/atvirokodosprendimai/wgmesh/pkg/mesh"
)

const (
	defaultStateDir  = "/var/lib/wgmesh"
	servicesFileName = "services.json"
	accountFileName  = "account.json"
	managedDomain    = "wgmesh.dev"
)

var validServiceName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

func serviceCmd() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh service <add|list|remove>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  add <name> <local-addr>    Register a service for managed ingress")
		fmt.Fprintln(os.Stderr, "  list                       List registered services")
		fmt.Fprintln(os.Stderr, "  remove <name>              Deregister a service")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "add":
		serviceAddCmd()
	case "list":
		serviceListCmd()
	case "remove":
		serviceRemoveCmd()
	default:
		fmt.Fprintf(os.Stderr, "Unknown service command: %s\n", os.Args[2])
		fmt.Fprintln(os.Stderr, "Available commands: add, list, remove")
		os.Exit(1)
	}
}

func serviceAddCmd() {
	fs := flag.NewFlagSet("service add", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (or set WGMESH_SECRET)")
	meshSubnet := fs.String("mesh-subnet", "", "Custom mesh subnet CIDR (e.g. 192.168.100.0/24)")
	protocol := fs.String("protocol", "http", "Origin protocol: http or https")
	healthPath := fs.String("health-path", "/", "Health check path")
	healthInterval := fs.Duration("health-interval", 30*time.Second, "Health check interval")
	account := fs.String("account", "", "Lighthouse API key (cr_...) — saved for future use")
	stateDir := fs.String("state-dir", defaultStateDir, "State directory")
	fs.Parse(os.Args[3:])

	args := fs.Args()
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh service add <name> <local-addr> [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example: wgmesh service add ollama :11434 --secret <SECRET>")
		os.Exit(1)
	}

	resolvedSecret := resolveSecret(*secret)
	if resolvedSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required (or set WGMESH_SECRET)")
		os.Exit(1)
	}

	name := args[0]
	localAddr := args[1]

	// Validate service name
	if !validServiceName.MatchString(name) {
		fmt.Fprintf(os.Stderr, "Error: invalid service name %q\n", name)
		fmt.Fprintln(os.Stderr, "Names must be lowercase alphanumeric with hyphens (e.g. ollama, my-api)")
		os.Exit(1)
	}

	// Validate protocol
	if *protocol != "http" && *protocol != "https" {
		fmt.Fprintf(os.Stderr, "Error: invalid protocol %q (must be http or https)\n", *protocol)
		os.Exit(1)
	}

	// Parse local address
	port, err := parseLocalAddr(localAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid local address %q: %v\n", localAddr, err)
		fmt.Fprintln(os.Stderr, "Expected format: :PORT or HOST:PORT (e.g. :11434, 127.0.0.1:8080)")
		os.Exit(1)
	}

	// Derive mesh parameters from secret
	keys, err := crypto.DeriveKeys(resolvedSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to derive keys: %v\n", err)
		os.Exit(1)
	}
	meshID := keys.MeshID()

	// Resolve account
	accountPath := filepath.Join(*stateDir, accountFileName)
	acct, err := resolveAccount(accountPath, *account)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Provide an API key with: wgmesh service add <name> <addr> --account <cr_...> --secret <SECRET>")
		os.Exit(1)
	}

	// Discover lighthouse URL
	lighthouseURL := acct.LighthouseURL
	if lighthouseURL == "" {
		lighthouseURL, err = lighthouse.DiscoverLighthouse(meshID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to discover Lighthouse: %v\n", err)
			os.Exit(1)
		}
	}

	// Build domain name
	domain := fmt.Sprintf("%s.%s.%s", name, meshID, managedDomain)

	// We need the node's mesh IP — derive it from WireGuard pubkey + secret.
	// For service registration, we use a placeholder derivation since we don't
	// have the WG pubkey without a running daemon. We'll use the subnet info
	// to indicate which mesh this belongs to. The Lighthouse can resolve the
	// actual mesh IP from the origin field.
	// For now, use "auto" to let Lighthouse figure it out, or derive from local WG interface.
	customSubnet, err := crypto.ParseSubnetOrDefault(*meshSubnet)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	meshIP := deriveMeshIPForService(keys, resolvedSecret, customSubnet)

	// Create site via Lighthouse
	client := lighthouse.NewClient(lighthouseURL, acct.APIKey)
	site, err := client.CreateSite(lighthouse.CreateSiteRequest{
		Domain: domain,
		Origin: lighthouse.Origin{
			MeshIP:   meshIP,
			Port:     port,
			Protocol: *protocol,
			HealthCheck: lighthouse.HealthCheck{
				Path:     *healthPath,
				Interval: *healthInterval,
			},
		},
		TLS: "auto",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to register service: %v\n", err)
		os.Exit(1)
	}

	// Save to local state
	servicesPath := filepath.Join(*stateDir, servicesFileName)
	state, loadErr := mesh.LoadServices(servicesPath)
	if loadErr != nil && !os.IsNotExist(loadErr) {
		fmt.Fprintf(os.Stderr, "Warning: failed to load existing services: %v\n", loadErr)
		fmt.Fprintln(os.Stderr, "Creating fresh state — other registered services may be lost.")
	}
	state.Services[name] = mesh.ServiceEntry{
		SiteID:       site.ID,
		Name:         name,
		Domain:       domain,
		LocalAddr:    localAddr,
		Protocol:     *protocol,
		RegisteredAt: time.Now().UTC(),
	}
	if err := mesh.SaveServices(servicesPath, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: service registered but failed to save local state: %v\n", err)
	}

	fmt.Printf("Service registered: %s\n", name)
	fmt.Printf("  URL:    https://%s\n", domain)
	fmt.Printf("  Origin: %s (port %d, %s)\n", meshIP, port, *protocol)
	fmt.Printf("  Status: %s\n", site.Status)
}

func serviceListCmd() {
	fs := flag.NewFlagSet("service list", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (or set WGMESH_SECRET)")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	stateDir := fs.String("state-dir", defaultStateDir, "State directory")
	fs.Parse(os.Args[3:])

	resolvedSecret := resolveSecret(*secret)
	if resolvedSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required (or set WGMESH_SECRET)")
		os.Exit(1)
	}

	keys, err := crypto.DeriveKeys(resolvedSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to derive keys: %v\n", err)
		os.Exit(1)
	}
	meshID := keys.MeshID()

	// Try Lighthouse first
	accountPath := filepath.Join(*stateDir, accountFileName)
	acct, acctErr := mesh.LoadAccount(accountPath)

	var sites []lighthouse.Site
	var fromLighthouse bool

	if acctErr == nil {
		lighthouseURL := acct.LighthouseURL
		if lighthouseURL == "" {
			lighthouseURL, _ = lighthouse.DiscoverLighthouse(meshID)
		}
		if lighthouseURL != "" {
			client := lighthouse.NewClient(lighthouseURL, acct.APIKey)
			sites, err = client.ListSites()
			if err == nil {
				fromLighthouse = true
			}
		}
	}

	// Fallback to local state
	if !fromLighthouse {
		servicesPath := filepath.Join(*stateDir, servicesFileName)
		state, loadErr := mesh.LoadServices(servicesPath)
		if loadErr != nil {
			// LoadServices returns nil for not-exist, so any error here is
			// a real problem (corruption, permission denied, etc.).
			fmt.Fprintf(os.Stderr, "Error: failed to load local services: %v\n", loadErr)
			os.Exit(1)
		}
		for _, entry := range state.Services {
			sites = append(sites, lighthouse.Site{
				ID:     entry.SiteID,
				Domain: entry.Domain,
				Origin: lighthouse.Origin{Port: parsePortFromAddr(entry.LocalAddr)},
				Status: "local",
			})
		}
	}

	if len(sites) == 0 {
		fmt.Println("No services registered")
		return
	}

	if *jsonOutput {
		data, _ := json.MarshalIndent(sites, "", "  ")
		fmt.Println(string(data))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tURL\tPORT\tSTATUS")
	for _, site := range sites {
		name := extractServiceName(site.Domain, meshID)
		fmt.Fprintf(w, "%s\thttps://%s\t%d\t%s\n", name, site.Domain, site.Origin.Port, site.Status)
	}
	w.Flush()
}

func serviceRemoveCmd() {
	fs := flag.NewFlagSet("service remove", flag.ExitOnError)
	secret := fs.String("secret", "", "Mesh secret (or set WGMESH_SECRET)")
	stateDir := fs.String("state-dir", defaultStateDir, "State directory")
	fs.Parse(os.Args[3:])

	resolvedSecret := resolveSecret(*secret)
	if resolvedSecret == "" {
		fmt.Fprintln(os.Stderr, "Error: --secret is required (or set WGMESH_SECRET)")
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh service remove <name> [flags]")
		os.Exit(1)
	}
	name := args[0]

	keys, err := crypto.DeriveKeys(resolvedSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to derive keys: %v\n", err)
		os.Exit(1)
	}
	meshID := keys.MeshID()

	// Look up site ID from local state
	servicesPath := filepath.Join(*stateDir, servicesFileName)
	state, loadErr := mesh.LoadServices(servicesPath)
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load services: %v\n", loadErr)
		os.Exit(1)
	}

	entry, ok := state.Services[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: service %q not found in local state\n", name)
		os.Exit(1)
	}

	// Delete from Lighthouse
	accountPath := filepath.Join(*stateDir, accountFileName)
	acct, acctErr := mesh.LoadAccount(accountPath)

	if acctErr == nil {
		lighthouseURL := acct.LighthouseURL
		if lighthouseURL == "" {
			lighthouseURL, _ = lighthouse.DiscoverLighthouse(meshID)
		}
		if lighthouseURL != "" {
			client := lighthouse.NewClient(lighthouseURL, acct.APIKey)
			if err := client.DeleteSite(entry.SiteID); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete from Lighthouse: %v\n", err)
				fmt.Fprintln(os.Stderr, "Removing from local state only.")
			}
		}
	}

	// Remove from local state
	delete(state.Services, name)
	if err := mesh.SaveServices(servicesPath, state); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to update local state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Service removed: %s\n", name)
}

// resolveSecret returns the secret from the flag or WGMESH_SECRET env var.
// The value is normalized to strip any wgmesh:// URI wrapper.
func resolveSecret(flagValue string) string {
	if flagValue != "" {
		return normalizeSecret(flagValue)
	}
	return normalizeSecret(os.Getenv("WGMESH_SECRET"))
}

// normalizeSecret strips the wgmesh:// URI prefix, optional version segment,
// and query parameters so callers can pass secrets in either raw or URI form.
// Example: "wgmesh://v1/K7x2...?relay=true" → "K7x2..."
func normalizeSecret(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	const uriPrefix = "wgmesh://"
	if !strings.HasPrefix(input, uriPrefix) {
		return input
	}

	input = strings.TrimPrefix(input, uriPrefix)

	// Strip optional version segment (e.g. "v1/")
	parts := strings.SplitN(input, "/", 2)
	if len(parts) == 2 {
		input = parts[1]
	} else {
		input = parts[0]
	}

	// Strip query parameters
	if idx := strings.Index(input, "?"); idx != -1 {
		input = input[:idx]
	}

	return input
}

// resolveAccount loads or creates account configuration.
func resolveAccount(path, apiKeyFlag string) (mesh.AccountConfig, error) {
	// If flag provided, save and use it (preserve existing LighthouseURL)
	if apiKeyFlag != "" {
		acct, _ := mesh.LoadAccount(path) // ignore error — may not exist yet
		acct.APIKey = apiKeyFlag
		if err := mesh.SaveAccount(path, acct); err != nil {
			return acct, fmt.Errorf("failed to save account: %w", err)
		}
		return acct, nil
	}

	// Try loading from disk
	acct, err := mesh.LoadAccount(path)
	if err != nil {
		return acct, fmt.Errorf("no account configured. Run: wgmesh service add <name> <addr> --account <cr_...> --secret <SECRET>")
	}
	return acct, nil
}

// parseLocalAddr extracts the port number from a [host]:port address.
// Supports formats: :PORT, HOST:PORT, [IPv6]:PORT
func parseLocalAddr(addr string) (int, error) {
	var portStr string

	if strings.HasPrefix(addr, "[") {
		// IPv6: [::1]:8080
		_, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return 0, fmt.Errorf("invalid address: %w", err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return 0, fmt.Errorf("invalid port: %w", err)
		}
		return validatePort(port)
	}

	if strings.HasPrefix(addr, ":") {
		// Bare port: :11434
		portStr = addr[1:]
	} else if strings.Count(addr, ":") == 1 {
		// host:port: 127.0.0.1:8080
		portStr = addr[strings.LastIndex(addr, ":")+1:]
	} else {
		// Try as bare number
		portStr = addr
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	return validatePort(port)
}

func validatePort(port int) (int, error) {
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of range (1-65535)", port)
	}
	return port, nil
}

// deriveMeshIPForService derives the mesh IP for service registration.
// Uses the local WireGuard public key if available, otherwise generates
// a deterministic key from the secret for registration purposes.
// If customSubnet is non-nil, uses subnet-aware derivation.
func deriveMeshIPForService(keys *crypto.DerivedKeys, secret string, customSubnet *net.IPNet) string {
	// Try to read the local node's WG pubkey from the persisted state
	// This matches how the daemon derives its mesh IP
	iface := "wg0"
	statePath := filepath.Join(defaultStateDir, iface+".json")
	data, err := os.ReadFile(statePath)
	if err == nil {
		var nodeState struct {
			PublicKey string `json:"public_key"`
		}
		if json.Unmarshal(data, &nodeState) == nil && nodeState.PublicKey != "" {
			if customSubnet != nil {
				ip, err := crypto.DeriveMeshIPInSubnet(customSubnet, nodeState.PublicKey, secret)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: failed to derive mesh IP in custom subnet: %v\n", err)
					return crypto.DeriveMeshIP(keys.MeshSubnet, nodeState.PublicKey, secret)
				}
				return ip
			}
			return crypto.DeriveMeshIP(keys.MeshSubnet, nodeState.PublicKey, secret)
		}
	}

	// Fallback: derive a registration mesh IP from the secret alone.
	// This won't match the daemon's mesh IP (which needs the WG pubkey),
	// but it gives the Lighthouse enough info to associate the origin.
	// The user should run `join` first to establish the WG identity.
	fmt.Fprintln(os.Stderr, "Warning: no local WireGuard identity found. Run 'wgmesh join' first for accurate mesh IP.")
	fmt.Fprintln(os.Stderr, "Using derived placeholder — service may need re-registration after join.")
	if customSubnet != nil {
		ip, err := crypto.DeriveMeshIPInSubnet(customSubnet, "unjoined", secret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to derive placeholder IP in custom subnet: %v\n", err)
			return crypto.DeriveMeshIP(keys.MeshSubnet, "unjoined", secret)
		}
		return ip
	}
	return crypto.DeriveMeshIP(keys.MeshSubnet, "unjoined", secret)
}

// extractServiceName extracts the service name from a domain like "ollama.abc123.wgmesh.dev".
func extractServiceName(domain, meshID string) string {
	suffix := "." + meshID + "." + managedDomain
	if strings.HasSuffix(domain, suffix) {
		return strings.TrimSuffix(domain, suffix)
	}
	return domain
}

// parsePortFromAddr extracts port from ":11434" or "host:11434" format.
func parsePortFromAddr(addr string) int {
	port, _ := parseLocalAddr(addr)
	return port
}
