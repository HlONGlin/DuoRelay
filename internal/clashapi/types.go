package clashapi

import "time"

// KernelType represents the type of kernel
type KernelType string

const (
	KernelSingBox KernelType = "singbox"
	KernelMihomo  KernelType = "mihomo"
)

// Version represents the version response
type Version struct {
	Premium bool   `json:"premium"`
	Meta    bool   `json:"meta"`
	Version string `json:"version"`
}

// Proxy represents a proxy node
type Proxy struct {
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	UDP          bool                `json:"udp"`
	History      []DelayHistory      `json:"history"`
	All          []string            `json:"all,omitempty"`           // For proxy groups
	Now          string              `json:"now,omitempty"`           // For proxy groups
	Alive        bool                `json:"alive,omitempty"`         // mihomo only
	ID           string              `json:"id,omitempty"`            // mihomo only
	DialerProxy  string              `json:"dialer-proxy,omitempty"`  // mihomo only
	Extra        map[string]any      `json:"extra,omitempty"`         // mihomo only
	ProviderName string              `json:"provider-name,omitempty"` // mihomo only
}

// ProxiesResponse represents the proxies response
type ProxiesResponse struct {
	Proxies map[string]Proxy `json:"proxies"`
}

// DelayHistory represents delay test history
type DelayHistory struct {
	Time  time.Time `json:"time"`
	Delay int       `json:"delay"`
}

// DelayTestResponse represents delay test response
type DelayTestResponse struct {
	Delay   int    `json:"delay"`
	Message string `json:"message,omitempty"` // Error message if test failed
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

// ConnectionsResponse represents connections response
type ConnectionsResponse struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Connections   []Connection `json:"connections"`
	Memory        int64        `json:"memory,omitempty"` // mihomo only
}

// TrafficData represents traffic statistics
type TrafficData struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// LogMessage represents a log message
type LogMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Message string `json:"message"`
}
