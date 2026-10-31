package mihomo

import (
	"fmt"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

// Generator implements the mihomo config generator
type Generator struct{}

// NewGenerator creates a new mihomo generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates mihomo YAML config
func (g *Generator) Generate(nodes []*ir.Node, opts *generator.GenerateOptions) ([]byte, error) {
	if opts == nil {
		opts = generator.DefaultGenerateOptions()
	}

	// Filter nodes that can run on mihomo
	var validNodes []*ir.Node
	for _, node := range nodes {
		if node.CanRunOn(ir.KernelMihomo) {
			validNodes = append(validNodes, node)
		}
	}

	config := map[string]any{
		"mixed-port": opts.MixedPort,
		"allow-lan":  false,
		"mode":       "rule",
		"log-level":  opts.LogLevel,
		"dns":        g.generateDNS(opts),
		"proxies":    g.generateProxies(validNodes),
		"proxy-groups": g.generateProxyGroups(validNodes),
		"rules":      g.generateRules(opts),
	}

	return yaml.Marshal(config)
}

func (g *Generator) generateDNS(opts *generator.GenerateOptions) map[string]any {
	nameservers := make([]string, len(opts.DNSServers))
	copy(nameservers, opts.DNSServers)

	dns := map[string]any{
		"enable":      true,
		"nameserver":  nameservers,
		"enhanced-mode": "normal",
	}

	if opts.DNSFakeIP {
		dns["enhanced-mode"] = "fake-ip"
	}

	return dns
}

func (g *Generator) generateProxies(nodes []*ir.Node) []map[string]any {
	proxies := make([]map[string]any, 0, len(nodes))

	// TODO: Convert IR nodes to mihomo proxy format
	// This will be implemented in P1-11
	for _, node := range nodes {
		proxy := g.convertNode(node)
		if proxy != nil {
			proxies = append(proxies, proxy)
		}
	}

	return proxies
}

func (g *Generator) generateProxyGroups(nodes []*ir.Node) []map[string]any {
	if len(nodes) == 0 {
		return []map[string]any{}
	}

	proxyNames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		proxyNames = append(proxyNames, node.Name)
	}

	return []map[string]any{
		{
			"name":    "PROXY",
			"type":    "select",
			"proxies": append([]string{"DIRECT"}, proxyNames...),
		},
	}
}

func (g *Generator) generateRules(opts *generator.GenerateOptions) []string {
	return []string{
		"MATCH," + opts.DefaultOutbound,
	}
}

func (g *Generator) convertNode(node *ir.Node) map[string]any {
	// Placeholder - will be implemented for each protocol in P1-11
	return map[string]any{
		"name":   node.Name,
		"type":   node.Protocol,
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}
}

// Validate validates mihomo config by calling mihomo -t
func (g *Generator) Validate(config []byte) error {
	// TODO: Implement in P1-12 - call `mihomo -t`
	return fmt.Errorf("validation not implemented yet")
}

// Kernel returns the kernel type
func (g *Generator) Kernel() ir.KernelType {
	return ir.KernelMihomo
}
