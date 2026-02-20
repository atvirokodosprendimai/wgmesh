// Package proxy implements a simple reverse proxy that routes incoming HTTP
// requests to origin servers based on the Host header.
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// Proxy routes HTTP requests to origin servers based on the Host header.
type Proxy struct {
	origins   map[string]*url.URL
	transport http.RoundTripper
}

// New creates a Proxy from a map of domain→origin URL strings.
// Each value must be a valid URL (e.g. "http://10.0.0.2:3000").
// Returns an error if any origin URL cannot be parsed or the map is empty.
func New(origins map[string]string) (*Proxy, error) {
	if len(origins) == 0 {
		return nil, fmt.Errorf("proxy: origins map must not be empty")
	}
	parsed := make(map[string]*url.URL, len(origins))
	for domain, raw := range origins {
		if raw == "" {
			return nil, fmt.Errorf("proxy: empty URL for domain %q", domain)
		}
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("proxy: invalid URL for domain %q: %w", domain, err)
		}
		parsed[domain] = u
	}
	return &Proxy{
		origins: parsed,
		transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
	}, nil
}

// ServeHTTP satisfies http.Handler. It strips the port from the Host header,
// looks up the origin, and reverse-proxies the request. Returns 502 if the
// host is unknown or the origin is unreachable.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	target, ok := p.origins[host]
	if !ok {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = p.transport
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
	}

	r.Header.Set("X-Forwarded-Host", r.Host)
	rp.ServeHTTP(w, r)
}
