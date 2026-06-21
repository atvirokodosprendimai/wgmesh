package analytics

import (
	"context"
	"testing"
	"time"
)

func TestGenerateSessionID(t *testing.T) {
	t.Parallel()

	id1, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID() failed: %v", err)
	}

	if len(id1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("GenerateSessionID() length = %d, want 32", len(id1))
	}

	// Ensure uniqueness
	id2, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID() failed: %v", err)
	}

	if id1 == id2 {
		t.Error("GenerateSessionID() produced duplicate IDs")
	}
}

func TestValidateEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		{"valid landing viewed", EventTrialLandingViewed, true},
		{"valid form started", EventTrialFormStarted, true},
		{"valid email submitted", EventTrialEmailSubmitted, true},
		{"valid email verified", EventTrialEmailVerified, true},
		{"valid account created", EventTrialAccountCreated, true},
		{"valid install started", EventTrialInstallStarted, true},
		{"valid install completed", EventTrialInstallCompleted, true},
		{"valid mesh active", EventTrialMeshActive, true},
		{"invalid type", EventType("invalid_event"), false},
		{"empty type", EventType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidateEventType(tt.eventType); got != tt.want {
				t.Errorf("ValidateEventType(%q) = %v, want %v", tt.eventType, got, tt.want)
			}
		})
	}
}

func TestValidateErrorType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errType ErrorType
		want    bool
	}{
		{"valid validation error", ErrorTypeValidation, true},
		{"valid rate limit error", ErrorTypeRateLimit, true},
		{"valid server error", ErrorTypeServerError, true},
		{"valid network error", ErrorTypeNetworkError, true},
		{"valid unknown error", ErrorTypeUnknown, true},
		{"invalid type", ErrorType("invalid_error"), false},
		{"empty type", ErrorType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ValidateErrorType(tt.errType); got != tt.want {
				t.Errorf("ValidateErrorType(%q) = %v, want %v", tt.errType, got, tt.want)
			}
		})
	}
}

func TestGetStageIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stage EventType
		want  int
	}{
		{"landing viewed", EventTrialLandingViewed, 0},
		{"form started", EventTrialFormStarted, 1},
		{"email submitted", EventTrialEmailSubmitted, 2},
		{"email verified", EventTrialEmailVerified, 3},
		{"account created", EventTrialAccountCreated, 4},
		{"install started", EventTrialInstallStarted, 5},
		{"install completed", EventTrialInstallCompleted, 6},
		{"mesh active", EventTrialMeshActive, 7},
		{"invalid stage", EventType("invalid"), -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := GetStageIndex(tt.stage); got != tt.want {
				t.Errorf("GetStageIndex(%q) = %d, want %d", tt.stage, got, tt.want)
			}
		})
	}
}

func TestIsStageAfter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stageA EventType
		stageB EventType
		want   bool
	}{
		{"mesh active after landing", EventTrialMeshActive, EventTrialLandingViewed, true},
		{"account created after form", EventTrialAccountCreated, EventTrialFormStarted, true},
		{"form started after landing", EventTrialFormStarted, EventTrialLandingViewed, true},
		{"landing not after mesh", EventTrialLandingViewed, EventTrialMeshActive, false},
		{"same stage", EventTrialFormStarted, EventTrialFormStarted, false},
		{"invalid stage A", EventType("invalid"), EventTrialLandingViewed, false},
		{"invalid stage B", EventTrialLandingViewed, EventType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsStageAfter(tt.stageA, tt.stageB); got != tt.want {
				t.Errorf("IsStageAfter(%q, %q) = %v, want %v", tt.stageA, tt.stageB, got, tt.want)
			}
		})
	}
}

func TestTracker_Track(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	sessionID, _ := GenerateSessionID()

	// Track a simple event
	err := tracker.Track(ctx, Event{
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
		UserID:    "user123",
		Metadata: EventMetadata{
			Browser:    "Firefox",
			DeviceType: "desktop",
		},
	})

	if err != nil {
		t.Fatalf("Track() failed: %v", err)
	}

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Track() stored %d events, want 1", len(events))
	}

	if events[0].Type != EventTrialLandingViewed {
		t.Errorf("Track() event type = %q, want %q", events[0].Type, EventTrialLandingViewed)
	}

	if events[0].SessionID != sessionID {
		t.Errorf("Track() session ID = %q, want %q", events[0].SessionID, sessionID)
	}

	if events[0].UserID != "user123" {
		t.Errorf("Track() user ID = %q, want %q", events[0].UserID, "user123")
	}

	if events[0].Metadata.Browser != "Firefox" {
		t.Errorf("Track() browser = %q, want %q", events[0].Metadata.Browser, "Firefox")
	}
}

func TestTracker_Track_AutoGeneratesID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	sessionID, _ := GenerateSessionID()

	err := tracker.Track(ctx, Event{
		ID:        "", // Empty ID should be auto-generated
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
	})

	if err != nil {
		t.Fatalf("Track() failed: %v", err)
	}

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Track() stored %d events, want 1", len(events))
	}

	if events[0].ID == "" {
		t.Error("Track() did not auto-generate event ID")
	}
}

func TestTracker_Track_AutoGeneratesTimestamp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	sessionID, _ := GenerateSessionID()

	before := time.Now().UTC()

	err := tracker.Track(ctx, Event{
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
	})

	if err != nil {
		t.Fatalf("Track() failed: %v", err)
	}

	events := handler.GetEvents()
	if events[0].Timestamp.Before(before) {
		t.Error("Track() timestamp is before the test started")
	}

	if events[0].Timestamp.After(time.Now().UTC().Add(5 * time.Second)) {
		t.Error("Track() timestamp is far in the future")
	}
}

func TestTracker_Track_RequiresSessionID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	err := tracker.Track(ctx, Event{
		Type:      EventTrialLandingViewed,
		SessionID: "", // Empty session ID should cause error
	})

	if err == nil {
		t.Error("Track() with empty session ID should fail, but didn't")
	}
}

func TestTracker_TrackTrialLandingViewed(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	sessionID, _ := GenerateSessionID()

	err := tracker.TrackTrialLandingViewed(ctx, sessionID, "user123", EventMetadata{
		Referrer:    "https://example.com",
		UTMSource:   "google",
		UTMMedium:   "cpc",
		UTMCampaign: "spring2024",
	})

	if err != nil {
		t.Fatalf("TrackTrialLandingViewed() failed: %v", err)
	}

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("TrackTrialLandingViewed() stored %d events, want 1", len(events))
	}

	if events[0].Type != EventTrialLandingViewed {
		t.Errorf("TrackTrialLandingViewed() event type = %q, want %q", events[0].Type, EventTrialLandingViewed)
	}

	if events[0].Metadata.Referrer != "https://example.com" {
		t.Errorf("TrackTrialLandingViewed() referrer = %q, want %q", events[0].Metadata.Referrer, "https://example.com")
	}

	if events[0].Metadata.UTMSource != "google" {
		t.Errorf("TrackTrialLandingViewed() utm_source = %q, want %q", events[0].Metadata.UTMSource, "google")
	}
}

func TestTracker_TrackError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	sessionID, _ := GenerateSessionID()

	err := tracker.TrackError(ctx, sessionID, "user123", "trial_email_submitted", ErrorTypeValidation, "INVALID_EMAIL", "Invalid email format", map[string]interface{}{
		"email": "not-an-email",
		"field": "email",
	})

	if err != nil {
		t.Fatalf("TrackError() failed: %v", err)
	}

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("TrackError() stored %d events, want 1", len(events))
	}

	if events[0].Error == nil {
		t.Fatal("TrackError() event has no error data")
	}

	if events[0].Error.ErrorType != ErrorTypeValidation {
		t.Errorf("TrackError() error type = %q, want %q", events[0].Error.ErrorType, ErrorTypeValidation)
	}

	if events[0].Error.ErrorCode != "INVALID_EMAIL" {
		t.Errorf("TrackError() error code = %q, want %q", events[0].Error.ErrorCode, "INVALID_EMAIL")
	}

	if events[0].Error.UserMessage != "Invalid email format" {
		t.Errorf("TrackError() user message = %q, want %q", events[0].Error.UserMessage, "Invalid email format")
	}

	if events[0].Error.Context["email"] != "not-an-email" {
		t.Errorf("TrackError() context email = %q, want %q", events[0].Error.Context["email"], "not-an-email")
	}
}

func TestTracker_AllFunnelStages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()
	tracker := NewTracker()
	tracker.AddHandler(handler)

	sessionID, _ := GenerateSessionID()
	userID := "user123"
	metadata := EventMetadata{
		Browser:    "Chrome",
		DeviceType: "desktop",
	}

	stages := []func(context.Context, string, string, EventMetadata) error{
		tracker.TrackTrialLandingViewed,
		tracker.TrackTrialFormStarted,
		tracker.TrackTrialEmailSubmitted,
		tracker.TrackTrialEmailVerified,
		tracker.TrackTrialAccountCreated,
		tracker.TrackTrialInstallStarted,
		tracker.TrackTrialInstallCompleted,
		tracker.TrackTrialMeshActive,
	}

	for i, stage := range stages {
		if err := stage(ctx, sessionID, userID, metadata); err != nil {
			t.Fatalf("Stage %d failed: %v", i, err)
		}
	}

	events := handler.GetEvents()
	if len(events) != len(stages) {
		t.Fatalf("Got %d events, want %d", len(events), len(stages))
	}

	// Verify events are in correct order
	expectedOrder := FunnelStageOrder
	for i, event := range events {
		if event.Type != expectedOrder[i] {
			t.Errorf("Event %d type = %q, want %q", i, event.Type, expectedOrder[i])
		}
	}
}

func TestTracker_MultipleHandlers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler1 := NewMemoryHandler()
	handler2 := NewMemoryHandler()
	logHandler := NewLogHandler("TEST")

	tracker := NewTracker()
	tracker.AddHandler(handler1)
	tracker.AddHandler(handler2)
	tracker.AddHandler(logHandler)

	sessionID, _ := GenerateSessionID()

	err := tracker.Track(ctx, Event{
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
	})

	if err != nil {
		t.Fatalf("Track() failed: %v", err)
	}

	// Both memory handlers should have the event
	if handler1.Count() != 1 {
		t.Errorf("Handler1 has %d events, want 1", handler1.Count())
	}

	if handler2.Count() != 1 {
		t.Errorf("Handler2 has %d events, want 1", handler2.Count())
	}
}
