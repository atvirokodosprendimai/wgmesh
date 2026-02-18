output "node_ips" {
  description = "Map of node short name to public IPv4"
  value       = { for k, v in hcloud_server.nodes : k => v.ipv4_address }
}

output "node_roles" {
  description = "Map of node short name to role (introducer/node)"
  value       = { for k, v in local.nodes : k => v.role }
}

output "node_locations" {
  description = "Map of node short name to Hetzner datacenter"
  value       = { for k, v in hcloud_server.nodes : k => v.datacenter }
}

output "ssh_key_name" {
  value = hcloud_ssh_key.ci.name
}

output "server_type" {
  description = "Server type used for provisioned VMs"
  value       = var.server_type
}

output "locations" {
  description = "Locations used for VM placement"
  value       = var.locations
}
