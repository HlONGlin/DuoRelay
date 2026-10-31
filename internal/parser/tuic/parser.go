package tuic

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolTUIC }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "tuic://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseShareURI(input, "tuic")
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		return nil, fmt.Errorf("%w: uuid/password", parser.ErrMissingField)
	}
	uuid := u.User.Username()
	pass, _ := u.User.Password()
	if pass == "" {
		return nil, fmt.Errorf("%w: password", parser.ErrMissingField)
	}
	n := parser.NewNode(ir.ProtocolTUIC, "", u)
	parser.SetName(n, u.Fragment)
	q := u.Query()
	n.Proto["uuid"], n.Proto["password"], n.Proto["udp"] = uuid, pass, true
	if v := q.Get("congestion_control"); v != "" {
		n.Proto["congestion_control"] = v
	}
	if v := q.Get("udp_relay_mode"); v != "" {
		n.Proto["udp_relay_mode"] = v
	}
	parser.SetTLS(n, q)
	return n, nil
}
