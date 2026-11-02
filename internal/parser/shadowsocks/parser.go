package shadowsocks

import (
	"fmt"
	"net/url"
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
	input = strings.TrimSpace(input)
	if !p.CanParse(input) {
		return nil, parser.ErrInvalidFormat
	}
	rest := input[len("ss://"):]
	end := len(rest)
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		end = i
	}
	if !strings.Contains(rest[:end], "@") {
		raw, err := parser.DecodeBase64(rest[:end])
		if err != nil {
			return nil, fmt.Errorf("%w: SS payload", parser.ErrInvalidFormat)
		}
		at := strings.LastIndexByte(string(raw), '@')
		if at < 0 {
			return nil, fmt.Errorf("%w: SS authority", parser.ErrInvalidFormat)
		}
		credentials, endpoint := string(raw[:at]), string(raw[at+1:])
		method, password, ok := strings.Cut(credentials, ":")
		if !ok {
			return nil, parser.ErrInvalidFormat
		}
		input = "ss://" + url.UserPassword(method, password).String() + "@" + endpoint + rest[end:]
	}
	u, err := parser.ParseShareURI(input, "ss")
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		return nil, fmt.Errorf("%w: method/password", parser.ErrMissingField)
	}
	user := u.User.Username()
	password, plain := u.User.Password()
	if plain {
		user += ":" + password
	} else if raw, err := parser.DecodeBase64(user); err == nil {
		user = string(raw)
	}
	parts := strings.SplitN(user, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: method/password", parser.ErrInvalidFormat)
	}
	n := parser.NewNode(ir.ProtocolShadowsocks, "", u)
	if parts[0] == "" || parts[1] == "" {
		return nil, parser.ErrMissingField
	}
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
