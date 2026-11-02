package shadowtls

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolShadowTLS }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "shadowtls://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseEndpointURI(input, "shadowtls", 443)
	if err != nil {
		return nil, err
	}
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("%w: password", parser.ErrMissingField)
	}
	n := parser.NewNode(ir.ProtocolShadowTLS, "", u)
	parser.SetName(n, u.Fragment)
	q := u.Query()
	n.Proto["password"] = u.User.Username()
	n.Proto["version"] = first(q.Get("version"), "3")
	n.Proto["host"] = first(q.Get("host"), q.Get("handshake_server"))
	if n.Proto["host"] == "" {
		n.Proto["host"] = u.Hostname()
	}
	n.Security.Type = "tls"
	n.Security.SNI = first(q.Get("sni"), u.Hostname())
	return n, nil
}
func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
