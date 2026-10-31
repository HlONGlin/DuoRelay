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

	config := map[string]any{
		"log": map[string]any{
			"level": opts.LogLevel,
		},
		"dns": g.generateDNS(opts),
		"inbounds": g.generateInbounds(opts),
		"outbounds": g.generateOutbounds(validNodes, opts),
		"route": g.generateRoute(opts),
	}

	return json.MarshalIndent(config, "", "  ")
}

func (g *Generator) generateDNS(opts *generator.GenerateOptions) map[string]any {
	servers := make([]map[string]any, 0, len(opts.DNSServers))
	for _, server := range opts.DNSServers {
		servers = append(servers, map[string]any{
			"address": server,
		})
	}

	dns := map[string]any{
		"servers": servers,
	}

	if opts.DNSFakeIP {
		dns["fakeip"] = map[string]any{
			"enabled": true,
		}
	}

	return dns
}

func (g *Generator) generateInbounds(opts *generator.GenerateOptions) []map[string]any {
	inbounds := []map[string]any{}

	if opts.EnableTUN {
		inbounds = append(inbounds, map[string]any{
			"type": "tun",
			"tag":  "tun-in",
			"inet4_address": "172.19.0.1/30",
			"auto_route": true,
			"stack": opts.TUNStack,
		})
	} else {
		inbounds = append(inbounds, map[string]any{
			"type": "mixed",
			"tag":  "mixed-in",
			"listen": opts.InboundHost,
			"listen_port": opts.InboundPort,
		})
	}

	return inbounds
}

func (g *Generator) generateOutbounds(nodes []*ir.Node, opts *generator.GenerateOptions) []map[string]any {
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

	// TODO: Convert IR nodes to sing-box outbound format
	// This will be implemented in P1-10
	for _, node := range nodes {
		outbound := g.convertNode(node)
		if outbound != nil {
			outbounds = append(outbounds, outbound)
		}
	}

	return outbounds
}

func (g *Generator) generateRoute(opts *generator.GenerateOptions) map[string]any {
	return map[string]any{
		"rules": []map[string]any{
			{
				"inbound": []string{"tun-in", "mixed-in"},
				"outbound": opts.DefaultOutbound,
			},
		},
		"final": opts.DefaultOutbound,
	}
}

func (g *Generator) convertNode(node *ir.Node) map[string]any {
	// Placeholder - will be implemented for each protocol in P1-10
	return map[string]any{
		"type": node.Protocol,
		"tag":  node.Name,
		"server": node.Server.Host,
		"server_port": node.Server.Port,
	}
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
