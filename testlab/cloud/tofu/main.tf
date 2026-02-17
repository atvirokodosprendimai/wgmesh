provider "hcloud" {
  token = var.hcloud_token
}

locals {
  # First VM is introducer, rest are nodes
  node_names = concat(
    ["introducer"],
    [for i in range(1, var.vm_count) : "node-${["a", "b", "c", "d", "e", "f", "g", "h"][i - 1]}"]
  )

  nodes = {
    for i, name in local.node_names : name => {
      role     = i == 0 ? "introducer" : "node"
      location = var.locations[i % length(var.locations)]
    }
  }
}

resource "hcloud_ssh_key" "ci" {
  name       = "${var.prefix}-${var.run_id}-key"
  public_key = file(var.ssh_public_key_path)
}

resource "hcloud_server" "nodes" {
  for_each    = local.nodes
  name        = "${var.prefix}-${var.run_id}-${each.key}"
  server_type = var.server_type
  image       = var.image
  location    = each.value.location
  ssh_keys    = [hcloud_ssh_key.ci.id]

  labels = {
    role    = each.value.role
    run     = var.run_id
    created = formatdate("YYYYMMDD-hhmmss", timestamp())
    managed = "opentofu"
  }

  # Prevent accidental recreation on label changes
  lifecycle {
    ignore_changes = [labels["created"]]
  }
}
