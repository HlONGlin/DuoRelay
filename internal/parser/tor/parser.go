// Package tor imports an existing Tor SOCKS5 endpoint, not a Tor process.
package tor

import (
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolTor }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "tor+socks5") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "tor+socks5", ir.ProtocolTor, 9050)
	if err != nil {
		return nil, err
	}
	o.Credentials(false)
	o.Node.Proto["mode"], o.Node.Proto["version"] = "socks5", "5"
	if o.Bool("udp") {
		o.Invalid("Tor SOCKS does not support UDP")
	}
	o.Node.Proto["udp"] = false
	return o.Finish()
}
