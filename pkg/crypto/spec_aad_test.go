package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two specs and the operator note must keep naming the rules ADR-0001
// introduced; this only checks that the terms exist, so a spec rewrite cannot
// silently drop them.
func TestSpecNamesAADRollout(t *testing.T) {
	root := filepath.Join("..", "..")
	cases := []struct {
		file  string
		terms []string
	}{
		{
			filepath.Join("eidos", "spec - crypto - secret-derived keys envelope encryption membership proofs and rotation.md"),
			[]string{"SealEnvelopeAAD", "LegacyAAD", "EnvelopeCapabilityVersion"},
		},
		{
			filepath.Join("eidos", "spec - discovery peer exchange - udp advertisement rendezvous and hole-punching.md"),
			[]string{"nonce cache", "LegacyAAD", "SealEnvelopeAAD"},
		},
		{
			filepath.Join("docs", "envelope-rollout.md"),
			[]string{"N+1", "N+2", "LegacyEnvelopeOpens", "ReplayedDatagrams", "ADR-0001"},
		},
	}
	for _, c := range cases {
		data, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		for _, term := range c.terms {
			if !strings.Contains(string(data), term) {
				t.Errorf("%s does not mention %q", c.file, term)
			}
		}
	}
}
