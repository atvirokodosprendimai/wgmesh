package mesh

import (
	"net"
	"sync"
)

type Node struct {
	Hostname   string `json:"hostname"`
	MeshIP     net.IP `json:"mesh_ip"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key,omitempty"`

	SSHHost string `json:"ssh_host"`
	SSHPort int    `json:"ssh_port"`

	PublicEndpoint string `json:"public_endpoint,omitempty"`
	ListenPort     int    `json:"listen_port"`

	BehindNAT bool `json:"behind_nat"`

	RoutableNetworks []string `json:"routable_networks,omitempty"`

	IsLocal bool `json:"is_local"`

	// Actual hostname from the remote server
	ActualHostname string `json:"actual_hostname,omitempty"`
	// FQDN from the remote server
	FQDN string `json:"fqdn,omitempty"`
}

type Group struct {
	Description string   `json:"description,omitempty"`
	Members     []string `json:"members"` // hostnames
}

type AccessPolicy struct {
	Name                  string   `json:"name"`
	Description           string   `json:"description,omitempty"`
	FromGroups            []string `json:"from_groups"`
	ToGroups              []string `json:"to_groups"`
	AllowMeshIPs          bool     `json:"allow_mesh_ips"`
	AllowRoutableNetworks bool     `json:"allow_routable_networks"`
}

type PeerAccess struct {
	AllowMeshIP           bool
	AllowRoutableNetworks bool
}

type Mesh struct {
	InterfaceName  string            `json:"interface_name"`
	Network        string            `json:"network"`
	ListenPort     int               `json:"listen_port"`
	Nodes          map[string]*Node  `json:"nodes"`
	LocalHostname  string            `json:"local_hostname"`
	Groups         map[string]*Group `json:"groups,omitempty"`
	AccessPolicies []*AccessPolicy   `json:"access_policies,omitempty"`
	mu             sync.RWMutex      `json:"-"`
}
