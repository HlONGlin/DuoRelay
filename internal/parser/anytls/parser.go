package anytls

import (
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolAnyTLS }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "anytls") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "anytls", ir.ProtocolAnyTLS, 443)
	if err != nil {
		return nil, err
	}
	o.Password("password")
	o.TLS()
	// Intervals are seconds in this import envelope, as in mihomo options.
	o.Int("idle_session_check_interval", 0, 2147483647)
	o.Int("idle_session_timeout", 0, 2147483647)
	o.Int("min_idle_session", 0, 2147483647)
	o.String("client_metadata")
	o.Bool("disable_reuse")
	o.Node.Proto["udp"] = true
	return o.Finish()
}
