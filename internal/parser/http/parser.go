package http

import (
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolHTTP }
func (Parser) CanParse(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
func (p Parser) Parse(input string) (any, error) {
	scheme := "http"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "https://") {
		scheme = "https"
	}
	u, err := parser.ParseEndpointURI(input, scheme, 0)
	if err != nil {
		return nil, err
	}
	n := parser.NewNode(ir.ProtocolHTTP, "", u)
	parser.SetName(n, u.Fragment)
	if scheme == "https" {
		n.Security.Type = "tls"
		n.Security.SNI = u.Hostname()
	}
	if u.User != nil {
		n.Proto["username"] = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			n.Proto["password"] = pass
		}
	}
	return n, nil
}
