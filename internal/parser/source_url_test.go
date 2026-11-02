package parser_test

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/defaults"
	"github.com/holi/duorelay/internal/parser/uriopts"
)

func TestNewNodeSanitizesSourceURLWithoutMutatingInput(t *testing.T) {
	for _, input := range []string{
		"ssh://alice:password@example.com:22/private-key?private_key=secret&token=token#fragment-secret",
		"trojan://password@example.com:443?password=query-secret&unknown=credential#fragment-secret",
		"wireguard://[2001:db8::1]:51820?private_key=secret&public_key=public#fragment-secret",
		"https://alice:password@example.com:443?access%5Ftoken=secret&malformed=%zz#fragment-secret",
	} {
		t.Run(input, func(t *testing.T) {
			u, err := url.Parse(input)
			if err != nil {
				t.Fatal(err)
			}
			original := u.String()
			n := parser.NewNode("test", "test", u)
			want := (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
			if n.Source.URL != want || u.String() != original {
				t.Fatalf("source = %q, want %q; original URL mutated: %v", n.Source.URL, want, u.String() != original)
			}
		})
	}
	if parser.SanitizedSourceURL(nil) != "" {
		t.Fatal("nil source must remain empty")
	}
	if got := parser.SanitizedSourceURL(&url.URL{Scheme: "test", Opaque: "private-key", Fragment: "token"}); got != "test:" {
		t.Fatalf("opaque source leaked: %q", got)
	}
}

func TestURIOptionsPreservesCredentialsOutsideSource(t *testing.T) {
	o, err := uriopts.Open("naive://alice:password@example.com:443?token=secret&private_key=key#office", "naive", "naive", 443)
	if err != nil {
		t.Fatal(err)
	}
	o.Credentials(true)
	if o.Node.Source.URL != "naive://example.com:443" || o.Node.Name != "office" || o.Node.Proto["username"] != "alice" || o.Node.Proto["password"] != "password" || o.Value("token") != "secret" || o.Value("private_key") != "key" {
		t.Fatalf("source was not sanitized or parsing inputs changed: %+v", o.Node)
	}
}

func TestSourceURLAcrossProtocolParsers(t *testing.T) {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	tests := []struct{ protocol, input, source, field, value string }{
		{"shadowsocks", "ss://" + encode("aes-256-gcm:secret-password") + "@example.com:8388?token=query-token#office", "ss://example.com:8388", "password", "secret-password"},
		{"vmess", "vmess://" + encode(`{"add":"example.com","port":"443","id":"secret-uuid","net":"tcp","ps":"office"}`), "vmess://example.com:443", "uuid", "secret-uuid"},
		{"vless", "vless://secret-uuid@example.com:443?token=query-token#office", "vless://example.com:443", "uuid", "secret-uuid"},
		{"trojan", "trojan://secret-password@example.com:443?token=query-token#office", "trojan://example.com:443", "password", "secret-password"},
		{"hysteria2", "hysteria2://secret-password@example.com:443?obfs=salamander&obfs-password=obfs-secret#office", "hysteria2://example.com:443", "password", "secret-password"},
		{"tuic", "tuic://secret-uuid:secret-password@example.com:443?token=query-token#office", "tuic://example.com:443", "password", "secret-password"},
		{"socks5", "socks5://alice:secret-password@example.com:1080?token=query-token#office", "socks5://example.com:1080", "password", "secret-password"},
		{"http", "http://alice:secret-password@example.com:8080?token=query-token#office", "http://example.com:8080", "password", "secret-password"},
		{"http", "https://alice:secret-password@example.com:443?token=query-token#office", "https://example.com:443", "password", "secret-password"},
		{"shadowsocksr", "ssr://" + encode("example.com:8388:origin:aes-256-cfb:plain:"+encode("secret-password")), "ss://example.com:8388", "password", "secret-password"},
		{"wireguard", "wireguard://example.com:51820?private_key=secret-private-key&public_key=public-key&preshared_key=secret-psk#office", "wireguard://example.com:51820", "private_key", "secret-private-key"},
		{"snell", "snell://secret-psk@example.com:443?version=4&token=query-token#office", "snell://example.com:443", "psk", "secret-psk"},
		{"hysteria", "hysteria://secret-auth@example.com:443?token=query-token#office", "hysteria://example.com:443", "auth", "secret-auth"},
		{"ssh", "ssh://alice:secret-password@example.com:22?private_key=secret-private-key#office", "ssh://example.com:22", "private_key", "secret-private-key"},
		{"shadowtls", "shadowtls://secret-password@example.com:443?version=3&token=query-token#office", "shadowtls://example.com:443", "password", "secret-password"},
		{"openvpn", "openvpn://alice:secret-userinfo@example.com:443?proto=tcp&username=alice&password=secret-password&token=query-token#office", "openvpn://example.com:443", "password", "secret-password"},
		{"openconnect", `{"type":"openconnect","server":"vpn.example","username":"alice","password":"secret-password"}`, "", "password", "secret-password"},
	}
	r := defaults.New()
	for _, tt := range tests {
		t.Run(tt.protocol+"/"+tt.source, func(t *testing.T) {
			p, err := r.Get(tt.protocol)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := p.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			n := parsed.(*ir.Node)
			if n.Source.URL != tt.source || n.Proto[tt.field] != tt.value {
				t.Fatalf("source = %q, want %q; credential %s = %v, want %q", n.Source.URL, tt.source, tt.field, n.Proto[tt.field], tt.value)
			}
		})
	}
}

func TestSourceURLAcrossURIOptionsParsers(t *testing.T) {
	r := defaults.New()
	for _, protocol := range []string{"anytls", "masque", "mieru", "naive", "shadowquic", "sudoku", "tor", "trusttunnel"} {
		t.Run(protocol, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/samples/" + protocol + "_samples.json")
			if err != nil {
				t.Fatal(err)
			}
			var samples []struct{ Link string }
			if err := json.Unmarshal(data, &samples); err != nil || len(samples) == 0 {
				t.Fatalf("invalid samples: %v", err)
			}
			p, err := r.Get(protocol)
			if err != nil {
				t.Fatal(err)
			}
			for _, sample := range samples {
				parsed, err := p.Parse(sample.Link)
				if err != nil {
					t.Fatal(err)
				}
				n := parsed.(*ir.Node)
				u, err := url.Parse(sample.Link)
				if err != nil {
					t.Fatal(err)
				}
				want := (&url.URL{Scheme: u.Scheme, Host: net.JoinHostPort(n.Server.Host, strconv.Itoa(n.Server.Port))}).String()
				if n.Source.URL != want {
					t.Fatalf("source = %q, want endpoint %q", n.Source.URL, want)
				}
			}
		})
	}
}

func TestOpenVPNEncodedProfilesDoNotLeakSource(t *testing.T) {
	config := "client\nremote vpn.example 1194\n<key>\nsecret-private-key\n</key>\n<tls-crypt>\nsecret-token\n</tls-crypt>"
	encodings := map[string]string{
		"base64":         base64.StdEncoding.EncodeToString([]byte(config)),
		"raw URL base64": base64.RawURLEncoding.EncodeToString([]byte(config)),
		"escaped":        url.QueryEscape(config),
	}
	p, err := defaults.New().Get(ir.ProtocolOpenVPN)
	if err != nil {
		t.Fatal(err)
	}
	for name, payload := range encodings {
		t.Run(name, func(t *testing.T) {
			parsed, err := p.Parse("openvpn://" + payload + "#office")
			if err != nil {
				t.Fatal(err)
			}
			n := parsed.(*ir.Node)
			if n.Source.URL != "openvpn://vpn.example:1194" || n.Name != "office" || n.Proto["key"] != "secret-private-key" || n.Proto["tls_crypt"] != "secret-token" || n.Proto["config"] != config {
				t.Fatalf("encoded profile leaked or parsing changed: %+v", n)
			}
		})
	}
}
