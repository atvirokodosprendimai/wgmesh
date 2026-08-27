package analytics

import (
	"context"
	"sync"
)

// MemoryHandler stores events in memory for testing and development
type MemoryHandler struct {
	events []Event
	mu     sync.RWMutex
}

// NewMemoryHandler creates a new in-memory event handler
func NewMemoryHandler() *MemoryHandler {
	return &MemoryHandler{
		events: make([]Event, 0),
	}
}

// Handle stores an event in memory
func (h *MemoryHandler) Handle(ctx context.Context, event Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	return nil
}

// GetEvents returns all stored events
func (h *MemoryHandler) GetEvents() []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	events := make([]Event, len(h.events))
	copy(events, h.events)
	return events
}

// GetEventsBySession returns events for a specific session
func (h *MemoryHandler) GetEventsBySession(sessionID string) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []Event
	for _, e := range h.events {
		if e.SessionID == sessionID {
			result = append(result, e)
		}
	}
	return result
}

// Clear removes all stored events
func (h *MemoryHandler) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = make([]Event, 0)
}

// Count returns the number of stored events
func (h *MemoryHandler) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.events)
}

// CountByType returns the count of events by type
func (h *MemoryHandler) CountByType(eventType EventType) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, e := range h.events {
		if e.Type == eventType {
			count++
		}
	}
	return count
}
