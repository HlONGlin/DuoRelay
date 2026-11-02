package ssr

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolShadowsocksR }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "ssr://")
}
func (p Parser) Parse(input string) (any, error) {
	raw := strings.TrimSpace(input)
	payload := strings.TrimPrefix(strings.TrimPrefix(raw, "ssr://"), "SSR://")
	decoded, err := parser.DecodeBase64(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: ssr payload", parser.ErrInvalidFormat)
	}
	parts := strings.SplitN(string(decoded), "/?", 2)
	base := strings.Split(parts[0], ":")
	if len(base) < 6 {
		return nil, fmt.Errorf("%w: ssr fields", parser.ErrInvalidFormat)
	}
	port := base[1]
	host := base[0]
	uinput := "ss://x@" + host + ":" + port
	u, err := parser.ParseShareURI(uinput, "ss")
	if err != nil {
		return nil, err
	}
	n := parser.NewNode(ir.ProtocolShadowsocksR, "", u)
	n.Server.Host = host
	fmt.Sscanf(port, "%d", &n.Server.Port)
	n.Proto["protocol"], n.Proto["method"], n.Proto["obfs"] = decodeField(base[2]), decodeField(base[3]), decodeField(base[4])
	n.Proto["password"] = decodeField(base[5])
	if len(parts) == 2 {
		for _, item := range strings.Split(parts[1], "&") {
			kv := strings.SplitN(item, "=", 2)
			if len(kv) == 2 {
				n.Proto[kv[0]] = decodeField(kv[1])
			}
		}
	}
	return n, nil
}
func decodeField(v string) string {
	if b, e := parser.DecodeBase64(v); e == nil {
		return string(b)
	}
	return v
}
