package ssh

import "testing"

func TestDialAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		port int
		want string
	}{
		{"192.0.2.10", 22, "192.0.2.10:22"},
		{"node1.example.com", 2222, "node1.example.com:2222"},
		{"::1", 22, "[::1]:22"},
		{"2001:db8::5", 2222, "[2001:db8::5]:2222"},
	}
	for _, tt := range tests {
		if got := dialAddr(tt.host, tt.port); got != tt.want {
			t.Errorf("dialAddr(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
		}
	}
}
