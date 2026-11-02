package openvpn

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Parser struct{}

type remote struct {
	host    string
	port    int
	network string
}

func New() Parser { return Parser{} }

func (Parser) Protocol() string { return ir.ProtocolOpenVPN }

func (Parser) CanParse(input string) bool {
	value := strings.TrimSpace(input)
	if value == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(value), "openvpn://") {
		return true
	}
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "---") {
		return false
	}
	return looksLikeConfig(value)
}

func looksLikeConfig(value string) bool {
	hasRemote := false
	hasMarker := false
	scanner := bufio.NewScanner(strings.NewReader(value))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
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

func (Parser) Parse(input string) (any, error) {
	value := strings.TrimSpace(input)
	if strings.HasPrefix(strings.ToLower(value), "openvpn://") {
		return parseShareURI(value)
	}
	return parseConfig(value, "", "")
}

func parseShareURI(input string) (*ir.Node, error) {
	payload := input[len("openvpn://"):]
	fragment := ""
	if index := strings.LastIndex(payload, "#"); index >= 0 {
		fragment = payload[index+1:]
		payload = payload[:index]
	}
	if decoded, err := parser.DecodeBase64(payload); err == nil {
		if config := string(decoded); strings.Contains(config, "remote ") {
			name := ""
			if fragment != "" {
				if unescaped, unescapeErr := url.QueryUnescape(fragment); unescapeErr == nil {
					name = unescaped
				}
			}
			return parseConfig(config, name, input)
		}
	}
	if unescaped, err := url.QueryUnescape(payload); err == nil && strings.Contains(unescaped, "remote ") && strings.Contains(unescaped, "\n") {
		name := ""
		if fragment != "" {
			if decoded, unescapeErr := url.QueryUnescape(fragment); unescapeErr == nil {
				name = decoded
			}
		}
		return parseConfig(unescaped, name, input)
	}
	return parseSimpleURI(payload, fragment, input)
}

func parseSimpleURI(payload, fragment, source string) (*ir.Node, error) {
	u, err := url.Parse("openvpn://" + payload)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("%w: openvpn URI", parser.ErrInvalidFormat)
	}
	port := 1194
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: port", parser.ErrInvalidField)
		}
	}
	query := u.Query()
	network, err := normalizeNetwork(query.Get("proto"))
	if err != nil {
		return nil, err
	}
	name := ""
	if decoded, unescapeErr := url.QueryUnescape(fragment); unescapeErr == nil {
		name = decoded
	}
	if name == "" {
		name = query.Get("name")
	}
	if name == "" {
		name = "OpenVPN " + u.Hostname()
	}
	node := newNode(name)
	node.Server = ir.Server{Host: u.Hostname(), Port: port}
	node.Transport = ir.Transport{Type: network}
	node.Security = ir.Security{Type: "tls"}
	node.Proto["mode"] = "tls"
	node.Proto["network"] = network
	node.Proto["remotes"] = []map[string]any{{"host": u.Hostname(), "port": port, "network": network}}
	node.Source.URL = parser.SanitizedSourceURL(u)
	if username := query.Get("username"); username != "" {
		node.Proto["username"] = username
	}
	if password := query.Get("password"); password != "" {
		node.Proto["password"] = password
	}
	for _, key := range []string{"cipher", "auth", "dev", "data-ciphers", "data-ciphers-fallback"} {
		if value := query.Get(key); value != "" {
			node.Proto[strings.ReplaceAll(key, "-", "_")] = value
		}
	}
	return node, nil
}

func parseConfig(config, name, source string) (*ir.Node, error) {
	remotes, globalNetwork, options, inline, err := scanConfig(config)
	if err != nil {
		return nil, err
	}
	if len(remotes) == 0 {
		return nil, fmt.Errorf("%w: remote", parser.ErrMissingField)
	}
	first := remotes[0]
	if name == "" {
		name = "OpenVPN " + first.host
	}
	node := newNode(name)
	node.Server = ir.Server{Host: first.host, Port: first.port}
	node.Transport = ir.Transport{Type: first.network}
	mode := "tls"
	securityType := "tls"
	if _, ok := options["secret"]; ok {
		mode = "static_key"
		securityType = "static_key"
	}
	if _, ok := inline["secret"]; ok {
		mode = "static_key"
		securityType = "static_key"
	}
	node.Security = ir.Security{Type: securityType}
	node.Proto["config"] = config
	node.Proto["mode"] = mode
	node.Proto["network"] = globalNetwork
	remoteValues := make([]map[string]any, 0, len(remotes))
	for _, item := range remotes {
		remoteValues = append(remoteValues, map[string]any{"host": item.host, "port": item.port, "network": item.network})
	}
	node.Proto["remotes"] = remoteValues
	if source != "" {
		// Encoded profile URLs contain the entire config, including inline keys.
		node.Source.URL = parser.SanitizedSourceURL(&url.URL{Scheme: "openvpn", Host: net.JoinHostPort(first.host, strconv.Itoa(first.port))})
	}
	for key, values := range options {
		switch key {
		case "remote", "secret", "proto", "dev", "cipher", "auth", "data-ciphers", "data-ciphers-fallback",
			"comp-lzo", "compress", "topology", "ping", "ping-restart", "tun-mtu", "mtu",
			"verify-x509-name", "auth-user-pass", "remote-random", "route-nopull", "redirect-gateway",
			"remote-cert-tls", "key-direction", "ca", "cert", "key", "tls-auth", "tls-crypt", "tls-crypt-v2":
		default:
			continue
		}
		switch key {
		case "remote", "proto", "secret":
			continue
		case "remote-random", "route-nopull":
			node.Proto[strings.ReplaceAll(key, "-", "_")] = true
		case "auth-user-pass":
			node.Proto["auth_user_pass"] = true
			if len(values) > 0 {
				node.Proto["auth_user_pass_file"] = values[0]
			}
		case "verify-x509-name":
			node.Proto["verify_x509_name"] = strings.Join(values, " ")
		case "data-ciphers":
			node.Proto["data_ciphers"] = splitCiphers(values)
		case "tun-mtu", "mtu", "ping", "ping-restart":
			if len(values) > 0 {
				if number, err := strconv.Atoi(values[0]); err == nil {
					node.Proto[strings.ReplaceAll(key, "-", "_")] = number
				}
			}
		case "ca", "cert", "key", "tls-auth", "tls-crypt", "tls-crypt-v2":
			if len(values) > 0 {
				node.Proto[strings.ReplaceAll(key, "-", "_")+"_path"] = values[0]
			}
		default:
			if len(values) > 0 {
				node.Proto[strings.ReplaceAll(key, "-", "_")] = strings.Join(values, " ")
			}
		}
	}
	inlineKeys := map[string]string{
		"ca":           "ca",
		"cert":         "cert",
		"key":          "key",
		"tls-auth":     "tls_auth",
		"tls-crypt":    "tls_crypt",
		"tls-crypt-v2": "tls_crypt_v2",
		"secret":       "secret",
	}
	for tag, content := range inline {
		if key, ok := inlineKeys[strings.ToLower(tag)]; ok {
			node.Proto[key] = content
		}
	}
	if len(inline) > 0 {
		node.Extra["inline"] = inline
	}
	return node, nil
}

func scanConfig(config string) ([]remote, string, map[string][]string, map[string]string, error) {
	remotes := make([]remote, 0)
	options := map[string][]string{}
	inline := map[string]string{}
	globalNetwork := "udp"
	block := ""
	var blockLines []string
	scanner := bufio.NewScanner(strings.NewReader(config))
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)
		if block != "" {
			if strings.EqualFold(trimmed, "</"+block+">") {
				inline[strings.ToLower(block)] = strings.Join(blockLines, "\n")
				block = ""
				blockLines = nil
			} else {
				blockLines = append(blockLines, line)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") && !strings.HasPrefix(trimmed, "</") {
			block = strings.Trim(trimmed, "<>")
			blockLines = nil
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		fields := strings.Fields(trimmed)
		key := strings.ToLower(fields[0])
		values := fields[1:]
		options[key] = values
		if key == "proto" && len(values) > 0 {
			network, err := normalizeNetwork(values[0])
			if err != nil {
				return nil, "", nil, nil, err
			}
			globalNetwork = network
		}
		if key == "remote" {
			if len(values) == 0 || values[0] == "" {
				return nil, "", nil, nil, fmt.Errorf("%w: remote host", parser.ErrMissingField)
			}
			port := 1194
			network := ""
			if len(values) > 1 {
				parsed, err := strconv.Atoi(values[1])
				if err != nil || parsed < 1 || parsed > 65535 {
					return nil, "", nil, nil, fmt.Errorf("%w: remote port", parser.ErrInvalidField)
				}
				port = parsed
			}
			if len(values) > 2 {
				parsed, err := normalizeNetwork(values[2])
				if err != nil {
					return nil, "", nil, nil, err
				}
				network = parsed
			}
			remotes = append(remotes, remote{host: values[0], port: port, network: network})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, "", nil, nil, err
	}
	if block != "" {
		return nil, "", nil, nil, fmt.Errorf("%w: unterminated <%s> block", parser.ErrInvalidFormat, block)
	}
	for index := range remotes {
		if remotes[index].network == "" {
			remotes[index].network = globalNetwork
		}
	}
	return remotes, globalNetwork, options, inline, nil
}

func normalizeNetwork(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "udp", "udp4", "udp6":
		return "udp", nil
	case "tcp", "tcp4", "tcp6", "tcp-client", "tcp-server":
		return "tcp", nil
	default:
		return "", fmt.Errorf("%w: openvpn transport %q", parser.ErrInvalidField, value)
	}
}

func splitCiphers(values []string) []string {
	var result []string
	for _, value := range values {
		for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ':' || r == ',' }) {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func newNode(name string) *ir.Node {
	return &ir.Node{
		Name:      name,
		Protocol:  ir.ProtocolOpenVPN,
		Owner:     ir.GetProtocolOwner(ir.ProtocolOpenVPN),
		Transport: ir.Transport{Type: "udp"},
		Security:  ir.Security{Type: "tls"},
		Proto:     map[string]any{},
		Extra:     map[string]any{},
		Source:    ir.Source{Type: "import"},
	}
}
