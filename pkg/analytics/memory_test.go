package analytics

import (
	"context"
	"testing"
)

func TestMemoryHandler_Handle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	sessionID, _ := GenerateSessionID()

	event := Event{
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
		UserID:    "user123",
	}

	err := handler.Handle(ctx, event)
	if err != nil {
		t.Fatalf("Handle() failed: %v", err)
	}

	events := handler.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Handle() stored %d events, want 1", len(events))
	}

	if events[0].SessionID != sessionID {
		t.Errorf("Handle() session ID = %q, want %q", events[0].SessionID, sessionID)
	}
}

func TestMemoryHandler_GetEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	sessionID1, _ := GenerateSessionID()
	sessionID2, _ := GenerateSessionID()

	events := []Event{
		{Type: EventTrialLandingViewed, SessionID: sessionID1, UserID: "user1"},
		{Type: EventTrialFormStarted, SessionID: sessionID1, UserID: "user1"},
		{Type: EventTrialLandingViewed, SessionID: sessionID2, UserID: "user2"},
	}

	for _, e := range events {
		if err := handler.Handle(ctx, e); err != nil {
			t.Fatalf("Handle() failed: %v", err)
		}
	}

	retrieved := handler.GetEvents()
	if len(retrieved) != len(events) {
		t.Fatalf("GetEvents() returned %d events, want %d", len(retrieved), len(events))
	}
}

func TestMemoryHandler_GetEventsBySession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	sessionID1, _ := GenerateSessionID()
	sessionID2, _ := GenerateSessionID()

	events := []Event{
		{Type: EventTrialLandingViewed, SessionID: sessionID1},
		{Type: EventTrialFormStarted, SessionID: sessionID1},
		{Type: EventTrialLandingViewed, SessionID: sessionID2},
		{Type: EventTrialFormStarted, SessionID: sessionID2},
	}

	for _, e := range events {
		if err := handler.Handle(ctx, e); err != nil {
			t.Fatalf("Handle() failed: %v", err)
		}
	}

	session1Events := handler.GetEventsBySession(sessionID1)
	if len(session1Events) != 2 {
		t.Fatalf("GetEventsBySession() returned %d events, want 2", len(session1Events))
	}

	for _, e := range session1Events {
		if e.SessionID != sessionID1 {
			t.Errorf("GetEventsBySession() returned event with wrong session ID: %q", e.SessionID)
		}
	}

	session2Events := handler.GetEventsBySession(sessionID2)
	if len(session2Events) != 2 {
		t.Fatalf("GetEventsBySession() returned %d events, want 2", len(session2Events))
	}
}

func TestMemoryHandler_Clear(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	sessionID, _ := GenerateSessionID()

	event := Event{
		Type:      EventTrialLandingViewed,
		SessionID: sessionID,
	}

	handler.Handle(ctx, event)

	if handler.Count() != 1 {
		t.Fatalf("Count() = %d, want 1 before clear", handler.Count())
	}

	handler.Clear()

	if handler.Count() != 0 {
		t.Errorf("Count() = %d, want 0 after clear", handler.Count())
	}
}

func TestMemoryHandler_Count(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	sessionID, _ := GenerateSessionID()

	if handler.Count() != 0 {
		t.Errorf("Count() = %d, want 0 initially", handler.Count())
	}

	for i := 0; i < 5; i++ {
		handler.Handle(ctx, Event{
			Type:      EventTrialLandingViewed,
			SessionID: sessionID,
		})
	}

	if handler.Count() != 5 {
		t.Errorf("Count() = %d, want 5 after adding events", handler.Count())
	}
}

func TestMemoryHandler_CountByType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	sessionID, _ := GenerateSessionID()

	handler.Handle(ctx, Event{Type: EventTrialLandingViewed, SessionID: sessionID})
	handler.Handle(ctx, Event{Type: EventTrialFormStarted, SessionID: sessionID})
	handler.Handle(ctx, Event{Type: EventTrialFormStarted, SessionID: sessionID})
	handler.Handle(ctx, Event{Type: EventTrialEmailSubmitted, SessionID: sessionID})

	if handler.CountByType(EventTrialLandingViewed) != 1 {
		t.Errorf("CountByType(LandingViewed) = %d, want 1", handler.CountByType(EventTrialLandingViewed))
	}

	if handler.CountByType(EventTrialFormStarted) != 2 {
		t.Errorf("CountByType(FormStarted) = %d, want 2", handler.CountByType(EventTrialFormStarted))
	}

	if handler.CountByType(EventTrialEmailSubmitted) != 1 {
		t.Errorf("CountByType(EmailSubmitted) = %d, want 1", handler.CountByType(EventTrialEmailSubmitted))
	}

	if handler.CountByType(EventTrialMeshActive) != 0 {
		t.Errorf("CountByType(MeshActive) = %d, want 0", handler.CountByType(EventTrialMeshActive))
	}
}

func TestMemoryHandler_Concurrency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handler := NewMemoryHandler()

	// Test concurrent writes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			sessionID, _ := GenerateSessionID()
			handler.Handle(ctx, Event{
				Type:      EventTrialLandingViewed,
				SessionID: sessionID,
			})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if handler.Count() != 10 {
		t.Errorf("Count() after concurrent writes = %d, want 10", handler.Count())
	}
}
