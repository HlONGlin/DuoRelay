package clashapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Clash API client
type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

// NewClient creates a new Clash API client
func NewClient(baseURL, secret string) *Client {
	return &Client{
		baseURL: baseURL,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Version represents version information
type Version struct {
	Premium bool   `json:"premium"`
	Version string `json:"version"`
	Meta    bool   `json:"meta"`
}

// GetVersion gets kernel version
func (c *Client) GetVersion(ctx context.Context) (*Version, error) {
	var ver Version
	if err := c.get(ctx, "/version", &ver); err != nil {
		return nil, err
	}
	return &ver, nil
}

// Proxy represents a proxy node
type Proxy struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	UDP     bool     `json:"udp"`
	Delay   int      `json:"delay,omitempty"`
	History []Delay  `json:"history,omitempty"`
	All     []string `json:"all,omitempty"`
	Now     string   `json:"now,omitempty"`
}

// Delay represents delay test result
type Delay struct {
	Time  time.Time `json:"time"`
	Delay int       `json:"delay"`
}

// GetProxies gets all proxies
func (c *Client) GetProxies(ctx context.Context) (map[string]*Proxy, error) {
	var result struct {
		Proxies map[string]*Proxy `json:"proxies"`
	}
	if err := c.get(ctx, "/proxies", &result); err != nil {
		return nil, err
	}
	return result.Proxies, nil
}

// TestDelay tests proxy delay
func (c *Client) TestDelay(ctx context.Context, name string, url string, timeout int) (int, error) {
	path := fmt.Sprintf("/proxies/%s/delay?url=%s&timeout=%d", name, url, timeout)
	var result struct {
		Delay int `json:"delay"`
	}
	if err := c.get(ctx, path, &result); err != nil {
		return 0, err
	}
	return result.Delay, nil
}

// Connection represents a connection
type Connection struct {
	ID          string    `json:"id"`
	Metadata    Metadata  `json:"metadata"`
	Upload      int64     `json:"upload"`
	Download    int64     `json:"download"`
	Start       time.Time `json:"start"`
	Chains      []string  `json:"chains"`
	Rule        string    `json:"rule"`
	RulePayload string    `json:"rulePayload"`
}

// Metadata represents connection metadata
type Metadata struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
	DNSMode         string `json:"dnsMode"`
	ProcessPath     string `json:"processPath"`
}

// GetConnections gets all connections
func (c *Client) GetConnections(ctx context.Context) ([]Connection, error) {
	var result struct {
		Connections []Connection `json:"connections"`
		DownloadTotal int64      `json:"downloadTotal"`
		UploadTotal   int64      `json:"uploadTotal"`
	}
	if err := c.get(ctx, "/connections", &result); err != nil {
		return nil, err
	}
	return result.Connections, nil
}

// Traffic represents traffic statistics
type Traffic struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// get performs a GET request
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// put performs a PUT request
func (c *Client) put(ctx context.Context, path string, body interface{}) error {
	// TODO: Implement for switching proxies
	return fmt.Errorf("not implemented")
}
