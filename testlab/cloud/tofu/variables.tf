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
  default = "cpx22"
}

# Alternative server types to try if primary fails
variable "fallback_server_types" {
  type        = list(string)
  default     = ["cpx32", "cax11", "cax21", "cx23", "cx33", "ccx13", "ccx23"]
  description = "Fallback server types if primary is unavailable"
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
  default     = ["nbg1", "fsn1", "hel1"]
  description = "Hetzner datacenters to distribute VMs across (cax11 ARM64 only available in EU)"
}
