package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxy_ServeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		makeOrigins  func(originURL string) map[string]string
		requestHost  string
		wantStatus   int
		wantBodyHint string
	}{
		{
			name:         "known host proxies to origin",
			makeOrigins:  func(u string) map[string]string { return map[string]string{"example.com": u} },
			requestHost:  "example.com",
			wantStatus:   http.StatusOK,
			wantBodyHint: "origin-ok",
		},
		{
			name:        "unknown host returns 502",
			makeOrigins: func(u string) map[string]string { return map[string]string{"example.com": u} },
			requestHost: "unknown.com",
			wantStatus:  http.StatusBadGateway,
		},
		{
			name:        "host with port strips port for lookup",
			makeOrigins: func(u string) map[string]string { return map[string]string{"example.com": u} },
			requestHost: "example.com:80",
			wantStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Each subtest owns its origin server — avoid close-before-use race.
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("origin-ok"))
			}))
			t.Cleanup(origin.Close)

			p, err := New(tt.makeOrigins(origin.URL))
			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "http://"+tt.requestHost+"/", nil)
			req.Host = tt.requestHost
			rr := httptest.NewRecorder()

			p.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
			if tt.wantBodyHint != "" && !strings.Contains(rr.Body.String(), tt.wantBodyHint) {
				t.Errorf("body = %q, want to contain %q", rr.Body.String(), tt.wantBodyHint)
			}
		})
	}
}

func TestNew_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		origins map[string]string
		wantErr bool
	}{
		{
			name:    "empty URL returns error",
			origins: map[string]string{"example.com": ""},
			wantErr: true,
		},
		{
			name:    "URL without scheme returns error",
			origins: map[string]string{"example.com": "10.0.0.1:3000"},
			wantErr: true,
		},
		{
			name:    "valid URL succeeds",
			origins: map[string]string{"example.com": "http://10.0.0.1:3000"},
			wantErr: false,
		},
		{
			name:    "empty origins map succeeds",
			origins: map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tt.origins)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
