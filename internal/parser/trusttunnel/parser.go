package trusttunnel

import (
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolTrustTunnel }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "trusttunnel") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "trusttunnel", ir.ProtocolTrustTunnel, 443)
	if err != nil {
		return nil, err
	}
	o.Credentials(true)
	if o.Bool("quic") {
		o.Node.Transport.Type = "quic"
		o.TLS("h3")
	} else {
		o.TLS("h2")
	}
	o.Bool("udp")
	o.Bool("health_check")
	o.Int("max_connections", 0, 2147483647)
	min, hasMin := o.Int("min_streams", 0, 2147483647)
	max, hasMax := o.Int("max_streams", 0, 2147483647)
	if hasMin && hasMax && max > 0 && min > max {
		o.Invalid("min_streams > max_streams")
	}
	o.Enum("congestion_controller", "", "bbr", "cubic", "new_reno")
	o.Int("cwnd", 1, 2147483647)
	return o.Finish()
}
