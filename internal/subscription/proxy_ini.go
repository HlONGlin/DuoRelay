package subscription

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

func proxyType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ss", "shadowsocks":
		return "ss"
	case "ssr", "shadowsocksr":
		return "ssr"
	case "vmess", "vless", "trojan", "http", "https", "socks5", "socks5-tls", "snell", "anytls", "hysteria2", "tuic":
		return strings.ToLower(strings.TrimSpace(value))
	}
	return ""
}

func looksLikeINI(content []byte) bool {
	for _, line := range strings.Split(strings.ToLower(string(content)), "\n") {
		line = strings.TrimSpace(line)
		if line == "[proxy]" || line == "[server_local]" {
			return true
		}
		left, right, ok := strings.Cut(line, "=")
		if ok && strings.Contains(right, ",") && (proxyType(left) != "" || proxyType(strings.SplitN(right, ",", 2)[0]) != "") {
			return true
		}
	}
	return false
}

// splitProxyFields keeps commas inside quoted credentials/header blocks intact.
func splitProxyFields(value string) ([]string, error) {
	var fields []string
	var quote rune
	depth, start := 0, 0
	escaped := false
	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if r == '\\' {
				escaped = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("invalid option braces")
			}
		case ',':
			if depth == 0 {
				fields = append(fields, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	if quote != 0 || depth != 0 {
		return nil, fmt.Errorf("unclosed quoted option")
	}
	return append(fields, strings.TrimSpace(value[start:])), nil
}

func unquoteOption(value string) string {
	value = strings.TrimSpace(value)
	if decoded, err := strconv.Unquote(value); err == nil {
		return decoded
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func (m *Manager) parseProxyINI(content []byte) ([]*ir.Node, error) {
	nodes := []*ir.Node{}
	section := ""
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			continue
		}
		if section != "" && section != "[proxy]" && section != "[server_local]" {
			continue
		}
		left, right, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected proxy definition", i+1)
		}
		fields, err := splitProxyFields(right)
		if err != nil {
			return nil, fmt.Errorf("line %d: malformed proxy options", i+1)
		}
		typ := proxyType(left)
		qx := typ != "" && strings.Contains(fields[0], ":")
		if !qx {
			typ = proxyType(fields[0])
		}
		if typ == "" {
			switch strings.ToLower(fields[0]) {
			case "direct", "reject", "reject-drop":
				continue
			}
			return nil, fmt.Errorf("line %d: unsupported proxy type", i+1)
		}
		entry := map[string]any{"name": unquoteOption(left), "type": typ}
		start := 3
		if qx {
			host, port, err := net.SplitHostPort(fields[0])
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid proxy endpoint", i+1)
			}
			entry["server"], entry["port"] = host, port
			start = 1
		} else {
			if len(fields) < 3 {
				return nil, fmt.Errorf("line %d: missing server or port", i+1)
			}
			entry["server"], entry["port"] = unquoteOption(fields[1]), fields[2]
		}
		opts := map[string]string{}
		var positional []string
		for _, field := range fields[start:] {
			if strings.HasPrefix(field, "\"") || strings.HasPrefix(field, "'") {
				positional = append(positional, unquoteOption(field))
				continue
			}
			key, value, ok := strings.Cut(field, "=")
			if !ok {
				positional = append(positional, unquoteOption(field))
				continue
			}
			key = strings.ToLower(strings.TrimSpace(key))
			if _, exists := opts[key]; exists {
				return nil, fmt.Errorf("line %d: duplicate proxy option", i+1)
			}
			opts[key] = unquoteOption(value)
		}
		allowedOptions := " password username uuid psk flow encrypt-method method tls over-tls skip-cert-verify udp-relay udp tls-verification sni tls-host tls-name client-fingerprint alpn obfs ws transport ws-path path host obfs-uri ws-headers obfs-host protocol protocol-param obfs-param version alter-id tag fast-open tfo udp-over-tcp aead "
		for key, value := range opts {
			if !strings.Contains(allowedOptions, " "+key+" ") {
				return nil, fmt.Errorf("line %d: unsupported proxy option", i+1)
			}
			if key == "transport" && value != "tcp" && value != "ws" {
				return nil, fmt.Errorf("line %d: unsupported proxy transport", i+1)
			}
			if (key == "udp-over-tcp" && value != "false") || (key == "aead" && value != "true") {
				return nil, fmt.Errorf("line %d: unsupported legacy transport option", i+1)
			}
		}
		if qx {
			entry["name"] = opts["tag"]
		}
		if (typ == "ss" || typ == "ssr" || typ == "vmess") && len(positional) >= 2 {
			opts["encrypt-method"], opts["password"] = positional[0], positional[1]
		} else if len(positional) == 1 {
			opts["password"] = positional[0]
		}
		for _, key := range []string{"password", "username", "uuid", "psk", "flow"} {
			if opts[key] != "" {
				entry[key] = opts[key]
			}
		}
		if typ == "vmess" || typ == "vless" {
			entry["uuid"] = firstText(entry, "uuid", "username", "password")
			delete(entry, "password")
		}
		entry["cipher"] = firstNonBlank(opts["encrypt-method"], opts["method"], "auto")
		if typ == "https" {
			entry["type"], entry["tls"] = "http", true
		}
		if typ == "socks5-tls" {
			entry["type"], entry["tls"] = "socks5", true
		}
		if qx && typ == "http" && opts["over-tls"] == "true" {
			entry["tls"] = true
		}
		for _, pair := range [][2]string{{"tls", "tls"}, {"over-tls", "tls"}, {"skip-cert-verify", "skip-cert-verify"}, {"udp-relay", "udp"}, {"udp", "udp"}} {
			if value, ok := opts[pair[0]]; ok {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return nil, fmt.Errorf("line %d: invalid boolean option", i+1)
				}
				entry[pair[1]] = b
			}
		}
		if value, ok := opts["tls-verification"]; ok {
			b, err := strconv.ParseBool(value)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid TLS verification option", i+1)
			}
			entry["skip-cert-verify"] = !b
		}
		entry["servername"] = firstNonBlank(opts["sni"], opts["tls-host"], opts["tls-name"])
		entry["client-fingerprint"] = opts["client-fingerprint"]
		if opts["alpn"] != "" {
			entry["alpn"] = strings.Split(opts["alpn"], ",")
		}
		obfs := strings.ToLower(opts["obfs"])
		if opts["ws"] == "true" || opts["transport"] == "ws" || obfs == "ws" || obfs == "wss" {
			entry["network"] = "ws"
			headers := map[string]any{}
			for _, header := range strings.Split(strings.Trim(opts["ws-headers"], "{}"), "|") {
				key, value, ok := strings.Cut(header, ":")
				if ok {
					headers[strings.TrimSpace(key)] = strings.TrimSpace(value)
				}
			}
			if opts["obfs-host"] != "" {
				headers["Host"] = opts["obfs-host"]
			}
			if opts["host"] != "" {
				headers["Host"] = opts["host"]
			}
			entry["ws-opts"] = map[string]any{"path": firstNonBlank(opts["ws-path"], opts["path"], opts["obfs-uri"], "/"), "headers": headers}
			if obfs == "wss" {
				entry["tls"] = true
				if entry["servername"] == "" {
					entry["servername"] = opts["obfs-host"]
				}
			}
			if typ == "ss" {
				entry["plugin"] = "v2ray-plugin"
				entry["plugin_opts"] = "mode=websocket;host=" + text(headers["Host"]) + ";path=" + firstNonBlank(opts["ws-path"], opts["path"], opts["obfs-uri"], "/")
				if entry["tls"] == true {
					entry["plugin_opts"] = text(entry["plugin_opts"]) + ";tls"
				}
				delete(entry, "network")
				delete(entry, "tls")
				delete(entry, "ws-opts")
			}
		} else if obfs == "over-tls" {
			entry["tls"] = true
			entry["servername"] = firstNonBlank(text(entry["servername"]), opts["obfs-host"])
		} else if typ == "ssr" {
			// SSR obfuscation is mapped below without a SIP003 plugin.
		} else if obfs == "http" || obfs == "tls" {
			if typ != "ss" {
				return nil, fmt.Errorf("line %d: unsupported obfuscation for proxy type", i+1)
			}
			entry["plugin"], entry["plugin_opts"] = "obfs-local", "obfs="+obfs+";obfs-host="+opts["obfs-host"]
		} else if obfs != "" && obfs != "none" {
			return nil, fmt.Errorf("line %d: unsupported obfuscation", i+1)
		}
		if typ == "ssr" {
			entry["protocol"], entry["protocol-param"] = opts["protocol"], opts["protocol-param"]
			entry["obfs"], entry["obfs-param"] = opts["obfs"], opts["obfs-param"]
		}
		if typ == "snell" {
			entry["version"] = number(opts["version"])
		}
		if typ == "tuic" {
			entry["uuid"] = opts["uuid"]
		}
		if opts["alter-id"] != "" {
			entry["alterId"] = number(opts["alter-id"])
		} else {
			entry["alterId"] = 0
		}
		if err := requireProxyCredentials(entry); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		node, err := nativeNode(entry, "clash")
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid proxy endpoint", i+1)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func requireProxyCredentials(entry map[string]any) error {
	typ := text(entry["type"])
	if (typ == "ss" || typ == "ssr" || typ == "trojan" || typ == "anytls" || typ == "hysteria2" || typ == "tuic") && firstText(entry, "password") == "" {
		return fmt.Errorf("missing proxy password")
	}
	if (typ == "vmess" || typ == "vless" || typ == "tuic") && firstText(entry, "uuid") == "" {
		return fmt.Errorf("missing proxy UUID")
	}
	if typ == "snell" && firstText(entry, "psk") == "" {
		return fmt.Errorf("missing proxy PSK")
	}
	if (typ == "ss" || typ == "ssr") && firstText(entry, "cipher") == "auto" {
		return fmt.Errorf("missing proxy cipher")
	}
	return nil
}
