package singbox

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// sing-box 1.14.4 models OpenVPN as the root endpoint type "openvpn-client"
// (option.OpenVPNClientEndpointOptions in option/openvpn.go). It is not an
// outbound, so OpenVPN nodes are serialized under the root "endpoints" array.
const endpointTypeOpenVPNClient = "openvpn-client"

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// openVPNBuilder reads the OpenVPN IR (node.Proto / node.Extra) and, when the
// IR carries the original profile text (Proto["config"]), the raw directives
// the parser does not lift into Proto (repeated lines, bare flags, file
// arguments such as `tls-auth <file> <dir>`, `ifconfig`, `route`, ...).
type openVPNBuilder struct {
	node   *ir.Node
	proto  map[string]any
	dirs   map[string][][]string
	inline map[string]string
}

func (g *Generator) convertOpenVPNEndpoint(node *ir.Node) (map[string]any, error) {
	b := &openVPNBuilder{node: node, proto: node.Proto, dirs: map[string][][]string{}, inline: map[string]string{}}
	if b.proto == nil {
		b.proto = map[string]any{}
	}
	if config, ok := b.proto["config"].(string); ok {
		b.dirs = scanOpenVPNDirectives(config)
	}
	b.inline = inlineBlocks(node.Extra["inline"])

	endpoint, err := b.build()
	if err != nil {
		return nil, fmt.Errorf("openvpn node %q: %w", node.Name, err)
	}
	return endpoint, nil
}

func (b *openVPNBuilder) build() (map[string]any, error) {
	if err := b.rejectUnsupported(); err != nil {
		return nil, err
	}

	mode := "tls"
	if value, ok := b.proto["mode"].(string); ok && value != "" {
		mode = value
	}
	if mode != "tls" && mode != "static_key" {
		return nil, fmt.Errorf("unsupported mode %q", mode)
	}
	staticKey := mode == "static_key"

	endpoint := map[string]any{
		"type": endpointTypeOpenVPNClient,
		"tag":  b.node.Name,
		"mode": mode,
	}

	if err := b.applyRemotes(endpoint); err != nil {
		return nil, err
	}
	if err := b.applyTunnel(endpoint, staticKey); err != nil {
		return nil, err
	}
	if err := b.applyDataChannel(endpoint, staticKey); err != nil {
		return nil, err
	}
	if err := b.applyTiming(endpoint, staticKey); err != nil {
		return nil, err
	}
	if err := b.applyRouting(endpoint, staticKey); err != nil {
		return nil, err
	}
	if staticKey {
		if err := b.applyStaticKey(endpoint); err != nil {
			return nil, err
		}
	} else {
		if err := b.applyAuthentication(endpoint); err != nil {
			return nil, err
		}
		tls, err := b.buildTLS()
		if err != nil {
			return nil, err
		}
		endpoint["tls"] = tls
	}
	if err := b.applyDial(endpoint); err != nil {
		return nil, err
	}
	return endpoint, nil
}

// rejectUnsupported fails on profile features that have no field in the
// pinned sing-box schema, instead of silently dropping them.
func (b *openVPNBuilder) rejectUnsupported() error {
	if b.node.Security.Opts != nil {
		if insecure, _ := b.node.Security.Opts["insecure"].(bool); insecure {
			return fmt.Errorf("insecure TLS has no OpenVPN endpoint equivalent (use tls.peer_fingerprint or a CA)")
		}
	}
	for _, name := range []string{"pkcs12", "http-proxy", "socks-proxy", "tls-ciphersuites", "connection", "tls-server", "cryptoapicert"} {
		if b.has(name) {
			return fmt.Errorf("directive %q is not supported by sing-box %s", name, pinnedSingBoxVersion)
		}
	}
	for _, name := range []string{"pkcs12", "connection"} {
		if _, ok := b.inline[name]; ok {
			return fmt.Errorf("inline <%s> block is not supported by sing-box %s", name, pinnedSingBoxVersion)
		}
	}
	if args := b.last("dev"); len(args) > 0 && !strings.HasPrefix(strings.ToLower(args[0]), "tun") {
		return fmt.Errorf("dev %q is not supported: sing-box OpenVPN endpoints only provide layer-3 tun", args[0])
	}
	if args := b.last("dev-type"); len(args) > 0 && !strings.EqualFold(args[0], "tun") {
		return fmt.Errorf("dev-type %q is not supported", args[0])
	}
	if value, _ := b.proto["dev"].(string); value != "" && !strings.HasPrefix(strings.ToLower(value), "tun") {
		return fmt.Errorf("dev %q is not supported: sing-box OpenVPN endpoints only provide layer-3 tun", value)
	}
	for _, args := range b.dirs["proto"] {
		if len(args) > 0 && strings.EqualFold(args[0], "tcp-server") {
			return fmt.Errorf("proto tcp-server is not a client transport")
		}
	}
	return nil
}

const pinnedSingBoxVersion = "1.14.4"

func (b *openVPNBuilder) applyRemotes(endpoint map[string]any) error {
	type remoteEntry struct {
		host    string
		port    int
		network string
	}
	var remotes []remoteEntry
	switch items := b.proto["remotes"].(type) {
	case []map[string]any:
		for _, item := range items {
			remotes = append(remotes, remoteFromMap(item))
		}
	case []any:
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				remotes = append(remotes, remoteFromMap(item))
			}
		}
	}
	if len(remotes) == 0 {
		remotes = append(remotes, remoteEntry{host: b.node.Server.Host, port: b.node.Server.Port, network: b.node.Transport.Type})
	}

	servers := make([]map[string]any, 0, len(remotes))
	for _, remote := range remotes {
		if remote.host == "" {
			return fmt.Errorf("remote host is empty")
		}
		if remote.port < 1 || remote.port > 65535 {
			return fmt.Errorf("remote %s: invalid port %d", remote.host, remote.port)
		}
		entry := map[string]any{"server": remote.host, "server_port": remote.port}
		if remote.network != "" {
			if !validOpenVPNNetwork(remote.network) {
				return fmt.Errorf("remote %s: unsupported network %q", remote.host, remote.network)
			}
			entry["network"] = remote.network
		}
		servers = append(servers, entry)
	}
	endpoint["servers"] = servers
	if network, ok := b.proto["network"].(string); ok && network != "" {
		if !validOpenVPNNetwork(network) {
			return fmt.Errorf("unsupported network %q", network)
		}
		endpoint["network"] = network
	}
	if flag, _ := b.proto["remote_random"].(bool); flag || b.has("remote-random") {
		endpoint["remote_random"] = true
	}
	return nil
}

func remoteFromMap(item map[string]any) struct {
	host    string
	port    int
	network string
} {
	host, _ := item["host"].(string)
	network, _ := item["network"].(string)
	port, _ := toInt(item["port"])
	return struct {
		host    string
		port    int
		network string
	}{host, port, network}
}

func validOpenVPNNetwork(network string) bool {
	switch network {
	case "udp", "udp4", "udp6", "tcp", "tcp4", "tcp6":
		return true
	}
	return false
}

func (b *openVPNBuilder) applyTunnel(endpoint map[string]any, staticKey bool) error {
	if topology, ok := b.proto["topology"].(string); ok && topology != "" {
		switch topology {
		case "net30", "p2p", "subnet":
			endpoint["topology"] = topology
		default:
			return fmt.Errorf("unsupported topology %q", topology)
		}
	}
	if mtu, ok := toInt(b.proto["tun_mtu"]); ok && mtu > 0 {
		endpoint["mtu"] = mtu
	} else if mtu, ok := toInt(b.proto["mtu"]); ok && mtu > 0 {
		endpoint["mtu"] = mtu
	}
	if args := b.last("tun-mtu"); len(args) > 0 && endpoint["mtu"] == nil {
		if mtu, err := strconv.Atoi(args[0]); err == nil && mtu > 0 {
			endpoint["mtu"] = mtu
		}
	}
	if staticKey {
		return b.applyIfconfig(endpoint)
	}
	return nil
}

// applyIfconfig maps `ifconfig` / `ifconfig-ipv6`, which static_key mode
// requires (sing-box: "missing `address` in `static_key` mode").
func (b *openVPNBuilder) applyIfconfig(endpoint map[string]any) error {
	var addresses []string
	if args := b.last("ifconfig"); len(args) > 0 {
		if len(args) < 2 {
			return fmt.Errorf("ifconfig requires local and remote addresses")
		}
		local, err := netip.ParseAddr(args[0])
		if err != nil || !local.Is4() {
			return fmt.Errorf("ifconfig local %q is not an IPv4 address", args[0])
		}
		peer, err := netip.ParseAddr(args[1])
		if err != nil || !peer.Is4() {
			return fmt.Errorf("ifconfig remote %q is not an IPv4 address (subnet netmask form is unsupported in static_key mode)", args[1])
		}
		bits := 30
		if topology, _ := endpoint["topology"].(string); topology == "p2p" {
			bits = 32
		}
		addresses = append(addresses, netip.PrefixFrom(local, bits).String())
		endpoint["peer_address"] = peer.String()
	}
	if args := b.last("ifconfig-ipv6"); len(args) > 0 {
		if len(args) < 2 {
			return fmt.Errorf("ifconfig-ipv6 requires local prefix and remote address")
		}
		prefix, err := netip.ParsePrefix(args[0])
		if err != nil || !prefix.Addr().Is6() {
			return fmt.Errorf("ifconfig-ipv6 local %q is not an IPv6 prefix", args[0])
		}
		peer, err := netip.ParseAddr(args[1])
		if err != nil || !peer.Is6() {
			return fmt.Errorf("ifconfig-ipv6 remote %q is not an IPv6 address", args[1])
		}
		addresses = append(addresses, prefix.String())
		endpoint["peer_address_ipv6"] = peer.String()
	}
	if len(addresses) == 0 {
		return fmt.Errorf("static_key mode requires ifconfig in the profile (sing-box needs a tunnel address)")
	}
	endpoint["address"] = addresses
	return nil
}

func (b *openVPNBuilder) applyDataChannel(endpoint map[string]any, staticKey bool) error {
	cipher, _ := b.proto["cipher"].(string)
	fallback, _ := b.proto["data_ciphers_fallback"].(string)
	ciphers := toStringList(b.proto["data_ciphers"])

	if staticKey {
		if len(ciphers) > 0 || fallback != "" {
			return fmt.Errorf("data-ciphers/data-ciphers-fallback are not supported in static_key mode; use cipher")
		}
		if cipher != "" {
			endpoint["cipher"] = cipher
		}
	} else {
		if len(ciphers) > 0 {
			endpoint["data_ciphers"] = ciphers
		}
		// sing-box rejects `cipher` in TLS mode. OpenVPN 2.5+ treats `cipher` as
		// the fallback for peers that cannot negotiate data-ciphers.
		if fallback == "" {
			fallback = cipher
		}
		if fallback != "" {
			endpoint["data_ciphers_fallback"] = fallback
		}
	}
	if auth, ok := b.proto["auth"].(string); ok && auth != "" {
		endpoint["auth"] = auth
	}

	// comp-lzo
	if args, ok := b.dirs["comp-lzo"]; ok {
		mode := "yes"
		if last := args[len(args)-1]; len(last) > 0 {
			mode = strings.ToLower(last[0])
		}
		switch mode {
		case "yes", "no", "adaptive":
			endpoint["compression_lzo"] = mode
		default:
			return fmt.Errorf("unsupported comp-lzo mode %q", mode)
		}
	} else if mode, ok := b.proto["comp_lzo"].(string); ok && mode != "" {
		endpoint["compression_lzo"] = strings.ToLower(mode)
	}
	// compress
	if args, ok := b.dirs["compress"]; ok {
		last := args[len(args)-1]
		if len(last) == 0 {
			return fmt.Errorf("bare `compress` has no verified sing-box equivalent; specify an algorithm")
		}
		switch strings.ToLower(last[0]) {
		case "lz4", "lz4-v2", "stub", "stub-v2":
			endpoint["compression"] = strings.ToLower(last[0])
		default:
			return fmt.Errorf("unsupported compress algorithm %q", last[0])
		}
	} else if mode, ok := b.proto["compress"].(string); ok && mode != "" {
		endpoint["compression"] = strings.ToLower(mode)
	}
	if args := b.last("allow-compression"); len(args) > 0 {
		endpoint["allow_compression"] = strings.ToLower(args[0])
	}

	if args, ok := b.dirs["mssfix"]; ok {
		last := args[len(args)-1]
		switch {
		case len(last) == 0:
			// bare mssfix: default calculation
		default:
			value, err := strconv.Atoi(last[0])
			if err != nil || value < 0 {
				return fmt.Errorf("invalid mssfix %q", last[0])
			}
			if value == 0 {
				endpoint["mss_fix_disabled"] = true
			} else {
				endpoint["mss_fix"] = value
				if len(last) > 1 {
					if last[1] != "mtu" && last[1] != "fixed" {
						return fmt.Errorf("invalid mssfix mode %q", last[1])
					}
					endpoint["mss_fix_mode"] = last[1]
				}
			}
		}
	}
	if args := b.last("fragment"); len(args) > 0 {
		value, err := strconv.Atoi(args[0])
		if err != nil || value < 0 {
			return fmt.Errorf("invalid fragment %q", args[0])
		}
		endpoint["fragment"] = value
	}
	if args := b.last("replay-window"); len(args) > 0 {
		window, err := strconv.Atoi(args[0])
		if err != nil || window < 0 {
			return fmt.Errorf("invalid replay-window %q", args[0])
		}
		endpoint["replay_window"] = window
		if len(args) > 1 {
			seconds, err := strconv.Atoi(args[1])
			if err != nil || seconds < 0 {
				return fmt.Errorf("invalid replay-window time %q", args[1])
			}
			endpoint["replay_window_time"] = fmt.Sprintf("%ds", seconds)
		}
	}
	if args, ok := b.dirs["explicit-exit-notify"]; ok {
		count := 1
		if last := args[len(args)-1]; len(last) > 0 {
			value, err := strconv.Atoi(last[0])
			if err != nil || value < 0 {
				return fmt.Errorf("invalid explicit-exit-notify %q", last[0])
			}
			count = value
		}
		if count > 0 {
			endpoint["explicit_exit_notify"] = count
		}
	}
	return nil
}

func (b *openVPNBuilder) applyTiming(endpoint map[string]any, staticKey bool) error {
	if value, ok := toInt(b.proto["ping"]); ok && value > 0 {
		endpoint["ping_interval"] = fmt.Sprintf("%ds", value)
	}
	if value, ok := toInt(b.proto["ping_restart"]); ok {
		if value == 0 {
			endpoint["ping_restart_disabled"] = true
		} else if value > 0 {
			endpoint["ping_restart"] = fmt.Sprintf("%ds", value)
		}
	}

	tlsOnly := map[string]string{"reneg-sec": "", "reneg-bytes": "", "reneg-pkts": "", "tls-timeout": "", "hand-window": ""}
	if staticKey {
		for name := range tlsOnly {
			if b.has(name) {
				return fmt.Errorf("%s is not supported in static_key mode", name)
			}
		}
		return nil
	}
	if args := b.last("reneg-sec"); len(args) > 0 {
		value, err := strconv.Atoi(args[0])
		if err != nil || value < 0 {
			return fmt.Errorf("invalid reneg-sec %q", args[0])
		}
		if value == 0 {
			endpoint["renegotiate_disabled"] = true
		} else {
			endpoint["renegotiate_interval"] = fmt.Sprintf("%ds", value)
		}
	}
	for directive, field := range map[string]string{"reneg-bytes": "renegotiate_bytes", "reneg-pkts": "renegotiate_packets"} {
		if args := b.last(directive); len(args) > 0 {
			value, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid %s %q", directive, args[0])
			}
			endpoint[field] = value
		}
	}
	for directive, field := range map[string]string{"tls-timeout": "tls_timeout", "hand-window": "handshake_window"} {
		if args := b.last(directive); len(args) > 0 {
			value, err := strconv.Atoi(args[0])
			if err != nil || value < 0 {
				return fmt.Errorf("invalid %s %q", directive, args[0])
			}
			endpoint[field] = fmt.Sprintf("%ds", value)
		}
	}
	return nil
}

func (b *openVPNBuilder) applyRouting(endpoint map[string]any, staticKey bool) error {
	noPull, _ := b.proto["route_nopull"].(bool)
	if noPull || b.has("route-nopull") {
		if staticKey {
			return fmt.Errorf("route-nopull is not supported in static_key mode")
		}
		endpoint["route_no_pull"] = true
	}
	if filters := b.dirs["pull-filter"]; len(filters) > 0 {
		if staticKey {
			return fmt.Errorf("pull-filter is not supported in static_key mode")
		}
		out := make([]map[string]any, 0, len(filters))
		for _, args := range filters {
			if len(args) < 2 {
				return fmt.Errorf("pull-filter requires an action and text")
			}
			action := strings.ToLower(args[0])
			if action != "accept" && action != "ignore" && action != "reject" {
				return fmt.Errorf("unsupported pull-filter action %q", args[0])
			}
			out = append(out, map[string]any{"action": action, "text": strings.Join(args[1:], " ")})
		}
		endpoint["pull_filters"] = out
	}

	// redirect-gateway: a bare directive has no value, so the parser only keeps
	// it in Proto when flags are present.
	redirectFlags, redirect := []string{}, b.has("redirect-gateway")
	for _, args := range b.dirs["redirect-gateway"] {
		redirectFlags = append(redirectFlags, args...)
	}
	if value, ok := b.proto["redirect_gateway"].(string); ok && value != "" {
		redirect = true
		if !b.has("redirect-gateway") {
			redirectFlags = append(redirectFlags, strings.Fields(value)...)
		}
	} else if flag, _ := b.proto["redirect_gateway"].(bool); flag {
		redirect = true
	}
	if redirect {
		endpoint["redirect_gateway"] = true
		var flags []string
		for _, flag := range redirectFlags {
			if strings.EqualFold(flag, "block-local") {
				return fmt.Errorf("redirect-gateway block-local is not supported by sing-box")
			}
			flags = append(flags, flag)
		}
		if len(flags) > 0 {
			endpoint["redirect_gateway_flags"] = flags
		}
	}
	if b.has("redirect-private") {
		endpoint["redirect_private"] = true
	}
	if b.has("block-ipv6") {
		endpoint["block_ipv6"] = true
	}
	if args := b.last("route-gateway"); len(args) > 0 {
		gateway, err := netip.ParseAddr(args[0])
		if err != nil || !gateway.Is4() {
			return fmt.Errorf("route-gateway %q must be an IPv4 address", args[0])
		}
		endpoint["route_gateway"] = gateway.String()
	}
	if args := b.last("route-metric"); len(args) > 0 {
		metric, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid route-metric %q", args[0])
		}
		endpoint["route_metric"] = metric
	}

	var routes []string
	for _, args := range b.dirs["route"] {
		prefix, err := parseOpenVPNRoute(args)
		if err != nil {
			return err
		}
		routes = append(routes, prefix)
	}
	for _, args := range b.dirs["route-ipv6"] {
		prefix, err := parseOpenVPNRouteIPv6(args)
		if err != nil {
			return err
		}
		routes = append(routes, prefix)
	}
	if len(routes) > 0 {
		endpoint["routes"] = routes
	}
	return nil
}

// parseOpenVPNRoute handles `route network [netmask [gateway [metric]]]`.
// Only routes through the VPN gateway map onto sing-box `routes`; anything
// else (net_gateway, explicit gateway, metric) is rejected.
func parseOpenVPNRoute(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("route requires a network")
	}
	if len(args) > 2 && !strings.EqualFold(args[2], "vpn_gateway") && args[2] != "default" {
		return "", fmt.Errorf("route %s: gateway %q has no sing-box equivalent", args[0], args[2])
	}
	if len(args) > 3 {
		return "", fmt.Errorf("route %s: per-route metric has no sing-box equivalent", args[0])
	}
	network := args[0]
	if network == "default" {
		network = "0.0.0.0"
	}
	address, err := netip.ParseAddr(network)
	if err != nil || !address.Is4() {
		return "", fmt.Errorf("route network %q must be an IPv4 address", args[0])
	}
	mask := "255.255.255.255"
	if len(args) > 1 {
		mask = args[1]
	}
	maskAddr, err := netip.ParseAddr(mask)
	if err != nil || !maskAddr.Is4() {
		return "", fmt.Errorf("route netmask %q is invalid", mask)
	}
	m := maskAddr.As4()
	value := uint32(m[0])<<24 | uint32(m[1])<<16 | uint32(m[2])<<8 | uint32(m[3])
	bits := 0
	for value&0x80000000 != 0 {
		bits++
		value <<= 1
	}
	if value != 0 {
		return "", fmt.Errorf("route netmask %q is not contiguous", mask)
	}
	return netip.PrefixFrom(address, bits).Masked().String(), nil
}

func parseOpenVPNRouteIPv6(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("route-ipv6 requires a prefix")
	}
	if len(args) > 1 && !strings.EqualFold(args[1], "vpn_gateway") {
		return "", fmt.Errorf("route-ipv6 %s: gateway %q has no sing-box equivalent", args[0], args[1])
	}
	if len(args) > 2 {
		return "", fmt.Errorf("route-ipv6 %s: per-route metric has no sing-box equivalent", args[0])
	}
	prefix, err := netip.ParsePrefix(args[0])
	if err != nil || !prefix.Addr().Is6() {
		return "", fmt.Errorf("route-ipv6 %q is not an IPv6 prefix", args[0])
	}
	return prefix.Masked().String(), nil
}

func (b *openVPNBuilder) applyAuthentication(endpoint map[string]any) error {
	username, _ := b.proto["username"].(string)
	password, _ := b.proto["password"].(string)
	if username != "" {
		endpoint["username"] = username
	}
	if password != "" {
		endpoint["password"] = password
	}
	wantsAuth, _ := b.proto["auth_user_pass"].(bool)
	if (wantsAuth || b.has("auth-user-pass")) && username == "" && password == "" {
		return fmt.Errorf("auth-user-pass requires credentials but the IR has no username/password (credential files and interactive prompts are not resolved)")
	}
	if args := b.last("auth-retry"); len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "none", "nointeract", "interact":
			endpoint["auth_retry"] = strings.ToLower(args[0])
		default:
			return fmt.Errorf("unsupported auth-retry %q", args[0])
		}
	}
	if args := b.last("static-challenge"); len(args) > 0 {
		endpoint["static_challenge"] = args[0]
		if len(args) > 1 && args[1] == "1" {
			endpoint["static_challenge_echo"] = true
		}
	}
	return nil
}

func (b *openVPNBuilder) applyStaticKey(endpoint map[string]any) error {
	if b.has("auth-user-pass") || b.has("ca") || b.has("cert") || b.has("key") ||
		b.has("tls-auth") || b.has("tls-crypt") || b.has("tls-crypt-v2") {
		return fmt.Errorf("TLS/credential options are not supported in static_key mode")
	}
	if username, _ := b.proto["username"].(string); username != "" {
		return fmt.Errorf("username/password authentication is not supported in static_key mode")
	}
	direction := ""
	if content, ok := b.proto["secret"].(string); ok && content != "" {
		endpoint["static_key"] = content
	} else if args := b.last("secret"); len(args) > 0 {
		endpoint["static_key_path"] = args[0]
		if len(args) > 1 {
			direction = args[1]
		}
	} else {
		return fmt.Errorf("static_key mode requires a secret key")
	}
	if direction == "" {
		direction, _ = b.proto["key_direction"].(string)
	}
	if args := b.last("key-direction"); len(args) > 0 {
		direction = args[0]
	}
	mapped, err := mapKeyDirection(direction)
	if err != nil {
		return err
	}
	if mapped != "" {
		endpoint["key_direction"] = mapped
	}
	return nil
}

func mapKeyDirection(value string) (string, error) {
	switch value {
	case "":
		return "", nil
	case "0":
		return "server", nil
	case "1":
		return "client", nil
	}
	return "", fmt.Errorf("unsupported key direction %q", value)
}

func (b *openVPNBuilder) buildTLS() (map[string]any, error) {
	tls := map[string]any{}

	// Server certificate verification name.
	if value, ok := b.proto["verify_x509_name"].(string); ok && value != "" {
		fields := strings.Fields(value)
		tls["server_name"] = fields[0]
		if len(fields) > 1 {
			switch fields[1] {
			case "subject", "name", "name-prefix":
				tls["server_name_type"] = fields[1]
			default:
				return nil, fmt.Errorf("unsupported verify-x509-name type %q", fields[1])
			}
		}
	}

	// CA / client certificate / key: inline content wins over file paths.
	if err := setMaterial(tls, b.proto, "certificate", "ca"); err != nil {
		return nil, err
	}
	if err := setMaterial(tls, b.proto, "client_certificate", "cert"); err != nil {
		return nil, err
	}
	if err := setMaterial(tls, b.proto, "client_key", "key"); err != nil {
		return nil, err
	}
	_, hasCert := tls["client_certificate"]
	_, hasCertPath := tls["client_certificate_path"]
	_, hasKey := tls["client_key"]
	_, hasKeyPath := tls["client_key_path"]
	if (hasCert || hasCertPath) != (hasKey || hasKeyPath) {
		return nil, fmt.Errorf("client certificate and key must both be set or both be empty")
	}

	var fingerprints []string
	collect := func(value string) error {
		for _, part := range strings.Fields(value) {
			normalized := strings.ToLower(strings.ReplaceAll(part, ":", ""))
			if !fingerprintPattern.MatchString(normalized) {
				return fmt.Errorf("invalid peer-fingerprint %q (need SHA-256)", part)
			}
			fingerprints = append(fingerprints, normalized)
		}
		return nil
	}
	for _, args := range b.dirs["peer-fingerprint"] {
		for _, arg := range args {
			if err := collect(arg); err != nil {
				return nil, err
			}
		}
	}
	if content, ok := b.inline["peer-fingerprint"]; ok {
		if err := collect(content); err != nil {
			return nil, err
		}
	}
	if len(fingerprints) > 0 {
		tls["peer_fingerprint"] = fingerprints
	}
	if _, ok := tls["certificate"]; !ok {
		if _, ok := tls["certificate_path"]; !ok && len(fingerprints) == 0 {
			return nil, fmt.Errorf("TLS mode requires a CA (ca / <ca>) or peer-fingerprint")
		}
	}

	if _, ok := b.inline["crl-verify"]; ok {
		return nil, fmt.Errorf("inline <crl-verify> is not supported; sing-box only accepts crl_path")
	}
	if args := b.last("crl-verify"); len(args) > 0 {
		if len(args) > 1 && strings.EqualFold(args[1], "dir") {
			return nil, fmt.Errorf("crl-verify directory mode is not supported")
		}
		tls["crl_path"] = args[0]
	}

	// Remote certificate purpose. OpenVPN checks nothing by default whereas
	// sing-box defaults to "server", so the absence is made explicit.
	eku := b.last("remote-cert-eku")
	remoteTLS, _ := b.proto["remote_cert_tls"].(string)
	if args := b.last("remote-cert-tls"); len(args) > 0 {
		remoteTLS = args[0]
	}
	switch {
	case len(eku) > 0 && remoteTLS != "":
		return nil, fmt.Errorf("remote-cert-eku conflicts with remote-cert-tls")
	case len(eku) > 0:
		tls["remote_certificate_eku"] = strings.Join(eku, " ")
	case remoteTLS == "server" || remoteTLS == "client":
		tls["remote_certificate_tls"] = remoteTLS
	case remoteTLS == "":
		tls["remote_certificate_tls"] = "none"
	default:
		return nil, fmt.Errorf("unsupported remote-cert-tls %q", remoteTLS)
	}
	if args := b.last("remote-cert-ku"); len(args) > 0 {
		tls["remote_certificate_ku"] = args
	}
	if args := b.last("ns-cert-type"); len(args) > 0 {
		tls["ns_certificate_type"] = strings.ToLower(args[0])
	}
	for directive, field := range map[string]string{"tls-version-min": "version_min", "tls-version-max": "version_max"} {
		if args := b.last(directive); len(args) > 0 {
			switch args[0] {
			case "1.0", "1.1", "1.2", "1.3":
				tls[field] = args[0]
			default:
				return nil, fmt.Errorf("unsupported %s %q", directive, args[0])
			}
		}
	}
	if args := b.last("tls-cipher"); len(args) > 0 {
		tls["cipher"] = args[0]
	}
	if args := b.last("tls-groups"); len(args) > 0 {
		tls["groups"] = args[0]
	}
	if args := b.last("tls-cert-profile"); len(args) > 0 {
		tls["certificate_profile"] = strings.ToLower(args[0])
	}

	wrap, err := b.buildControlWrap()
	if err != nil {
		return nil, err
	}
	if wrap != nil {
		tls["control_wrap"] = wrap
	}
	return tls, nil
}

func (b *openVPNBuilder) buildControlWrap() (map[string]any, error) {
	type source struct{ name, protoKey, directive string }
	var found []source
	for _, candidate := range []source{
		{"tls_auth", "tls_auth", "tls-auth"},
		{"tls_crypt", "tls_crypt", "tls-crypt"},
		{"tls_crypt_v2", "tls_crypt_v2", "tls-crypt-v2"},
	} {
		content, _ := b.proto[candidate.protoKey].(string)
		_, pathOK := b.proto[candidate.protoKey+"_path"].(string)
		if content != "" || pathOK || b.has(candidate.directive) {
			found = append(found, candidate)
		}
	}
	if len(found) == 0 {
		return nil, nil
	}
	if len(found) > 1 {
		return nil, fmt.Errorf("only one of tls-auth, tls-crypt and tls-crypt-v2 can be configured")
	}
	chosen := found[0]
	wrap := map[string]any{"type": chosen.name}
	direction := ""
	args := b.last(chosen.directive)
	if content, _ := b.proto[chosen.protoKey].(string); content != "" {
		wrap["key"] = content
	} else if len(args) > 0 {
		wrap["key_path"] = args[0]
		if chosen.name == "tls_auth" && len(args) > 1 {
			direction = args[1]
		}
	} else if path, ok := b.proto[chosen.protoKey+"_path"].(string); ok {
		wrap["key_path"] = path
	}
	if chosen.name == "tls_auth" {
		if direction == "" {
			direction, _ = b.proto["key_direction"].(string)
		}
		if kd := b.last("key-direction"); len(kd) > 0 {
			direction = kd[0]
		}
		mapped, err := mapKeyDirection(direction)
		if err != nil {
			return nil, err
		}
		if mapped != "" {
			wrap["direction"] = mapped
		}
	}
	return wrap, nil
}

func (b *openVPNBuilder) applyDial(endpoint map[string]any) error {
	if args := b.last("local"); len(args) > 0 {
		address, err := netip.ParseAddr(args[0])
		if err != nil {
			return fmt.Errorf("local %q must be an IP address", args[0])
		}
		if address.Is4() {
			endpoint["inet4_bind_address"] = address.String()
		} else {
			endpoint["inet6_bind_address"] = address.String()
		}
	}
	return nil
}

// setMaterial maps a Proto key pair (<name> inline content, <name>_path) onto
// sing-box inline/path fields. Inline content takes precedence.
func setMaterial(target, proto map[string]any, field, name string) error {
	if content, ok := proto[name].(string); ok && content != "" {
		target[field] = content
		return nil
	}
	if path, ok := proto[name+"_path"].(string); ok && path != "" {
		target[field+"_path"] = path
	}
	return nil
}

func (b *openVPNBuilder) has(name string) bool {
	_, ok := b.dirs[name]
	return ok
}

// last returns the arguments of the final occurrence of a directive.
func (b *openVPNBuilder) last(name string) []string {
	entries := b.dirs[name]
	if len(entries) == 0 {
		return nil
	}
	return entries[len(entries)-1]
}

// scanOpenVPNDirectives tokenizes an OpenVPN profile into directive -> all
// occurrences (argument lists). Inline <tag> blocks are skipped; they are
// carried by the IR (Proto / Extra["inline"]).
func scanOpenVPNDirectives(config string) map[string][][]string {
	result := map[string][][]string{}
	block := ""
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if block != "" {
			if strings.EqualFold(trimmed, "</"+block+">") {
				block = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") && !strings.HasPrefix(trimmed, "</") {
			block = strings.Trim(trimmed, "<>")
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		fields := splitOpenVPNLine(trimmed)
		if len(fields) == 0 {
			continue
		}
		key := strings.ToLower(fields[0])
		result[key] = append(result[key], fields[1:])
	}
	return result
}

// splitOpenVPNLine splits on whitespace honoring simple "..." / '...' quoting.
func splitOpenVPNLine(line string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	inField := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
			inField = true
		case r == ' ' || r == '\t':
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		default:
			current.WriteRune(r)
			inField = true
		}
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields
}

func inlineBlocks(value any) map[string]string {
	result := map[string]string{}
	switch typed := value.(type) {
	case map[string]string:
		for key, content := range typed {
			result[strings.ToLower(key)] = content
		}
	case map[string]any:
		for key, content := range typed {
			if text, ok := content.(string); ok {
				result[strings.ToLower(key)] = text
			}
		}
	}
	return result
}

func toInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	}
	return 0, false
}

func toStringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}
