package otel

import (
	"io"
	"log"
	"os"
	"strings"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// logBridgeWriter is an io.Writer that intercepts log.Printf output,
// parses [Tag] prefixes into structured attributes, and emits OTel log records.
// It also writes all output to stderr to preserve existing behavior.
type logBridgeWriter struct {
	stderr io.Writer
	logger otellog.Logger
}

// Write implements io.Writer. It parses each log line for a [Component] prefix,
// extracts it as an attribute, and emits an OTel log record.
func (w *logBridgeWriter) Write(p []byte) (int, error) {
	// Always write to stderr first
	n, err := w.stderr.Write(p)

	// Parse the log line for OTel emission
	line := strings.TrimSpace(string(p))
	if line == "" {
		return n, err
	}

	component, body := parseLogLine(line)

	var record otellog.Record
	record.SetTimestamp(time.Now())
	record.SetBody(otellog.StringValue(body))
	record.SetSeverity(otellog.SeverityInfo)
	record.AddAttributes(otellog.String("component", component))

	w.logger.Emit(nil, record) //nolint:staticcheck // nil context is fine for fire-and-forget

	return n, err
}

// parseLogLine extracts a [Tag] prefix from a log line.
// Input:  "2026/02/17 12:00:00 [DHT] bootstrap complete"
// Output: component="dht", body="bootstrap complete"
//
// If no [Tag] is found, component is "general" and body is the full line
// (with the stdlib log timestamp prefix stripped if present).
func parseLogLine(line string) (component, body string) {
	// Strip stdlib log timestamp prefix (e.g. "2026/02/17 12:00:00 ")
	// Format: YYYY/MM/DD HH:MM:SS — 20 chars
	stripped := line
	if len(line) > 20 && line[4] == '/' && line[7] == '/' && line[10] == ' ' && line[13] == ':' {
		stripped = strings.TrimSpace(line[20:])
	}

	// Look for [Tag] prefix
	if len(stripped) > 2 && stripped[0] == '[' {
		end := strings.IndexByte(stripped, ']')
		if end > 1 {
			component = strings.ToLower(stripped[1:end])
			body = strings.TrimSpace(stripped[end+1:])
			return component, body
		}
	}

	return "general", stripped
}

// InstallLogBridge replaces log.SetOutput with a writer that forwards
// log.Printf output to both stderr and the OTel LoggerProvider.
// Existing log.Printf calls require zero changes.
func InstallLogBridge(lp *sdklog.LoggerProvider) {
	logger := lp.Logger("wgmesh.log")
	log.SetOutput(&logBridgeWriter{
		stderr: os.Stderr,
		logger: logger,
	})
}
