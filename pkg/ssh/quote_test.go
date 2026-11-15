package ssh

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var hostileValues = []string{
	"a; touch PWNED",
	"$(id)",
	"`id`",
	"a\nb",
	`a"b`,
	"it's",
	"a b",
	"a|b",
	"a&b",
	">x",
	"[fd00::1]:51820",
	"*",
	"?",
	"[x]",
	"",
}

func TestQuoteLeavesSafeValuesUnchanged(t *testing.T) {
	t.Parallel()

	for _, v := range []string{
		"wg0",
		"10.99.0.2/32",
		"192.168.1.10:51820",
		"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQ=",
		"0.0.0.0/0",
	} {
		if got := Quote(v); got != v {
			t.Errorf("Quote(%q) = %q, want unchanged", v, got)
		}
	}
}

func TestQuoteNeutralisesHostileValues(t *testing.T) {
	t.Parallel()

	for _, v := range hostileValues {
		got := Quote(v)
		if len(got) < 2 || got[0] != '\'' || got[len(got)-1] != '\'' {
			t.Errorf("Quote(%q) = %q, want single-quoted", v, got)
		}
	}
	if got := Quote(""); got != "''" {
		t.Errorf("Quote(\"\") = %q, want ''", got)
	}
	if got, want := Quote("it's"), `'it'\''s'`; got != want {
		t.Errorf("Quote(it's) = %q, want %q", got, want)
	}
}

func TestQuoteRoundTripsThroughSh(t *testing.T) {
	t.Parallel()

	for _, v := range hostileValues {
		// Each value runs in its own empty directory holding files x and a, so
		// a glob would expand and a command that escaped Quote would leave PWNED.
		dir := t.TempDir()
		for _, name := range []string{"x", "a"} {
			if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("/bin/sh", "-c", "printf %s "+Quote(v))
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("sh for %q: %v", v, err)
		}
		if string(out) != v {
			t.Errorf("sh printed %q, want %q", out, v)
		}
		if _, err := os.Stat(filepath.Join(dir, "PWNED")); !os.IsNotExist(err) {
			t.Errorf("sh for %q created PWNED (stat err %v)", v, err)
		}
	}
}

func TestCmdQuotesEachArgument(t *testing.T) {
	t.Parallel()

	got := Cmd("wg", "set", "wg0; id", "peer")
	if want := "wg set 'wg0; id' peer"; got != want {
		t.Errorf("Cmd = %q, want %q", got, want)
	}
}
