package wireguard

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeKeyWG answers `genkey` with a padded private key and `pubkey` with
// "pub(<stdin>)"; `pubkey` fails when $FAKEWG_DIR/failpub exists.
func fakeKeyWG(t *testing.T) string {
	return fakeWGScript(t,
		"case \"$1\" in\n"+
			"genkey) printf '  PRIVKEY  \\n' ;;\n"+
			"pubkey)\n"+
			"  in=$(cat)\n"+
			"  if [ -e \"$FAKEWG_DIR/failpub\" ]; then echo boom >&2; exit 1; fi\n"+
			"  printf 'pub(%s)\\n' \"$in\" ;;\n"+
			"esac\n")
}

func TestGenerateKeyPairParsesGenkeyAndPubkey(t *testing.T) {
	fakeKeyWG(t)

	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if priv != "PRIVKEY" {
		t.Fatalf("private = %q, want trimmed PRIVKEY", priv)
	}
	if pub != "pub(PRIVKEY)" {
		t.Fatalf("public = %q, want pub(PRIVKEY): genkey output trimmed and piped to pubkey", pub)
	}
}

func TestGenerateKeyPairReportsFailure(t *testing.T) {
	dir := fakeKeyWG(t)
	if err := os.WriteFile(filepath.Join(dir, "failpub"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	priv, pub, err := GenerateKeyPair()
	if err == nil {
		t.Fatal("want an error when pubkey fails")
	}
	if priv != "" || pub != "" {
		t.Fatalf("keys = %q, %q, want both empty on error", priv, pub)
	}
}
