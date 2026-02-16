# Specification: Issue #70

## Classification
fix

## Deliverables
code + documentation

## Problem Analysis

When running wgmesh in Docker containers (using docker-compose.yml or other Docker deployment methods), the application stores WireGuard key pairs and persistent state in `/var/lib/wgmesh/` directory. However, the current Docker configuration only mounts `/data` as a volume.

**The Problem:**
- In **native/systemd mode**, wgmesh stores state files in `/var/lib/wgmesh/`:
  - `{interface}.json` - WireGuard keypair (public + private keys)
  - `{interface}-peers.json` - Cached peer data (24-hour expiration)
  - `{interface}-dht.nodes` - DHT bootstrap nodes cache
  
- In **Docker/container mode**, the Dockerfile sets `WORKDIR /data` and the docker-compose.yml mounts `./data/nodeX:/data`

- **However**, the daemon code hardcodes state paths to `/var/lib/wgmesh/`:
  ```go
  // pkg/daemon/daemon.go:129
  stateFile := fmt.Sprintf("/var/lib/wgmesh/%s.json", d.config.InterfaceName)
  
  // pkg/daemon/cache.go:
  return filepath.Join("/var/lib/wgmesh", fmt.Sprintf("%s-peers.json", interfaceName))
  
  // pkg/discovery/dht.go:
  return filepath.Join("/var/lib/wgmesh", fmt.Sprintf("%s-dht.nodes", d.config.InterfaceName))
  ```

**Impact:**
When deploying a new version of the container (pulling a new image), the container filesystem is reset, and since `/var/lib/wgmesh/` is **not** mounted as a volume, all state files including the WireGuard private/public key pair are lost. This causes:

1. **New identity on each restart**: Node gets a new WireGuard keypair and mesh IP
2. **Lost peer connections**: Other nodes still have the old public key in their peer cache
3. **Lost DHT state**: DHT bootstrap nodes need to be rediscovered from scratch
4. **Network disruption**: Existing connections break until all nodes rediscover the new identity

## Proposed Approach

Add `/var/lib/wgmesh` as a Docker volume mount to persist WireGuard keys and state across container restarts and upgrades.

### Solution Options

**Option 1: Mount /var/lib/wgmesh as volume (Recommended)**
- Keep current code that uses `/var/lib/wgmesh/`
- Add volume mount in docker-compose.yml: `./data/node1:/var/lib/wgmesh`
- This aligns with systemd deployment where `/var/lib/wgmesh` is the standard location

**Option 2: Environment variable override (More flexible but more complex)**
- Add `--state-dir` flag or `WGMESH_STATE_DIR` environment variable
- Default to `/var/lib/wgmesh` for systemd compatibility
- Allow override to `/data` in Docker

**Decision: Use Option 1** for simplicity and consistency:
- Minimal code changes (documentation only)
- Consistent state directory location across deployment methods
- Follows standard `/var/lib/{service}` convention from systemd
- Users can still use `/data` for other purposes if needed

### Implementation Steps

1. **Update docker-compose.yml**:
   - Change volume mount from `./data/node1:/data` to `./data/node1:/var/lib/wgmesh`
   - Update for all node examples (node1, node2, node3, advanced)
   - Keep `/data` directory in Dockerfile for backward compatibility and other uses

2. **Update DOCKER-COMPOSE.md documentation**:
   - Explain the purpose of `/var/lib/wgmesh` volume
   - Document what files are persisted there
   - Update "Persistent State" section to reference `/var/lib/wgmesh` instead of `/data`
   - Update volume examples to use `./data/gateway:/var/lib/wgmesh`

3. **Update DOCKER.md documentation**:
   - Update docker run examples to mount `/var/lib/wgmesh` for decentralized mode
   - Clarify that `/data` is for centralized mode state files
   - Add example: `docker run -v $(pwd)/wgmesh-state:/var/lib/wgmesh ...`

4. **Update README.md documentation**:
   - Update Docker examples to use correct volume mount
   - Clarify the difference between centralized mode (uses `-state` flag, any path) and decentralized mode (hardcoded `/var/lib/wgmesh`)

5. **Verify .env.example and other examples**:
   - Check if any other configuration files need updates

## Affected Files

### Code Files (Volume Configuration)
1. **docker-compose.yml** - Change volume mounts for all nodes

### Documentation Files
1. **DOCKER-COMPOSE.md** - Update volume mount documentation and examples
2. **DOCKER.md** - Update docker run examples
3. **README.md** - Update Docker usage section if it contains volume mount examples

### Files NOT Changed
- **Dockerfile** - Keep `/data` directory for backward compatibility and other uses
- **pkg/daemon/daemon.go** - No changes needed (already uses `/var/lib/wgmesh`)
- **pkg/daemon/cache.go** - No changes needed
- **pkg/discovery/dht.go** - No changes needed

## Test Strategy

### Manual Testing

1. **Test with new volume mount**:
   ```bash
   # Start node with new volume configuration
   docker-compose up -d wgmesh-node
   
   # Verify state files are created in host directory
   ls -la ./data/node1/
   # Should show: wg0.json, wg0-peers.json, wg0-dht.nodes
   
   # Check container can read/write files
   docker-compose exec wgmesh-node ls -la /var/lib/wgmesh/
   ```

2. **Test persistence across restarts**:
   ```bash
   # Record the public key
   docker-compose exec wgmesh-node wg show wg0 public-key > key1.txt
   
   # Stop and restart container
   docker-compose down
   docker-compose up -d wgmesh-node
   
   # Verify same public key (keys persisted)
   docker-compose exec wgmesh-node wg show wg0 public-key > key2.txt
   diff key1.txt key2.txt  # Should be identical
   ```

3. **Test upgrade scenario (simulated)**:
   ```bash
   # Start with current config
   docker-compose up -d wgmesh-node
   
   # Save keypair
   docker-compose exec wgmesh-node wg show wg0 public-key > before.txt
   
   # Pull new image (simulating upgrade)
   docker-compose pull wgmesh-node
   docker-compose up -d wgmesh-node
   
   # Verify keys still exist
   docker-compose exec wgmesh-node wg show wg0 public-key > after.txt
   diff before.txt after.txt  # Should be identical
   ```

4. **Test multi-node mesh**:
   ```bash
   # Start multiple nodes
   docker-compose up -d wgmesh-node wgmesh-node2
   
   # Verify each has its own persistent state
   ls -la ./data/node1/  # wg0.json
   ls -la ./data/node2/  # wg1.json
   ```

### Verification Checklist

- [ ] Volume mount creates host directory if it doesn't exist
- [ ] State files (*.json, *.nodes) appear in host directory
- [ ] WireGuard keys persist across container restarts
- [ ] WireGuard keys persist across image upgrades
- [ ] Multiple nodes can run with separate state directories
- [ ] File permissions are correct (0600 for key files, 0700 for directory)
- [ ] Documentation accurately describes volume mount purpose
- [ ] Examples in documentation are consistent

### Edge Cases to Test

1. **First-time startup**: Empty volume directory, keys should be generated
2. **Existing keys**: Container should load and use existing keys from volume
3. **Multiple interfaces**: Different interface names should create separate state files
4. **Permission issues**: Verify container user can write to mounted directory

## Estimated Complexity

**low** (30-45 minutes)

- Simple configuration change in docker-compose.yml
- No application code changes required
- Documentation updates are straightforward
- Testing is manual but quick to verify
- Low risk: Only affects Docker deployments, not core functionality
