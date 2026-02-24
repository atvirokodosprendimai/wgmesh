package main

// Phase 4 — deploy status ring buffer.
//
// POST /api/deploy/events  — called by CI after each deploy; requires Bearer token.
// GET  /api/deploy/status  — returns last event + aggregate success rate.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const deployRingSize = 50

// deployEvent is one CI deploy outcome written via POST /api/deploy/events.
type deployEvent struct {
	SHA       string    `json:"sha"`
	Slot      string    `json:"slot"`
	Outcome   string    `json:"outcome"`
	Timestamp time.Time `json:"timestamp"`
	DurationS float64   `json:"duration_s"`
}

var (
	deployRing     [deployRingSize]deployEvent
	deployRingHead int  // next write position
	deployRingFull bool // true once all slots have been written at least once
	deployRingMu   sync.Mutex
)

// handleDeployEvents accepts a JSON deploy event from CI and appends it to
// the ring buffer. Requires Authorization: Bearer $DEPLOY_TOKEN.
func handleDeployEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := os.Getenv("DEPLOY_TOKEN")
	if token == "" {
		http.Error(w, "DEPLOY_TOKEN not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var ev deployEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	deployRingMu.Lock()
	deployRing[deployRingHead] = ev
	deployRingHead = (deployRingHead + 1) % deployRingSize
	if !deployRingFull && deployRingHead == 0 {
		deployRingFull = true
	}
	deployRingMu.Unlock()

	mDeployEvents.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", ev.Outcome)))
	slog.InfoContext(ctx, "deploy event recorded",
		"sha", ev.SHA, "slot", ev.Slot, "outcome", ev.Outcome, "duration_s", ev.DurationS)

	w.WriteHeader(http.StatusOK)
}

// handleDeployStatus returns the last deploy event plus an aggregate success
// rate over all ring entries. Returns 204 if no events have been received yet.
func handleDeployStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	deployRingMu.Lock()
	head := deployRingHead
	full := deployRingFull
	ring := deployRing // copy under lock
	deployRingMu.Unlock()

	count := head
	if full {
		count = deployRingSize
	}
	if count == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Most recent write is at (head-1) mod ring size.
	lastIdx := (head - 1 + deployRingSize) % deployRingSize
	last := ring[lastIdx]

	// Success rate over all valid entries (not ordered by time, order irrelevant for rate).
	var successes int
	for i := 0; i < count; i++ {
		if ring[i].Outcome == "success" {
			successes++
		}
	}
	successRate := float64(successes) / float64(count) * 100

	resp := map[string]interface{}{
		"sha":              last.SHA,
		"slot":             last.Slot,
		"outcome":          last.Outcome,
		"timestamp":        last.Timestamp,
		"duration_s":       last.DurationS,
		"age_s":            time.Since(last.Timestamp).Seconds(),
		"success_rate_pct": successRate,
		"total_events":     count,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(ctx, "writing /api/deploy/status", "error", err)
	}
}
