package lighthouse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client is an HTTP client for the Lighthouse REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a Lighthouse API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateSiteRequest is the payload for POST /v1/sites.
type CreateSiteRequest struct {
	Domain string `json:"domain"`
	Origin Origin `json:"origin"`
	TLS    string `json:"tls,omitempty"`
}

// CreateSite registers a new site with the Lighthouse.
func (c *Client) CreateSite(req CreateSiteRequest) (*Site, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.baseURL+"/v1/sites", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lighthouse unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var site Site
	if err := json.NewDecoder(resp.Body).Decode(&site); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &site, nil
}

// ListSites returns all sites for the authenticated org.
func (c *Client) ListSites() ([]Site, error) {
	httpReq, err := http.NewRequest("GET", c.baseURL+"/v1/sites", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lighthouse unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result struct {
		Sites []Site `json:"sites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Sites, nil
}

// DeleteSite soft-deletes a site by ID.
func (c *Client) DeleteSite(id string) error {
	httpReq, err := http.NewRequest("DELETE", c.baseURL+"/v1/sites/"+id, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("lighthouse unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return c.readError(resp)
	}

	return nil
}

// DiscoverLighthouse resolves the Lighthouse URL for a mesh.
// Discovery chain:
//  1. DNS SRV lookup: _lighthouse._tcp.<meshID>.wgmesh.dev
//  2. Fallback: https://lighthouse.<meshID>.wgmesh.dev
func DiscoverLighthouse(meshID string) (string, error) {
	_, addrs, err := net.LookupSRV("lighthouse", "tcp", meshID+".wgmesh.dev")
	if err == nil && len(addrs) > 0 {
		host := addrs[0].Target
		port := addrs[0].Port
		// SRV targets end with a dot — trim it
		if len(host) > 0 && host[len(host)-1] == '.' {
			host = host[:len(host)-1]
		}
		return fmt.Sprintf("https://%s:%d", host, port), nil
	}

	return fmt.Sprintf("https://lighthouse.%s.wgmesh.dev", meshID), nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}

func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	var problem struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(body, &problem) == nil && problem.Detail != "" {
		return fmt.Errorf("lighthouse API error (%d): %s", resp.StatusCode, problem.Detail)
	}

	if len(body) > 0 {
		return fmt.Errorf("lighthouse API error (%d): %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("lighthouse API error (%d)", resp.StatusCode)
}
