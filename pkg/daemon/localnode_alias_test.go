package daemon_test

import (
	"reflect"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/daemon"
	"github.com/atvirokodosprendimai/wgmesh/pkg/node"
)

// TestLocalNodeIsOneType: a method added to node.LocalNode must reach the
// value initLocalNode, discovery and the tests use,
// so daemon.LocalNode and node.LocalNode must be one type, not two.
func TestLocalNodeIsOneType(t *testing.T) {
	if reflect.TypeOf(daemon.LocalNode{}) != reflect.TypeOf(node.LocalNode{}) {
		t.Fatalf("daemon.LocalNode (%v) and node.LocalNode (%v) are different types", reflect.TypeOf(daemon.LocalNode{}), reflect.TypeOf(node.LocalNode{}))
	}
}
