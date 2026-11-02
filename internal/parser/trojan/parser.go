package trojan

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolTrojan }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "trojan://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseShareURI(input, "trojan")
	if err != nil {
		return nil, err
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("%w: password", parser.ErrMissingField)
	}
	n := parser.NewNode(ir.ProtocolTrojan, "", u)
	parser.SetName(n, u.Fragment)
	n.Proto["password"] = u.User.Username()
	n.Proto["udp"] = true
	if err := parser.SetTransport(n, u.Query()); err != nil {
		return nil, err
	}
	parser.SetTLS(n, u.Query())
	n.Security.Type = "tls"
	return n, nil
}
