// Package mieru supports DuoRelay's endpoint URI envelope. The subscription
// importer handles native mieru:// protobuf and multi-binding mierus:// links.
package mieru

import (
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolMieru }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "mieru") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "mieru", ir.ProtocolMieru, 0)
	if err != nil {
		return nil, err
	}
	o.Credentials(true)
	transport := o.Enum("transport", "tcp", "tcp", "udp")
	o.Node.Transport.Type = transport
	o.Node.Proto["transport"] = strings.ToUpper(transport)
	if v := strings.TrimPrefix(strings.ToUpper(o.Value("multiplexing")), "MULTIPLEXING_"); v != "" {
		switch v {
		case "DEFAULT", "OFF", "LOW", "MIDDLE", "HIGH":
			o.Node.Proto["multiplexing"] = "MULTIPLEXING_" + v
		default:
			o.Invalid("multiplexing")
		}
	}
	if v := strings.TrimPrefix(strings.ToUpper(o.Value("handshake_mode")), "HANDSHAKE_"); v != "" {
		switch v {
		case "DEFAULT", "STANDARD", "NO_WAIT":
			o.Node.Proto["handshake_mode"] = "HANDSHAKE_" + v
		default:
			o.Invalid("handshake_mode")
		}
	}
	o.Bool("udp")
	return o.Finish()
}
