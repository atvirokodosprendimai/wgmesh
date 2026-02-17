variable "hcloud_token" {
  type      = string
  sensitive = true
}

variable "run_id" {
  type        = string
  description = "Unique identifier for this CI run (e.g., GitHub run ID)"
}

variable "prefix" {
  type    = string
  default = "wgmesh-ci"
}

variable "vm_count" {
  type        = number
  default     = 5
  description = "Total VMs: 1 introducer + (N-1) nodes"
}

variable "server_type" {
  type    = string
  default = "cax11" # ARM64 Ampere, 2 vCPU, 4GB
}

variable "image" {
  type    = string
  default = "ubuntu-24.04"
}

variable "ssh_public_key_path" {
  type = string
}

variable "locations" {
  type        = list(string)
  default     = ["hel1", "nbg1", "fsn1"]
  description = "Hetzner datacenters to distribute VMs across"
}
