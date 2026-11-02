package snell

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolSnell }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "snell://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseEndpointURI(input, "snell", 443)
	if err != nil {
		return nil, err
	}
	n := parser.NewNode(ir.ProtocolSnell, "", u)
	parser.SetName(n, u.Fragment)
	q := u.Query()
	user := ""
	if u.User != nil {
		user = u.User.Username()
	}
	n.Proto["psk"] = first(user, q.Get("psk"))
	if n.Proto["psk"] == "" {
		return nil, fmt.Errorf("%w: psk", parser.ErrMissingField)
	}
	n.Proto["version"] = first(q.Get("version"), "3")
	if q.Get("obfs") != "" {
		n.Proto["obfs"] = q.Get("obfs")
	}
	return n, nil
}
func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
