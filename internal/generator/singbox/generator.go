package singbox

import (
	"encoding/json"
	"fmt"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
)

// Generator implements the sing-box config generator
type Generator struct{}

// NewGenerator creates a new sing-box generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates sing-box JSON config
func (g *Generator) Generate(nodes []*ir.Node, opts *generator.GenerateOptions) ([]byte, error) {
	if opts == nil {
		opts = generator.DefaultGenerateOptions()
	}

	// Filter nodes that can run on sing-box
	var validNodes []*ir.Node
	for _, node := range nodes {
		if node.CanRunOn(ir.KernelSingBox) {
			validNodes = append(validNodes, node)
		}
	}

	dns, resolver := g.generateDNS(opts)

	config := map[string]any{
		"log": map[string]any{
			"level": opts.LogLevel,
		},
		"dns":       dns,
		"inbounds":  g.generateInbounds(opts),
		"outbounds": g.generateOutbounds(validNodes),
		"route":     g.generateRoute(opts, resolver),
	}

	return json.MarshalIndent(config, "", "  ")
}

// generateDNS builds the DNS config using the sing-box 1.12+ server format.
// It returns the config and the tag of the default domain resolver.
func (g *Generator) generateDNS(opts *generator.GenerateOptions) (map[string]any, string) {
	servers := make([]map[string]any, 0, len(opts.DNSServers)+1)
	rules := make([]map[string]any, 0, 1)
	resolver := ""

	for i, server := range opts.DNSServers {
		tag := fmt.Sprintf("dns-%d", i)
		servers = append(servers, map[string]any{
			"type":   "udp",
			"server": server,
			"tag":    tag,
		})
		if i == 0 {
			resolver = tag
		}
	}

	// sing-box 1.14 requires a domain resolver for outbound domain addresses.
	if len(servers) == 0 {
		resolver = "dns-default"
		servers = append(servers, map[string]any{
			"type": "local",
			"tag":  resolver,
		})
	}

	if opts.DNSFakeIP {
		servers = append(servers, map[string]any{
			"type":        "fakeip",
			"tag":         "dns-fakeip",
			"inet4_range": "198.18.0.0/15",
			"inet6_range": "fc00::/18",
		})
		rules = append(rules, map[string]any{
			"query_type": []string{"A", "AAAA"},
			"server":     "dns-fakeip",
		})
	}

	dns := map[string]any{
		"servers": servers,
	}
	if len(rules) > 0 {
		dns["rules"] = rules
	}

	return dns, resolver
}

func (g *Generator) generateInbounds(opts *generator.GenerateOptions) []map[string]any {
	inbounds := []map[string]any{}

	if opts.EnableTUN {
		inbounds = append(inbounds, map[string]any{
			"type":       "tun",
			"tag":        "tun-in",
			"address":    []string{"172.19.0.1/30"},
			"auto_route": true,
			"stack":      opts.TUNStack,
		})
	} else {
		inbounds = append(inbounds, map[string]any{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      opts.InboundHost,
			"listen_port": opts.InboundPort,
		})
	}

	return inbounds
}

func (g *Generator) generateOutbounds(nodes []*ir.Node) []map[string]any {
	outbounds := []map[string]any{
		{
			"type": "direct",
			"tag":  "DIRECT",
		},
		{
			"type": "block",
			"tag":  "REJECT",
		},
	}

	for _, node := range nodes {
		outbound := g.convertNode(node)
		if outbound != nil {
			outbounds = append(outbounds, outbound)
		}
	}

	return outbounds
}

func (g *Generator) generateRoute(opts *generator.GenerateOptions, resolver string) map[string]any {
	route := map[string]any{
		"rules": []map[string]any{
			{
				"inbound":  []string{"tun-in", "mixed-in"},
				"outbound": opts.DefaultOutbound,
			},
		},
		"final": opts.DefaultOutbound,
	}

	if resolver != "" {
		route["default_domain_resolver"] = resolver
	}

	return route
}

func (g *Generator) convertNode(node *ir.Node) map[string]any {
	switch node.Protocol {
	case ir.ProtocolShadowsocks:
		return g.convertShadowsocks(node)
	case ir.ProtocolVMess:
		return g.convertVMess(node)
	default:
		// Fallback for unsupported protocols
		return map[string]any{
			"type":        node.Protocol,
			"tag":         node.Name,
			"server":      node.Server.Host,
			"server_port": node.Server.Port,
		}
	}
}

func (g *Generator) convertShadowsocks(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "shadowsocks",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}

	// Extract protocol-specific fields
	if method, ok := node.Proto["method"].(string); ok {
		outbound["method"] = method
	}
	if password, ok := node.Proto["password"].(string); ok {
		outbound["password"] = password
	}

	// Plugin support (legacy sing-box plugin fields are still accepted)
	if plugin, ok := node.Proto["plugin"].(string); ok {
		outbound["plugin"] = plugin
		if pluginOpts, ok := node.Proto["plugin_opts"].(string); ok {
			outbound["plugin_opts"] = pluginOpts
		}
	}

	return outbound
}

func (g *Generator) convertVMess(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "vmess",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}

	// Extract protocol-specific fields
	if uuid, ok := node.Proto["uuid"].(string); ok {
		outbound["uuid"] = uuid
	}
	if alterID, ok := node.Proto["alter_id"].(int); ok {
		outbound["alter_id"] = alterID
	} else if alterID, ok := node.Proto["alter_id"].(float64); ok {
		outbound["alter_id"] = int(alterID)
	}
	if security, ok := node.Proto["security"].(string); ok {
		outbound["security"] = security
	}
	if packetEncoding, ok := node.Proto["packet_encoding"].(string); ok {
		outbound["packet_encoding"] = packetEncoding
	}

	// Transport layer
	if node.Transport.Type != "" && node.Transport.Type != "tcp" {
		transport := map[string]any{
			"type": node.Transport.Type,
		}

		switch node.Transport.Type {
		case "ws":
			if node.Transport.Path != "" {
				transport["path"] = node.Transport.Path
			}
			if node.Transport.Host != "" {
				transport["headers"] = map[string]any{
					"Host": node.Transport.Host,
				}
			}
			if maxEarlyData, ok := node.Transport.Opts["max_early_data"].(int); ok {
				transport["max_early_data"] = maxEarlyData
			} else if maxEarlyData, ok := node.Transport.Opts["max_early_data"].(float64); ok {
				transport["max_early_data"] = int(maxEarlyData)
			}
			if earlyDataHeader, ok := node.Transport.Opts["early_data_header_name"].(string); ok {
				transport["early_data_header_name"] = earlyDataHeader
			}

		case "grpc":
			if node.Transport.Path != "" {
				transport["service_name"] = node.Transport.Path
			}
			if idleTimeout, ok := node.Transport.Opts["idle_timeout"].(int); ok {
				transport["idle_timeout"] = fmt.Sprintf("%ds", idleTimeout)
			} else if idleTimeout, ok := node.Transport.Opts["idle_timeout"].(float64); ok {
				transport["idle_timeout"] = fmt.Sprintf("%ds", int(idleTimeout))
			}
			if permitWithoutStream, ok := node.Transport.Opts["permit_without_stream"].(bool); ok {
				transport["permit_without_stream"] = permitWithoutStream
			}
		}

		outbound["transport"] = transport
	}

	// Security/TLS layer
	if node.Security.Type == "tls" {
		tls := map[string]any{
			"enabled": true,
		}
		if node.Security.SNI != "" {
			tls["server_name"] = node.Security.SNI
		}
		if len(node.Security.ALPN) > 0 {
			tls["alpn"] = node.Security.ALPN
		}
		if node.Security.Fingerprint != "" {
			tls["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": node.Security.Fingerprint,
			}
		}
		outbound["tls"] = tls
	}

	return outbound
}

// Validate validates sing-box config by calling sing-box check
func (g *Generator) Validate(config []byte) error {
	// TODO: Implement in P1-12 - call `sing-box check`
	return fmt.Errorf("validation not implemented yet")
}

// Kernel returns the kernel type
func (g *Generator) Kernel() ir.KernelType {
	return ir.KernelSingBox
}
