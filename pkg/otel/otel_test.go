package otel

import (
	"context"
	"os"
	"testing"
)

func TestInit_NoEndpoint(t *testing.T) {
	t.Parallel()

	// Ensure no endpoint is set
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	shutdown, err := Init(context.Background(), "test-service", "v0.0.1")
	if err != nil {
		t.Fatalf("Init() with no endpoint should not error, got: %v", err)
	}

	// Shutdown should be safe to call
	shutdown(context.Background())
}

func TestInit_NoEndpoint_ReturnsNoopShutdown(t *testing.T) {
	t.Parallel()

	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	shutdown, _ := Init(context.Background(), "test-service", "v0.0.1")

	// Calling shutdown multiple times should be safe
	shutdown(context.Background())
	shutdown(context.Background())
}

func TestParseLogLine_WithTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		line          string
		wantComponent string
		wantBody      string
	}{
		{
			name:          "tagged with timestamp",
			line:          "2026/02/17 12:00:00 [DHT] bootstrap complete",
			wantComponent: "dht",
			wantBody:      "bootstrap complete",
		},
		{
			name:          "tagged without timestamp",
			line:          "[Exchange] peer found at 192.168.1.1:51820",
			wantComponent: "exchange",
			wantBody:      "peer found at 192.168.1.1:51820",
		},
		{
			name:          "no tag with timestamp",
			line:          "2026/02/17 12:00:00 plain log message",
			wantComponent: "general",
			wantBody:      "plain log message",
		},
		{
			name:          "no tag no timestamp",
			line:          "plain log message",
			wantComponent: "general",
			wantBody:      "plain log message",
		},
		{
			name:          "multi-word tag",
			line:          "[NAT] detected cone NAT",
			wantComponent: "nat",
			wantBody:      "detected cone NAT",
		},
		{
			name:          "empty body after tag",
			line:          "[OTel]",
			wantComponent: "otel",
			wantBody:      "",
		},
		{
			name:          "tag with timestamp prefix",
			line:          "2026/02/17 21:34:09 [Health] probing peer abc123",
			wantComponent: "health",
			wantBody:      "probing peer abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			component, body := parseLogLine(tt.line)
			if component != tt.wantComponent {
				t.Errorf("parseLogLine(%q) component = %q, want %q", tt.line, component, tt.wantComponent)
			}
			if body != tt.wantBody {
				t.Errorf("parseLogLine(%q) body = %q, want %q", tt.line, body, tt.wantBody)
			}
		})
	}
}

func TestBuildResource(t *testing.T) {
	t.Parallel()

	res, err := buildResource(context.Background(), "wgmesh", "v1.0.0")
	if err != nil {
		t.Fatalf("buildResource() error = %v", err)
	}
	if res == nil {
		t.Fatal("buildResource() returned nil resource")
	}

	// Verify the resource has the expected attributes
	attrs := res.Attributes()
	found := make(map[string]bool)
	for _, attr := range attrs {
		found[string(attr.Key)] = true
	}

	for _, key := range []string{"service.name", "service.version", "host.name"} {
		if !found[key] {
			t.Errorf("buildResource() missing attribute %q", key)
		}
	}
}
