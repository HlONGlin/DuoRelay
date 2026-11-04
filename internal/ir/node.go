package ir

import "time"

// KernelType represents the kernel type
type KernelType string

const (
	KernelSingBox KernelType = "sing-box"
	KernelMihomo  KernelType = "mihomo"
)

// KernelSet represents which kernels can run this node
type KernelSet uint8

const (
	KernelSetSingBox KernelSet = 1 << iota
	KernelSetMihomo
	KernelSetBoth = KernelSetSingBox | KernelSetMihomo
)

// Node represents the unified intermediate representation of a proxy node
type Node struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Protocol string    `json:"protocol"` // One of 28 protocols
	Owner    KernelSet `json:"owner"`    // Which kernel(s) can run this

	Server      Server         `json:"server"`
	Transport   Transport      `json:"transport"`
	Security    Security       `json:"security"`
	Proto       map[string]any `json:"proto"`  // Protocol-specific fields
	Extra       map[string]any `json:"extra"`  // Passthrough fields
	Source      Source         `json:"source"` // Origin info
	Composition *Composition   `json:"composition,omitempty"`

	// Management fields
	GroupID   string   `json:"group_id,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Favorite  bool     `json:"favorite"`
	SortOrder int      `json:"sort_order"`
	Status    string   `json:"status"` // active, error, testing, etc.

	// Test results
	LastDelayMs int       `json:"last_delay_ms"`
	LastTestAt  time.Time `json:"last_test_at,omitempty"`

	UpdatedAt time.Time `json:"updated_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Composition struct {
	Mode        string              `json:"mode"`
	Kernel      KernelType          `json:"kernel"`
	NodeIDs     []string            `json:"node_ids"`
	LoadBalance *LoadBalanceOptions `json:"load_balance,omitempty"`
}

type LoadBalanceOptions struct {
	Strategy string `json:"strategy"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	Timeout  int    `json:"timeout"`
	Lazy     bool   `json:"lazy"`
}

func (n *Node) IsComposition() bool {
	return n != nil && IsCompositionProtocol(n.Protocol)
}

// Server represents server connection info
type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Transport represents transport layer config
type Transport struct {
	Type string         `json:"type"` // tcp, ws, grpc, h2, quic, etc.
	Path string         `json:"path,omitempty"`
	Host string         `json:"host,omitempty"`
	Opts map[string]any `json:"opts,omitempty"`
}

// Security represents security/TLS config
type Security struct {
	Type        string         `json:"type"` // none, tls, reality
	SNI         string         `json:"sni,omitempty"`
	ALPN        []string       `json:"alpn,omitempty"`
	Fingerprint string         `json:"fingerprint,omitempty"`
	Opts        map[string]any `json:"opts,omitempty"`
}

// Source represents where this node came from
type Source struct {
	Type           string `json:"type"` // subscription, manual, import
	SubscriptionID string `json:"subscription_id,omitempty"`
	URL            string `json:"url,omitempty"`
}

// CanRunOn checks if this node can run on the specified kernel
func (n *Node) CanRunOn(kernel KernelType) bool {
	if n == nil {
		return false
	}
	if n.Protocol == ProtocolVLESS && n.Transport.Type == "xhttp" && kernel == KernelSingBox {
		return false
	}
	switch kernel {
	case KernelSingBox:
		return n.Owner&KernelSetSingBox != 0
	case KernelMihomo:
		return n.Owner&KernelSetMihomo != 0
	default:
		return false
	}
}

// PreferredKernel returns the preferred kernel for this node
func (n *Node) PreferredKernel() KernelType {
	if n.Owner == KernelSetBoth {
		return KernelSingBox // Default to sing-box for shared protocols
	}
	if n.Owner&KernelSetSingBox != 0 {
		return KernelSingBox
	}
	return KernelMihomo
}
