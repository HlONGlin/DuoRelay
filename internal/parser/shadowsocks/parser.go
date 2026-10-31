package shadowsocks

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolShadowsocks }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "ss://")
}

func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseShareURI(input, "ss")
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		return nil, fmt.Errorf("%w: method/password", parser.ErrMissingField)
	}
	user := u.User.Username()
	raw, err := parser.DecodeBase64(user)
	if err == nil {
		user = string(raw)
	}
	parts := strings.SplitN(user, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: method/password", parser.ErrInvalidFormat)
	}
	n := parser.NewNode(ir.ProtocolShadowsocks, "", u)
	parser.SetName(n, u.Fragment)
	n.Proto["method"], n.Proto["password"], n.Proto["udp"] = parts[0], parts[1], true
	if plugin := u.Query().Get("plugin"); plugin != "" {
		fields := strings.Split(plugin, ";")
		n.Proto["plugin"] = fields[0]
		if len(fields) > 1 {
			n.Proto["plugin_opts"] = strings.Join(fields[1:], ";")
		}
	}
	return n, nil
}
