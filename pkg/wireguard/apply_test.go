package wireguard

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeWGScript installs body as the `wg` shell script and returns the
// directory exported as $FAKEWG_DIR, where the script may leave files.
func fakeWGScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FAKEWG_DIR", dir)
	path := filepath.Join(dir, "wg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(SetWGPath(path))
	return dir
}

// fakeWGRecorder records one argument per line in $FAKEWG_DIR/args and stdin
// in $FAKEWG_DIR/stdin; it fails with "boom" when $FAKEWG_DIR/fail exists.
func fakeWGRecorder(t *testing.T) string {
	return fakeWGScript(t,
		"printf '%s\\n' \"$@\" > \"$FAKEWG_DIR/args\"\n"+
			"cat > \"$FAKEWG_DIR/stdin\"\n"+
			"if [ -e \"$FAKEWG_DIR/fail\" ]; then echo boom >&2; exit 1; fi\n")
}

func recordedArgs(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "args"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func recordedStdin(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "stdin"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetPeerPassesPresharedKeyOnStdinNotArgv(t *testing.T) {
	dir := fakeWGRecorder(t)
	var psk [32]byte
	for i := range psk {
		psk[i] = byte(i + 1)
	}
	b64 := base64.StdEncoding.EncodeToString(psk[:])

	if err := SetPeer("wg0", "keyA", psk, "", ""); err != nil {
		t.Fatal(err)
	}

	if got := recordedStdin(t, dir); got != b64+"\n" {
		t.Fatalf("stdin = %q, want %q", got, b64+"\n")
	}
	args := recordedArgs(t, dir)
	want := []string{"set", "wg0", "peer", "keyA", "preshared-key", "/dev/stdin", "persistent-keepalive", "25"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
}

func TestSetPeerArgsOrder(t *testing.T) {
	var zero [32]byte

	t.Run("all fields", func(t *testing.T) {
		dir := fakeWGRecorder(t)
		if err := SetPeer("wg0", "keyA", zero, "203.0.113.5:51820", "10.0.0.2/32,10.0.0.3/32"); err != nil {
			t.Fatal(err)
		}
		want := []string{"set", "wg0", "peer", "keyA",
			"endpoint", "203.0.113.5:51820",
			"allowed-ips", "10.0.0.2/32,10.0.0.3/32",
			"persistent-keepalive", "25"}
		if got := recordedArgs(t, dir); !reflect.DeepEqual(got, want) {
			t.Fatalf("args = %q, want %q", got, want)
		}
		if got := recordedStdin(t, dir); got != "" {
			t.Fatalf("zero key must send nothing on stdin, got %q", got)
		}
	})

	t.Run("empty endpoint and allowed ips omitted", func(t *testing.T) {
		dir := fakeWGRecorder(t)
		if err := SetPeer("wg0", "keyA", zero, "", ""); err != nil {
			t.Fatal(err)
		}
		want := []string{"set", "wg0", "peer", "keyA", "persistent-keepalive", "25"}
		if got := recordedArgs(t, dir); !reflect.DeepEqual(got, want) {
			t.Fatalf("args = %q, want %q", got, want)
		}
	})

	t.Run("failure carries the output", func(t *testing.T) {
		dir := fakeWGRecorder(t)
		if err := os.WriteFile(filepath.Join(dir, "fail"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		err := SetPeer("wg0", "keyA", zero, "", "")
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("err = %v, want one containing the wg output", err)
		}
	})
}

func TestRemovePeerArgs(t *testing.T) {
	dir := fakeWGRecorder(t)
	if err := RemovePeer("wg0", "keyA"); err != nil {
		t.Fatal(err)
	}
	want := []string{"set", "wg0", "peer", "keyA", "remove"}
	if got := recordedArgs(t, dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %q, want %q", got, want)
	}

	if err := os.WriteFile(filepath.Join(dir, "fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	err := RemovePeer("wg0", "keyA")
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want one containing the wg output", err)
	}
}
