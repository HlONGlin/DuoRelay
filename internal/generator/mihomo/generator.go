package mihomo

import (
	"fmt"
	"runtime"
	"strconv"
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
	if err := generator.ValidateRoutingRules(opts.RoutingRules); err != nil {
		return nil, err
	}
	reservedPorts := generator.ReservedSOCKSPorts(opts)
	if err := generator.ValidateSOCKSListeners(opts.SOCKSListeners, reservedPorts...); err != nil {
		return nil, err
	}
	var err error
	nodes, err = generator.PrepareComposition(nodes, opts.Composition, ir.KernelMihomo)
	if err != nil {
		return nil, err
	}

	// Filter nodes that can run on mihomo
	var validNodes []*ir.Node
	for _, node := range nodes {
		if node.CanRunOn(ir.KernelMihomo) {
			validNodes = append(validNodes, node)
		}
	}
	runtimeNodes := append([]*ir.Node(nil), validNodes...)
	for _, node := range opts.ExtraNodes {
		if node == nil || !node.CanRunOn(ir.KernelMihomo) {
			return nil, fmt.Errorf("SOCKS5 端口目标节点不支持 mihomo")
		}
		runtimeNodes = append(runtimeNodes, node)
	}
	prepared := generator.PrepareRuntimeNames(runtimeNodes)
	listeners := append([]generator.SOCKSListener(nil), opts.SOCKSListeners...)
	for i := range listeners {
		for j, original := range opts.ExtraNodes {
			if listeners[i].Outbound == original.Name && len(validNodes)+j < len(prepared) {
				listeners[i].Outbound = prepared[len(validNodes)+j].Name
				break
			}
		}
	}
	validNodes = prepared[:len(validNodes)]
	runtimeNodes = prepared
	proxies, err := g.generateProxies(runtimeNodes)
	if err != nil {
		return nil, err
	}
	proxyGroups := g.generateProxyGroups(validNodes)
	if c := opts.Composition; c != nil {
		names := make([]string, len(validNodes))
		for i, node := range validNodes {
			names[i] = node.Name
		}
		if c.Mode == "load_balance" {
			lb := c.LoadBalance
			group := map[string]any{"name": "PROXY", "proxies": names, "url": lb.URL, "interval": lb.Interval, "timeout": lb.Timeout, "lazy": lb.Lazy}
			if lb.Strategy == "url-test" || lb.Strategy == "fallback" {
				group["type"] = lb.Strategy
			} else {
				group["type"] = "load-balance"
				group["strategy"] = lb.Strategy
			}
			proxyGroups = []map[string]any{group}
		} else {
			plan, _ := generator.PlanChain(validNodes, ir.KernelMihomo, true)
			for i, p := range proxies[:len(validNodes)] {
				if validNodes[i].Protocol != ir.ProtocolHTTP && validNodes[i].Protocol != ir.ProtocolSSH {
					p["udp"] = plan.Hops[i].UDP
				}
			}
			for i := 1; i < len(validNodes); i++ {
				proxies[i]["dialer-proxy"] = names[i-1]
			}
			proxyGroups = []map[string]any{{"name": "PROXY", "type": "select", "proxies": []string{names[len(names)-1]}}}
		}
	}
	available := map[string]struct{}{"DIRECT": {}, "REJECT": {}}
	for _, proxy := range proxies {
		if name, ok := proxy["name"].(string); ok && name != "" {
			available[name] = struct{}{}
		}
	}
	if len(proxyGroups) > 0 {
		available["PROXY"] = struct{}{}
	}
	if err := generator.ValidateOutboundReferences(opts.DefaultOutbound, opts.RoutingRules, available); err != nil {
		return nil, err
	}
	if err := generator.ValidateSOCKSListenerOutbounds(listeners, available); err != nil {
		return nil, err
	}

	config := map[string]any{
		"mixed-port":   opts.MixedPort,
		"allow-lan":    false,
		"mode":         "rule",
		"log-level":    opts.LogLevel,
		"dns":          g.generateDNS(opts),
		"proxies":      proxies,
		"proxy-groups": proxyGroups,
		"rules":        g.generateRulesWithListeners(opts, listeners),
	}
	mode := opts.TransparentMode
	if opts.EnableTUN {
		mode = "tun"
	}
	if mode == "tun" {
		config["ipv6"] = true
		tun := map[string]any{
			"enable":                true,
			"stack":                 opts.TUNStack,
			"auto-route":            true,
			"strict-route":          true,
			"auto-detect-interface": true,
			"dns-mode":              "hijack",
			"dns-hijack":            []string{"any:53", "tcp://any:53"},
			"inet6-address":         []string{"fdfe:dcba:9876::1/126"},
		}
		if runtime.GOOS == "linux" {
			tun["auto-redirect"] = true
		}
		config["tun"] = tun
	} else if mode == "redirect" {
		config["redir-port"] = opts.RedirectPort
	} else if mode == "tproxy" {
		config["tproxy-port"] = opts.TProxyPort
	}
	if opts.SOCKSOnly || len(opts.SOCKSListeners) > 0 {
		config["bind-address"] = opts.InboundHost
		if opts.SOCKSOnly && (opts.SOCKSPort < 1 || opts.SOCKSPort > 65535 || opts.SOCKSPort == opts.MixedPort) {
			return nil, fmt.Errorf("SOCKS5 端口无效或与混合端口冲突")
		}
		listeners := make([]map[string]any, 0, len(opts.SOCKSListeners)+1)
		if opts.SOCKSOnly {
			listeners = append(listeners, map[string]any{"name": "socks-in", "type": "socks", "port": opts.SOCKSPort, "listen": opts.InboundHost, "udp": true})
		}
		for _, listener := range opts.SOCKSListeners {
			listeners = append(listeners, map[string]any{"name": generator.SOCKSListenerTag(listener.Port), "type": "socks", "port": listener.Port, "listen": opts.InboundHost, "udp": true})
		}
		config["listeners"] = listeners
	}
	if opts.Composition != nil && opts.Composition.Mode == "chain" {
		plan, _ := generator.PlanChain(validNodes, ir.KernelMihomo, true)
		if !plan.UDP {
			generator.RestrictChainUDP(config, ir.KernelMihomo)
		}
	}

	return yaml.Marshal(config)
}

func (g *Generator) generateRulesWithListeners(opts *generator.GenerateOptions, listeners []generator.SOCKSListener) []string {
	copy := *opts
	copy.SOCKSListeners = listeners
	return g.generateRules(&copy)
}

func (g *Generator) generateDNS(opts *generator.GenerateOptions) map[string]any {
	nameservers := make([]string, len(opts.DNSServers))
	copy(nameservers, opts.DNSServers)
	for i, server := range nameservers {
		nameservers[i] = server + "#" + generator.DNSDetour(opts)
	}

	dns := map[string]any{
		"enable":                  true,
		"nameserver":              nameservers,
		"default-nameserver":      []string{generator.DNSBootstrap(opts)},
		"proxy-server-nameserver": []string{generator.DNSBootstrap(opts)},
	}

	if opts.DNSFakeIP {
		dns["enhanced-mode"] = "fake-ip"
	}

	return dns
}

func (g *Generator) generateProxies(nodes []*ir.Node) ([]map[string]any, error) {
	proxies := make([]map[string]any, 0, len(nodes))

	for _, node := range nodes {
		if err := ir.ValidateConnectionFeatures(node); err != nil {
			return nil, err
		}
		if node.Protocol == ir.ProtocolOpenVPN {
			proxy, err := g.convertOpenVPN(node)
			if err != nil {
				return nil, fmt.Errorf("OpenVPN node %q: %w", node.Name, err)
			}
			proxies = append(proxies, proxy)
			continue
		}
		proxy := g.convertNode(node)
		if proxy == nil {
			return nil, fmt.Errorf("mihomo generator does not support protocol %q", node.Protocol)
		}
		if node.Transport.Type == "ws" && proxy["network"] == "ws" {
			if headers := generator.WebSocketHeaders(node); len(headers) > 0 {
				opts, _ := proxy["ws-opts"].(map[string]any)
				if opts == nil {
					opts = map[string]any{}
					proxy["ws-opts"] = opts
				}
				opts["headers"] = headers
			}
		}
		proxies = append(proxies, proxy)
	}

	return proxies, nil
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
	rules := make([]string, 0, len(opts.RoutingRules)+1)
	for _, listener := range opts.SOCKSListeners {
		rules = append(rules, "IN-NAME,"+generator.SOCKSListenerTag(listener.Port)+","+listener.Outbound)
	}
	if opts.SOCKSOnly {
		// Enforce ingress scope before any user rule can select a proxy.
		rules = append(rules, "NOT,((IN-NAME,socks-in)),DIRECT")
	}
	for _, rule := range opts.RoutingRules {
		if !rule.Enabled && rule.ID != "" {
			continue
		}
		kind := map[string]string{
			"domain":         "DOMAIN",
			"domain_suffix":  "DOMAIN-SUFFIX",
			"domain_keyword": "DOMAIN-KEYWORD",
			"ip_cidr":        "IP-CIDR",
			"source_ip_cidr": "SRC-IP-CIDR",
			"port":           "DST-PORT",
			"network":        "NETWORK",
			"protocol":       "PROTOCOL",
			"geosite":        "GEOSITE",
			"geoip":          "GEOIP",
		}[rule.Match]
		for _, value := range rule.Values {
			rules = append(rules, kind+","+value+","+rule.Outbound)
		}
	}
	rules = append(rules, "MATCH,"+opts.DefaultOutbound)
	return rules
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
	case ir.ProtocolWireGuard:
		return g.convertWireGuard(node)
	case ir.ProtocolShadowsocksR:
		return g.convertShadowsocksR(node)
	case ir.ProtocolMieru:
		return g.convertMieru(node)
	case ir.ProtocolSudoku:
		return g.convertSudoku(node)
	case ir.ProtocolShadowQUIC:
		return g.convertShadowQUIC(node)
	case ir.ProtocolTrustTunnel:
		return g.convertTrustTunnel(node)
	case ir.ProtocolMASQUE:
		return g.convertMASQUE(node)
	case ir.ProtocolTailscale:
		return g.convertTailscale(node)
	case ir.ProtocolEasyTier:
		return g.convertEasyTier(node)
	case ir.ProtocolZeroTier:
		return g.convertZeroTier(node)
	case ir.ProtocolRematch:
		return g.convertRematch(node)
	default:
		return nil
	}
}

func (g *Generator) convertTailscale(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name": node.Name,
		"type": "tailscale",
	}
	copyOverlayOptions(proxy, node.Proto, map[string]string{
		"state_directory":            "state-directory",
		"auth_key":                   "auth-key",
		"control_url":                "control-url",
		"hostname":                   "hostname",
		"exit_node":                  "exit-node",
		"exit_node_allow_lan_access": "exit-node-allow-lan-access",
		"accept_dns":                 "accept-dns",
		"accept_routes":              "accept-routes",
		"advertise_exit_node":        "advertise-exit-node",
		"advertise_routes":           "advertise-routes",
		"advertise_tags":             "advertise-tags",
		"ephemeral":                  "ephemeral",
		"login_server":               "login-server",
		"listen_port":                "listen-port",
		"udp":                        "udp",
	})
	return proxy
}

func (g *Generator) convertEasyTier(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name": node.Name,
		"type": "easytier",
	}
	copyOverlayOptions(proxy, node.Proto, map[string]string{
		"network_name":         "network-name",
		"network_secret":       "network-secret",
		"hostname":             "hostname",
		"ipv4":                 "ipv4",
		"dhcp":                 "dhcp",
		"peers":                "peers",
		"listeners":            "listeners",
		"no_listener":          "no-listener",
		"mapped_listeners":     "mapped-listeners",
		"exit_nodes":           "exit-nodes",
		"proxy_networks":       "proxy-networks",
		"instance_name":        "instance-name",
		"state_dir":            "state-dir",
		"udp":                  "udp",
		"accept_dns":           "accept-dns",
		"enable_exit_node":     "enable-exit-node",
		"enable_encryption":    "enable-encryption",
		"encryption_algorithm": "encryption-algorithm",
		"private_mode":         "private-mode",
		"latency_first":        "latency-first",
		"disable_p2p":          "disable-p2p",
		"enable_kcp_proxy":     "enable-kcp-proxy",
		"disable_kcp_input":    "disable-kcp-input",
		"enable_quic_proxy":    "enable-quic-proxy",
		"disable_quic_input":   "disable-quic-input",
		"mtu":                  "mtu",
		"tld_dns_zone":         "tld-dns-zone",
		"secure_mode":          "secure-mode",
		"local_private_key":    "local-private-key",
		"local_public_key":     "local-public-key",
	})
	if _, ok := node.Proto["network_name"]; !ok {
		return nil
	}
	return proxy
}

func (g *Generator) convertZeroTier(node *ir.Node) map[string]any {
	network, ok := node.Proto["network"].(string)
	if !ok || strings.TrimSpace(network) == "" {
		return nil
	}
	proxy := map[string]any{
		"name":    node.Name,
		"type":    "zerotier",
		"network": network,
	}
	copyOverlayOptions(proxy, node.Proto, map[string]string{
		"state_dir":           "state-dir",
		"identity_secret":     "identity-secret",
		"planet":              "planet",
		"tcp_fallback_mode":   "tcp-fallback-mode",
		"tcp_fallback_relay":  "tcp-fallback-relay",
		"remote_trace_target": "remote-trace-target",
		"ip_stack":            "ip-stack",
		"udp":                 "udp",
		"remote_dns_resolve":  "remote-dns-resolve",
		"low_bandwidth":       "low-bandwidth",
		"encrypted_hello":     "encrypted-hello",
		"mtu":                 "mtu",
		"physical_mtu":        "physical-mtu",
		"primary_port":        "primary-port",
		"secondary_port":      "secondary-port",
		"remote_trace_level":  "remote-trace-level",
		"dns":                 "dns",
		"orbit":               "orbit",
	})
	return proxy
}

func (g *Generator) convertRematch(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name": node.Name,
		"type": "rematch",
	}
	hasTarget := false
	if value, ok := node.Proto["target_rematch_name"].(string); ok && strings.TrimSpace(value) != "" {
		proxy["target-rematch-name"] = value
		hasTarget = true
	}
	if value, ok := node.Proto["target_sub_rule"].(string); ok && strings.TrimSpace(value) != "" {
		proxy["target-sub-rule"] = value
		hasTarget = true
	}
	if !hasTarget {
		return nil
	}
	return proxy
}

func copyOverlayOptions(target map[string]any, source map[string]any, fields map[string]string) {
	for sourceKey, targetKey := range fields {
		if value, ok := source[sourceKey]; ok {
			target[targetKey] = value
		}
	}
}

func (g *Generator) convertSnell(node *ir.Node) map[string]any {
	version, err := strconv.Atoi(fmt.Sprint(node.Proto["version"]))
	if err != nil || (version < 1 || version > 5) {
		return nil
	}
	if version == 5 {
		// mihomo accepts v5 links but uses its backward-compatible v4 client.
		version = 4
	}
	psk, ok := node.Proto["psk"].(string)
	if !ok || psk == "" {
		return nil
	}
	proxy := map[string]any{
		"name":    node.Name,
		"type":    "snell",
		"server":  node.Server.Host,
		"port":    node.Server.Port,
		"psk":     psk,
		"version": version,
	}
	if obfs, ok := node.Proto["obfs"].(string); ok && obfs != "" {
		if obfs != "http" && obfs != "tls" && obfs != "shadow-tls" && obfs != "restls" && obfs != "jls" {
			return nil
		}
		proxy["obfs-opts"] = map[string]any{"mode": obfs}
	}
	return proxy
}

func (g *Generator) convertSOCKS5(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "socks5",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}
	if username, ok := node.Proto["username"].(string); ok && username != "" {
		proxy["username"] = username
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		proxy["password"] = password
	}
	if udp, ok := node.Proto["udp"].(bool); ok {
		proxy["udp"] = udp
	}
	return proxy
}

func (g *Generator) convertHTTP(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "http",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}
	if username, ok := node.Proto["username"].(string); ok && username != "" {
		proxy["username"] = username
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		proxy["password"] = password
	}
	if node.Security.Type == "tls" {
		proxy["tls"] = true
		if node.Security.SNI != "" {
			proxy["sni"] = node.Security.SNI
		}
		if node.Security.Opts != nil {
			if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
				proxy["skip-cert-verify"] = insecure
			}
		}
	}
	return proxy
}

func (g *Generator) convertSSH(node *ir.Node) map[string]any {
	username, ok := node.Proto["username"].(string)
	if !ok || username == "" {
		return nil
	}
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "ssh",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"username": username,
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		proxy["password"] = password
	}
	if privateKey, ok := node.Proto["private_key"].(string); ok && privateKey != "" {
		proxy["private-key"] = privateKey
	}
	return proxy
}

func (g *Generator) convertWireGuard(node *ir.Node) map[string]any {
	privateKey, privateOK := node.Proto["private_key"].(string)
	publicKey, publicOK := node.Proto["public_key"].(string)
	if !privateOK || privateKey == "" || !publicOK || publicKey == "" {
		return nil
	}
	addresses := protocolStringSlice(node.Proto["address"])
	if len(addresses) == 0 {
		addresses = protocolStringSlice(node.Proto["local_address"])
	}
	allowedIPs := protocolStringSlice(node.Proto["allowed_ips"])
	if len(addresses) == 0 || len(allowedIPs) == 0 {
		return nil
	}
	proxy := map[string]any{
		"name":        node.Name,
		"type":        "wireguard",
		"server":      node.Server.Host,
		"port":        node.Server.Port,
		"ip":          addresses[0],
		"private-key": privateKey,
		"public-key":  publicKey,
		"allowed-ips": allowedIPs,
	}
	if len(addresses) > 1 {
		if strings.Contains(addresses[1], ":") {
			proxy["ipv6"] = addresses[1]
		}
	}
	if preSharedKey, ok := node.Proto["preshared_key"].(string); ok && preSharedKey != "" {
		proxy["pre-shared-key"] = preSharedKey
	}
	if mtu, ok := protocolInt(node.Proto["mtu"]); ok && mtu > 0 {
		proxy["mtu"] = mtu
	}
	return proxy
}

func (g *Generator) convertShadowsocksR(node *ir.Node) map[string]any {
	password, passwordOK := node.Proto["password"].(string)
	cipher, cipherOK := node.Proto["method"].(string)
	protocol, protocolOK := node.Proto["protocol"].(string)
	obfs, obfsOK := node.Proto["obfs"].(string)
	if !passwordOK || password == "" || !cipherOK || cipher == "" || !protocolOK || protocol == "" || !obfsOK || obfs == "" {
		return nil
	}
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "ssr",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"password": password,
		"cipher":   cipher,
		"protocol": protocol,
		"obfs":     obfs,
	}
	if udp, ok := node.Proto["udp"].(bool); ok {
		proxy["udp"] = udp
	}
	for source, target := range map[string]string{
		"protocol-param": "protocol-param",
		"obfs-param":     "obfs-param",
	} {
		if value, ok := node.Proto[source].(string); ok && value != "" {
			proxy[target] = value
		}
	}
	return proxy
}

func (g *Generator) convertMieru(node *ir.Node) map[string]any {
	username, usernameOK := node.Proto["username"].(string)
	password, passwordOK := node.Proto["password"].(string)
	transport, transportOK := node.Proto["transport"].(string)
	if !usernameOK || username == "" || !passwordOK || password == "" || !transportOK || transport == "" {
		return nil
	}
	proxy := map[string]any{
		"name":      node.Name,
		"type":      "mieru",
		"server":    node.Server.Host,
		"port":      node.Server.Port,
		"username":  username,
		"password":  password,
		"transport": transport,
	}
	if portRange, ok := node.Proto["port_range"].(string); ok && portRange != "" {
		delete(proxy, "port")
		proxy["port-range"] = portRange
	}
	if udp, ok := node.Proto["udp"].(bool); ok {
		proxy["udp"] = udp
	}
	if value, ok := node.Proto["multiplexing"].(string); ok && value != "" {
		proxy["multiplexing"] = value
	}
	if value, ok := node.Proto["handshake_mode"].(string); ok && value != "" {
		proxy["handshake-mode"] = value
	}
	if value, ok := node.Proto["traffic_pattern"].(string); ok && value != "" {
		proxy["traffic-pattern"] = value
	}
	return proxy
}

func (g *Generator) convertSudoku(node *ir.Node) map[string]any {
	key, ok := node.Proto["key"].(string)
	if !ok || key == "" {
		return nil
	}
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "sudoku",
		"server": node.Server.Host,
		"port":   node.Server.Port,
		"key":    key,
	}
	stringFields := map[string]string{
		"aead_method":    "aead-method",
		"table_type":     "table-type",
		"http_mask_mode": "http-mask-mode",
		"http_mask_host": "http-mask-host",
		"path_root":      "path-root",
		"multiplex":      "multiplex",
	}
	for source, target := range stringFields {
		if value, ok := node.Proto[source].(string); ok && value != "" {
			proxy[target] = value
		}
	}
	intFields := map[string]string{
		"padding_min": "padding-min",
		"padding_max": "padding-max",
	}
	for source, target := range intFields {
		if value, ok := protocolInt(node.Proto[source]); ok {
			proxy[target] = value
		}
	}
	for source, target := range map[string]string{
		"enable_pure_downlink": "enable-pure-downlink",
		"http_mask":            "http-mask",
		"http_mask_tls":        "http-mask-tls",
		"udp":                  "udp",
	} {
		if value, ok := node.Proto[source].(bool); ok {
			proxy[target] = value
		}
	}
	return proxy
}

func (g *Generator) convertShadowQUIC(node *ir.Node) map[string]any {
	username, _ := node.Proto["username"].(string)
	password, passwordOK := node.Proto["password"].(string)
	if !passwordOK || password == "" {
		return nil
	}
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "shadowquic",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"username": username,
		"password": password,
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		proxy["alpn"] = node.Security.ALPN
	}
	if versions := protocolStringSlice(node.Proto["quic_versions"]); len(versions) > 0 {
		proxy["quic-versions"] = versions
	}
	for source, target := range map[string]string{
		"udp_over_stream":       "udp-over-stream",
		"zero_rtt":              "zero-rtt",
		"disable_mtu_discovery": "disable-mtu-discovery",
	} {
		if value, ok := node.Proto[source].(bool); ok {
			proxy[target] = value
		}
	}
	for source, target := range map[string]string{
		"keep_alive_interval":     "keep-alive-interval",
		"recv_window_conn":        "recv-window-conn",
		"recv_window":             "recv-window",
		"max_open_streams":        "max-open-streams",
		"max_datagram_frame_size": "max-datagram-frame-size",
		"cwnd":                    "cwnd",
	} {
		if value, ok := protocolInt(node.Proto[source]); ok {
			proxy[target] = value
		}
	}
	if value, ok := node.Proto["congestion_controller"].(string); ok && value != "" {
		proxy["congestion-controller"] = value
	}
	return proxy
}

func (g *Generator) convertTrustTunnel(node *ir.Node) map[string]any {
	username, usernameOK := node.Proto["username"].(string)
	password, passwordOK := node.Proto["password"].(string)
	if !usernameOK || username == "" || !passwordOK || password == "" {
		return nil
	}
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "trusttunnel",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"username": username,
		"password": password,
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		proxy["alpn"] = node.Security.ALPN
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			proxy["skip-cert-verify"] = insecure
		}
	}
	for source, target := range map[string]string{
		"udp":          "udp",
		"health_check": "health-check",
		"quic":         "quic",
	} {
		if value, ok := node.Proto[source].(bool); ok {
			proxy[target] = value
		}
	}
	for source, target := range map[string]string{
		"max_connections": "max-connections",
		"min_streams":     "min-streams",
		"max_streams":     "max-streams",
		"cwnd":            "cwnd",
	} {
		if value, ok := protocolInt(node.Proto[source]); ok {
			proxy[target] = value
		}
	}
	if value, ok := node.Proto["congestion_controller"].(string); ok && value != "" {
		proxy["congestion-controller"] = value
	}
	return proxy
}

func (g *Generator) convertMASQUE(node *ir.Node) map[string]any {
	privateKey, privateOK := node.Proto["private_key"].(string)
	publicKey, publicOK := node.Proto["public_key"].(string)
	if !privateOK || privateKey == "" || !publicOK || publicKey == "" {
		return nil
	}
	proxy := map[string]any{
		"name":        node.Name,
		"type":        "masque",
		"server":      node.Server.Host,
		"port":        node.Server.Port,
		"private-key": privateKey,
		"public-key":  publicKey,
	}
	if value, ok := node.Proto["network"].(string); ok && value != "" {
		proxy["network"] = value
	}
	if value, ok := node.Proto["uri"].(string); ok && value != "" {
		proxy["uri"] = value
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if value, ok := node.Proto["ip"].(string); ok && value != "" {
		proxy["ip"] = value
	}
	if value, ok := node.Proto["ipv6"].(string); ok && value != "" {
		proxy["ipv6"] = value
	}
	for source, target := range map[string]string{
		"udp":                "udp",
		"remote_dns_resolve": "remote-dns-resolve",
	} {
		if value, ok := node.Proto[source].(bool); ok {
			proxy[target] = value
		}
	}
	for source, target := range map[string]string{
		"mtu":               "mtu",
		"handshake_timeout": "handshake-timeout",
		"cwnd":              "cwnd",
	} {
		if value, ok := protocolInt(node.Proto[source]); ok {
			proxy[target] = value
		}
	}
	if value, ok := node.Proto["congestion_controller"].(string); ok && value != "" {
		proxy["congestion-controller"] = value
	}
	if dns := protocolStringSlice(node.Proto["dns"]); len(dns) > 0 {
		proxy["dns"] = dns
	}
	return proxy
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
		"name":    node.Name,
		"type":    "vmess",
		"server":  node.Server.Host,
		"port":    node.Server.Port,
		"alterId": 0,
		"cipher":  "auto",
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

func (g *Generator) convertVLESS(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "vless",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}
	if uuid, ok := node.Proto["uuid"].(string); ok && uuid != "" {
		proxy["uuid"] = uuid
	}
	if flow, ok := node.Proto["flow"].(string); ok && flow != "" {
		proxy["flow"] = flow
	}
	proxy["udp"] = true

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
			wsOpts["headers"] = map[string]string{"Host": node.Transport.Host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
	case "grpc":
		if node.Transport.Path != "" {
			proxy["grpc-opts"] = map[string]any{"grpc-service-name": node.Transport.Path}
		}
	case "xhttp":
		options := map[string]any{}
		if node.Transport.Path != "" {
			options["path"] = node.Transport.Path
		}
		if node.Transport.Host != "" {
			options["host"] = node.Transport.Host
		}
		for key, value := range node.Transport.Opts {
			switch key {
			case "mode":
				options[key] = value
			case "scMaxEachPostBytes":
				options["sc-max-each-post-bytes"] = value
			case "scMinPostsIntervalMs":
				options["sc-min-posts-interval-ms"] = value
			case "xPaddingBytes":
				options["x-padding-bytes"] = value
			case "xPaddingKey":
				options["x-padding-key"] = value
			}
		}
		if len(options) > 0 {
			proxy["xhttp-opts"] = options
		}
	}

	if node.Security.Type == "tls" || node.Security.Type == "reality" {
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
		if node.Security.Type == "reality" {
			if node.Security.Fingerprint == "" {
				proxy["client-fingerprint"] = "chrome"
			}
			reality := map[string]any{}
			if publicKey, ok := node.Security.Opts["public_key"].(string); ok && publicKey != "" {
				reality["public-key"] = publicKey
			}
			if shortID, ok := node.Security.Opts["short_id"].(string); ok && shortID != "" {
				reality["short-id"] = shortID
			}
			proxy["reality-opts"] = reality
		}
	}

	return proxy
}

func (g *Generator) convertTrojan(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "trojan",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}
	if password, ok := node.Proto["password"].(string); ok && password != "" {
		proxy["password"] = password
	}
	proxy["udp"] = true
	network := node.Transport.Type
	if network == "" {
		network = "tcp"
	}
	if network == "ws" {
		wsOpts := map[string]any{}
		if node.Transport.Path != "" {
			wsOpts["path"] = node.Transport.Path
		}
		if node.Transport.Host != "" {
			wsOpts["headers"] = map[string]string{"Host": node.Transport.Host}
		}
		if len(wsOpts) > 0 {
			proxy["ws-opts"] = wsOpts
		}
	} else if network == "grpc" && node.Transport.Path != "" {
		proxy["grpc-opts"] = map[string]any{"grpc-service-name": node.Transport.Path}
	}
	if network != "tcp" {
		proxy["network"] = network
	}
	if node.Security.Type == "tls" || node.Security.Type == "reality" {
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
	return proxy
}

func (g *Generator) convertHysteria2(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "hysteria2",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"password": node.Proto["password"],
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		proxy["alpn"] = node.Security.ALPN
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			proxy["skip-cert-verify"] = insecure
		}
	}
	if obfs, ok := node.Proto["obfs"].(string); ok && obfs != "" {
		proxy["obfs"] = obfs
		if password, ok := node.Proto["obfs_password"].(string); ok && password != "" {
			proxy["obfs-password"] = password
		}
	}
	if ports, ok := node.Proto["port_hopping"].(string); ok && ports != "" {
		proxy["ports"] = ports
	}
	return proxy
}

func (g *Generator) convertTUIC(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "tuic",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"uuid":     node.Proto["uuid"],
		"password": node.Proto["password"],
	}
	if congestion, ok := node.Proto["congestion_control"].(string); ok && congestion != "" {
		proxy["congestion-controller"] = congestion
	}
	if relay, ok := node.Proto["udp_relay_mode"].(string); ok && relay != "" {
		proxy["udp-relay-mode"] = relay
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		proxy["alpn"] = node.Security.ALPN
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			proxy["skip-cert-verify"] = insecure
		}
	}
	return proxy
}

func (g *Generator) convertHysteria(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   "hysteria",
		"server": node.Server.Host,
		"port":   node.Server.Port,
	}
	if auth, ok := node.Proto["auth"].(string); ok && auth != "" {
		proxy["auth-str"] = auth
	}
	if up, ok := node.Proto["up_mbps"].(string); ok && up != "" {
		proxy["up"] = up
	}
	if down, ok := node.Proto["down_mbps"].(string); ok && down != "" {
		proxy["down"] = down
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		proxy["alpn"] = node.Security.ALPN
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			proxy["skip-cert-verify"] = insecure
		}
	}
	return proxy
}

func (g *Generator) convertAnyTLS(node *ir.Node) map[string]any {
	proxy := map[string]any{
		"name":     node.Name,
		"type":     "anytls",
		"server":   node.Server.Host,
		"port":     node.Server.Port,
		"password": node.Proto["password"],
	}
	if node.Security.SNI != "" {
		proxy["sni"] = node.Security.SNI
	}
	if len(node.Security.ALPN) > 0 {
		proxy["alpn"] = node.Security.ALPN
	}
	if node.Security.Opts != nil {
		if insecure, ok := node.Security.Opts["insecure"].(bool); ok {
			proxy["skip-cert-verify"] = insecure
		}
	}
	for _, key := range []string{"idle_session_check_interval", "idle_session_timeout", "min_idle_session", "client_metadata", "disable_reuse"} {
		if value, ok := node.Proto[key]; ok {
			proxy[strings.ReplaceAll(key, "_", "-")] = value
		}
	}
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
	return generator.ValidateYAMLConfig(config, "mode", "log-level", "dns", "proxies", "proxy-groups", "rules")
}

// Kernel returns the kernel type
func (g *Generator) Kernel() ir.KernelType {
	return ir.KernelMihomo
}
