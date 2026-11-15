package ssh

import (
	"os/exec"
	"testing"
)

var hostileValues = []string{
	"a; rm -rf /",
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
		out, err := exec.Command("/bin/sh", "-c", "printf %s "+Quote(v)).Output()
		if err != nil {
			t.Fatalf("sh for %q: %v", v, err)
		}
		if string(out) != v {
			t.Errorf("sh printed %q, want %q", out, v)
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
