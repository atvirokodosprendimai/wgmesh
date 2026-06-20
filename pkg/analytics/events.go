package analytics

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EventType represents the type of analytics event
type EventType string

const (
	// Trial funnel stages
	EventTrialLandingViewed    EventType = "trial_landing_viewed"
	EventTrialFormStarted      EventType = "trial_form_started"
	EventTrialEmailSubmitted   EventType = "trial_email_submitted"
	EventTrialEmailVerified    EventType = "trial_email_verified"
	EventTrialAccountCreated   EventType = "trial_account_created"
	EventTrialInstallStarted   EventType = "trial_install_started"
	EventTrialInstallCompleted EventType = "trial_install_completed"
	EventTrialMeshActive       EventType = "trial_mesh_active"
)

// ErrorType represents the category of error that occurred
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation_error"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeServerError  ErrorType = "server_error"
	ErrorTypeNetworkError ErrorType = "network_error"
	ErrorTypeUnknown      ErrorType = "unknown"
)

// EventMetadata holds contextual information about an event
type EventMetadata struct {
	Referrer    string `json:"referrer,omitempty"`
	UTMSource   string `json:"utm_source,omitempty"`
	UTMMedium   string `json:"utm_medium,omitempty"`
	UTMCampaign string `json:"utm_campaign,omitempty"`
	Browser     string `json:"browser,omitempty"`
	DeviceType  string `json:"device_type,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
}

// FunnelError captures error details for failed funnel stages
type FunnelError struct {
	Stage       string                 `json:"stage"`
	ErrorType   ErrorType              `json:"error_type"`
	ErrorCode   string                 `json:"error_code"`
	UserMessage string                 `json:"user_message,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Event represents a single analytics event
type Event struct {
	ID        string        `json:"event_id"`
	Type      EventType     `json:"event_type"`
	Timestamp time.Time     `json:"timestamp"`
	SessionID string        `json:"session_id"`
	UserID    string        `json:"user_id,omitempty"`
	Metadata  EventMetadata `json:"metadata,omitempty"`
	Error     *FunnelError  `json:"error,omitempty"`
}

// Handler defines the interface for processing events
type Handler interface {
	Handle(ctx context.Context, event Event) error
}

// Tracker manages analytics event collection and dispatch
type Tracker struct {
	handlers []Handler
	mu       sync.RWMutex
}

// NewTracker creates a new analytics tracker
func NewTracker() *Tracker {
	return &Tracker{
		handlers: make([]Handler, 0),
	}
}

// AddHandler registers an event handler
func (t *Tracker) AddHandler(handler Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers = append(t.handlers, handler)
}

// Track emits an event to all registered handlers
func (t *Tracker) Track(ctx context.Context, event Event) error {
	// Ensure event has required fields
	if event.ID == "" {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("failed to generate event ID: %w", err)
		}
		event.ID = id
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}

	t.mu.RLock()
	handlers := make([]Handler, len(t.handlers))
	copy(handlers, t.handlers)
	t.mu.RUnlock()

	// Dispatch to all handlers concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h Handler) {
			defer wg.Done()
			if err := h.Handle(ctx, event); err != nil {
				errChan <- fmt.Errorf("handler failed: %w", err)
			}
		}(handler)
	}

	wg.Wait()
	close(errChan)

	// Collect first error if any
	for err := range errChan {
		return err
	}

	return nil
}

// TrackTrialLandingViewed tracks when a user views the trial landing page
func (t *Tracker) TrackTrialLandingViewed(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialFormStarted tracks when a user begins the signup form
func (t *Tracker) TrackTrialFormStarted(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialFormStarted,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialEmailSubmitted tracks when a user submits their email
func (t *Tracker) TrackTrialEmailSubmitted(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialEmailSubmitted,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialEmailVerified tracks when a user verifies their email
func (t *Tracker) TrackTrialEmailVerified(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialEmailVerified,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialAccountCreated tracks successful account creation
func (t *Tracker) TrackTrialAccountCreated(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialAccountCreated,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialInstallStarted tracks when wgmesh installation begins
func (t *Tracker) TrackTrialInstallStarted(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialInstallStarted,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialInstallCompleted tracks successful wgmesh installation
func (t *Tracker) TrackTrialInstallCompleted(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialInstallCompleted,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackTrialMeshActive tracks when the first mesh becomes operational
func (t *Tracker) TrackTrialMeshActive(ctx context.Context, sessionID, userID string, metadata EventMetadata) error {
	return t.Track(ctx, Event{
		Type:      EventTrialMeshActive,
		SessionID: sessionID,
		UserID:    userID,
		Metadata:  metadata,
	})
}

// TrackError tracks an error that occurred during a funnel stage
func (t *Tracker) TrackError(ctx context.Context, sessionID, userID, stage string, errType ErrorType, errCode, userMessage string, context map[string]interface{}) error {
	return t.Track(ctx, Event{
		Type:      EventType(stage),
		SessionID: sessionID,
		UserID:    userID,
		Error: &FunnelError{
			Stage:       stage,
			ErrorType:   errType,
			ErrorCode:   errCode,
			UserMessage: userMessage,
			Context:     context,
			Timestamp:   time.Now().UTC(),
		},
	})
}

// generateID creates a unique identifier for events
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateSessionID creates a new session ID
func GenerateSessionID() (string, error) {
	return generateID()
}

// ValidateEventType checks if an event type is valid
func ValidateEventType(eventType EventType) bool {
	switch eventType {
	case EventTrialLandingViewed,
		EventTrialFormStarted,
		EventTrialEmailSubmitted,
		EventTrialEmailVerified,
		EventTrialAccountCreated,
		EventTrialInstallStarted,
		EventTrialInstallCompleted,
		EventTrialMeshActive:
		return true
	default:
		return false
	}
}

// ValidateErrorType checks if an error type is valid
func ValidateErrorType(errorType ErrorType) bool {
	switch errorType {
	case ErrorTypeValidation,
		ErrorTypeRateLimit,
		ErrorTypeServerError,
		ErrorTypeNetworkError,
		ErrorTypeUnknown:
		return true
	default:
		return false
	}
}

// MarshalJSON implements custom JSON marshaling for Event
func (e Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return json.Marshal(struct {
		Alias
	}{
		Alias: Alias(e),
	})
}

// FunnelStage defines the order of funnel stages
var FunnelStageOrder = []EventType{
	EventTrialLandingViewed,
	EventTrialFormStarted,
	EventTrialEmailSubmitted,
	EventTrialEmailVerified,
	EventTrialAccountCreated,
	EventTrialInstallStarted,
	EventTrialInstallCompleted,
	EventTrialMeshActive,
}

// GetStageIndex returns the index of a funnel stage, or -1 if not found
func GetStageIndex(stage EventType) int {
	for i, s := range FunnelStageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// IsStageAfter checks if stageA comes after stageB in the funnel
func IsStageAfter(stageA, stageB EventType) bool {
	idxA := GetStageIndex(stageA)
	idxB := GetStageIndex(stageB)
	return idxA > idxB && idxA >= 0 && idxB >= 0
}
