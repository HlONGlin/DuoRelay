package mihomo

import (
	"fmt"
	"strings"

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
		"mixed-port":   opts.MixedPort,
		"allow-lan":    false,
		"mode":         "rule",
		"log-level":    opts.LogLevel,
		"dns":          g.generateDNS(opts),
		"proxies":      g.generateProxies(validNodes),
		"proxy-groups": g.generateProxyGroups(validNodes),
		"rules":        g.generateRules(opts),
	}

	return yaml.Marshal(config)
}

func (g *Generator) generateDNS(opts *generator.GenerateOptions) map[string]any {
	nameservers := make([]string, len(opts.DNSServers))
	copy(nameservers, opts.DNSServers)

	dns := map[string]any{
		"enable":     true,
		"nameserver": nameservers,
	}

	if opts.DNSFakeIP {
		dns["enhanced-mode"] = "fake-ip"
	}

	return dns
}

func (g *Generator) generateProxies(nodes []*ir.Node) []map[string]any {
	proxies := make([]map[string]any, 0, len(nodes))

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
	switch node.Protocol {
	case ir.ProtocolShadowsocks:
		return g.convertShadowsocks(node)
	case ir.ProtocolVMess:
		return g.convertVMess(node)
	default:
		// Fallback for unsupported protocols
		return map[string]any{
			"name":   node.Name,
			"type":   node.Protocol,
			"server": node.Server.Host,
			"port":   node.Server.Port,
		}
	}
}

func (g *Generator) convertShadowsocks(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "ss",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}

	// Extract protocol-specific fields
	if method, ok := node.Proto["method"].(string); ok {
		proxy["cipher"] = method
	}
	if password, ok := node.Proto["password"].(string); ok {
		proxy["password"] = password
	}

	// UDP support
	if udp, ok := node.Proto["udp"].(bool); ok {
		proxy["udp"] = udp
	}

	// Plugin support
	if plugin, ok := node.Proto["plugin"].(string); ok && plugin != "" {
		proxy["plugin"] = mihomoPluginName(plugin)
		if pluginOpts, ok := node.Proto["plugin_opts"].(string); ok {
			if opts := parsePluginOpts(plugin, pluginOpts); len(opts) > 0 {
				proxy["plugin-opts"] = opts
			}
		}
	}

	return proxy
}

func (g *Generator) convertVMess(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "vmess",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}

	// Extract protocol-specific fields
	if uuid, ok := node.Proto["uuid"].(string); ok {
		proxy["uuid"] = uuid
	}
	if alterID, ok := node.Proto["alter_id"].(int); ok {
		proxy["alterId"] = alterID
	} else if alterID, ok := node.Proto["alter_id"].(float64); ok {
		proxy["alterId"] = int(alterID)
	}
	if security, ok := node.Proto["security"].(string); ok {
		proxy["cipher"] = security
	}

	// Transport layer
	network := node.Transport.Type
	if network == "" {
		network = "tcp"
	}
	proxy["network"] = network

	switch network {
	case "ws":
		wsOpts := map[string]any{}
		if node.Transport.Path != "" {
			wsOpts["path"] = node.Transport.Path
		}
		if node.Transport.Host != "" {
			wsOpts["headers"] = map[string]string{
				"Host": node.Transport.Host,
			}
		}
		if maxEarlyData, ok := node.Transport.Opts["max_early_data"].(int); ok {
			wsOpts["max-early-data"] = maxEarlyData
		} else if maxEarlyData, ok := node.Transport.Opts["max_early_data"].(float64); ok {
			wsOpts["max-early-data"] = int(maxEarlyData)
		}
		if earlyDataHeader, ok := node.Transport.Opts["early_data_header_name"].(string); ok {
			wsOpts["early-data-header-name"] = earlyDataHeader
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}

	case "grpc":
		if node.Transport.Path != "" {
			proxy["grpc-opts"] = map[string]any{
				"grpc-service-name": node.Transport.Path,
			}
		}
	}

	// Security/TLS layer
	if node.Security.Type == "tls" {
		proxy["tls"] = true
		if node.Security.SNI != "" {
			proxy["servername"] = node.Security.SNI
		}
		if len(node.Security.ALPN) > 0 {
			proxy["alpn"] = node.Security.ALPN
		}
		if node.Security.Fingerprint != "" {
			proxy["client-fingerprint"] = node.Security.Fingerprint
		}
	}

	// UDP support (default true for VMess)
	proxy["udp"] = true

	return proxy
}

// mihomoPluginName maps IR plugin names to mihomo plugin names
func mihomoPluginName(plugin string) string {
	switch plugin {
	case "obfs-local", "simple-obfs", "obfs":
		return "obfs"
	case "v2ray-plugin", "shadow-tls", "shadow-tls-lite":
		return plugin
	default:
		return plugin
	}
}

// parsePluginOpts converts the IR plugin_opts string ("key=value;key=value")
// into mihomo plugin-opts, translating SIP003 keys for known plugins.
func parsePluginOpts(plugin, pluginOpts string) map[string]any {
	opts := map[string]any{}
	for _, part := range strings.Split(pluginOpts, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			continue
		}
		key, value := kv[0], kv[1]

		switch plugin {
		case "obfs-local", "simple-obfs", "obfs":
			switch key {
			case "obfs":
				opts["mode"] = value
			case "obfs-host":
				opts["host"] = value
			}
		case "v2ray-plugin":
			opts[key] = pluginOptValue(value)
		default:
			opts[key] = pluginOptValue(value)
		}
	}
	return opts
}

// pluginOptValue converts boolean-like option values to bool.
func pluginOptValue(value string) any {
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		return value
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
