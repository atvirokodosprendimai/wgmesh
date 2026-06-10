package pilot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/atvirokodosprendimai/wgmesh/pkg/rpc"
)

// dialRPC creates an RPC client connected to the daemon socket.
func dialRPC() (*rpc.Client, error) {
	socketPath := os.Getenv("WGMESH_SOCKET")
	if socketPath == "" {
		socketPath = getSocketPath()
	}

	client, err := rpc.NewClient(socketPath)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	return client, nil
}

// getSocketPath determines the RPC socket path, matching the logic in
// pkg/rpc.GetSocketPath without importing the function (which would be
// fine but we keep the pilot package loosely coupled).
func getSocketPath() string {
	if path := os.Getenv("WGMESH_SOCKET"); path != "" {
		return path
	}
	if rpc.IsWritable("/var/run") {
		return "/var/run/wgmesh.sock"
	}
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "wgmesh.sock")
	}
	return "/tmp/wgmesh.sock"
}
