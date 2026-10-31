package clashapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is the HTTP client for Clash API
type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
	kernelType KernelType
}

// NewClient creates a new Clash API client
func NewClient(baseURL, secret string, kernelType KernelType) *Client {
	return &Client{
		baseURL:    baseURL,
		secret:     secret,
		kernelType: kernelType,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// doRequest performs an HTTP request with authentication
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	return resp, nil
}

// GetVersion gets the kernel version
func (c *Client) GetVersion(ctx context.Context) (*Version, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/version", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var version Version
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &version, nil
}

// GetProxies gets all proxies
func (c *Client) GetProxies(ctx context.Context) (*ProxiesResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/proxies", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var proxies ProxiesResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxies); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &proxies, nil
}

// GetConnections gets active connections
func (c *Client) GetConnections(ctx context.Context) (*ConnectionsResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/connections", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var connections ConnectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&connections); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &connections, nil
}

// TestDelay tests the delay of a proxy
func (c *Client) TestDelay(ctx context.Context, proxyName string, testURL string, timeout int) (*DelayTestResponse, error) {
	path := fmt.Sprintf("/proxies/%s/delay?timeout=%d&url=%s",
		url.PathEscape(proxyName),
		timeout,
		url.QueryEscape(testURL))

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result DelayTestResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Check if the response contains an error message
	if result.Message != "" && result.Delay == 0 {
		return nil, fmt.Errorf("delay test failed: %s", result.Message)
	}

	return &result, nil
}

// Close closes the HTTP client
func (c *Client) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}
