package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger records analytics events to a log file.
// For production use, events would be sent to an analytics service.
// This implementation provides file-based logging for development and internal dashboards.
type Logger struct {
	mu      sync.Mutex
	path    string // Path to event log file
	buffer  []Event
	bufSize int // Buffer size before flush
	flushed int // Number of events flushed
}

// LoggerConfig holds configuration for the analytics logger.
type LoggerConfig struct {
	LogPath string // Path to log file (default: ~/.wgmesh/analytics.log)
	BufSize int    // Buffer size (default: 100)
}

// NewLogger creates a new analytics logger.
func NewLogger(cfg LoggerConfig) (*Logger, error) {
	path := cfg.LogPath
	if path == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		path = filepath.Join(homeDir, ".wgmesh", "analytics.log")
	}

	bufSize := cfg.BufSize
	if bufSize <= 0 {
		bufSize = 100
	}

	l := &Logger{
		path:    path,
		bufSize: bufSize,
		buffer:  make([]Event, 0, bufSize),
	}

	return l, nil
}

// Log records an analytics event.
// Events are buffered and flushed to disk when the buffer is full.
func (l *Logger) Log(event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure event has timestamp
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Ensure event has ID
	if event.ID == "" {
		event.ID = generateEventID(event)
	}

	// Add to buffer
	l.buffer = append(l.buffer, event)

	// Flush if buffer is full
	if len(l.buffer) >= l.bufSize {
		if err := l.flush(); err != nil {
			return err
		}
	}

	return nil
}

// Flush writes buffered events to disk.
func (l *Logger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.flush()
}

// flush writes buffered events to disk without locking.
func (l *Logger) flush() error {
	if len(l.buffer) == 0 {
		return nil
	}

	// Create directory if needed
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Open file for appending
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	// Write each event as JSON line
	encoder := json.NewEncoder(f)
	for _, event := range l.buffer {
		if err := encoder.Encode(event); err != nil {
			return fmt.Errorf("encode event: %w", err)
		}
	}

	l.flushed += len(l.buffer)
	l.buffer = l.buffer[:0]

	return nil
}

// LogTrialSignup records a trial signup event.
func (l *Logger) LogTrialSignup(code, campaignID, source, accountID string) error {
	return l.Log(Event{
		Type: EventTrialSignup,
		Properties: map[string]string{
			PropCode:       code,
			PropCampaignID: campaignID,
			PropSource:     source,
			PropAccountID:  accountID,
		},
	})
}

// LogTrialActivation records a trial activation event.
func (l *Logger) LogTrialActivation(accountID string) error {
	return l.Log(Event{
		Type: EventTrialActivation,
		Properties: map[string]string{
			PropAccountID: accountID,
		},
	})
}

// LogTrialConversion records a trial conversion event.
func (l *Logger) LogTrialConversion(accountID string) error {
	return l.Log(Event{
		Type: EventTrialConversion,
		Properties: map[string]string{
			PropAccountID: accountID,
		},
	})
}

// LogPromoRedeemed records a promo redemption event.
func (l *Logger) LogPromoRedeemed(code, campaignID, accountID string) error {
	return l.Log(Event{
		Type: EventPromoRedeemed,
		Properties: map[string]string{
			PropCode:       code,
			PropCampaignID: campaignID,
			PropAccountID:  accountID,
		},
	})
}

// LogCommunityClick records a click-through from community outreach.
func (l *Logger) LogCommunityClick(communityID, source, url string) error {
	return l.Log(Event{
		Type: EventCommunityClick,
		Properties: map[string]string{
			PropCommunityID: communityID,
			PropSource:      source,
			PropClickURL:    url,
		},
	})
}

// generateEventID creates a unique event ID.
func generateEventID(event Event) string {
	return fmt.Sprintf("%s-%d", event.Type, time.Now().UnixNano())
}
