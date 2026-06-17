package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempLogger(t *testing.T) *Logger {
	t.Helper()

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "analytics.log")

	logger, err := NewLogger(LoggerConfig{LogPath: logPath, BufSize: 5})
	if err != nil {
		t.Fatalf("NewLogger() failed: %v", err)
	}

	return logger
}

func TestNewLogger(t *testing.T) {
	t.Run("creates logger", func(t *testing.T) {
		logger := tempLogger(t)
		if logger == nil {
			t.Error("logger is nil")
		}
	})

	t.Run("uses default path", func(t *testing.T) {
		// Set HOME to temp dir
		tmpDir := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		os.Setenv("HOME", tmpDir)

		logger, err := NewLogger(LoggerConfig{})
		if err != nil {
			t.Fatalf("NewLogger() failed: %v", err)
		}

		expectedPath := filepath.Join(tmpDir, ".wgmesh", "analytics.log")
		if logger.path != expectedPath {
			t.Errorf("path = %q, want %q", logger.path, expectedPath)
		}
	})
}

func TestLog(t *testing.T) {
	t.Run("logs event", func(t *testing.T) {
		logger := tempLogger(t)

		event := Event{
			Type:      EventTrialSignup,
			Timestamp: time.Now(),
			Properties: map[string]string{
				PropAccountID: "test-account",
			},
		}

		err := logger.Log(event)
		if err != nil {
			t.Fatalf("Log() failed: %v", err)
		}

		if len(logger.buffer) != 1 {
			t.Errorf("buffer length = %d, want 1", len(logger.buffer))
		}
	})

	t.Run("assigns ID if missing", func(t *testing.T) {
		logger := tempLogger(t)

		event := Event{
			Type: EventTrialSignup,
		}

		err := logger.Log(event)
		if err != nil {
			t.Fatalf("Log() failed: %v", err)
		}

		if logger.buffer[0].ID == "" {
			t.Error("event ID not assigned")
		}
	})

	t.Run("assigns timestamp if missing", func(t *testing.T) {
		logger := tempLogger(t)

		event := Event{
			Type: EventTrialSignup,
			ID:   "test-id",
		}

		err := logger.Log(event)
		if err != nil {
			t.Fatalf("Log() failed: %v", err)
		}

		if logger.buffer[0].Timestamp.IsZero() {
			t.Error("timestamp not assigned")
		}
	})
}

func TestFlush(t *testing.T) {
	t.Run("flushes buffer when full", func(t *testing.T) {
		logger := tempLogger(t)

		// Log 5 events (buffer size is 5)
		for i := 0; i < 5; i++ {
			err := logger.Log(Event{Type: EventTrialSignup})
			if err != nil {
				t.Fatalf("Log() failed: %v", err)
			}
		}

		// Buffer should be flushed
		if len(logger.buffer) != 0 {
			t.Errorf("buffer not flushed, length = %d", len(logger.buffer))
		}

		if logger.flushed != 5 {
			t.Errorf("flushed = %d, want 5", logger.flushed)
		}
	})

	t.Run("writes to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "analytics.log")

		logger, err := NewLogger(LoggerConfig{LogPath: logPath, BufSize: 2})
		if err != nil {
			t.Fatalf("NewLogger() failed: %v", err)
		}

		// Log 2 events to trigger flush
		for i := 0; i < 2; i++ {
			err := logger.Log(Event{
				Type: EventTrialSignup,
				Properties: map[string]string{
					PropAccountID: fmt.Sprintf("account-%d", i),
				},
			})
			if err != nil {
				t.Fatalf("Log() failed: %v", err)
			}
		}

		// Verify file exists
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("log file not created")
		}

		// Read and verify content
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile() failed: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) != 2 {
			t.Errorf("file has %d lines, want 2", len(lines))
		}

		// Parse and verify first event
		var event Event
		if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
			t.Fatalf("Unmarshal() failed: %v", err)
		}

		if event.Type != EventTrialSignup {
			t.Errorf("event type = %q, want %q", event.Type, EventTrialSignup)
		}
	})

	t.Run("manual flush", func(t *testing.T) {
		logger := tempLogger(t)

		err := logger.Log(Event{Type: EventTrialSignup})
		if err != nil {
			t.Fatalf("Log() failed: %v", err)
		}

		err = logger.Flush()
		if err != nil {
			t.Fatalf("Flush() failed: %v", err)
		}

		if len(logger.buffer) != 0 {
			t.Errorf("buffer not flushed, length = %d", len(logger.buffer))
		}
	})
}

func TestLogTrialSignup(t *testing.T) {
	logger := tempLogger(t)

	err := logger.LogTrialSignup("CODE123", "campaign-1", "discord", "account-1")
	if err != nil {
		t.Fatalf("LogTrialSignup() failed: %v", err)
	}

	if len(logger.buffer) != 1 {
		t.Errorf("buffer length = %d, want 1", len(logger.buffer))
	}

	event := logger.buffer[0]
	if event.Type != EventTrialSignup {
		t.Errorf("event type = %q, want %q", event.Type, EventTrialSignup)
	}

	if event.Properties[PropCode] != "CODE123" {
		t.Errorf("code property = %q, want CODE123", event.Properties[PropCode])
	}
}

func TestLogTrialActivation(t *testing.T) {
	logger := tempLogger(t)

	err := logger.LogTrialActivation("account-1")
	if err != nil {
		t.Fatalf("LogTrialActivation() failed: %v", err)
	}

	if logger.buffer[0].Type != EventTrialActivation {
		t.Error("wrong event type")
	}
}

func TestLogTrialConversion(t *testing.T) {
	logger := tempLogger(t)

	err := logger.LogTrialConversion("account-1")
	if err != nil {
		t.Fatalf("LogTrialConversion() failed: %v", err)
	}

	if logger.buffer[0].Type != EventTrialConversion {
		t.Error("wrong event type")
	}
}

func TestLogPromoRedeemed(t *testing.T) {
	logger := tempLogger(t)

	err := logger.LogPromoRedeemed("CODE123", "campaign-1", "account-1")
	if err != nil {
		t.Fatalf("LogPromoRedeemed() failed: %v", err)
	}

	if logger.buffer[0].Type != EventPromoRedeemed {
		t.Error("wrong event type")
	}
}

func TestLogCommunityClick(t *testing.T) {
	logger := tempLogger(t)

	err := logger.LogCommunityClick("community-1", "discord", "https://example.com")
	if err != nil {
		t.Fatalf("LogCommunityClick() failed: %v", err)
	}

	if logger.buffer[0].Type != EventCommunityClick {
		t.Error("wrong event type")
	}

	if logger.buffer[0].Properties[PropCommunityID] != "community-1" {
		t.Error("community ID property missing")
	}
}
