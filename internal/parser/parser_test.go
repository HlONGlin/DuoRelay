package parser_test

import (
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/defaults"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  parser.Format
	}{
		{"uri", "ss://YWVzLTI1Ni1nY206cGFzcw==@example.com:443", parser.FormatURI},
		{"json", `{"protocol":"vmess"}`, parser.FormatJSON},
		{"yaml", "proxies:\n  - name: test", parser.FormatYAML},
		{"base64", "c3M6Ly9leGFtcGxl", parser.FormatBase64},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.DetectFormat(tt.input); got != tt.want {
				t.Fatalf("DetectFormat() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultRegistryParsesAll18Samples(t *testing.T) {
	r := defaults.New()
	samples := []struct {
		protocol string
		links    []string
	}{
		{ir.ProtocolShadowsocks, []string{
			"ss://YWVzLTI1Ni1nY206dGVzdC1wYXNzd29yZA==@example.com:8388#Test-SS-Node",
			"ss://YWVzLTI1Ni1nY206dGVzdC1wYXNzd29yZA==@example.com:8388?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dwww.bing.com#SS-Obfs",
			"ss://MjAyMi1ibGFrZTMtYWVzLTI1Ni1nY206dGVzdC1rZXk=@test.example.com:8388#SS-2022",
		}},
		{ir.ProtocolVMess, []string{
			"vmess://eyJ2IjoiMiIsInBzIjoiVGVzdC1WTWVzcyIsImFkZCI6ImV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiYjhjYTU3YzItZWIzOS00OTFhLTg0MjUtYmJhODI4ZmJhZDJkIiwiYWlkIjoiMCIsInNjeSI6ImF1dG8iLCJuZXQiOiJ0Y3AiLCJ0eXBlIjoibm9uZSIsImhvc3QiOiIiLCJwYXRoIjoiIiwidGxzIjoidGxzIiwic25pIjoiIiwiYWxwbiI6IiJ9",
			"vmess://eyJ2IjoiMiIsInBzIjoiVk1lc3MtV1MiLCJhZGQiOiJ3cy5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6ImI4Y2E1N2MyLWViMzktNDkxYS04NDI1LWJiYTgyOGZiYWQyZCIsImFpZCI6IjAiLCJzY3kiOiJhdXRvIiwibmV0Ijoid3MiLCJ0eXBlIjoibm9uZSIsImhvc3QiOiJ3cy5leGFtcGxlLmNvbSIsInBhdGgiOiIvdm1lc3MiLCJ0bHMiOiJ0bHMiLCJzbmkiOiJ3cy5leGFtcGxlLmNvbSIsImFscG4iOiIifQ==",
			"vmess://eyJ2IjoiMiIsInBzIjoiVk1lc3MtZ1JQQyIsImFkZCI6ImdycGMuZXhhbXBsZS5jb20iLCJwb3J0IjoiNDQzIiwiaWQiOiJiOGNhNTdjMi1lYjM5LTQ5MWEtODQyNS1iYmE4MjhmYmFkMmQiLCJhaWQiOiIwIiwic2N5IjoiYXV0byIsIm5ldCI6ImdycGMiLCJ0eXBlIjoiZ3VuIiwiaG9zdCI6ImdycGMuZXhhbXBsZS5jb20iLCJwYXRoIjoidm1lc3MtZ3JwYyIsInRscyI6InRscyIsInNuaSI6ImdycGMuZXhhbXBsZS5jb20iLCJhbHBuIjoiIn0=",
		}},
		{ir.ProtocolVLESS, []string{
			"vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@example.com:443?security=tls&sni=example.com&type=tcp#VLESS-TLS",
			"vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@ws.example.com:443?type=ws&security=tls&path=%2Fvless&host=ws.example.com&sni=ws.example.com#VLESS-WS",
			"vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@reality.example.com:443?security=reality&sni=www.microsoft.com&pbk=test-public-key&sid=test-short-id&type=tcp&flow=xtls-rprx-vision#VLESS-Reality",
		}},
		{ir.ProtocolTrojan, []string{
			"trojan://test-password@example.com:443?sni=example.com#Trojan-TLS",
			"trojan://test-password@ws.example.com:443?type=ws&path=%2Ftrojan&host=ws.example.com&sni=ws.example.com#Trojan-WS",
			"trojan://test-password@grpc.example.com:443?type=grpc&serviceName=trojan-grpc&sni=grpc.example.com#Trojan-gRPC",
		}},
		{ir.ProtocolHysteria2, []string{
			"hysteria2://test-password@example.com:443?sni=example.com#Hysteria2-Basic",
			"hysteria2://test-password@example.com:443?sni=example.com&obfs=salamander&obfs-password=obfs-secret#Hysteria2-Obfs",
			"hysteria2://test-password@example.com:10000-20000?sni=example.com#Hysteria2-Ports",
		}},
		{ir.ProtocolTUIC, []string{
			"tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&sni=example.com#TUIC-v5",
			"tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@example.com:443?congestion_control=cubic&alpn=h3&sni=example.com#TUIC-ALPN",
			"tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@test.example.com:8443#TUIC-Minimal",
		}},
	}
	for _, group := range samples {
		p, err := r.Get(group.protocol)
		if err != nil {
			t.Fatalf("Get(%s): %v", group.protocol, err)
		}
		for i, link := range group.links {
			n, err := p.Parse(link)
			if err != nil {
				t.Errorf("%s sample %d: %v", group.protocol, i+1, err)
				continue
			}
			node, ok := n.(*ir.Node)
			if !ok {
				t.Errorf("%s sample %d returned %T, want *ir.Node", group.protocol, i+1, n)
				continue
			}
			if node.Protocol != group.protocol || node.Server.Host == "" || node.Server.Port == 0 {
				t.Errorf("%s sample %d produced invalid node: %+v", group.protocol, i+1, node)
			}
		}
	}
}

func TestParserRejectsMalformedInput(t *testing.T) {
	r := defaults.New()
	for _, tc := range []struct{ proto, input string }{
		{ir.ProtocolShadowsocks, "ss://not-a-valid-user@example.com:8388"},
		{ir.ProtocolVLESS, "vless://@example.com:443"},
		{ir.ProtocolTrojan, "trojan://@example.com:443"},
		{ir.ProtocolTUIC, "tuic://uuid@example.com:443"},
	} {
		p, _ := r.Get(tc.proto)
		if _, err := p.Parse(tc.input); err == nil {
			t.Errorf("%s accepted malformed input", tc.proto)
		}
	}
}

func TestDefaultRegistryParseAny(t *testing.T) {
	r := defaults.New()
	if _, err := r.ParseAny("vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@example.com:443#node"); err != nil {
		t.Fatal(err)
	}
}

func TestHelpersAndRegistryErrors(t *testing.T) {
	if parser.DetectFormat("") != parser.FormatUnknown {
		t.Fatal("empty format")
	}
	if parser.DetectFormat("not-base64!") != parser.FormatUnknown {
		t.Fatal("unknown format")
	}
	if _, err := parser.ParseShareURI("http://example.com:443", "ss"); err == nil {
		t.Fatal("wrong scheme accepted")
	}
	if _, err := parser.ParseShareURI("ss://example.com", "ss"); err == nil {
		t.Fatal("missing port accepted")
	}
	if _, err := parser.ParseShareURI("ss://example.com:0", "ss"); err == nil {
		t.Fatal("invalid port accepted")
	}
	if err := parser.Require("", "field"); err == nil {
		t.Fatal("missing required value accepted")
	}
	if _, err := parser.DecodeBase64("!"); err == nil {
		t.Fatal("invalid base64 accepted")
	}
	r := parser.NewRegistry()
	if _, err := r.Get("missing"); err == nil {
		t.Fatal("missing parser accepted")
	}
	if _, err := r.ParseAny("unknown://example.com:443"); err == nil {
		t.Fatal("unknown protocol accepted")
	}
}
