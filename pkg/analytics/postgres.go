package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PostgresHandler stores analytics events in PostgreSQL
type PostgresHandler struct {
	db *sql.DB
}

// NewPostgresHandler creates a new PostgreSQL event handler
func NewPostgresHandler(db *sql.DB) *PostgresHandler {
	return &PostgresHandler{db: db}
}

// Handle stores an event in the database
func (h *PostgresHandler) Handle(ctx context.Context, event Event) error {
	// Serialize metadata and error to JSON
	var metadataJSON []byte
	var errJSON []byte
	var err = error(nil)

	if event.Metadata != (EventMetadata{}) {
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	if event.Error != nil {
		errJSON, err = json.Marshal(event.Error)
		if err != nil {
			return fmt.Errorf("failed to marshal error: %w", err)
		}
	}

	query := `
		INSERT INTO trial_signup_events (
			event_id, event_type, timestamp, session_id, user_id,
			metadata, error_data
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (event_id) DO NOTHING
	`

	_, err = h.db.ExecContext(
		ctx,
		query,
		event.ID,
		string(event.Type),
		event.Timestamp,
		event.SessionID,
		nullString(event.UserID),
		nullBytes(metadataJSON),
		nullBytes(errJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// EnsureTable creates the trial_signup_events table if it doesn't exist
func (h *PostgresHandler) EnsureTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS trial_signup_events (
			event_id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL CHECK (event_type IN (
				'trial_landing_viewed',
				'trial_form_started',
				'trial_email_submitted',
				'trial_email_verified',
				'trial_account_created',
				'trial_install_started',
				'trial_install_completed',
				'trial_mesh_active'
			)),
			timestamp TIMESTAMPTZ NOT NULL,
			session_id TEXT NOT NULL,
			user_id TEXT,
			metadata JSONB,
			error_data JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_trial_events_session_id ON trial_signup_events(session_id);
		CREATE INDEX IF NOT EXISTS idx_trial_events_user_id ON trial_signup_events(user_id);
		CREATE INDEX IF NOT EXISTS idx_trial_events_timestamp ON trial_signup_events(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_trial_events_type_timestamp ON trial_signup_events(event_type, timestamp DESC);
	`

	_, err := h.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// GetEventsBySession retrieves all events for a given session
func (h *PostgresHandler) GetEventsBySession(ctx context.Context, sessionID string) ([]Event, error) {
	query := `
		SELECT event_id, event_type, timestamp, session_id, user_id, metadata, error_data
		FROM trial_signup_events
		WHERE session_id = $1
		ORDER BY timestamp ASC
	`

	rows, err := h.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var metadataJSON, errJSON []byte
		var userID sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.Type,
			&event.Timestamp,
			&event.SessionID,
			&userID,
			&metadataJSON,
			&errJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event.UserID = userID.String

		if metadataJSON != nil {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		if errJSON != nil {
			if err := json.Unmarshal(errJSON, &event.Error); err != nil {
				return nil, fmt.Errorf("failed to unmarshal error: %w", err)
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// GetFunnelMetrics returns funnel metrics for a date range
func (h *PostgresHandler) GetFunnelMetrics(ctx context.Context, startDate, endDate time.Time) ([]FunnelStageMetrics, error) {
	query := `
		WITH funnel_stages AS (
			SELECT
				event_type,
				COUNT(DISTINCT session_id) as unique_sessions
			FROM trial_signup_events
			WHERE timestamp >= $1 AND timestamp <= $2
			GROUP BY event_type
		)
		SELECT
			fs.event_type,
			fs.unique_sessions,
			LAG(fs.unique_sessions, 1, 0) OVER (ORDER BY 
				CASE fs.event_type
					WHEN 'trial_landing_viewed' THEN 1
					WHEN 'trial_form_started' THEN 2
					WHEN 'trial_email_submitted' THEN 3
					WHEN 'trial_email_verified' THEN 4
					WHEN 'trial_account_created' THEN 5
					WHEN 'trial_install_started' THEN 6
					WHEN 'trial_install_completed' THEN 7
					WHEN 'trial_mesh_active' THEN 8
				END
			) as previous_stage
		FROM funnel_stages fs
		ORDER BY 
			CASE fs.event_type
				WHEN 'trial_landing_viewed' THEN 1
				WHEN 'trial_form_started' THEN 2
				WHEN 'trial_email_submitted' THEN 3
				WHEN 'trial_email_verified' THEN 4
				WHEN 'trial_account_created' THEN 5
				WHEN 'trial_install_started' THEN 6
				WHEN 'trial_install_completed' THEN 7
				WHEN 'trial_mesh_active' THEN 8
			END
	`

	rows, err := h.db.QueryContext(ctx, query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnel metrics: %w", err)
	}
	defer rows.Close()

	var metrics []FunnelStageMetrics
	for rows.Next() {
		var m FunnelStageMetrics
		var previousStage sql.NullInt64

		err := rows.Scan(&m.EventType, &m.UniqueSessions, &previousStage)
		if err != nil {
			return nil, fmt.Errorf("failed to scan funnel metrics: %w", err)
		}

		m.EventType = EventType(m.EventType)
		if previousStage.Valid && previousStage.Int64 > 0 {
			m.ConversionRate = float64(m.UniqueSessions) / float64(previousStage.Int64) * 100
		}

		metrics = append(metrics, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return metrics, nil
}

// FunnelStageMetrics represents metrics for a single funnel stage
type FunnelStageMetrics struct {
	EventType      EventType
	UniqueSessions int64
	PreviousStage  int64
	ConversionRate float64
}

// CleanupOldEvents removes events older than the retention period
func (h *PostgresHandler) CleanupOldEvents(ctx context.Context, retentionDays int) (int64, error) {
	query := `
		DELETE FROM trial_signup_events
		WHERE timestamp < NOW() - INTERVAL '1 day' * $1
	`

	result, err := h.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old events: %w", err)
	}

	return result.RowsAffected()
}

// nullString converts a string to sql.NullString
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullBytes converts []byte to sql.NullString for JSON storage
func nullBytes(b []byte) []byte {
	if b == nil {
		return []byte("{}")
	}
	return b
}
