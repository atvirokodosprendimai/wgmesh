package mesh

import (
	"net"
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

type Mesh struct {
	InterfaceName string           `json:"interface_name"`
	Network       string           `json:"network"`
	ListenPort    int              `json:"listen_port"`
	Nodes         map[string]*Node `json:"nodes"`
	LocalHostname string           `json:"local_hostname"`
}
