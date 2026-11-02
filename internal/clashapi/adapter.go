package clashapi

import (
	"context"
	"fmt"
)

// Adapter provides a unified interface to interact with different kernel Clash APIs
// It abstracts away the differences between sing-box and mihomo implementations
type Adapter struct {
	client *Client
}

// NewAdapter creates a new Clash API adapter
func NewAdapter(baseURL, secret string, kernelType KernelType) *Adapter {
	return &Adapter{
		client: NewClient(baseURL, secret, kernelType),
	}
}

// GetVersion returns the kernel version information
// Both sing-box and mihomo support this endpoint with identical response format
func (a *Adapter) GetVersion(ctx context.Context) (*Version, error) {
	return a.client.GetVersion(ctx)
}

// GetProxies returns all configured proxies
// Note: mihomo returns additional fields (alive, id, dialer-proxy, etc.)
// The adapter preserves all fields for compatibility
func (a *Adapter) GetProxies(ctx context.Context) (*ProxiesResponse, error) {
	return a.client.GetProxies(ctx)
}

// GetProxy returns a specific proxy by name
func (a *Adapter) GetProxy(ctx context.Context, name string) (*Proxy, error) {
	proxies, err := a.GetProxies(ctx)
	if err != nil {
		return nil, err
	}

	proxy, ok := proxies.Proxies[name]
	if !ok {
		return nil, fmt.Errorf("proxy %s not found", name)
	}

	return &proxy, nil
}

// GetConnections returns active connections
// Note: mihomo includes a 'memory' field that sing-box does not provide
func (a *Adapter) GetConnections(ctx context.Context) (*ConnectionsResponse, error) {
	return a.client.GetConnections(ctx)
}

// ReadTraffic reads one sample from the kernel's continuous traffic stream.
func (a *Adapter) ReadTraffic(ctx context.Context) (*TrafficData, error) {
	return a.client.ReadTraffic(ctx)
}

// TestProxyDelay tests the latency of a specific proxy
// Both kernels support this but may return different error formats
func (a *Adapter) TestProxyDelay(ctx context.Context, proxyName string, testURL string, timeoutMs int) (*DelayTestResponse, error) {
	if testURL == "" {
		testURL = "http://www.gstatic.com/generate_204"
	}
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}

	return a.client.TestDelay(ctx, proxyName, testURL, timeoutMs)
}

// Close releases resources used by the adapter
func (a *Adapter) Close() error {
	return a.client.Close()
}

// GetKernelType returns the kernel type this adapter is configured for
func (a *Adapter) GetKernelType() KernelType {
	return a.client.kernelType
}
