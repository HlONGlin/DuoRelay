package subscription

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"gopkg.in/yaml.v3"
)

func decodeBase64(content []byte) ([]byte, error) {
	content = bytes.Join(bytes.Fields(content), nil)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if data, err := encoding.DecodeString(string(content)); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 subscription")
}

// ParseFormat recognizes native configuration lists, share links and base64.
func ParseFormat(content []byte) (string, error) {
	return detectFormat(content, 0)
}

type ParseSkip struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

func detectFormat(content []byte, depth int) (string, error) {
	var err error
	content, err = normalizeContent(content)
	if err != nil {
		return "", err
	}
	if len(content) == 0 {
		return "", fmt.Errorf("empty subscription")
	}
	if hasOpenVPNProfile(content) {
		return "openvpn", nil
	}
	var root map[string]any
	if json.Unmarshal(content, &root) == nil {
		if _, ok := root["outbounds"]; ok {
			list, _ := root["outbounds"].([]any)
			for _, entry := range list {
				if _, ok := object(entry)["protocol"]; ok {
					return "xray", nil
				}
			}
			return "sing-box", nil
		}
		if _, ok := root["proxies"]; ok {
			return "clash", nil
		}
		if _, ok := root["settings"]; ok && text(root["protocol"]) != "" {
			return "xray-single", nil
		}
		if looksLikeV2RayN(root) {
			return "v2rayn", nil
		}
		if _, ok := root["servers"]; ok {
			return "sip008", nil
		}
		if _, ok := root["protocol"]; ok {
			return "xray-single", nil
		}
	}
	var entries []map[string]any
	if json.Unmarshal(content, &entries) == nil && len(entries) > 0 {
		if firstText(entries[0], "add", "address") != "" {
			return "v2rayn", nil
		}
		if _, ok := entries[0]["server_port"]; ok {
			if text(entries[0]["type"]) != "" {
				return "sing-box-list", nil
			}
			return "sip008", nil
		}
		if _, ok := entries[0]["type"]; ok {
			return "clash-list", nil
		}
		return "v2rayn", nil
	}
	if yaml.Unmarshal(content, &root) == nil {
		if _, ok := root["proxies"]; ok {
			return "clash", nil
		}
	}
	if looksLikeINI(content) {
		return "proxy-ini", nil
	}
	if bytes.Contains(content, []byte("://")) && !bytes.HasPrefix(content, []byte("{")) && !bytes.HasPrefix(content, []byte("[")) {
		return "links", nil
	}
	if depth < 3 {
		if data, err := decodeBase64(content); err == nil {
			if format, err := detectFormat(data, depth+1); err == nil {
				if format == "links" {
					return "base64", nil
				}
				return format + "-base64", nil
			}
		}
	}
	return "", fmt.Errorf("unrecognized subscription format")
}

func hasOpenVPNProfile(content []byte) bool {
	hasRemote, hasMarker := false, false
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		fields := strings.Fields(string(line))
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") || strings.HasPrefix(fields[0], ";") {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "remote":
			hasRemote = true
		case "client", "dev", "proto":
			hasMarker = true
		}
	}
	return hasRemote && hasMarker
}

func (m *Manager) Parse(content []byte) ([]*ir.Node, error) {
	nodes, _, err := m.ParseDetailed(content)
	return nodes, err
}

// ParseDetailed keeps usable share-link lines when individual lines are not
// supported. A document with no usable nodes still fails.
func (m *Manager) ParseDetailed(content []byte) ([]*ir.Node, []ParseSkip, error) {
	var err error
	content, err = normalizeContent(content)
	if err != nil {
		return nil, nil, err
	}
	format, err := ParseFormat(content)
	if err != nil {
		return nil, nil, err
	}
	for format == "base64" || strings.HasSuffix(format, "-base64") {
		content, err = decodeBase64(content)
		if err != nil {
			return nil, nil, err
		}
		content, err = normalizeContent(content)
		if err != nil {
			return nil, nil, err
		}
		if format == "base64" {
			format = "links"
		} else {
			format = strings.TrimSuffix(format, "-base64")
		}
	}
	switch format {
	case "proxy-ini":
		return m.parseINIDetailed(content)
	case "sip008", "xray", "xray-single", "v2rayn":
		return m.parseJSONDetailed(content, format)
	}
	nodes := []*ir.Node{}
	if format == "openvpn" {
		if m.Registry == nil {
			return nil, nil, fmt.Errorf("OpenVPN profile parser registry is not configured")
		}
		value, err := m.Registry.ParseAny(string(content))
		if err != nil {
			return nil, nil, fmt.Errorf("invalid OpenVPN profile")
		}
		node, ok := value.(*ir.Node)
		if !ok {
			return nil, nil, fmt.Errorf("OpenVPN parser did not return an IR node")
		}
		if err := validateParsedNode(node); err != nil {
			return nil, nil, err
		}
		return []*ir.Node{node}, nil, nil
	}
	if format == "links" {
		if m.Registry == nil {
			return nil, nil, fmt.Errorf("share-link parser registry is not configured")
		}
		skipped := []ParseSkip{}
		for index, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
				continue
			}
			if isNativeMieru(line) {
				items, err := parseNativeMieru(line)
				if err != nil {
					skipped = append(skipped, ParseSkip{Line: index + 1, Reason: err.Error()})
					continue
				}
				nodes = append(nodes, items...)
				continue
			}
			if strings.HasPrefix(strings.ToLower(line), "ssd://") {
				items, err := parseSSD(line)
				if err != nil {
					skipped = append(skipped, ParseSkip{Line: index + 1, Reason: "invalid SSD subscription"})
					continue
				}
				nodes = append(nodes, items...)
				continue
			}
			value, err := m.Registry.ParseAny(line)
			if err != nil {
				reason := "invalid or unsupported share link"
				if errors.Is(err, parser.ErrUnsupportedFeature) {
					reason = err.Error()
				}
				if strings.Contains(strings.ToLower(line), "xhttp") && strings.HasPrefix(strings.ToLower(line), "vless://") {
					reason = "VLESS XHTTP: " + reason
				}
				skipped = append(skipped, ParseSkip{Line: index + 1, Reason: reason})
				continue
			}
			var node *ir.Node
			switch value := value.(type) {
			case *ir.Node:
				node = value
			case ir.Node:
				node = &value
			default:
				skipped = append(skipped, ParseSkip{Line: index + 1, Reason: "parser did not return an IR node"})
				continue
			}
			if err := validateParsedNode(node); err != nil {
				skipped = append(skipped, ParseSkip{Line: index + 1, Reason: err.Error()})
				continue
			}
			nodes = append(nodes, node)
		}
		if len(nodes) == 0 {
			return nil, skipped, fmt.Errorf("subscription contains no usable nodes")
		}
		return nodes, skipped, nil
	}
	if format == "v2rayn" {
		nodes, err := m.parseV2RayNJSON(content)
		return nodes, nil, err
	}
	var root struct {
		Proxies   []map[string]any `yaml:"proxies"`
		Outbounds []map[string]any `yaml:"outbounds"`
	}
	if format == "clash-list" {
		if err := yaml.Unmarshal(content, &root.Proxies); err != nil {
			return nil, nil, fmt.Errorf("invalid proxy list")
		}
	} else if format == "sing-box-list" {
		if err := yaml.Unmarshal(content, &root.Outbounds); err != nil {
			return nil, nil, fmt.Errorf("invalid outbound list")
		}
		format = "sing-box"
	} else if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, nil, fmt.Errorf("invalid subscription document")
	}
	entries := root.Proxies
	if format == "sing-box" {
		entries = root.Outbounds
	}
	skipped := []ParseSkip{}
	for index, entry := range entries {
		typ := text(entry["type"])
		if typ == "direct" || typ == "reject" || typ == "reject-drop" {
			continue
		}
		if format == "sing-box" {
			switch typ {
			case "direct", "block", "dns", "selector", "urltest":
				continue
			}
		}
		node, err := nativeNode(entry, format)
		if err != nil {
			skipped = append(skipped, ParseSkip{Line: index + 1, Reason: err.Error()})
			continue
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return nil, skipped, fmt.Errorf("subscription contains no usable nodes")
	}
	return nodes, skipped, nil
}

func text(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func number(v any) int { value, _ := strconv.Atoi(text(v)); return value }
func object(v any) map[string]any {
	value, _ := v.(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}
func texts(v any) []string {
	var values []string
	switch list := v.(type) {
	case []any:
		for _, value := range list {
			values = append(values, text(value))
		}
	case []string:
		values = list
	}
	return values
}
func validateParsedNode(n *ir.Node) error {
	if n == nil || !ir.IsValidProtocol(n.Protocol) || strings.TrimSpace(n.Server.Host) == "" || n.Server.Port < 1 || n.Server.Port > 65535 {
		return fmt.Errorf("invalid protocol or server address")
	}
	if n.Name == "" {
		n.Name = n.Server.Host + ":" + strconv.Itoa(n.Server.Port)
	}
	if n.Owner == 0 {
		n.Owner = ir.GetProtocolOwner(n.Protocol)
	}
	var credentials []string
	switch n.Protocol {
	case ir.ProtocolVLESS, ir.ProtocolVMess:
		credentials = []string{"uuid"}
	case ir.ProtocolShadowsocks:
		credentials = []string{"method", "password"}
	case ir.ProtocolTrojan, ir.ProtocolAnyTLS:
		credentials = []string{"password"}
	}
	for _, key := range credentials {
		if value, ok := n.Proto[key].(string); !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing required connection credentials")
		}
	}
	return ir.ValidateConnectionFeatures(n)
}

func looksLikeV2RayN(entry map[string]any) bool {
	if entry == nil {
		return false
	}
	if _, ok := entry["configs"]; ok {
		return true
	}
	protocol := strings.ToLower(text(entry["protocol"]))
	return protocol == "vmess" || protocol == "vless" || protocol == "trojan" || protocol == "shadowsocks" || protocol == "ss" || protocol == "socks" || protocol == "http" || protocol == "anytls" || protocol == "mieru" || (text(entry["id"]) != "" && (text(entry["add"]) != "" || text(entry["address"]) != ""))
}

func (m *Manager) parseV2RayNJSON(content []byte) ([]*ir.Node, error) {
	var raw any
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("invalid V2RayN JSON")
	}
	entries := []map[string]any{}
	appendEntry := func(value any) {
		if entry, ok := value.(map[string]any); ok {
			entries = append(entries, entry)
		}
	}
	switch value := raw.(type) {
	case map[string]any:
		if list, ok := value["configs"].([]any); ok {
			for _, item := range list {
				appendEntry(item)
			}
		} else {
			appendEntry(value)
		}
	case []any:
		for _, item := range value {
			appendEntry(item)
		}
	default:
		return nil, fmt.Errorf("invalid V2RayN JSON entries")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("empty V2RayN JSON entries")
	}
	nodes := make([]*ir.Node, 0, len(entries))
	for index, entry := range entries {
		node, err := nativeNode(entry, "v2rayn")
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", index+1, err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func firstText(entry map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(text(entry[key])); value != "" {
			return value
		}
	}
	return ""
}

func v2rayNNode(entry map[string]any) (*ir.Node, error) {
	typ := strings.ToLower(firstText(entry, "protocol", "type"))
	switch typ {
	case "", "vmess":
		typ = ir.ProtocolVMess
	case "vless":
		typ = ir.ProtocolVLESS
	case "trojan":
		typ = ir.ProtocolTrojan
	case "ss", "shadowsocks":
		typ = ir.ProtocolShadowsocks
	case "socks", "socks5":
		typ = ir.ProtocolSOCKS5
	case "http", "https":
		typ = ir.ProtocolHTTP
	case "anytls":
		typ = ir.ProtocolAnyTLS
	case "mieru":
		typ = ir.ProtocolMieru
	default:
		return nil, fmt.Errorf("unsupported V2RayN protocol %q", typ)
	}
	host := firstText(entry, "add", "address", "server", "host")
	port := number(entry["port"])
	if port == 0 {
		port = number(entry["server_port"])
	}
	if port == 0 {
		port = 443
	}
	if host == "" {
		return nil, fmt.Errorf("missing server address")
	}
	base, _ := url.Parse(typ + "://" + net.JoinHostPort(host, strconv.Itoa(port)))
	n := &ir.Node{
		Protocol:  typ,
		Owner:     ir.GetProtocolOwner(typ),
		Server:    ir.Server{Host: host, Port: port},
		Proto:     map[string]any{},
		Extra:     map[string]any{"v2rayn": entry},
		Transport: ir.Transport{Type: "tcp", Opts: map[string]any{}},
		Security:  ir.Security{Type: "none", Opts: map[string]any{}},
		Source:    ir.Source{Type: "import", URL: base.String()},
	}
	n.Name = firstText(entry, "ps", "remarks", "name", "tag")
	if n.Name == "" {
		n.Name = host + ":" + strconv.Itoa(port)
	}
	if typ == ir.ProtocolVMess {
		n.Proto["uuid"] = firstText(entry, "id", "uuid")
		n.Proto["alter_id"] = number(entry["aid"])
		if n.Proto["alter_id"] == 0 {
			n.Proto["alter_id"] = number(entry["alterId"])
		}
		n.Proto["security"] = firstText(entry, "scy", "security")
		if n.Proto["security"] == "" {
			n.Proto["security"] = "auto"
		}
	} else if typ == ir.ProtocolVLESS {
		n.Proto["uuid"] = firstText(entry, "id", "uuid")
		n.Proto["encryption"] = firstText(entry, "encryption")
		if n.Proto["encryption"] == "" {
			n.Proto["encryption"] = "none"
		}
		n.Proto["flow"] = firstText(entry, "flow")
	} else if typ == ir.ProtocolTrojan {
		n.Proto["password"] = firstText(entry, "password", "id")
	} else if typ == ir.ProtocolShadowsocks {
		n.Proto["method"] = firstText(entry, "method", "cipher", "security")
		n.Proto["password"] = firstText(entry, "password")
	} else if typ == ir.ProtocolAnyTLS {
		n.Proto["password"] = firstText(entry, "password", "id")
		n.Proto["udp"] = true
		n.Security.Type = "tls"
	} else if typ == ir.ProtocolMieru {
		n.Proto["username"] = firstText(entry, "username", "user")
		n.Proto["password"] = firstText(entry, "password")
	} else {
		n.Proto["username"] = firstText(entry, "username", "user")
		n.Proto["password"] = firstText(entry, "password")
	}
	network := strings.ToLower(firstText(entry, "net", "network", "transport"))
	if network != "" && network != "tcp" && network != "none" {
		n.Transport.Type = network
	}
	if n.Transport.Type == "xhttp" {
		for _, key := range []string{"mode", "scMaxEachPostBytes", "scMinPostsIntervalMs", "xPaddingBytes", "xPaddingKey", "extra", "xmux", "scStreamUpServerSecs", "xPaddingObfsMode", "xPaddingVerify"} {
			if value, ok := entry[key]; ok {
				n.Transport.Opts[key] = value
			}
		}
	}
	n.Transport.Path = firstText(entry, "path")
	n.Transport.Host = firstText(entry, "host")
	tls := firstText(entry, "tls")
	if tls == "" {
		tls = firstText(entry, "security")
	}
	if strings.EqualFold(tls, "reality") {
		n.Security.Type = "reality"
	} else if strings.EqualFold(tls, "tls") || entry["tls"] == true {
		n.Security.Type = "tls"
	}
	if typ == ir.ProtocolAnyTLS {
		n.Security.Type = "tls"
	}
	n.Security.SNI = firstText(entry, "sni", "serverName", "servername")
	if n.Security.Type == "none" && n.Security.SNI != "" {
		n.Security.Type = "tls"
	}
	if n.Security.Type == "reality" {
		n.Security.Opts["public_key"] = firstText(entry, "publicKey", "pbk")
		n.Security.Opts["short_id"] = firstText(entry, "shortId", "sid")
	}
	n.Security.Fingerprint = firstText(entry, "fp", "fingerprint", "clientFingerprint")
	if alpn := firstText(entry, "alpn"); alpn != "" {
		n.Security.ALPN = strings.Split(alpn, ",")
	}
	for _, key := range []string{"allowInsecure", "insecure", "skip-cert-verify"} {
		if value, ok := entry[key]; ok {
			skip, err := strconv.ParseBool(text(value))
			if err != nil {
				return nil, fmt.Errorf("invalid TLS verification flag")
			}
			n.Security.Opts["insecure"] = skip
		}
	}
	if typ == ir.ProtocolTrojan {
		n.Security.Type = "tls"
	}
	if n.Transport.Type == "grpc" {
		n.Transport.Path = firstText(entry, "serviceName", "service_name", "path")
		n.Transport.Opts["service_name"] = n.Transport.Path
	}
	if (typ == ir.ProtocolVMess || typ == ir.ProtocolVLESS) && strings.TrimSpace(text(n.Proto["uuid"])) == "" {
		return nil, fmt.Errorf("missing uuid")
	}
	if typ == ir.ProtocolTrojan && strings.TrimSpace(text(n.Proto["password"])) == "" {
		return nil, fmt.Errorf("missing password")
	}
	if typ == ir.ProtocolAnyTLS && strings.TrimSpace(text(n.Proto["password"])) == "" {
		return nil, fmt.Errorf("missing password")
	}
	if typ == ir.ProtocolShadowsocks && (firstText(n.Proto, "method") == "" || firstText(n.Proto, "password") == "") {
		return nil, fmt.Errorf("missing Shadowsocks method or password")
	}
	if typ == ir.ProtocolMieru && (strings.TrimSpace(text(n.Proto["username"])) == "" || strings.TrimSpace(text(n.Proto["password"])) == "") {
		return nil, fmt.Errorf("missing credentials")
	}
	if typ == ir.ProtocolMieru {
		transport := strings.ToLower(firstText(entry, "transport", "network", "net"))
		if transport == "" {
			transport = "tcp"
		}
		n.Proto["transport"] = strings.ToUpper(transport)
		if udp, ok := entry["udp"].(bool); ok {
			n.Proto["udp"] = udp
		}
	}
	if err := validateParsedNode(n); err != nil {
		return nil, err
	}
	return n, nil
}
func nativeNode(entry map[string]any, format string) (*ir.Node, error) {
	if format == "v2rayn" {
		return v2rayNNode(entry)
	}
	typ := text(entry["type"])
	switch typ {
	case "ss":
		typ = ir.ProtocolShadowsocks
	case "ssr":
		typ = ir.ProtocolShadowsocksR
	case "socks":
		typ = ir.ProtocolSOCKS5
	case "hy2":
		typ = ir.ProtocolHysteria2
	}
	n := &ir.Node{Protocol: typ, Owner: ir.GetProtocolOwner(typ), Name: text(entry["name"]), Server: ir.Server{Host: text(entry["server"]), Port: number(entry["port"])}, Proto: map[string]any{}, Extra: map[string]any{}, Transport: ir.Transport{Type: "tcp", Opts: map[string]any{}}, Security: ir.Security{Type: "none", Opts: map[string]any{}}}
	if format == "sing-box" {
		n.Name = text(entry["tag"])
		n.Server.Port = number(entry["server_port"])
	}
	// Keep the native record for fields that have no canonical IR equivalent.
	n.Extra[format] = entry
	for key, value := range entry {
		n.Proto[key] = value
	}
	for _, key := range []string{"type", "tag", "name", "server", "port", "server_port", "tls", "transport", "network", "ws-opts", "grpc-opts"} {
		delete(n.Proto, key)
	}
	if format == "clash" {
		if typ == ir.ProtocolShadowsocks {
			n.Proto["method"] = entry["cipher"]
		}
		if typ == ir.ProtocolMieru && text(entry["transport"]) != "" {
			n.Proto["transport"] = text(entry["transport"])
		}
		if typ == ir.ProtocolVMess {
			n.Proto["security"] = entry["cipher"]
			n.Proto["alter_id"] = number(entry["alterId"])
		}
		if network := text(entry["network"]); network != "" {
			n.Transport.Type = network
		}
		if n.Transport.Type == "xhttp" {
			opts := object(entry["xhttp-opts"])
			n.Transport.Path, n.Transport.Host = text(opts["path"]), text(opts["host"])
			for key, value := range opts {
				switch key {
				case "path", "host":
				case "mode":
					n.Transport.Opts[key] = value
				case "sc-max-each-post-bytes":
					n.Transport.Opts["scMaxEachPostBytes"] = value
				case "sc-min-posts-interval-ms":
					n.Transport.Opts["scMinPostsIntervalMs"] = value
				case "x-padding-bytes":
					n.Transport.Opts["xPaddingBytes"] = value
				case "x-padding-key":
					n.Transport.Opts["xPaddingKey"] = value
				default:
					return nil, fmt.Errorf("unsupported XHTTP option")
				}
			}
		}
		ws := object(entry["ws-opts"])
		if n.Transport.Type == "ws" {
			n.Transport.Path = text(ws["path"])
			n.Transport.Host = text(object(ws["headers"])["Host"])
			n.Transport.Opts["headers"] = object(ws["headers"])
		}
		if n.Transport.Type == "grpc" {
			n.Transport.Path = text(object(entry["grpc-opts"])["grpc-service-name"])
		}
		if entry["tls"] == true || typ == ir.ProtocolTrojan || typ == ir.ProtocolHysteria || typ == ir.ProtocolHysteria2 || typ == ir.ProtocolTUIC || typ == ir.ProtocolAnyTLS {
			n.Security.Type = "tls"
		}
		n.Security.SNI = text(entry["servername"])
		if n.Security.SNI == "" {
			n.Security.SNI = text(entry["sni"])
		}
		n.Security.ALPN = texts(entry["alpn"])
		n.Security.Fingerprint = text(entry["client-fingerprint"])
		if skip, ok := entry["skip-cert-verify"].(bool); ok {
			n.Security.Opts["insecure"] = skip
		}
		if reality, ok := entry["reality-opts"].(map[string]any); ok {
			n.Security.Type = "reality"
			n.Security.Opts["public_key"] = reality["public-key"]
			n.Security.Opts["short_id"] = reality["short-id"]
		}
	} else {
		transport := object(entry["transport"])
		if typ := text(transport["type"]); typ != "" {
			n.Transport.Type = typ
		}
		n.Transport.Path = text(transport["path"])
		if n.Transport.Type == "grpc" {
			n.Transport.Path = text(transport["service_name"])
		}
		n.Transport.Host = text(object(transport["headers"])["Host"])
		if n.Transport.Type == "ws" {
			n.Transport.Opts["headers"] = object(transport["headers"])
		}
		for key, value := range transport {
			if key != "type" && key != "path" && key != "headers" && key != "service_name" {
				n.Transport.Opts[key] = value
			}
		}
		tls := object(entry["tls"])
		if tls["enabled"] == true {
			n.Security.Type = "tls"
		}
		n.Security.SNI = text(tls["server_name"])
		n.Security.ALPN = texts(tls["alpn"])
		n.Security.Fingerprint = text(object(tls["utls"])["fingerprint"])
		for key, value := range tls {
			if key != "enabled" && key != "server_name" && key != "alpn" && key != "utls" {
				n.Security.Opts[key] = value
			}
		}
		if reality := object(tls["reality"]); reality["enabled"] == true {
			n.Security.Type = "reality"
			n.Security.Opts["public_key"] = reality["public_key"]
			n.Security.Opts["short_id"] = reality["short_id"]
		}
	}
	if err := validateParsedNode(n); err != nil {
		return nil, err
	}
	return n, nil
}
