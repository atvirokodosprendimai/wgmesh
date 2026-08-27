package analytics

import (
	"context"
	"fmt"
	"log"
)

// LogHandler logs events to standard output for debugging
type LogHandler struct {
	prefix string
	logger *log.Logger
}

// NewLogHandler creates a new logging event handler
func NewLogHandler(prefix string) *LogHandler {
	return &LogHandler{
		prefix: prefix,
		logger: log.Default(),
	}
}

// Handle logs an event
func (h *LogHandler) Handle(ctx context.Context, event Event) error {
	msg := fmt.Sprintf("[%s] Event: %s | Session: %s | User: %s | Time: %s",
		h.prefix,
		event.Type,
		event.SessionID,
		event.UserID,
		event.Timestamp.Format("2006-01-02T15:04:05Z"),
	)

	if event.Error != nil {
		msg += fmt.Sprintf(" | ERROR: type=%s code=%s msg=%s",
			event.Error.ErrorType,
			event.Error.ErrorCode,
			event.Error.UserMessage,
		)
	}

	h.logger.Println(msg)
	return nil
}
