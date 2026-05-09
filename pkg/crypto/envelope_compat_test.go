package crypto

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type envelopeFixtureExpected struct {
	Plaintext string `json:"plaintext"`
	Version   int    `json:"version"`
}

func TestEnvelopeCompatibilityFixtures(t *testing.T) {
	if os.Getenv("WGMESH_REGEN_FIXTURES") != "" {
		regenEnvelopeFixtures(t)
	}

	replayEnvelopeCompatibilityFixtures(t, filepath.Join("..", "..", "testdata", "compat", "envelope"))
}

func TestEnvelopeCompatibilityFixturesReplayIndependentVersions(t *testing.T) {
	fixtureRoot := t.TempDir()
	sourceDir := filepath.Join("..", "..", "testdata", "compat", "envelope", "v1")
	writeSyntheticEnvelopeFixture(t, sourceDir, filepath.Join(fixtureRoot, "v0"), 0)
	writeSyntheticEnvelopeFixture(t, sourceDir, filepath.Join(fixtureRoot, "v1"), 1)

	got := replayEnvelopeCompatibilityFixtures(t, fixtureRoot)
	want := []int{0, 1}
	if len(got) != len(want) {
		t.Fatalf("replayed versions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("replayed versions = %v, want %v", got, want)
		}
	}
}

func replayEnvelopeCompatibilityFixtures(t *testing.T, fixtureRoot string) []int {
	t.Helper()

	versions, err := filepath.Glob(filepath.Join(fixtureRoot, "v*"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	if len(versions) == 0 {
		t.Fatalf("no envelope compatibility fixtures found under %s", fixtureRoot)
	}

	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatalf("derive fixture keys: %v", err)
	}

	replayedVersions := make([]int, 0, len(versions))
	for _, versionDir := range versions {
		versionDir := versionDir
		dirVersion, err := envelopeFixtureVersion(versionDir)
		if err != nil {
			t.Fatalf("parse fixture version: %v", err)
		}
		replayedVersions = append(replayedVersions, dirVersion)
		t.Run(filepath.Base(versionDir), func(t *testing.T) {
			envelopeData, err := os.ReadFile(filepath.Join(versionDir, "envelope.bin"))
			if err != nil {
				t.Fatalf("read envelope.bin: %v", err)
			}
			expectedData, err := os.ReadFile(filepath.Join(versionDir, "expected.json"))
			if err != nil {
				t.Fatalf("read expected.json: %v", err)
			}

			var expected envelopeFixtureExpected
			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatalf("unmarshal expected.json: %v", err)
			}
			if expected.Version != dirVersion {
				t.Fatalf("fixture version = %d, want directory version %d", expected.Version, dirVersion)
			}
			if expected.Version > EnvelopeCapabilityVersion {
				t.Fatalf("fixture version = %d exceeds current envelope capability %d", expected.Version, EnvelopeCapabilityVersion)
			}

			wantPlaintext, err := base64.StdEncoding.DecodeString(expected.Plaintext)
			if err != nil {
				t.Fatalf("decode expected plaintext: %v", err)
			}

			var meta struct {
				Timestamp int64 `json:"timestamp"`
			}
			if err := json.Unmarshal(wantPlaintext, &meta); err != nil {
				t.Fatalf("unmarshal expected plaintext metadata: %v", err)
			}

			restoreNow := setTestNow(time.Unix(meta.Timestamp, 0).Add(time.Second))
			defer restoreNow()

			_, gotAnnouncement, err := OpenEnvelope(envelopeData, keys.GossipKey)
			if err != nil {
				t.Fatalf("open envelope: %v", err)
			}
			gotPlaintext, err := json.Marshal(gotAnnouncement)
			if err != nil {
				t.Fatalf("marshal decoded announcement: %v", err)
			}
			if string(gotPlaintext) != string(wantPlaintext) {
				t.Fatalf("plaintext = %s, want %s", gotPlaintext, wantPlaintext)
			}

			t.Log("SealEnvelope uses a random nonce; fixture replay intentionally skips deterministic reseal comparison")
		})
	}

	return replayedVersions
}

const envelopeFixtureSecret = "wgmesh-envelope-compatibility-secret"

func envelopeFixtureVersion(versionDir string) (int, error) {
	base := filepath.Base(versionDir)
	if !strings.HasPrefix(base, "v") {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(strings.TrimPrefix(base, "v"))
}

func writeSyntheticEnvelopeFixture(t *testing.T, sourceDir, targetDir string, version int) {
	t.Helper()

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("create synthetic fixture dir: %v", err)
	}
	envelopeData, err := os.ReadFile(filepath.Join(sourceDir, "envelope.bin"))
	if err != nil {
		t.Fatalf("read source envelope.bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "envelope.bin"), envelopeData, 0o644); err != nil {
		t.Fatalf("write synthetic envelope.bin: %v", err)
	}

	expectedData, err := os.ReadFile(filepath.Join(sourceDir, "expected.json"))
	if err != nil {
		t.Fatalf("read source expected.json: %v", err)
	}
	var expected envelopeFixtureExpected
	if err := json.Unmarshal(expectedData, &expected); err != nil {
		t.Fatalf("unmarshal source expected.json: %v", err)
	}
	expected.Version = version
	expectedData, err = json.MarshalIndent(expected, "", "  ")
	if err != nil {
		t.Fatalf("marshal synthetic expected.json: %v", err)
	}
	expectedData = append(expectedData, '\n')
	if err := os.WriteFile(filepath.Join(targetDir, "expected.json"), expectedData, 0o644); err != nil {
		t.Fatalf("write synthetic expected.json: %v", err)
	}
}

func regenEnvelopeFixtures(t *testing.T) {
	t.Helper()

	keys, err := DeriveKeys(envelopeFixtureSecret)
	if err != nil {
		t.Fatalf("derive fixture keys: %v", err)
	}

	payload := PeerAnnouncement{
		Protocol:   ProtocolVersion,
		WGPubKey:   base64.StdEncoding.EncodeToString(make([]byte, 32)),
		Hostname:   "compat-v1",
		MeshIP:     "10.42.0.7",
		WGEndpoint: "127.0.0.1:51820",
		Timestamp:  1800000000,
	}
	envelope, err := SealEnvelope(MessageTypeAnnounce, payload, keys.GossipKey)
	if err != nil {
		t.Fatalf("seal fixture envelope: %v", err)
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal fixture plaintext: %v", err)
	}

	dir := filepath.Join("..", "..", "testdata", "compat", "envelope", "v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "envelope.bin"), envelope, 0o644); err != nil {
		t.Fatalf("write envelope.bin: %v", err)
	}
	expected := envelopeFixtureExpected{
		Plaintext: base64.StdEncoding.EncodeToString(plaintext),
		Version:   EnvelopeCapabilityVersion,
	}
	expectedData, err := json.MarshalIndent(expected, "", "  ")
	if err != nil {
		t.Fatalf("marshal expected.json: %v", err)
	}
	expectedData = append(expectedData, '\n')
	if err := os.WriteFile(filepath.Join(dir, "expected.json"), expectedData, 0o644); err != nil {
		t.Fatalf("write expected.json: %v", err)
	}
}

func setTestNow(t time.Time) func() {
	oldNow := now
	now = func() time.Time {
		return t
	}
	return func() {
		now = oldNow
	}
}
