package generator

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// Generator defines the interface for config generators
type Generator interface {
	// Generate generates kernel config from IR nodes
	Generate(nodes []*ir.Node, opts *GenerateOptions) ([]byte, error)

	// Validate validates the generated config's syntax and required structure.
	// Runtime-specific semantic validation is performed by kernel.Manager.
	Validate(config []byte) error

	// Kernel returns the kernel type this generator targets
	Kernel() ir.KernelType
}

// GenerateOptions contains options for config generation
type GenerateOptions struct {
	// Inbound config
	InboundPort     int
	InboundHost     string
	EnableTUN       bool
	TransparentMode string
	RedirectPort    int
	TProxyPort      int
	TUNStack        string
	MixedPort       int
	SystemProxy     bool
	SOCKSOnly       bool
	SOCKSPort       int
	Composition     *Composition

	// DNS config
	DNSServers []string
	DNSFakeIP  bool

	// Routing
	RuleSets        []string
	RoutingRules    []RoutingRule
	DefaultOutbound string

	// Logging
	LogLevel string

	// Advanced
	Extra map[string]any
}

// RoutingRule is a portable rule that can be rendered by both supported kernels.
// Each rule has one matcher kind and one or more values; values are OR'ed within
// the matcher and rules are evaluated in order.
type RoutingRule struct {
	ID       string   `json:"id,omitempty"`
	Source   string   `json:"source,omitempty"`
	Enabled  bool     `json:"enabled"`
	Match    string   `json:"match"`
	Values   []string `json:"values"`
	Outbound string   `json:"outbound"`
}

var routingMatchKinds = map[string]struct{}{
	"domain":         {},
	"domain_suffix":  {},
	"domain_keyword": {},
	"ip_cidr":        {},
	"source_ip_cidr": {},
	"port":           {},
	"network":        {},
	"protocol":       {},
	"geosite":        {},
	"geoip":          {},
}

// ValidateRoutingRules validates the portable routing rule model before a
// kernel-specific generator renders it.
func ValidateRoutingRules(rules []RoutingRule) error {
	if len(rules) > 256 {
		return fmt.Errorf("too many routing rules: %d", len(rules))
	}
	for i, rule := range rules {
		if rule.Source == "" {
			rule.Source = "user"
		}
		switch rule.Source {
		case "internal", "bypass", "user":
		default:
			return fmt.Errorf("routing rule %d: unsupported source %q", i, rule.Source)
		}
		if _, ok := routingMatchKinds[rule.Match]; !ok {
			return fmt.Errorf("routing rule %d: unsupported match %q", i, rule.Match)
		}
		if len(rule.Values) == 0 || len(rule.Values) > 128 {
			return fmt.Errorf("routing rule %d: values must contain 1 to 128 items", i)
		}
		if strings.TrimSpace(rule.Outbound) == "" {
			return fmt.Errorf("routing rule %d: outbound is required", i)
		}
		for j, value := range rule.Values {
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("routing rule %d value %d is empty", i, j)
			}
			if rule.Match == "port" {
				port, err := strconv.ParseUint(value, 10, 16)
				if err != nil || port == 0 {
					return fmt.Errorf("routing rule %d value %d: port must be 1..65535", i, j)
				}
			}
		}
	}
	return nil
}

// ValidateOutboundReferences checks that the final route and every rule point
// to an outbound that the selected kernel configuration will actually create.
func ValidateOutboundReferences(defaultOutbound string, rules []RoutingRule, available map[string]struct{}) error {
	check := func(kind, value string) error {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s outbound is required", kind)
		}
		if _, ok := available[value]; !ok {
			return fmt.Errorf("%s outbound %q is not available", kind, value)
		}
		return nil
	}
	if err := check("default", defaultOutbound); err != nil {
		return err
	}
	for i, rule := range rules {
		if err := check(fmt.Sprintf("routing rule %d", i), rule.Outbound); err != nil {
			return err
		}
	}
	return nil
}

// DefaultGenerateOptions returns default generation options
func DefaultGenerateOptions() *GenerateOptions {
	return &GenerateOptions{
		InboundPort:     1080,
		InboundHost:     "127.0.0.1",
		EnableTUN:       false,
		TransparentMode: "off",
		RedirectPort:    12345,
		TProxyPort:      12346,
		TUNStack:        "system",
		MixedPort:       7890,
		DNSServers:      []string{"223.5.5.5", "8.8.8.8"},
		DNSFakeIP:       false,
		DefaultOutbound: "DIRECT",
		LogLevel:        "info",
		Extra:           make(map[string]any),
	}
}
