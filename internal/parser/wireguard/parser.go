package wireguard

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolWireGuard }
func (Parser) CanParse(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(s, "wireguard://") || strings.HasPrefix(s, "{") && strings.Contains(s, "private_key")
}
func (p Parser) Parse(input string) (any, error) {
	s := strings.TrimSpace(input)
	if strings.HasPrefix(strings.ToLower(s), "wireguard://") {
		u, err := parser.ParseEndpointURI(s, "wireguard", 51820)
		if err != nil {
			return nil, err
		}
		n := parser.NewNode(ir.ProtocolWireGuard, "", u)
		parser.SetName(n, u.Fragment)
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return nil, fmt.Errorf("%w: wireguard query", parser.ErrInvalidFormat)
		}
		for _, k := range []string{"private_key", "public_key", "preshared_key", "allowed_ips", "endpoint"} {
			if len(q[k]) > 1 {
				return nil, fmt.Errorf("%w: duplicate %s", parser.ErrInvalidField, k)
			}
			if v := q.Get(k); v != "" {
				n.Proto[k] = v
			}
		}
		if err := requireKeys(n.Proto); err != nil {
			return nil, err
		}
		if _, err := endpoint(u.Host); err != nil {
			return nil, err
		}
		if value := q.Get("endpoint"); value != "" {
			if _, err := endpoint(value); err != nil {
				return nil, err
			}
		}
		return n, nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(s), &raw); err != nil || raw == nil {
		return nil, fmt.Errorf("%w: wireguard config", parser.ErrInvalidFormat)
	}
	if err := requireKeys(raw); err != nil {
		return nil, err
	}
	n := &ir.Node{Protocol: ir.ProtocolWireGuard, Owner: ir.GetProtocolOwner(ir.ProtocolWireGuard), Proto: raw, Extra: map[string]any{}, Source: ir.Source{Type: "import"}}
	if v, ok := raw["name"].(string); ok {
		n.Name = v
	}
	if _, exists := raw["endpoint"]; exists {
		v, err := requiredString(raw, "endpoint")
		if err != nil {
			return nil, err
		}
		n.Server, err = endpoint(v)
		if err != nil {
			return nil, err
		}
	}
	_, hasServer := raw["server"]
	_, hasPort := raw["port"]
	if hasServer || hasPort || n.Server.Host == "" {
		host, err := requiredString(raw, "server")
		if err != nil {
			return nil, err
		}
		value, exists := raw["port"]
		if !exists {
			return nil, fmt.Errorf("%w: port", parser.ErrMissingField)
		}
		port, ok := value.(float64)
		if !ok || port < 1 || port > 65535 || port != float64(int(port)) {
			return nil, fmt.Errorf("%w: port", parser.ErrInvalidField)
		}
		n.Server, err = endpoint(net.JoinHostPort(host, strconv.Itoa(int(port))))
		if err != nil {
			return nil, err
		}
	}
	return n, nil
}

// The supported envelopes describe a single peer with top-level key fields.
// Keep optional JSON fields intact, but reject incomplete imported nodes.
func requireKeys(raw map[string]any) error {
	for _, key := range []string{"private_key", "public_key"} {
		if _, err := requiredString(raw, key); err != nil {
			return err
		}
	}
	return nil
}

func requiredString(raw map[string]any, key string) (string, error) {
	value, exists := raw[key]
	if !exists || value == nil {
		return "", fmt.Errorf("%w: %s", parser.ErrMissingField, key)
	}
	v, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %s", parser.ErrInvalidField, key)
	}
	if err := parser.Require(v, key); err != nil {
		return "", err
	}
	return v, nil
}

func endpoint(value string) (ir.Server, error) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil || strings.TrimSpace(host) == "" || strings.ContainsAny(host, " /\\@?#\t\r\n") || strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return ir.Server{}, fmt.Errorf("%w: endpoint", parser.ErrInvalidField)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return ir.Server{}, fmt.Errorf("%w: endpoint port", parser.ErrInvalidField)
	}
	return ir.Server{Host: host, Port: port}, nil
}
