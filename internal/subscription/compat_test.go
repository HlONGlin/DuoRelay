package subscription

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf16"

	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

const sampleUUID = "b8ca57c2-eb39-491a-8425-bba828fbad2d"
const sipExample = `{"version":1,"servers":[{"remarks":"SIP node","server":"ss.example","server_port":8388,"method":"aes-256-gcm","password":"p@ss:+#","plugin":"obfs-local","plugin_opts":"obfs=http;obfs-host=cdn.example"}]}`
const xrayExample = `{"outbounds":[{"protocol":"freedom"},{"tag":"reality grpc","protocol":"vless","settings":{"vnext":[{"address":"vless.example","port":443,"users":[{"id":"` + sampleUUID + `","encryption":"none","flow":""}]}]},"streamSettings":{"network":"grpc","grpcSettings":{"serviceName":"grpc-service"},"security":"reality","realitySettings":{"serverName":"cdn.example","publicKey":"test-key","shortId":"aabb","fingerprint":"chrome"}}}]}`
const nativeMieruFixture = "mieru://CjMKBm5hdGl2ZRINCgVhbGljZRIEcGFzcxoWEg1taWVydS5leGFtcGxlGgUIuBcQAioCCAQ="

func TestSubscriptionCompatibility(t *testing.T) {
	m := NewManager()
	vmess := "vmess://" + base64.StdEncoding.EncodeToString([]byte(v2rayNOne))
	legacySS := "ss://" + base64.RawStdEncoding.EncodeToString([]byte("aes-256-gcm:p@ss:[x]@ss.example:8388")) + "#SS%2B%2520"
	mixed := strings.Join([]string{"# v2rayN link subscription", vmess, "vless://" + sampleUUID + "@vless.example:443?type=grpc&serviceName=svc&security=reality&pbk=key&sid=aa&fp=chrome#VLESS", "trojan://pass@trojan.example:443#Trojan", legacySS, "HY2://user:password@hy.example:443?insecure=1#HY2", "anytls://pass@any.example:443#AnyTLS", "mierus://alice:pass@mieru.example?profile=home&port=3000-3010&protocol=TCP&port=4000&protocol=UDP"}, "\r\n")
	for _, tc := range []struct {
		name, body string
		count      int
		check      func(*testing.T, []*ir.Node)
	}{
		{"v2rayn-mixed", mixed, 8, func(t *testing.T, nodes []*ir.Node) {
			if nodes[1].Transport.Path != "svc" || nodes[1].Security.Fingerprint != "chrome" {
				t.Fatal("lost gRPC or fingerprint")
			}
			if nodes[2].Security.Type != "tls" || nodes[4].Security.Type != "tls" || nodes[4].Proto["password"] != "user:password" || nodes[4].Security.Opts["insecure"] != true {
				t.Fatal("lost TLS or hy2 credentials")
			}
			if nodes[3].Name != "SS+%20" || nodes[3].Proto["password"] != "p@ss:[x]" {
				t.Fatal("SS credentials or fragment decoded incorrectly")
			}
			if nodes[6].Proto["port_range"] != "3000-3010" || nodes[7].Proto["transport"] != "UDP" {
				t.Fatal("lost Mieru bindings")
			}
		}},
		{"sip008", sipExample, 1, func(t *testing.T, nodes []*ir.Node) {
			if nodes[0].Proto["plugin_opts"] != "obfs=http;obfs-host=cdn.example" {
				t.Fatal("lost SIP008 plugin")
			}
		}},
		{"sip008-array", `[{"server":"ss.example","server_port":8388,"method":"aes-256-gcm","password":"pass"}]`, 1, nil},
		{"ssd", "ssd://" + base64.StdEncoding.EncodeToString([]byte(`{"port":8388,"encryption":"aes-256-gcm","password":"default","servers":[{"server":"one.example","remarks":"one"},{"server":"two.example","port":8389,"password":"override"}]}`)), 2, func(t *testing.T, nodes []*ir.Node) {
			if nodes[1].Server.Port != 8389 || nodes[1].Proto["password"] != "override" {
				t.Fatal("lost SSD overrides")
			}
		}},
		{"surge", "[General]\nloglevel=notify\n[Proxy]\nDirect = direct\nOffice = ss, ss.example, 8388, encrypt-method=aes-256-gcm, password=\"p,ass\", obfs=http, obfs-host=cdn.example\nWS = vmess, vmess.example, 443, username=" + sampleUUID + ", ws=true, ws-path=/ws, ws-headers=Host:cdn.example|Origin:https://cdn.example, tls=true, skip-cert-verify=false\n[Rule]\nFINAL,Direct", 2, func(t *testing.T, nodes []*ir.Node) {
			if nodes[0].Proto["password"] != "p,ass" || nodes[1].Proto["uuid"] != sampleUUID || nodes[1].Transport.Host != "cdn.example" {
				t.Fatal("lost Surge credentials/headers")
			}
		}},
		{"loon", "[Proxy]\nSS = Shadowsocks, ss.example, 8388, aes-256-gcm, \"p=ass,word\"\nVM = Vmess, vm.example, 443, auto, \"" + sampleUUID + "\", transport=ws, path=/ws, host=cdn.example, tls=true, tls-name=cdn.example", 2, func(t *testing.T, nodes []*ir.Node) {
			if nodes[0].Proto["password"] != "p=ass,word" || nodes[1].Security.SNI != "cdn.example" {
				t.Fatal("Loon quoted credentials or SNI lost")
			}
		}},
		{"quantumult-x", "[server_local]\nshadowsocks=ss.example:8388, method=aes-256-gcm, password=pass, obfs=tls, obfs-host=cdn.example, tag=SS\nvmess=vm.example:443, method=chacha20-poly1305, password=" + sampleUUID + ", obfs=wss, obfs-host=cdn.example, obfs-uri=/ws, tls-verification=true, tag=VM\n[filter_local]\nfinal,direct", 2, func(t *testing.T, nodes []*ir.Node) {
			if nodes[1].Security.Type != "tls" || nodes[1].Transport.Path != "/ws" || nodes[1].Security.Opts["insecure"] != false {
				t.Fatal("lost Quantumult X TLS/WS")
			}
		}},
		{"xray", xrayExample, 1, func(t *testing.T, nodes []*ir.Node) {
			if nodes[0].Transport.Path != "grpc-service" || nodes[0].Security.Type != "reality" || nodes[0].Security.Opts["public_key"] != "test-key" {
				t.Fatal("lost Xray stream settings")
			}
		}},
		{"native-mieru", nativeMieruFixture, 1, func(t *testing.T, nodes []*ir.Node) {
			if nodes[0].Server.Port != 3000 || nodes[0].Proto["transport"] != "TCP" || nodes[0].Proto["multiplexing"] != "MULTIPLEXING_HIGH" {
				t.Fatal("incorrect native protobuf profile")
			}
		}},
		{"vmess-endpoint", "vmess://auto:" + sampleUUID + "@vm.example:443?type=ws&path=%2Fws&security=tls", 1, nil},
		{"clash-proxy-array", `[{"name":"AnyTLS","type":"anytls","server":"any.example","port":443,"password":"pass"}]`, 1, nil},
		{"sing-box-outbound-array", `[{"type":"anytls","tag":"AnyTLS","server":"any.example","server_port":443,"password":"pass","tls":{"enabled":true}}]`, 1, nil},
		{"v2rayn-typed-array", `[{"type":"vmess","address":"vm.example","port":443,"id":"` + sampleUUID + `"}]`, 1, nil},
		{"ss-plain", "ss://aes-256-gcm:p%40ss%3Aword@ss.example:8388", 1, func(t *testing.T, nodes []*ir.Node) {
			if nodes[0].Proto["password"] != "p@ss:word" {
				t.Fatal("lost plain SS password")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, encoding := range []struct {
				name string
				body []byte
			}{{"plain", []byte(tc.body)}, {"base64", []byte(base64.RawURLEncoding.EncodeToString([]byte(tc.body)))}, {"double-base64", []byte(base64.StdEncoding.EncodeToString([]byte(base64.StdEncoding.EncodeToString([]byte(tc.body)))))}} {
				t.Run(encoding.name, func(t *testing.T) {
					nodes, err := m.Parse(encoding.body)
					if err != nil || len(nodes) != tc.count {
						t.Fatalf("got %d nodes, want %d; error: %v", len(nodes), tc.count, err)
					}
					if tc.check != nil {
						tc.check(t, nodes)
					}
				})
			}
		})
	}
}

func TestNativeMieruFragmentProvidesNodeName(t *testing.T) {
	m := NewManager()
	nodes, err := m.Parse([]byte(nativeMieruFixture + "#Provider%20Mieru"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Provider Mieru" {
		t.Fatalf("native Mieru name = %q, want Provider Mieru", nodes[0].Name)
	}
}

func TestMieruProfileNameIsNotExpandedForSingleBinding(t *testing.T) {
	m := NewManager()
	nodes, err := m.Parse([]byte("mierus://alice:pass@54.255.193.12?profile=default&port=37065&protocol=TCP"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "default" {
		t.Fatalf("Mieru profile name = %q, want default", nodes[0].Name)
	}
	nodes, err = m.Parse([]byte("mierus://alice:pass@54.255.193.12?profile=default&port=37065&protocol=TCP&port=37066&protocol=UDP"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Name == "default" || nodes[1].Name == "default" || !strings.Contains(nodes[0].Name, "37065") || !strings.Contains(nodes[1].Name, "37066") {
		t.Fatalf("multi-binding names were not disambiguated: %#v", nodes)
	}
}

func TestMieruShareLinkUsesFragmentName(t *testing.T) {
	m := NewManager()
	nodes, err := m.Parse([]byte("mierus://m772535f6:adhPCqX4NGN5Znk4kuFt@54.255.193.12?handshake-mode=HANDSHAKE_NO_WAIT&mtu=1400&multiplexing=MULTIPLEXING_OFF&port=37065&profile=default&protocol=TCP&traffic-pattern=CIXu%2BasFEAAiCAgBEAEYBCAIKgUIABCAATICCAA%3D#%E6%96%B0%E5%8A%A0%E5%9D%A11%28aws%29"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "新加坡1(aws)" {
		t.Fatalf("Mieru share-link name = %q, want 新加坡1(aws)", nodes[0].Name)
	}
}

func TestClashMieruNameIsPreserved(t *testing.T) {
	m := NewManager()
	content := []byte("proxies:\n- name: 新加坡1(aws)\n  type: mieru\n  server: 54.255.193.12\n  username: user\n  password: pass\n  transport: TCP\n  port: 37065\n")
	nodes, err := m.Parse(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "新加坡1(aws)" {
		t.Fatalf("Clash Mieru name = %q, want 新加坡1(aws)", nodes[0].Name)
	}
}

func TestSubscriptionEncodingsAndFailures(t *testing.T) {
	m := NewManager()
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		words := utf16.Encode([]rune("# 中文\r" + shareLink))
		body := make([]byte, 2+len(words)*2)
		order.PutUint16(body, 0xfeff)
		for i, word := range words {
			order.PutUint16(body[2+i*2:], word)
		}
		if nodes, err := m.Parse(body); err != nil || len(nodes) != 1 {
			t.Fatalf("UTF-16: %v", err)
		}
	}
	if nodes, err := m.Parse([]byte(" \xef\xbb\xbf" + shareLink)); err != nil || len(nodes) != 1 {
		t.Fatalf("BOM: %v", err)
	}
	for _, body := range []string{
		`{"outbounds":"malformed"}`, `{"outbounds":null}`, `{"servers":[{"server":"ss.example","server_port":0,"method":"aes-256-gcm","password":"secret"}]}`,
		"[Proxy]\nBad = ss, host, 8388, password=\"secret", "[Proxy]\nBad = vmess, host, 443, username=" + sampleUUID + ", tls=bad",
		"mierus://alice:secret@host?profile=a&port=3000&protocol=TCP&port=4000", "mierus://alice:secret@host?profile=a&port=3000-99999&protocol=TCP", "mieru://truncated", "ssd://garbage",
		shareLink + "\nunknown://secret@host:443", "hy2://secret@host:99999", "ss://aes-256-gcm:@host:8388",
	} {
		nodes, err := m.Parse([]byte(body))
		if err == nil && len(nodes) != 0 {
			t.Fatalf("invalid document accepted")
		}
		if err != nil && strings.Contains(err.Error(), "secret") {
			t.Fatalf("credentials leaked in error")
		}
	}
	for _, body := range [][]byte{{0x80}, {0x42, 0xff}, {0x00}, {0x08, 0x80}, {0x0a, 0x02, 0x01}} {
		if _, err := readWire(body); err == nil {
			t.Fatal("malformed protobuf accepted")
		}
	}
}

func TestImportToGeneratorPreservesProtocolOptions(t *testing.T) {
	m := NewManager()
	nodes, err := m.Parse([]byte(xrayExample))
	if err != nil {
		t.Fatal(err)
	}
	config, err := singbox.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sb struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if json.Unmarshal(config, &sb) != nil {
		t.Fatal("bad sing-box output")
	}
	found := false
	for _, out := range sb.Outbounds {
		if out["type"] == "vless" {
			found = true
			if object(out["transport"])["service_name"] != "grpc-service" || object(object(out["tls"])["reality"])["public_key"] != "test-key" {
				t.Fatal("generated gRPC/Reality differs from imported node")
			}
		}
	}
	if !found {
		t.Fatal("VLESS endpoint missing from generated config")
	}
	nodes, err = m.Parse([]byte("mierus://alice:pass@mieru.example?profile=range&port=3000-3010&protocol=TCP"))
	if err != nil {
		t.Fatal(err)
	}
	config, err = mihomo.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	var mh struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if yaml.Unmarshal(config, &mh) != nil || len(mh.Proxies) != 1 || mh.Proxies[0]["port-range"] != "3000-3010" || mh.Proxies[0]["port"] != nil {
		t.Fatal("generated Mieru range differs from imported binding")
	}
	nodes, err = m.Parse([]byte("[Proxy]\nWS = vmess, vm.example, 443, username=" + sampleUUID + ", ws=true, ws-headers=Host:cdn.example|Origin:https://cdn.example, tls=true"))
	if err != nil {
		t.Fatal(err)
	}
	config, err = mihomo.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if yaml.Unmarshal(config, &mh) != nil || object(object(mh.Proxies[0]["ws-opts"])["headers"])["Origin"] != "https://cdn.example" {
		t.Fatal("mihomo dropped Origin header")
	}
	config, err = singbox.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(config, &sb) != nil {
		t.Fatal("bad sing-box output")
	}
	found = false
	for _, out := range sb.Outbounds {
		if out["type"] == "vmess" {
			found = true
			if object(object(out["transport"])["headers"])["Origin"] != "https://cdn.example" {
				t.Fatal("sing-box dropped Origin header")
			}
		}
	}
	if !found {
		t.Fatal("VMess node missing")
	}
}

func TestHTTPSubscriptionCompatibilityAndAtomicFailure(t *testing.T) {
	m := newTestManager(t)
	var body atomic.Value
	body.Store(base64.StdEncoding.EncodeToString([]byte("anytls://pass@any.example:443#AnyTLS\n" + nativeMieruFixture)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body.Load().(string))) }))
	defer server.Close()
	ctx := context.Background()
	sub := &Subscription{Name: "compatibility", URL: server.URL}
	if err := m.Add(ctx, sub); err != nil {
		t.Fatal(err)
	}
	nodes, err := m.Update(ctx, sub.ID)
	if err != nil || len(nodes) != 2 {
		t.Fatalf("HTTP Base64 subscription: %v", err)
	}
	body.Store("anytls://pass@any.example:443#AnyTLS\nunknown://secret@unknown:443")
	if _, err := m.Update(ctx, sub.ID); err == nil {
		t.Fatal("partial refresh accepted")
	}
	saved, err := m.Store.Nodes(ctx, sub.ID)
	if err != nil || len(saved) != 2 {
		t.Fatal("failed parse removed previous nodes")
	}
	status, _ := m.Get(ctx, sub.ID)
	if status.LastStatus != "error" || strings.Contains(status.LastError, "secret") {
		t.Fatal("bad failure status")
	}
}
