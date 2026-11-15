package daemon

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

func TestReadProbeReply(t *testing.T) {
	t.Parallel()

	newReader := func(s string) *bufio.Reader {
		return bufio.NewReaderSize(strings.NewReader(s), probeMaxLineBytes)
	}

	got, err := readProbeReply(newReader("pong\n"))
	if err != nil || got != "pong" {
		t.Fatalf("readProbeReply(pong) = %q, %v; want pong, nil", got, err)
	}

	_, err = readProbeReply(newReader(strings.Repeat("a", 1<<20)))
	if !errors.Is(err, bufio.ErrBufferFull) {
		t.Fatalf("newline-free reply: err = %v, want bufio.ErrBufferFull", err)
	}
}
