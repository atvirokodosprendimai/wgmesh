package daemon

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/atvirokodosprendimai/wgmesh/pkg/routes"
)

// TestCalculateRouteDiff verifies add/remove logic without any exec calls.
func TestCalculateRouteDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		current       []routes.Entry
		desired       []routes.Entry
		wantAddLen    int
		wantRemoveLen int
	}{
		{
			name:          "empty both",
			current:       nil,
			desired:       nil,
			wantAddLen:    0,
			wantRemoveLen: 0,
		},
		{
			name:    "add new route",
			current: nil,
			desired: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			wantAddLen:    1,
			wantRemoveLen: 0,
		},
		{
			name: "remove stale route",
			current: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			desired:       nil,
			wantAddLen:    0,
			wantRemoveLen: 1,
		},
		{
			name: "no change when identical",
			current: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			desired: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			wantAddLen:    0,
			wantRemoveLen: 0,
		},
		{
			name: "gateway change triggers remove and add",
			current: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.1"},
			},
			desired: []routes.Entry{
				{Network: "192.168.1.0/24", Gateway: "10.0.0.2"},
			},
			wantAddLen:    1,
			wantRemoveLen: 1,
		},
		{
			name: "mix of add remove and keep",
			current: []routes.Entry{
				{Network: "10.0.0.0/8", Gateway: "172.16.0.1"},
				{Network: "stale.net/24", Gateway: "172.16.0.1"},
			},
			desired: []routes.Entry{
				{Network: "10.0.0.0/8", Gateway: "172.16.0.1"},
				{Network: "192.168.0.0/16", Gateway: "172.16.0.1"},
			},
			wantAddLen:    1,
			wantRemoveLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			add, remove := routes.CalculateDiff(tt.current, tt.desired)
			if len(add) != tt.wantAddLen {
				t.Errorf("toAdd: got %d, want %d", len(add), tt.wantAddLen)
			}
			if len(remove) != tt.wantRemoveLen {
				t.Errorf("toRemove: got %d, want %d", len(remove), tt.wantRemoveLen)
			}
		})
	}
}

// TestNormalizeNetwork verifies host-route normalization.
func TestNormalizeNetwork(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"10.0.0.1", "10.0.0.1/32"},
		{"10.0.0.1/32", "10.0.0.1/32"},
		{"192.168.0.0/24", "192.168.0.0/24"},
		{"fd00::1", "fd00::1/128"},
		{"fd00::1/128", "fd00::1/128"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := routes.NormalizeNetwork(tt.input)
			if got != tt.want {
				t.Errorf("normalizeNetwork(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestGetCurrentRoutes uses a mock executor so no real "ip" binary is required.
func TestGetCurrentRoutes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("getCurrentRoutes is Linux-only")
	}

	ipOutput := strings.Join([]string{
		"192.168.1.0/24 via 10.0.0.1 dev wg0 proto static",
		"10.0.0.0/8 via 10.0.0.2 dev wg0 proto static",
		"",
	}, "\n")

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			if name == "ip" {
				return &MockCommand{outputFunc: func() ([]byte, error) {
					return []byte(ipOutput), nil
				}}
			}
			return &MockCommand{}
		},
	}

	withMockExecutor(t, mock, func() {
		routes, err := getCurrentRoutes("wg0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(routes) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(routes))
		}
		if routes[0].Network != "192.168.1.0/24" {
			t.Errorf("route[0].Network = %q, want 192.168.1.0/24", routes[0].Network)
		}
		if routes[0].Gateway != "10.0.0.1" {
			t.Errorf("route[0].Gateway = %q, want 10.0.0.1", routes[0].Gateway)
		}
	})
}

// TestGetCurrentRoutesSkipsNoGateway verifies that routes without "via" are ignored.
func TestGetCurrentRoutesSkipsNoGateway(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("getCurrentRoutes is Linux-only")
	}

	// "local" route lines have no "via" — must be skipped
	ipOutput := "10.0.0.0/8 dev wg0 proto kernel scope link src 10.0.0.1\n"

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			return &MockCommand{outputFunc: func() ([]byte, error) {
				return []byte(ipOutput), nil
			}}
		},
	}

	withMockExecutor(t, mock, func() {
		routes, err := getCurrentRoutes("wg0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(routes) != 0 {
			t.Errorf("expected 0 routes (no via), got %d", len(routes))
		}
	})
}

// TestApplyRouteDiff_AddAndRemove verifies the commands issued during a diff apply.
func TestApplyRouteDiff_AddAndRemove(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("applyRouteDiff is Linux-only")
	}

	var cmds []string
	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			cmds = append(cmds, name+" "+strings.Join(args, " "))
			return &MockCommand{}
		},
	}

	toAdd := []routes.Entry{{Network: "10.0.1.0/24", Gateway: "10.0.0.1"}}
	toRemove := []routes.Entry{{Network: "10.0.2.0/24", Gateway: "10.0.0.1"}}

	withMockExecutor(t, mock, func() {
		if err := applyRouteDiff("wg0", toAdd, toRemove); err != nil {
			t.Fatalf("applyRouteDiff failed: %v", err)
		}
	})

	found := func(substr string) bool {
		for _, c := range cmds {
			if strings.Contains(c, substr) {
				return true
			}
		}
		return false
	}

	if !found("route del 10.0.2.0/24") {
		t.Errorf("expected 'route del' command, got: %v", cmds)
	}
	if !found("route replace 10.0.1.0/24") {
		t.Errorf("expected 'route replace' command, got: %v", cmds)
	}
	if !found("sysctl") {
		t.Errorf("expected sysctl ip_forward command, got: %v", cmds)
	}
}

// TestApplyRouteDiff_AddFailure verifies error propagation when "ip route replace" fails.
func TestApplyRouteDiff_AddFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("applyRouteDiff is Linux-only")
	}

	mock := &MockCommandExecutor{
		commandFunc: func(name string, args ...string) Command {
			if name == "ip" && len(args) > 0 && args[0] == "route" && args[1] == "replace" {
				return &MockCommand{
					combinedOutputFunc: func() ([]byte, error) {
						return []byte("RTNETLINK answers: Operation not permitted"), fmt.Errorf("exit status 2")
					},
				}
			}
			return &MockCommand{}
		},
	}

	toAdd := []routes.Entry{{Network: "10.0.1.0/24", Gateway: "10.0.0.1"}}

	withMockExecutor(t, mock, func() {
		err := applyRouteDiff("wg0", toAdd, nil)
		if err == nil {
			t.Fatal("expected error from failed route replace, got nil")
		}
		if !strings.Contains(err.Error(), "failed to add route") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
