package node

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/crypto"
)

// TestMeasureBindingGrinding measures how many WireGuard key pairs a member who
// holds the secret must generate before one derives, with nonce 0 in the
// default subnet, to a chosen victim's address (ADR-0002 Open Question 1). It
// is a measurement for a human, not a gate: it runs only with WGMESH_MEASURE=1.
// Keys are made in process (X25519 on random bytes, base64 as `wg pubkey`
// prints them), so the count is what matters, not the time per pair.
func TestMeasureBindingGrinding(t *testing.T) {
	if os.Getenv("WGMESH_MEASURE") != "1" {
		t.Skip("measurement: set WGMESH_MEASURE=1")
	}
	const (
		trials   = 20
		maxTries = 2_000_000
		secret   = "measure-binding-grinding-secret"
	)
	subnet := [2]byte{42, 7}
	newPub := func() string {
		priv, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		return base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())
	}

	start := time.Now()
	counts := make([]int, 0, trials)
	for i := 0; i < trials; i++ {
		victim := crypto.DeriveMeshIP(subnet, newPub(), secret)
		n := 0
		for ; n < maxTries; n++ {
			if crypto.DeriveMeshIP(subnet, newPub(), secret) == victim {
				break
			}
		}
		if n == maxTries {
			t.Fatalf("trial %d: no match in %d key pairs", i, maxTries)
		}
		counts = append(counts, n+1)
	}
	sort.Ints(counts)
	sum := 0
	for _, c := range counts {
		sum += c
	}
	t.Logf("key pairs until one derives to the victim's address (nonce 0, default subnet), %d trials: mean %d, median %d, min %d, max %d; %s",
		trials, sum/trials, counts[trials/2], counts[0], counts[trials-1], time.Since(start).Round(time.Second))
}
