package naive

import (
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolNaive }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "naive+https") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "naive+https", ir.ProtocolNaive, 443)
	if err != nil {
		return nil, err
	}
	o.Credentials(true)
	o.TLS()
	o.Int("insecure_concurrency", 0, 65535)
	if o.Bool("quic") {
		o.Node.Transport.Type = "quic"
	}
	o.Enum("quic_congestion_control", "", "bbr", "bbr2", "cubic", "reno")
	return o.Finish()
}
