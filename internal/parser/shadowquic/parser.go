package shadowquic

import (
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolShadowQUIC }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "shadowquic") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "shadowquic", ir.ProtocolShadowQUIC, 443)
	if err != nil {
		return nil, err
	}
	if o.URL.User != nil {
		if _, pair := o.URL.User.Password(); pair {
			o.Credentials(true)
		} else {
			o.Password("password")
		}
	} else {
		o.Password("password")
	}
	o.TLS("h3")
	// mihomo's ShadowQUIC always verifies the certificate.
	if o.Node.Security.Opts["insecure"] == true {
		o.Invalid("skip_cert_verify unsupported by ShadowQUIC")
	}
	o.Node.Transport.Type = "quic"
	o.Node.Proto["udp"] = true
	o.Bool("udp_over_stream")
	o.Bool("zero_rtt", "zeroRTT")
	o.Bool("disable_mtu_discovery")
	o.Int("keep_alive_interval", 0, 2147483647)
	o.Int("recv_window_conn", 0, 2147483647)
	o.Int("recv_window", 0, 2147483647)
	o.Int("max_open_streams", 1, 2147483647)
	o.Int("max_datagram_frame_size", 1, 65535)
	o.Int("cwnd", 1, 2147483647)
	o.Enum("congestion_controller", "", "bbr", "bbr_meta_v1", "bbr_meta_v2", "cubic", "new_reno")
	versions := o.List("quic_versions")
	for i, v := range versions {
		switch strings.ReplaceAll(strings.ToLower(v), "_", "-") {
		case "v1", "1", "rfc9000", "rfc-9000":
			versions[i] = "v1"
		case "v2", "2", "rfc9369", "rfc-9369":
			versions[i] = "v2"
		default:
			o.Invalid("quic_versions")
		}
	}
	return o.Finish()
}
