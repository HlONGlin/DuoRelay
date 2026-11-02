package socks5

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolSOCKS5 }
func (Parser) CanParse(input string) bool {
	s := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(s, "socks5://") || strings.HasPrefix(s, "socks://")
}
func (p Parser) Parse(input string) (any, error) {
	scheme := "socks5"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "socks://") {
		scheme = "socks"
	}
	u, err := parser.ParseEndpointURI(input, scheme, 1080)
	if err != nil {
		return nil, err
	}
	n := parser.NewNode(ir.ProtocolSOCKS5, "", u)
	parser.SetName(n, u.Fragment)
	n.Proto["udp"] = true
	if u.User != nil {
		n.Proto["username"] = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			n.Proto["password"] = pass
		}
	}
	if n.Server.Host == "" {
		return nil, fmt.Errorf("%w: server", parser.ErrMissingField)
	}
	return n, nil
}
