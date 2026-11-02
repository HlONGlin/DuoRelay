package wireguard

import (
	"errors"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestParser(t *testing.T) {
	p := New()
	if _, e := p.Parse("wireguard://example.com:51820?private_key=p&public_key=q"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse(`{"private_key":"p","public_key":"q","server":"example.com","port":51820}`); e != nil {
		t.Fatal(e)
	}
}

func TestValidEnvelopes(t *testing.T) {
	tests := []struct {
		name, input, host string
		port              int
	}{
		{"default port", "WIREGUARD://example.com?private_key=p&public_key=q", "example.com", 51820},
		{"IPv6 URI", "wireguard://[2001:db8::1]:1234?private_key=p&public_key=q", "2001:db8::1", 1234},
		{"URI peer endpoint", "wireguard://example.com:51820?private_key=p&public_key=q&endpoint=peer.example:1234", "example.com", 51820},
		{"JSON", `{"private_key":"p","public_key":"q","server":"example.com","port":51820}`, "example.com", 51820},
		{"JSON endpoint", `{"private_key":"p","public_key":"q","endpoint":"peer.example:1234"}`, "peer.example", 1234},
		{"JSON IPv6", `{"private_key":"p","public_key":"q","server":"2001:db8::1","port":1234}`, "2001:db8::1", 1234},
		{"JSON IPv6 endpoint", `{"private_key":"p","public_key":"q","endpoint":"[2001:db8::1]:1234"}`, "2001:db8::1", 1234},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			if !p.CanParse(tt.input) {
				t.Fatal("valid envelope was not detected")
			}
			parsed, err := p.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			n := parsed.(*ir.Node)
			if n.Protocol != ir.ProtocolWireGuard || n.Server.Host != tt.host || n.Server.Port != tt.port || n.Proto["private_key"] != "p" || n.Proto["public_key"] != "q" {
				t.Fatalf("unexpected node: %+v", n)
			}
		})
	}
}

func TestJSONPreservesOptionalFields(t *testing.T) {
	parsed, err := New().Parse(`{"name":"office","private_key":"p","public_key":"q","server":"example.com","port":51820,"preshared_key":"secret","allowed_ips":["0.0.0.0/0"],"mtu":1420,"extension":{"enabled":true}}`)
	if err != nil {
		t.Fatal(err)
	}
	n := parsed.(*ir.Node)
	if n.Name != "office" || n.Proto["preshared_key"] != "secret" || n.Proto["mtu"] != float64(1420) || n.Proto["extension"].(map[string]any)["enabled"] != true || len(n.Proto["allowed_ips"].([]any)) != 1 {
		t.Fatalf("optional fields were lost: %+v", n)
	}
}

func TestRejectsMalformedAndIncompleteEnvelopes(t *testing.T) {
	tests := []struct {
		name, input string
		want        error
	}{
		{"empty input", "", parser.ErrInvalidFormat},
		{"invalid JSON", `{"private_key":`, parser.ErrInvalidFormat},
		{"trailing JSON", `{"private_key":"p"} {}`, parser.ErrInvalidFormat},
		{"JSON null", `null`, parser.ErrInvalidFormat},
		{"JSON array", `[]`, parser.ErrInvalidFormat},
		{"JSON scalar", `true`, parser.ErrInvalidFormat},
		{"empty JSON object", `{}`, parser.ErrMissingField},
		{"JSON missing private key", `{"public_key":"q","server":"example.com","port":51820}`, parser.ErrMissingField},
		{"JSON missing public key", `{"private_key":"p","server":"example.com","port":51820}`, parser.ErrMissingField},
		{"JSON blank private key", `{"private_key":" ","public_key":"q","endpoint":"example.com:51820"}`, parser.ErrMissingField},
		{"JSON null private key", `{"private_key":null,"public_key":"q"}`, parser.ErrMissingField},
		{"JSON private key type", `{"private_key":42,"public_key":"q"}`, parser.ErrInvalidField},
		{"JSON public key type", `{"private_key":"p","public_key":[]}`, parser.ErrInvalidField},
		{"JSON empty public key", `{"private_key":"p","public_key":""}`, parser.ErrMissingField},
		{"JSON missing endpoint", `{"private_key":"p","public_key":"q"}`, parser.ErrMissingField},
		{"JSON missing host", `{"private_key":"p","public_key":"q","port":51820}`, parser.ErrMissingField},
		{"JSON blank host", `{"private_key":"p","public_key":"q","server":" ","port":51820}`, parser.ErrMissingField},
		{"JSON missing port", `{"private_key":"p","public_key":"q","server":"example.com"}`, parser.ErrMissingField},
		{"JSON fractional port", `{"private_key":"p","public_key":"q","server":"example.com","port":51820.5}`, parser.ErrInvalidField},
		{"JSON zero port", `{"private_key":"p","public_key":"q","server":"example.com","port":0}`, parser.ErrInvalidField},
		{"JSON oversized port", `{"private_key":"p","public_key":"q","server":"example.com","port":65536}`, parser.ErrInvalidField},
		{"JSON string port", `{"private_key":"p","public_key":"q","server":"example.com","port":"51820"}`, parser.ErrInvalidField},
		{"JSON endpoint type", `{"private_key":"p","public_key":"q","endpoint":12}`, parser.ErrInvalidField},
		{"JSON blank endpoint", `{"private_key":"p","public_key":"q","endpoint":""}`, parser.ErrMissingField},
		{"JSON endpoint missing port", `{"private_key":"p","public_key":"q","endpoint":"example.com"}`, parser.ErrInvalidField},
		{"JSON endpoint missing host", `{"private_key":"p","public_key":"q","endpoint":":51820"}`, parser.ErrInvalidField},
		{"JSON endpoint credentials", `{"private_key":"p","public_key":"q","endpoint":"user:secret@example.com:51820"}`, parser.ErrInvalidField},
		{"URI missing keys", "wireguard://example.com:51820", parser.ErrMissingField},
		{"URI missing private key", "wireguard://example.com:51820?public_key=q", parser.ErrMissingField},
		{"URI missing public key", "wireguard://example.com:51820?private_key=p", parser.ErrMissingField},
		{"URI blank private key", "wireguard://example.com:51820?private_key=%20&public_key=q", parser.ErrMissingField},
		{"URI empty public key", "wireguard://example.com:51820?private_key=p&public_key=", parser.ErrMissingField},
		{"URI missing endpoint", "wireguard://?private_key=p&public_key=q", parser.ErrInvalidFormat},
		{"URI missing host", "wireguard://:51820?private_key=p&public_key=q", parser.ErrInvalidFormat},
		{"URI zero port", "wireguard://example.com:0?private_key=p&public_key=q", parser.ErrInvalidField},
		{"URI duplicate private key", "wireguard://example.com:51820?private_key=p&private_key=x&public_key=q", parser.ErrInvalidField},
		{"URI invalid query escape", "wireguard://example.com:51820?private_key=p&public_key=q&ignored=%zz", parser.ErrInvalidFormat},
		{"URI malformed peer endpoint", "wireguard://example.com:51820?private_key=p&public_key=q&endpoint=peer.example", parser.ErrInvalidField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := New().Parse(tt.input)
			if n != nil || !errors.Is(err, tt.want) {
				t.Fatalf("Parse() = (%+v, %v), want nil and %v", n, err, tt.want)
			}
		})
	}
}
