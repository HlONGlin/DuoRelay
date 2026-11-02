package ssh

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolSSH }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "ssh://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseEndpointURI(input, "ssh", 22)
	if err != nil {
		return nil, err
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("%w: username", parser.ErrMissingField)
	}
	n := parser.NewNode(ir.ProtocolSSH, "", u)
	parser.SetName(n, u.Fragment)
	n.Proto["username"] = u.User.Username()
	if pass, ok := u.User.Password(); ok {
		n.Proto["password"] = pass
	}
	q := u.Query()
	if q.Get("private_key") != "" {
		n.Proto["private_key"] = q.Get("private_key")
	}
	return n, nil
}
