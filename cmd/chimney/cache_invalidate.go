package main

// Phase 5 — runtime cache invalidation API.
//
// POST /api/cache/invalidate — requires Bearer token.
// Deletes Dragonfly keys and in-memory entries matching the given prefix.

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type invalidateRequest struct {
	Prefix string `json:"prefix"`
	All    bool   `json:"all"`
}

// handleCacheInvalidate deletes Dragonfly keys and in-memory entries that
// match the requested prefix. Requires Authorization: Bearer $INVALIDATE_TOKEN.
//
// Body: {"prefix": "/pulls", "all": false}
// Response: {"deleted": N, "dragonfly": N, "memory": N}
//
// "all": true flushes the entire cache; only allowed when
// INVALIDATE_ALL_ALLOWED=true is set in the environment.
func handleCacheInvalidate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token := os.Getenv("INVALIDATE_TOKEN")
	if token == "" {
		http.Error(w, "INVALIDATE_TOKEN not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req invalidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.All && os.Getenv("INVALIDATE_ALL_ALLOWED") != "true" {
		http.Error(w, `"all" invalidation not permitted on this instance`, http.StatusForbidden)
		return
	}

	var dfDeleted, memDeleted int

	// --- Dragonfly ---
	if useRedis.Load() {
		scanPattern := cachePrefix + req.Prefix + "*"
		var cursor uint64
		for {
			keys, nextCursor, err := rdb.Scan(ctx, cursor, scanPattern, 100).Result()
			if err != nil {
				slog.WarnContext(ctx, "cache invalidate: dragonfly scan error", "error", err)
				break
			}
			if len(keys) > 0 {
				if n, err := rdb.Del(ctx, keys...).Result(); err != nil {
					slog.WarnContext(ctx, "cache invalidate: dragonfly del error", "error", err)
				} else {
					dfDeleted += int(n)
				}
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}

	// --- In-memory ---
	memCacheMu.Lock()
	for k := range memCache {
		if req.All || strings.HasPrefix(k, req.Prefix) {
			delete(memCache, k)
			memDeleted++
		}
	}
	memCacheMu.Unlock()

	total := dfDeleted + memDeleted

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.Int("chimney.cache.invalidated_keys", total))

	slog.InfoContext(ctx, "cache invalidated",
		"prefix", req.Prefix,
		"all", req.All,
		"dragonfly_deleted", dfDeleted,
		"memory_deleted", memDeleted,
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]int{
		"deleted":  total,
		"dragonfly": dfDeleted,
		"memory":   memDeleted,
	}); err != nil {
		slog.ErrorContext(ctx, "writing cache invalidate response", "error", err)
	}
}
