package ssh

import (
	"strings"
	"testing"
)

func TestDetectPublicIPCommandBoundsCurl(t *testing.T) {
	t.Parallel()

	for _, call := range strings.Split(detectPublicIPCommand, "||") {
		call = strings.TrimSpace(call)
		if !strings.HasPrefix(call, "curl ") {
			continue
		}
		if !strings.Contains(call, "--max-time 5") {
			t.Errorf("curl call has no --max-time bound: %q", call)
		}
	}
}
