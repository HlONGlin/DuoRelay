package openconnect

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolOpenConnect }

func (Parser) CanParse(input string) bool {
	var config map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(input)), &config) != nil {
		return false
	}
	typ, _ := config["type"].(string)
	return strings.EqualFold(typ, ir.ProtocolOpenConnect)
}

func (Parser) Parse(input string) (any, error) {
	var config map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &config); err != nil || config == nil {
		return nil, fmt.Errorf("%w: OpenConnect endpoint JSON", parser.ErrInvalidFormat)
	}
	typ, _ := config["type"].(string)
	if !strings.EqualFold(typ, ir.ProtocolOpenConnect) {
		return nil, fmt.Errorf("%w: expected OpenConnect endpoint", parser.ErrInvalidFormat)
	}
	server, _ := config["server"].(string)
	if strings.TrimSpace(server) == "" {
		return nil, fmt.Errorf("%w: server", parser.ErrMissingField)
	}
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	u, err := url.Parse(server)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("%w: server URL", parser.ErrInvalidField)
	}
	port := 443
	if rawPort := u.Port(); rawPort != "" {
		port, err = strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: server port", parser.ErrInvalidField)
		}
	}
	name, _ := config["tag"].(string)
	if name == "" {
		name = "OpenConnect " + u.Hostname()
	}
	node := &ir.Node{
		Name: name, Protocol: ir.ProtocolOpenConnect, Owner: ir.KernelSetSingBox,
		Server:    ir.Server{Host: u.Hostname(), Port: port},
		Transport: ir.Transport{Type: "tcp"}, Security: ir.Security{Type: "tls"},
		Proto: map[string]any{}, Extra: map[string]any{"sing-box": config}, Source: ir.Source{Type: "import"},
	}
	for _, key := range []string{"flavor", "username", "password", "auth_group", "cookie", "token", "reported_os", "user_agent", "version", "local_hostname", "system", "name", "mobile", "csd", "hip", "tncc", "mtu", "dpd_interval", "reconnect_timeout", "tls"} {
		if value, ok := config[key]; ok {
			node.Proto[key] = value
		}
	}
	if tls, ok := config["tls"].(map[string]any); ok {
		if sni, ok := tls["server_name"].(string); ok {
			node.Security.SNI = sni
		}
		if insecure, ok := tls["insecure"].(bool); ok {
			node.Security.Opts = map[string]any{"insecure": insecure}
		}
	}
	return node, nil
}
