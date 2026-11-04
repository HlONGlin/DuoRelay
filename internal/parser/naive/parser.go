package naive

import (
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolNaive }
func (Parser) CanParse(input string) bool {
	value := strings.ToLower(strings.TrimSpace(input))
	return uriopts.CanParse(value, "naive+https") || uriopts.CanParse(value, "naive+quic")
}

func (Parser) Parse(input string) (any, error) {
	scheme := "naive+https"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "naive+quic://") {
		scheme = "naive+quic"
	}
	o, err := uriopts.Open(input, scheme, ir.ProtocolNaive, 443)
	if err != nil {
		return nil, err
	}
	o.Credentials(true)
	o.TLS()
	o.Int("insecure_concurrency", 0, 65535)
	quic := o.Bool("quic")
	if scheme == "naive+quic" || quic {
		o.Node.Transport.Type = "quic"
	}
	o.Enum("quic_congestion_control", "", "bbr", "bbr2", "cubic", "reno")
	return o.Finish()
}
