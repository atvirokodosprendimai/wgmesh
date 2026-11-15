package ssh

import (
	"strings"
	"testing"
)

func TestDetectPublicIPCommandBoundsCurl(t *testing.T) {
	t.Parallel()

	curls := 0
	for _, call := range strings.Split(detectPublicIPCommand, "||") {
		fields := strings.Fields(call)
		if len(fields) == 0 || fields[0] != "curl" {
			continue
		}
		curls++
		bound := ""
		for i, f := range fields {
			if f == "--max-time" && i+1 < len(fields) {
				bound = fields[i+1]
			}
		}
		if bound != "5" {
			t.Errorf("curl call %q: --max-time = %q, want 5", strings.TrimSpace(call), bound)
		}
	}
	if curls == 0 {
		t.Errorf("no curl call found in %q", detectPublicIPCommand)
	}
}
