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
	SOCKSListeners  []SOCKSListener
	ExtraNodes      []*ir.Node
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

// SOCKSListener binds a local SOCKS5 listener directly to one generated outbound.
type SOCKSListener struct {
	Port     int
	Outbound string
}

const MaxSOCKSListeners = 32

func SOCKSListenerTag(port int) string { return fmt.Sprintf("duorelay-socks-%d", port) }

// ValidateSOCKSListeners checks listener limits, port collisions and outbound names.
// ReservedSOCKSPorts returns every configured local port that an extra SOCKS
// listener must not reuse.
func ReservedSOCKSPorts(opts *GenerateOptions) []int {
	if opts == nil {
		return nil
	}
	ports := []int{opts.InboundPort, opts.MixedPort}
	if opts.SOCKSOnly {
		ports = append(ports, opts.SOCKSPort)
	}
	mode := opts.TransparentMode
	if opts.EnableTUN {
		mode = "tun"
	}
	switch mode {
	case "redirect":
		ports = append(ports, opts.RedirectPort)
	case "tproxy":
		ports = append(ports, opts.TProxyPort)
	}
	return ports
}

func ValidateSOCKSListeners(listeners []SOCKSListener, reservedPorts ...int) error {
	if len(listeners) > MaxSOCKSListeners {
		return fmt.Errorf("最多可配置 %d 个额外 SOCKS5 端口", MaxSOCKSListeners)
	}
	reserved := make(map[int]struct{}, len(reservedPorts))
	for _, port := range reservedPorts {
		if port > 0 {
			reserved[port] = struct{}{}
		}
	}
	seen := make(map[int]struct{}, len(listeners))
	for _, listener := range listeners {
		if listener.Port < 1 || listener.Port > 65535 {
			return fmt.Errorf("SOCKS5 端口必须在 1..65535 之间")
		}
		if _, ok := seen[listener.Port]; ok {
			return fmt.Errorf("SOCKS5 端口 %d 重复", listener.Port)
		}
		if _, ok := reserved[listener.Port]; ok {
			return fmt.Errorf("SOCKS5 端口 %d 与现有监听端口冲突", listener.Port)
		}
		if strings.TrimSpace(listener.Outbound) == "" {
			return fmt.Errorf("SOCKS5 端口 %d 缺少目标节点", listener.Port)
		}
		seen[listener.Port] = struct{}{}
	}
	return nil
}

func ValidateSOCKSListenerOutbounds(listeners []SOCKSListener, available map[string]struct{}) error {
	for _, listener := range listeners {
		if _, ok := available[listener.Outbound]; !ok {
			return fmt.Errorf("SOCKS5 端口 %d 的目标节点 %q 未生成", listener.Port, listener.Outbound)
		}
	}
	return nil
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
