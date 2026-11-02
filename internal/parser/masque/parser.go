package masque

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"net/netip"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

type Parser struct{}

func New() Parser                         { return Parser{} }
func (Parser) Protocol() string           { return ir.ProtocolMASQUE }
func (Parser) CanParse(input string) bool { return uriopts.CanParse(input, "masque") }

func (Parser) Parse(input string) (any, error) {
	o, err := uriopts.Open(input, "masque", ir.ProtocolMASQUE, 443)
	if err != nil {
		return nil, err
	}
	o.Node.Owner = ir.KernelSetMihomo // Correct the legacy IR ownership for this kernel version.
	if o.URL.User != nil {
		o.Invalid("MASQUE uses EC keys, not userinfo")
	}
	private := o.String("private_key")
	public := o.String("public_key")
	o.Required(private, "private_key")
	o.Required(public, "public_key")
	if private != "" {
		data, decodeErr := base64.StdEncoding.DecodeString(private)
		if _, parseErr := x509.ParseECPrivateKey(data); decodeErr != nil || parseErr != nil {
			o.Invalid("private_key (base64 SEC1 EC)")
		}
	}
	if public != "" {
		data, decodeErr := base64.StdEncoding.DecodeString(public)
		key, parseErr := x509.ParsePKIXPublicKey(data)
		if _, ok := key.(*ecdsa.PublicKey); decodeErr != nil || parseErr != nil || !ok {
			o.Invalid("public_key (base64 PKIX ECDSA)")
		}
	}
	network := o.Enum("network", "h3", "h3", "h2", "h3-l4proxy")
	if network == "h2" {
		o.TLS("h2")
	} else {
		o.Node.Transport.Type = "quic"
		o.TLS("h3")
	}
	// Empty SNI lets the generator use mihomo's MASQUE service default,
	// which is distinct from the IP endpoint for common deployments.
	o.Node.Security.SNI = o.Value("sni", "servername", "peer")
	ip, ipv6 := o.String("ip"), o.String("ipv6")
	if network != "h3-l4proxy" && ip == "" && ipv6 == "" {
		o.Required("", "ip/ipv6")
	}
	for field, value := range map[string]string{"ip": ip, "ipv6": ipv6} {
		if value == "" {
			continue
		}
		var addr netip.Addr
		var parseErr error
		if strings.Contains(value, "/") {
			prefix, e := netip.ParsePrefix(value)
			addr, parseErr = prefix.Addr(), e
		} else {
			addr, parseErr = netip.ParseAddr(value)
		}
		if parseErr != nil || (field == "ip" && !addr.Is4()) || (field == "ipv6" && !addr.Is6()) {
			o.Invalid(field)
		}
	}
	o.String("uri")
	o.Int("mtu", 1280, 65535)
	o.Int("handshake_timeout", 0, 2147483647)
	o.Int("cwnd", 1, 2147483647)
	o.Enum("congestion_controller", "", "bbr", "cubic", "new_reno")
	o.Bool("udp")
	o.Bool("remote_dns_resolve")
	o.List("dns")
	return o.Finish()
}
