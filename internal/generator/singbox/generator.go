//go:build !lite

package singbox

import (
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"

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
	if err := generator.ValidateRoutingRules(opts.RoutingRules); err != nil {
		return nil, err
	}
	reservedPorts := generator.ReservedSOCKSPorts(opts)
	if err := generator.ValidateSOCKSListeners(opts.SOCKSListeners, reservedPorts...); err != nil {
		return nil, err
	}
	var err error
	nodes, err = generator.PrepareComposition(nodes, opts.Composition, ir.KernelSingBox)
	if err != nil {
		return nil, err
	}
	if opts.SOCKSOnly && (opts.SOCKSPort < 1 || opts.SOCKSPort > 65535 || opts.SOCKSPort == opts.InboundPort) {
		return nil, fmt.Errorf("SOCKS5 端口无效或与混合端口冲突")
	}

	// Filter nodes that can run on sing-box
	var validNodes []*ir.Node
	for _, node := range nodes {
		if node.CanRunOn(ir.KernelSingBox) {
			validNodes = append(validNodes, node)
		}
	}
	var extraNodes []*ir.Node
	for _, node := range opts.ExtraNodes {
		if node == nil || !node.CanRunOn(ir.KernelSingBox) {
			return nil, fmt.Errorf("SOCKS5 端口目标节点不支持 sing-box")
		}
		extraNodes = append(extraNodes, node)
	}
	runtimeNodes := generator.PrepareRuntimeNames(append(append([]*ir.Node{}, validNodes...), extraNodes...))
	validNodes = runtimeNodes[:len(validNodes)]
	extraNodes = runtimeNodes[len(validNodes):]
	listeners := append([]generator.SOCKSListener(nil), opts.SOCKSListeners...)
	for i := range listeners {
		for j, original := range opts.ExtraNodes {
			if listeners[i].Outbound == original.Name && j < len(extraNodes) {
				listeners[i].Outbound = extraNodes[j].Name
				break
			}
		}
	}

	dns, resolver, err := g.generateDNS(opts)
	if err != nil {
		return nil, err
	}

	// Separate endpoints from outbounds
	outbounds, endpoints, err := g.generateOutboundsAndEndpoints(validNodes)
	if err != nil {
		return nil, err
	}
	if c := opts.Composition; c != nil && c.Mode == "chain" {
		byTag := map[string]map[string]any{}
		for _, outbound := range append(append([]map[string]any{}, outbounds...), endpoints...) {
			byTag[outbound["tag"].(string)] = outbound
		}
		for i := 1; i < len(validNodes); i++ {
			if byTag[validNodes[i].Name] == nil {
				return nil, fmt.Errorf("链式代理节点未生成：%s", validNodes[i].Name)
			}
			byTag[validNodes[i].Name]["detour"] = validNodes[i-1].Name
		}
		last := validNodes[len(validNodes)-1].Name
		outbounds = append(outbounds, map[string]any{"type": "selector", "tag": "PROXY", "outbounds": []string{last}, "default": last})
	} else if usesProxyOutbound(opts) {
		proxyOutbounds := make([]string, 0, len(outbounds))
		for _, outbound := range outbounds {
			tag, _ := outbound["tag"].(string)
			if tag != "" && tag != "DIRECT" && tag != "REJECT" {
				proxyOutbounds = append(proxyOutbounds, tag)
			}
		}
		for _, endpoint := range endpoints {
			proxyOutbounds = append(proxyOutbounds, endpoint["tag"].(string))
		}
		if len(proxyOutbounds) == 0 {
			return nil, fmt.Errorf("PROXY outbound requires at least one compatible node")
		}
		outbounds = append(outbounds, map[string]any{
			"type":      "selector",
			"tag":       "PROXY",
			"outbounds": proxyOutbounds,
			"default":   proxyOutbounds[0],
		})
	}
	if len(extraNodes) > 0 {
		extraOutbounds, extraEndpoints, err := g.generateOutboundsAndEndpoints(extraNodes)
		if err != nil {
			return nil, err
		}
		// This helper includes the built-in tags, already emitted above.
		outbounds = append(outbounds, extraOutbounds[2:]...)
		endpoints = append(endpoints, extraEndpoints...)
	}
	available := make(map[string]struct{}, len(outbounds))
	for _, outbound := range outbounds {
		if tag, ok := outbound["tag"].(string); ok && tag != "" {
			available[tag] = struct{}{}
		}
	}
	for _, endpoint := range endpoints {
		available[endpoint["tag"].(string)] = struct{}{}
	}
	if err := generator.ValidateOutboundReferences(opts.DefaultOutbound, opts.RoutingRules, available); err != nil {
		return nil, err
	}
	if err := generator.ValidateSOCKSListenerOutbounds(listeners, available); err != nil {
		return nil, err
	}
	// Proxy server names use the bootstrap resolver through the route default;
	// user destinations on DIRECT must use the configured business DNS instead.
	// DNS transports use their own direct dialer so this cannot recurse.
	outbounds[0]["domain_resolver"] = dns["final"]
	config := map[string]any{
		"log": map[string]any{
			"level": opts.LogLevel,
		},
		"dns":      dns,
		"inbounds": g.generateInbounds(opts),
		"route":    g.generateRouteWithListeners(opts, resolver, listeners),
	}
	config["outbounds"] = outbounds
	if len(endpoints) > 0 {
		config["endpoints"] = endpoints
	}
	if opts.Composition != nil && opts.Composition.Mode == "chain" {
		plan, _ := generator.PlanChain(validNodes, ir.KernelSingBox, true)
		if !plan.UDP {
			generator.RestrictChainUDP(config, ir.KernelSingBox)
		}
	}

	return json.MarshalIndent(config, "", "  ")
}

func (g *Generator) generateRouteWithListeners(opts *generator.GenerateOptions, resolver string, listeners []generator.SOCKSListener) map[string]any {
	copy := *opts
	copy.SOCKSListeners = listeners
	return g.generateRoute(&copy, resolver)
}

func usesProxyOutbound(opts *generator.GenerateOptions) bool {
	if opts.DNSDetour == "PROXY" {
		return true
	}
	if opts.DefaultOutbound == "PROXY" {
		return true
	}
	for _, rule := range opts.RoutingRules {
		if !rule.Enabled && rule.ID != "" {
			continue
		}
		if rule.Outbound == "PROXY" {
			return true
		}
	}
	return false
}

// generateDNS builds the DNS config using the sing-box 1.12+ server format.
// It returns the config and the tag of the default domain resolver.
func (g *Generator) generateDNS(opts *generator.GenerateOptions) (map[string]any, string, error) {
	servers := make([]map[string]any, 0, len(opts.DNSServers)+1)
	rules := make([]map[string]any, 0, 1)
	resolver := "dns-bootstrap"
	bootstrap := generator.DNSBootstrap(opts)
	if net.ParseIP(bootstrap) == nil {
		return nil, "", fmt.Errorf("DNS bootstrap must be an IP address")
	}
	// An omitted detour uses the DNS transport's direct dialer. sing-box rejects
	// an explicit detour to a direct outbound without dialer options at startup.
	servers = append(servers, map[string]any{"type": "udp", "tag": resolver, "server": bootstrap})

	for i, server := range opts.DNSServers {
		address, err := generator.ParseDNSAddress(server)
		if err != nil {
			return nil, "", fmt.Errorf("DNS server %d: %w", i+1, err)
		}
		tag := fmt.Sprintf("dns-%d", i)
		serverConfig := map[string]any{
			"type":        address.Type,
			"server":      address.Host,
			"server_port": address.Port,
			"tag":         tag,
		}
		if detour := generator.DNSDetour(opts); detour != "DIRECT" {
			serverConfig["detour"] = detour
		}
		if address.Path != "" {
			serverConfig["path"] = address.Path
		}
		if net.ParseIP(address.Host) == nil {
			serverConfig["domain_resolver"] = resolver
		}
		servers = append(servers, serverConfig)
	}

	// sing-box 1.14 requires a domain resolver for outbound domain addresses.

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
	if len(opts.DNSServers) > 0 {
		dns["final"] = "dns-0"
	} else {
		dns["final"] = resolver
	}
	if len(rules) > 0 {
		dns["rules"] = rules
	}

	return dns, resolver, nil
}

func (g *Generator) generateInbounds(opts *generator.GenerateOptions) []map[string]any {
	inbounds := []map[string]any{}

	mode := opts.TransparentMode
	if opts.EnableTUN {
		mode = "tun"
	}
	if mode == "tun" {
		tun := map[string]any{
			"type":         "tun",
			"tag":          "tun-in",
			"address":      []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
			"auto_route":   true,
			"strict_route": true,
			"dns_mode":     "hijack",
			"stack":        opts.TUNStack,
		}
		if runtime.GOOS == "linux" {
			tun["auto_redirect"] = true
		}
		inbounds = append(inbounds, tun)
	} else if mode == "redirect" || mode == "tproxy" {
		port := opts.RedirectPort
		if mode == "tproxy" {
			port = opts.TProxyPort
		}
		inbounds = append(inbounds, map[string]any{
			"type": mode, "tag": mode + "-in", "listen": "127.0.0.1", "listen_port": port,
		})
	}
	if mode == "off" || opts.SOCKSOnly || opts.SystemProxy {
		inbounds = append(inbounds, map[string]any{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      opts.InboundHost,
			"listen_port": opts.InboundPort,
		})
	}
	if opts.SOCKSOnly {
		inbounds = append(inbounds, map[string]any{"type": "socks", "tag": "socks-in", "listen": opts.InboundHost, "listen_port": opts.SOCKSPort})
	}
	for _, listener := range opts.SOCKSListeners {
		inbounds = append(inbounds, map[string]any{"type": "socks", "tag": generator.SOCKSListenerTag(listener.Port), "listen": opts.InboundHost, "listen_port": listener.Port})
	}

	return inbounds
}

// generateOutboundsAndEndpoints separates nodes into outbounds and endpoints.
// OpenConnect and OpenVPN nodes go to the endpoints array; all others go to outbounds.
// OpenVPN nodes that cannot be represented faithfully return an error.
func (g *Generator) generateOutboundsAndEndpoints(nodes []*ir.Node) ([]map[string]any, []map[string]any, error) {
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
	var endpoints []map[string]any

	for _, node := range nodes {
		// OpenConnect and OpenVPN are endpoints, not outbounds
		if err := ir.ValidateConnectionFeatures(node); err != nil {
			return nil, nil, err
		}
		if node.Protocol == ir.ProtocolOpenConnect {
			endpoint := g.convertOpenConnectEndpoint(node)
			if endpoint != nil {
				endpoints = append(endpoints, endpoint)
			}
		} else if node.Protocol == ir.ProtocolOpenVPN {
			endpoint, err := g.convertOpenVPNEndpoint(node)
			if err != nil {
				return nil, nil, err
			}
			endpoints = append(endpoints, endpoint)
		} else if node.Protocol == ir.ProtocolWireGuard {
			endpoint := g.convertWireGuardEndpoint(node)
			if endpoint == nil {
				return nil, nil, fmt.Errorf("sing-box generator does not support incomplete protocol %q", node.Protocol)
			}
			endpoints = append(endpoints, endpoint)
		} else if node.Protocol == ir.ProtocolTailscale {
			endpoint := g.convertTailscaleEndpoint(node)
			if endpoint == nil {
				return nil, nil, fmt.Errorf("sing-box generator does not support incomplete protocol %q", node.Protocol)
			}
			endpoints = append(endpoints, endpoint)
		} else {
			outbound := g.convertNode(node)
			if outbound == nil {
				return nil, nil, fmt.Errorf("sing-box generator does not support protocol %q", node.Protocol)
			}
			if node.Transport.Type == "ws" {
				if transport, ok := outbound["transport"].(map[string]any); ok {
					if headers := generator.WebSocketHeaders(node); len(headers) > 0 {
						transport["headers"] = headers
					}
				}
			}
			outbounds = append(outbounds, outbound)
		}
	}

	return outbounds, endpoints, nil
}

func (g *Generator) generateRoute(opts *generator.GenerateOptions, resolver string) map[string]any {
	rules := make([]map[string]any, 0, len(opts.RoutingRules)+1)
	mode := opts.TransparentMode
	if opts.EnableTUN {
		mode = "tun"
	}
	if mode == "tun" {
		rules = append(rules, map[string]any{"inbound": []string{"tun-in"}, "action": "sniff"})
		rules = append(rules, map[string]any{"protocol": []string{"dns"}, "action": "hijack-dns"})
	}
	for _, listener := range opts.SOCKSListeners {
		rules = append(rules, map[string]any{"inbound": []string{generator.SOCKSListenerTag(listener.Port)}, "outbound": listener.Outbound})
	}
	if opts.SOCKSOnly {
		allowed := []string{"socks-in"}
		for _, listener := range opts.SOCKSListeners {
			allowed = append(allowed, generator.SOCKSListenerTag(listener.Port))
		}
		rules = append(rules, map[string]any{"inbound": allowed, "invert": true, "outbound": "DIRECT"})
	}
	ruleSets := make([]map[string]any, 0)
	for _, rule := range opts.RoutingRules {
		if !rule.Enabled && rule.ID != "" {
			continue
		}
		if rule.Match == "port" {
			for _, value := range rule.Values {
				port, err := strconv.ParseUint(strings.TrimSpace(value), 10, 16)
				if err != nil || port == 0 {
					continue
				}
				rules = append(rules, map[string]any{"port": uint16(port), "outbound": rule.Outbound})
			}
			continue
		}
		if rule.Match == "geosite" || rule.Match == "geoip" {
			tags := make([]string, 0, len(rule.Values))
			for _, value := range rule.Values {
				tag := strings.TrimSpace(value)
				if !strings.Contains(tag, "-") {
					tag = rule.Match + "-" + tag
				}
				repository := "sing-geosite"
				if rule.Match == "geoip" {
					repository = "sing-geoip"
				}
				tags = append(tags, tag)
				ruleSets = append(ruleSets, map[string]any{
					"tag":             tag,
					"type":            "remote",
					"format":          "binary",
					"url":             "https://raw.githubusercontent.com/SagerNet/" + repository + "/rule-set/" + tag + ".srs",
					"download_detour": "DIRECT",
				})
			}
			rules = append(rules, map[string]any{"rule_set": tags, "outbound": rule.Outbound})
			continue
		}
		rules = append(rules, map[string]any{
			rule.Match: rule.Values,
			"outbound": rule.Outbound,
		})
	}
	catchInbounds := []string{"tun-in", "mixed-in"}
	if mode == "redirect" || mode == "tproxy" {
		catchInbounds = []string{mode + "-in", "mixed-in"}
	}
	if opts.SOCKSOnly {
		catchInbounds = append(catchInbounds, "socks-in")
	}
	for _, listener := range opts.SOCKSListeners {
		catchInbounds = append(catchInbounds, generator.SOCKSListenerTag(listener.Port))
	}
	rules = append(rules, map[string]any{
		"inbound":  catchInbounds,
		"outbound": opts.DefaultOutbound,
	})
	route := map[string]any{
		"rules": rules,
		"final": opts.DefaultOutbound,
	}
	if mode == "tun" {
		route["auto_detect_interface"] = true
	}
	if len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
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
	case ir.ProtocolVLESS:
		return g.convertVLESS(node)
	case ir.ProtocolTrojan:
		return g.convertTrojan(node)
	case ir.ProtocolHysteria2:
		return g.convertHysteria2(node)
	case ir.ProtocolTUIC:
		return g.convertTUIC(node)
	case ir.ProtocolHysteria:
		return g.convertHysteria(node)
	case ir.ProtocolAnyTLS:
		return g.convertAnyTLS(node)
	case ir.ProtocolSnell:
		return g.convertSnell(node)
	case ir.ProtocolSOCKS5:
		return g.convertSOCKS5(node)
	case ir.ProtocolHTTP:
		return g.convertHTTP(node)
	case ir.ProtocolSSH:
		return g.convertSSH(node)
	case ir.ProtocolShadowTLS:
		return g.convertShadowTLS(node)
	case ir.ProtocolNaive:
		return g.convertNaive(node)
	case ir.ProtocolTor:
		return g.convertTor(node)
	case ir.ProtocolOpenConnect, ir.ProtocolOpenVPN, ir.ProtocolWireGuard:
		// Endpoint protocols must not be emitted as outbounds
		// It's handled separately as an endpoint
		return nil
	default:
		return nil
	}
}

func (g *Generator) convertWireGuardEndpoint(node *ir.Node) map[string]any {
	privateKey, privateOK := node.Proto["private_key"].(string)
	publicKey, publicOK := node.Proto["public_key"].(string)
	addresses := protocolStringSlice(node.Proto["address"])
	if len(addresses) == 0 {
		addresses = protocolStringSlice(node.Proto["local_address"])
	}
	allowedIPs := protocolStringSlice(node.Proto["allowed_ips"])
	if !privateOK || privateKey == "" || !publicOK || publicKey == "" || len(addresses) == 0 || len(allowedIPs) == 0 {
		return nil
	}
	peer := map[string]any{
		"address":     node.Server.Host,
		"port":        node.Server.Port,
		"public_key":  publicKey,
		"allowed_ips": allowedIPs,
	}
	if preSharedKey, ok := node.Proto["preshared_key"].(string); ok && preSharedKey != "" {
		peer["pre_shared_key"] = preSharedKey
	}
	endpoint := map[string]any{
		"type":        "wireguard",
		"tag":         node.Name,
		"address":     addresses,
		"private_key": privateKey,
		"peers":       []map[string]any{peer},
	}
	if mtu, ok := protocolInt(node.Proto["mtu"]); ok && mtu > 0 {
		endpoint["mtu"] = mtu
	}
	return endpoint
}

func (g *Generator) convertTailscaleEndpoint(node *ir.Node) map[string]any {
	endpoint := map[string]any{
		"type": "tailscale",
		"tag":  node.Name,
	}
	copyTailscaleOptions(endpoint, node.Proto)
	return endpoint
}

func copyTailscaleOptions(target map[string]any, source map[string]any) {
	for sourceKey, targetKey := range map[string]string{
		"state_directory":               "state_directory",
		"auth_key":                      "auth_key",
		"control_url":                   "control_url",
		"hostname":                      "hostname",
		"exit_node":                     "exit_node",
		"exit_node_allow_lan_access":    "exit_node_allow_lan_access",
		"accept_dns":                    "accept_dns",
		"accept_routes":                 "accept_routes",
		"advertise_exit_node":           "advertise_exit_node",
		"advertise_routes":              "advertise_routes",
		"advertise_tags":                "advertise_tags",
		"ephemeral":                     "ephemeral",
		"login_server":                  "login_server",
		"listen_port":                   "listen_port",
		"udp":                           "udp",
		"system_interface":              "system_interface",
		"system_interface_name":         "system_interface_name",
		"system_interface_mtu":          "system_interface_mtu",
		"relay_server_port":             "relay_server_port",
		"relay_server_static_endpoints": "relay_server_static_endpoints",
		"taildrop_directory":            "taildrop_directory",
	} {
		if value, ok := source[sourceKey]; ok {
			target[targetKey] = value
		}
	}
}

func protocolStringSlice(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := value.(string)
			if !ok || text == "" {
				return nil
			}
			result = append(result, text)
		}
		return result
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

func protocolInt(value any) (int, bool) {
	switch value := value.(type) {
	case int:
		return value, true
	case float64:
		return int(value), value == float64(int(value))
	}
	return 0, false
}

func (g *Generator) convertSnell(node *ir.Node) map[string]any {
	version, err := strconv.Atoi(fmt.Sprint(node.Proto["version"]))
	if err != nil || (version != 3 && version != 4 && version != 6) {
		return nil
	}
	if version == 3 {
		// sing-box 1.14.4 exposes the effective v3/v4 wire format as v4.
		version = 4
	}
	psk, ok := node.Proto["psk"].(string)
	if !ok || psk == "" {
		return nil
	}
	outbound := map[string]any{
		"type":        "snell",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
		"version":     version,
		"psk":         psk,
	}
	if obfs, ok := node.Proto["obfs"].(string); ok && obfs != "" {
		if version != 4 || (obfs != "none" && obfs != "http") {
			return nil
		}
		outbound["obfs_mode"] = obfs
	}
	return outbound
}

func (g *Generator) convertSOCKS5(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "socks",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
		"version":     "5",
	}
	if username, ok := node.Proto["username"].(string); ok && username != "" {
		outbound["username"] = username
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	return outbound
}

func (g *Generator) convertHTTP(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "http",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if username, ok := node.Proto["username"].(string); ok && username != "" {
		outbound["username"] = username
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	if node.Security.Type == "tls" {
		tls := map[string]any{"enabled": true}
		if node.Security.SNI != "" {
			tls["server_name"] = node.Security.SNI
		}
		if node.Security.Opts != nil {
			if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
				tls["insecure"] = insecure
			}
		}
		outbound["tls"] = tls
	}
	return outbound
}

func (g *Generator) convertSSH(node *ir.Node) map[string]any {
	username, ok := node.Proto["username"].(string)
	if !ok || username == "" {
		return nil
	}
	outbound := map[string]any{
		"type":        "ssh",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
		"user":        username,
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	if privateKey, ok := node.Proto["private_key"].(string); ok && privateKey != "" {
		if strings.Contains(privateKey, "PRIVATE KEY") {
			outbound["private_key"] = privateKey
		} else {
			outbound["private_key_path"] = privateKey
		}
	}
	return outbound
}

func (g *Generator) convertShadowTLS(node *ir.Node) map[string]any {
	password, passwordOK := node.Proto["password"].(string)
	versionText := fmt.Sprint(node.Proto["version"])
	version, err := strconv.Atoi(versionText)
	if !passwordOK || password == "" || err != nil || version < 1 || version > 3 {
		return nil
	}
	outbound := map[string]any{
		"type":        "shadowtls",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
		"version":     version,
		"password":    password,
	}
	tls := map[string]any{"enabled": true}
	if node.Security.SNI != "" {
		tls["server_name"] = node.Security.SNI
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			tls["insecure"] = insecure
		}
	}
	outbound["tls"] = tls
	return outbound
}

func (g *Generator) convertNaive(node *ir.Node) map[string]any {
	username, usernameOK := node.Proto["username"].(string)
	password, passwordOK := node.Proto["password"].(string)
	if !usernameOK || username == "" || !passwordOK || password == "" {
		return nil
	}
	outbound := map[string]any{
		"type":        "naive",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
		"username":    username,
		"password":    password,
	}
	if value, ok := node.Proto["insecure_concurrency"]; ok {
		if number, valid := protocolInt(value); valid && number >= 0 {
			outbound["insecure_concurrency"] = number
		}
	}
	if node.Transport.Type == "quic" {
		outbound["quic"] = true
	}
	if congestion, ok := node.Proto["quic_congestion_control"].(string); ok && congestion != "" {
		outbound["quic_congestion_control"] = congestion
	}
	tls := map[string]any{"enabled": true}
	if node.Security.SNI != "" {
		tls["server_name"] = node.Security.SNI
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			tls["insecure"] = insecure
		}
	}
	outbound["tls"] = tls
	return outbound
}

func (g *Generator) convertTor(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "socks",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
		"version":     "5",
	}
	if username, ok := node.Proto["username"].(string); ok && username != "" {
		outbound["username"] = username
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	return outbound
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
		if fingerprint := realityFingerprint(node); fingerprint != "" {
			tls["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": fingerprint,
			}
		}
		outbound["tls"] = tls
	}

	return outbound
}

func (g *Generator) convertVLESS(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "vless",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if uuid, ok := node.Proto["uuid"].(string); ok && uuid != "" {
		outbound["uuid"] = uuid
	}
	if flow, ok := node.Proto["flow"].(string); ok && flow != "" {
		outbound["flow"] = flow
	}

	if node.Transport.Type != "" && node.Transport.Type != "tcp" {
		transport := map[string]any{"type": node.Transport.Type}
		switch node.Transport.Type {
		case "ws":
			if node.Transport.Path != "" {
				transport["path"] = node.Transport.Path
			}
			if node.Transport.Host != "" {
				transport["headers"] = map[string]any{"Host": node.Transport.Host}
			}
		case "grpc":
			if node.Transport.Path != "" {
				transport["service_name"] = node.Transport.Path
			}
		case "xhttp":
			return nil
		}
		outbound["transport"] = transport
	}

	if node.Security.Type == "tls" || node.Security.Type == "reality" {
		tls := map[string]any{"enabled": true}
		if node.Security.SNI != "" {
			tls["server_name"] = node.Security.SNI
		}
		if len(node.Security.ALPN) > 0 {
			tls["alpn"] = node.Security.ALPN
		}
		fingerprint := node.Security.Fingerprint
		if node.Security.Type == "reality" && fingerprint == "" {
			fingerprint = "chrome"
		}
		if fingerprint != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
		}
		if node.Security.Type == "reality" {
			reality := map[string]any{"enabled": true}
			if publicKey, ok := node.Security.Opts["public_key"].(string); ok && publicKey != "" {
				reality["public_key"] = publicKey
			}
			if shortID, ok := node.Security.Opts["short_id"].(string); ok && shortID != "" {
				reality["short_id"] = shortID
			}
			tls["reality"] = reality
		}
		outbound["tls"] = tls
	}

	return outbound
}

func (g *Generator) convertTrojan(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "trojan",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	if node.Transport.Type != "" && node.Transport.Type != "tcp" {
		transport := map[string]any{"type": node.Transport.Type}
		switch node.Transport.Type {
		case "ws":
			if node.Transport.Path != "" {
				transport["path"] = node.Transport.Path
			}
			if node.Transport.Host != "" {
				transport["headers"] = map[string]any{"Host": node.Transport.Host}
			}
		case "grpc":
			if node.Transport.Path != "" {
				transport["service_name"] = node.Transport.Path
			}
		}
		outbound["transport"] = transport
	}
	if node.Security.Type == "tls" || node.Security.Type == "reality" {
		tls := map[string]any{"enabled": true}
		if node.Security.SNI != "" {
			tls["server_name"] = node.Security.SNI
		}
		if len(node.Security.ALPN) > 0 {
			tls["alpn"] = node.Security.ALPN
		}
		if fingerprint := realityFingerprint(node); fingerprint != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
		}
		outbound["tls"] = tls
	}
	return outbound
}

func (g *Generator) convertHysteria2(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "hysteria2",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	if obfs, ok := node.Proto["obfs"].(string); ok && obfs != "" {
		obfuscation := map[string]any{"type": obfs}
		if password, ok := node.Proto["obfs_password"].(string); ok && password != "" {
			obfuscation["password"] = password
		}
		outbound["obfs"] = obfuscation
	}
	tls := map[string]any{"enabled": true}
	if node.Security.SNI != "" {
		tls["server_name"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		tls["alpn"] = node.Security.ALPN
	}
	if fingerprint := realityFingerprint(node); fingerprint != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			tls["insecure"] = insecure
		}
	}
	outbound["tls"] = tls
	return outbound
}

func (g *Generator) convertTUIC(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "tuic",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if uuid, ok := node.Proto["uuid"].(string); ok && uuid != "" {
		outbound["uuid"] = uuid
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	if congestion, ok := node.Proto["congestion_control"].(string); ok && congestion != "" {
		outbound["congestion_control"] = congestion
	}
	if relay, ok := node.Proto["udp_relay_mode"].(string); ok && relay != "" {
		outbound["udp_relay_mode"] = relay
	}
	tls := map[string]any{"enabled": true}
	if node.Security.SNI != "" {
		tls["server_name"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		tls["alpn"] = node.Security.ALPN
	}
	if fingerprint := realityFingerprint(node); fingerprint != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			tls["insecure"] = insecure
		}
	}
	outbound["tls"] = tls
	return outbound
}

func (g *Generator) convertHysteria(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "hysteria",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if auth, ok := node.Proto["auth"].(string); ok && auth != "" {
		outbound["auth_str"] = auth
	}
	if up, ok := node.Proto["up_mbps"].(string); ok && up != "" {
		value, err := strconv.Atoi(up)
		if err != nil || value < 0 {
			return nil
		}
		outbound["up_mbps"] = value
	}
	if down, ok := node.Proto["down_mbps"].(string); ok && down != "" {
		value, err := strconv.Atoi(down)
		if err != nil || value < 0 {
			return nil
		}
		outbound["down_mbps"] = value
	}
	tls := map[string]any{"enabled": true}
	if node.Security.SNI != "" {
		tls["server_name"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		tls["alpn"] = node.Security.ALPN
	}
	if fingerprint := realityFingerprint(node); fingerprint != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
	}
	outbound["tls"] = tls
	return outbound
}

func (g *Generator) convertAnyTLS(node *ir.Node) map[string]any {
	outbound := map[string]any{
		"type":        "anytls",
		"tag":         node.Name,
		"server":      node.Server.Host,
		"server_port": node.Server.Port,
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		outbound["password"] = password
	}
	tls := map[string]any{"enabled": true}
	if node.Security.SNI != "" {
		tls["server_name"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		tls["alpn"] = node.Security.ALPN
	}
	if fingerprint := realityFingerprint(node); fingerprint != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fingerprint}
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			tls["insecure"] = insecure
		}
	}
	outbound["tls"] = tls
	return outbound
}

// convertOpenConnectEndpoint converts an OpenConnect node to an endpoint structure.
// Endpoints are serialized under root "endpoints" array, not "outbounds".
func (g *Generator) convertOpenConnectEndpoint(node *ir.Node) map[string]any {
	// Construct the server URL from Host and Port
	serverURL := fmt.Sprintf("https://%s:%d", node.Server.Host, node.Server.Port)

	endpoint := map[string]any{
		"type":   "openconnect",
		"tag":    node.Name,
		"server": serverURL,
	}

	// Copy protocol-specific fields from Proto
	for _, key := range []string{"flavor", "username", "password", "auth_group", "cookie", "token",
		"reported_os", "user_agent", "version", "local_hostname", "system", "name",
		"mobile", "csd", "hip", "tncc", "mtu", "dpd_interval", "reconnect_timeout"} {
		if value, ok := node.Proto[key]; ok {
			endpoint[key] = value
		}
	}

	// Build TLS configuration if present
	if node.Security.Type == "tls" {
		tls := map[string]any{}

		if node.Security.SNI != "" {
			tls["server_name"] = node.Security.SNI
		}

		if node.Security.Opts != nil {
			if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
				tls["insecure"] = insecure
			}
		}

		// Only add tls object if it has fields
		if len(tls) > 0 {
			endpoint["tls"] = tls
		}
	}

	return endpoint
}

func realityFingerprint(node *ir.Node) string {
	if node == nil {
		return ""
	}
	if node.Security.Fingerprint != "" {
		return node.Security.Fingerprint
	}
	if node.Security.Type == "reality" {
		return "chrome"
	}
	return ""
}

// Validate validates sing-box config by calling sing-box check
func (g *Generator) Validate(config []byte) error {
	return generator.ValidateJSONConfig(config, "log", "dns", "inbounds", "outbounds", "route")
}

// Kernel returns the kernel type
func (g *Generator) Kernel() ir.KernelType {
	return ir.KernelSingBox
}
