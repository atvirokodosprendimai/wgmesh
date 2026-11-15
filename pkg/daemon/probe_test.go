package daemon

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"
)

func TestHandleProbeConnection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T, client net.Conn)
	}{
		{
			name: "normal pings on a persistent session get pongs",
			run: func(t *testing.T, client net.Conn) {
				reader := bufio.NewReader(client)
				for i := 0; i < 2; i++ {
					if _, err := client.Write([]byte("ping\n")); err != nil {
						t.Fatalf("write ping %d: %v", i, err)
					}
					line, err := reader.ReadString('\n')
					if err != nil {
						t.Fatalf("read pong %d: %v", i, err)
					}
					if line != "pong\n" {
						t.Fatalf("got %q, want %q", line, "pong\n")
					}
				}
			},
		},
		{
			name: "newline-free oversize request is rejected",
			run: func(t *testing.T, client net.Conn) {
				// net.Pipe is unbuffered, so n is exactly what the server read
				// before closing the connection.
				n, err := client.Write(bytes.Repeat([]byte("a"), 1<<20))
				if err == nil {
					t.Fatal("expected server to close connection on oversize request")
				}
				if n > probeMaxLineBytes {
					t.Fatalf("server read %d bytes, want at most %d", n, probeMaxLineBytes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, server := net.Pipe()
			defer client.Close()
			_ = client.SetDeadline(time.Now().Add(5 * time.Second))

			done := make(chan struct{})
			go func() {
				handleProbeConnection(server)
				close(done)
			}()

			tt.run(t, client)

			client.Close()
			<-done
		})
	}
}
