package hysteria

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolHysteria }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "hysteria://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseEndpointURI(input, "hysteria", 443)
	if err != nil {
		return nil, err
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("%w: auth", parser.ErrMissingField)
	}
	n := parser.NewNode(ir.ProtocolHysteria, "", u)
	parser.SetName(n, u.Fragment)
	q := u.Query()
	n.Proto["auth"], n.Proto["udp"] = u.User.Username(), true
	if q.Get("upmbps") != "" {
		n.Proto["up_mbps"] = q.Get("upmbps")
	}
	if q.Get("downmbps") != "" {
		n.Proto["down_mbps"] = q.Get("downmbps")
	}
	parser.SetTLS(n, q)
	return n, nil
}
