package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxy_ServeHTTP(t *testing.T) {
	// Start a fake origin server that returns 200.
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	tests := []struct {
		name       string
		origins    map[string]string
		host       string
		wantStatus int
	}{
		{
			name:       "known host proxies to origin and returns 200",
			origins:    map[string]string{"example.com": origin.URL},
			host:       "example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown host returns 502",
			origins:    map[string]string{"example.com": origin.URL},
			host:       "unknown.com",
			wantStatus: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.origins)
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "http://"+tt.host+"/", nil)
			req.Host = tt.host
			rr := httptest.NewRecorder()

			p.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestNew_EmptyOriginsReturnsError(t *testing.T) {
	_, err := New(map[string]string{})
	if err == nil {
		t.Fatal("New(empty map) expected error, got nil")
	}
}

func TestNew_InvalidURLReturnsError(t *testing.T) {
	_, err := New(map[string]string{"example.com": ""})
	if err == nil {
		t.Fatal("New(empty URL) expected error, got nil")
	}
}
