package parser_test

import (
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/defaults"
	"testing"
)

func TestBProtocolParsers(t *testing.T) {
	r := defaults.New()
	tests := []struct{ name, proto, input string }{
		{"socks5", ir.ProtocolSOCKS5, "socks5://user:pass@example.com:1080#socks"},
		{"http", ir.ProtocolHTTP, "https://user:pass@example.com:443#https"},
		{"ssr", ir.ProtocolShadowsocksR, "ssr://ZXhhbXBsZS5jb206ODM4ODphdXRvOmFlcy0yNTYtY2ZiOm9iZnM6cGFzc3dvcmQ="},
		{"wireguard", ir.ProtocolWireGuard, "wireguard://example.com:51820?private_key=priv&public_key=pub&allowed_ips=0.0.0.0%2F0#wg"},
		{"snell", ir.ProtocolSnell, "snell://psk@example.com:443?version=4&obfs=http#snell"},
		{"hysteria", ir.ProtocolHysteria, "hysteria://auth@example.com:443?upmbps=20&downmbps=100&sni=example.com#hy"},
		{"ssh", ir.ProtocolSSH, "ssh://user:pass@example.com:22#ssh"},
		{"shadowtls", ir.ProtocolShadowTLS, "shadowtls://password@example.com:443?version=3&host=www.example.com&sni=www.example.com#st"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := r.Get(tt.proto)
			if err != nil {
				t.Fatal(err)
			}
			n, err := p.Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			node, ok := n.(*ir.Node)
			if !ok || node.Protocol != tt.proto {
				t.Fatalf("got %T %+v", n, n)
			}
		})
	}
}
