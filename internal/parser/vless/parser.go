package vless

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolVLESS }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "vless://")
}
func (p Parser) Parse(input string) (any, error) {
	u, err := parser.ParseShareURI(input, "vless")
	if err != nil {
		return nil, err
	}
	if u.User == nil {
		return nil, fmt.Errorf("%w: uuid", parser.ErrMissingField)
	}
	uuid := u.User.Username()
	if err := parser.Require(uuid, "uuid"); err != nil {
		return nil, err
	}
	n := parser.NewNode(ir.ProtocolVLESS, "", u)
	parser.SetName(n, u.Fragment)
	q := u.Query()
	if q.Get("pqv") != "" {
		return nil, fmt.Errorf("%w: Reality pqv is not supported by the pinned kernels", parser.ErrUnsupportedFeature)
	}
	if err := parser.SetTransport(n, q); err != nil {
		return nil, err
	}
	if n.Transport.Type == "xhttp" {
		// XHTTP is currently rendered by mihomo only; keep the node importable
		// while preventing a sing-box config that cannot represent it.
		n.Owner = ir.KernelSetMihomo
	}
	parser.SetTLS(n, q)
	n.Proto["uuid"], n.Proto["encryption"], n.Proto["flow"] = uuid, first(q.Get("encryption"), "none"), q.Get("flow")
	if n.Security.Type == "reality" {
		n.Security.Opts["public_key"], n.Security.Opts["short_id"] = q.Get("pbk"), q.Get("sid")
	}
	return n, nil
}
func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
