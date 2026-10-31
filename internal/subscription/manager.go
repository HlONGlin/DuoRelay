package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/holi/duorelay/internal/ir"
)

// Subscription represents a subscription
type Subscription struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"` // Encrypted in storage
	UserAgent string    `json:"user_agent"`
	Interval  int       `json:"interval"` // Update interval in hours

	LastUpdate time.Time `json:"last_update"`
	LastStatus string    `json:"last_status"`

	// Usage info from subscription headers
	Used   int64     `json:"used"`
	Total  int64     `json:"total"`
	Expire time.Time `json:"expire"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager manages subscriptions
type Manager struct {
	// TODO: Add dependencies (store, parser, etc.)
}

// NewManager creates a new subscription manager
func NewManager() *Manager {
	return &Manager{}
}

// Add adds a new subscription
func (m *Manager) Add(ctx context.Context, sub *Subscription) error {
	// TODO: Implement in P1-08
	return fmt.Errorf("not implemented")
}

// Update updates a subscription (fetch and parse)
func (m *Manager) Update(ctx context.Context, id string) ([]*ir.Node, error) {
	// TODO: Implement in P1-08
	// 1. Fetch subscription URL
	// 2. Detect format (Base64 / Clash YAML / sing-box JSON)
	// 3. Parse nodes
	// 4. Extract usage info from headers
	// 5. Update database
	return nil, fmt.Errorf("not implemented")
}

// ParseFormat detects subscription format
func ParseFormat(content []byte) (string, error) {
	// TODO: Implement format detection
	// - Base64: try decode, check for share links
	// - YAML: yaml.Unmarshal to check for 'proxies' key
	// - JSON: json.Unmarshal to check for 'outbounds' key
	return "", fmt.Errorf("not implemented")
}

// ExtractUsageInfo extracts usage from headers
func ExtractUsageInfo(headers map[string][]string) (used, total int64, expire time.Time) {
	// TODO: Implement header parsing
	// Common headers:
	// - subscription-userinfo: upload=...; download=...; total=...; expire=...
	// - profile-update-interval: ...
	return 0, 0, time.Time{}
}
