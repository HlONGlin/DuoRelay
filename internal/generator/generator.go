package generator

import (
	"github.com/holi/duorelay/internal/ir"
)

// Generator defines the interface for config generators
type Generator interface {
	// Generate generates kernel config from IR nodes
	Generate(nodes []*ir.Node, opts *GenerateOptions) ([]byte, error)

	// Validate validates the generated config
	Validate(config []byte) error

	// Kernel returns the kernel type this generator targets
	Kernel() ir.KernelType
}

// GenerateOptions contains options for config generation
type GenerateOptions struct {
	// Inbound config
	InboundPort      int
	InboundHost      string
	EnableTUN        bool
	TUNStack         string
	MixedPort        int

	// DNS config
	DNSServers       []string
	DNSFakeIP        bool

	// Routing
	RuleSets         []string
	DefaultOutbound  string

	// Logging
	LogLevel         string

	// Advanced
	Extra            map[string]any
}

// DefaultGenerateOptions returns default generation options
func DefaultGenerateOptions() *GenerateOptions {
	return &GenerateOptions{
		InboundPort:     1080,
		InboundHost:     "127.0.0.1",
		EnableTUN:       false,
		TUNStack:        "system",
		MixedPort:       7890,
		DNSServers:      []string{"223.5.5.5", "8.8.8.8"},
		DNSFakeIP:       false,
		DefaultOutbound: "DIRECT",
		LogLevel:        "info",
		Extra:           make(map[string]any),
	}
}
