// Package proxy implements an HTTP reverse proxy that routes incoming
// requests to upstream origins based on the request's Host header.
//
// Intended for use in the lighthouse/edge nodes to serve registered
// customer sites by forwarding traffic through the WireGuard mesh.
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// Proxy is an HTTP handler that routes requests to upstream origins
// based on the Host header. Unknown hosts receive a 502 response.
type Proxy struct {
	origins map[string]*url.URL
}

// New creates a Proxy from a map of domain → upstream URL strings.
// Each value must be a valid URL (e.g. "http://10.0.0.2:3000").
// Returns an error if any URL fails to parse.
func New(origins map[string]string) (*Proxy, error) {
	parsed := make(map[string]*url.URL, len(origins))
	for domain, raw := range origins {
		if raw == "" {
			return nil, fmt.Errorf("proxy: empty URL for domain %q", domain)
		}
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("proxy: invalid URL for domain %q: %w", domain, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("proxy: URL for domain %q must include scheme and host", domain)
		}
		parsed[domain] = u
	}
	return &Proxy{origins: parsed}, nil
}

// ServeHTTP implements http.Handler. It strips the port from the Host
// header, looks up the registered origin, and proxies the request.
// Returns 502 if no origin is registered for the host.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	// Strip port if present (e.g. "example.com:80" → "example.com")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(host)

	target, ok := p.origins[host]
	if !ok {
		http.Error(w, "502 Bad Gateway — no origin registered for host", http.StatusBadGateway)
		return
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
	}
	// Preserve the original Host for the upstream and set X-Forwarded-Host
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", r.Host)
		if r.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}
	rp.ServeHTTP(w, r)
}
