//go:build !lite

package singbox

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	anytlsparser "github.com/holi/duorelay/internal/parser/anytls"
	httpparser "github.com/holi/duorelay/internal/parser/http"
	hysteriaparser "github.com/holi/duorelay/internal/parser/hysteria"
	hysteria2parser "github.com/holi/duorelay/internal/parser/hysteria2"
	naiveparser "github.com/holi/duorelay/internal/parser/naive"
	shadowtlsparser "github.com/holi/duorelay/internal/parser/shadowtls"
	snellparser "github.com/holi/duorelay/internal/parser/snell"
	socks5parser "github.com/holi/duorelay/internal/parser/socks5"
	sshparser "github.com/holi/duorelay/internal/parser/ssh"
	torparser "github.com/holi/duorelay/internal/parser/tor"
	tuicparser "github.com/holi/duorelay/internal/parser/tuic"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func goldenPath(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "golden", name)
}

func kernelBinary() string {
	if runtime.GOOS == "windows" {
		return "sing-box.exe"
	}
	return "sing-box"
}

func assertGolden(t *testing.T, got []byte, name string) {
	t.Helper()

	path := goldenPath(name)
	if updateGolden {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("generated config does not match %s\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

func TestGenerateShadowsocks(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNode()}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertGolden(t, generated, "singbox-ss-basic.json")
}

func TestTUNEnablesStrictDualStackRoutingAndDNSHijack(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.EnableTUN = true
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNode()}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var config struct {
		Inbounds []map[string]any `json:"inbounds"`
		Route    map[string]any   `json:"route"`
	}
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	var tun map[string]any
	for _, inbound := range config.Inbounds {
		if inbound["type"] == "tun" {
			tun = inbound
		}
	}
	if tun == nil || tun["strict_route"] != true || tun["dns_mode"] != "hijack" {
		t.Fatalf("TUN hardening missing: %#v", tun)
	}
	addresses, ok := tun["address"].([]any)
	if !ok || len(addresses) != 2 {
		t.Fatalf("dual-stack TUN addresses missing: %#v", tun["address"])
	}
	if config.Route["auto_detect_interface"] != true {
		t.Fatalf("TUN route does not detect the outbound interface: %#v", config.Route)
	}
	rules, _ := config.Route["rules"].([]any)
	if len(rules) == 0 || rules[0].(map[string]any)["action"] != "hijack-dns" {
		t.Fatalf("DNS hijack rule missing or ordered after user routes: %#v", rules)
	}
	if runtime.GOOS == "linux" && tun["auto_redirect"] != true {
		t.Fatalf("Linux TUN auto redirect is disabled: %#v", tun)
	}
}

func TestGenerateVMessWebSocket(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleVMessNode()}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertGolden(t, generated, "singbox-vmess-ws.json")
}

func TestGenerateShadowsocksWithPlugin(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNodeWithPlugin()}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}

	outbounds, ok := config["outbounds"].([]any)
	if !ok {
		t.Fatalf("outbounds missing from generated config")
	}

	proxy := findOutbound(t, outbounds, "US-SS-Obfs")
	if proxy["plugin"] != "obfs-local" {
		t.Errorf("plugin = %v, want obfs-local", proxy["plugin"])
	}
	if proxy["plugin_opts"] != "obfs=http;obfs-host=www.bing.com" {
		t.Errorf("plugin_opts = %v", proxy["plugin_opts"])
	}
}

func TestGenerateFiltersNodesForOtherKernel(t *testing.T) {
	node := &ir.Node{
		Name:     "mihomo-only",
		Protocol: ir.ProtocolShadowsocksR,
		Owner:    ir.KernelSetMihomo,
		Server:   ir.Server{Host: "example.com", Port: 443},
	}

	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if strings.Contains(string(generated), "mihomo-only") {
		t.Errorf("generated config contains node owned by another kernel")
	}
}

func TestGenerateRejectsMalformedSnell(t *testing.T) {
	node := &ir.Node{
		Name: "malformed-snell", Protocol: ir.ProtocolSnell, Owner: ir.KernelSetBoth,
		Server: ir.Server{Host: "example.com", Port: 443},
	}
	if _, err := NewGenerator().Generate([]*ir.Node{node}, nil); err == nil || !strings.Contains(err.Error(), ir.ProtocolSnell) {
		t.Fatalf("Generate error = %v, want unsupported protocol error containing %q", err, ir.ProtocolSnell)
	}
}

func TestGenerateSnell(t *testing.T) {
	parsed, err := snellparser.New().Parse("snell://sample-psk@example.com:443?version=4&obfs=http#Snell")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbounds := config["outbounds"].([]any)
	outbound := findOutbound(t, outbounds, "Snell")
	if outbound["type"] != "snell" || outbound["psk"] != "sample-psk" || outbound["version"] != float64(4) {
		t.Fatalf("unexpected Snell outbound: %#v", outbound)
	}
	if outbound["obfs_mode"] != "http" {
		t.Fatalf("obfs_mode = %v, want http", outbound["obfs_mode"])
	}
}

func TestGenerateSnellPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	parsed, err := snellparser.New().Parse("snell://sample-psk@example.com:443?version=4&obfs=http#Snell")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box Snell check failed: %v\n%s", err, output)
	}
}

func TestGenerateSOCKS5AndHTTPS(t *testing.T) {
	socksNode, err := socks5parser.New().Parse("socks5://user:pass@example.com:1080#SOCKS5")
	if err != nil {
		t.Fatal(err)
	}
	httpNode, err := httpparser.New().Parse("https://user:pass@example.com:8443#HTTPS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{socksNode.(*ir.Node), httpNode.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbounds := config["outbounds"].([]any)
	socks := findOutbound(t, outbounds, "SOCKS5")
	if socks["type"] != "socks" || socks["version"] != "5" || socks["username"] != "user" || socks["password"] != "pass" {
		t.Fatalf("unexpected SOCKS5 outbound: %#v", socks)
	}
	http := findOutbound(t, outbounds, "HTTPS")
	if http["type"] != "http" || http["username"] != "user" || http["password"] != "pass" {
		t.Fatalf("unexpected HTTPS outbound: %#v", http)
	}
	if tls, ok := http["tls"].(map[string]any); !ok || tls["enabled"] != true || tls["server_name"] != "example.com" {
		t.Fatalf("unexpected HTTPS TLS: %#v", http["tls"])
	}
}

func TestGenerateSOCKS5AndHTTPSPassKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	socksNode, err := socks5parser.New().Parse("socks5://user:pass@example.com:1080#SOCKS5")
	if err != nil {
		t.Fatal(err)
	}
	httpNode, err := httpparser.New().Parse("https://user:pass@example.com:8443#HTTPS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{socksNode.(*ir.Node), httpNode.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box SOCKS5/HTTPS check failed: %v\n%s", err, output)
	}
}

func TestGenerateSSH(t *testing.T) {
	parsed, err := sshparser.New().Parse("ssh://alice:secret@example.com:22#SSH")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbound := findOutbound(t, config["outbounds"].([]any), "SSH")
	if outbound["type"] != "ssh" || outbound["user"] != "alice" || outbound["password"] != "secret" {
		t.Fatalf("unexpected SSH outbound: %#v", outbound)
	}
}

func TestGenerateSSHPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	parsed, err := sshparser.New().Parse("ssh://alice:secret@example.com:22#SSH")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box SSH check failed: %v\n%s", err, output)
	}
}

func wireGuardTestNode() *ir.Node {
	return &ir.Node{
		Name:     "WireGuard",
		Protocol: ir.ProtocolWireGuard,
		Owner:    ir.KernelSetBoth,
		Server:   ir.Server{Host: "example.com", Port: 51820},
		Proto: map[string]any{
			"private_key":   "YJX5N5XkMrzxmVzqzpXqbreUjLIuBv6E9Aobdrc4z1o=",
			"public_key":    "12SmDlc4k7EwUTXU0MaBnwJI6afMwbVT8/qnkejc7Eg=",
			"address":       []string{"10.0.0.2/32"},
			"allowed_ips":   []string{"0.0.0.0/0"},
			"preshared_key": "YJX5N5XkMrzxmVzqzpXqbreUjLIuBv6E9Aobdrc4z1o=",
			"mtu":           1420,
		},
	}
}

func TestGenerateWireGuardEndpoint(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{wireGuardTestNode()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	if len(config["outbounds"].([]any)) != 2 {
		t.Fatalf("WireGuard should not be emitted as an outbound: %#v", config["outbounds"])
	}
	endpoints := config["endpoints"].([]any)
	endpoint := findEndpoint(t, endpoints, "WireGuard")
	if endpoint["type"] != "wireguard" || endpoint["private_key"] == "" {
		t.Fatalf("unexpected WireGuard endpoint: %#v", endpoint)
	}
}

func TestGenerateWireGuardEndpointPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{wireGuardTestNode()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box WireGuard check failed: %v\n%s", err, output)
	}
}

func TestGenerateShadowTLS(t *testing.T) {
	parsed, err := shadowtlsparser.New().Parse("shadowtls://secret@example.com:443?version=3&host=www.example.com&sni=www.example.com#ShadowTLS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbound := findOutbound(t, config["outbounds"].([]any), "ShadowTLS")
	if outbound["type"] != "shadowtls" || outbound["version"] != float64(3) || outbound["password"] != "secret" {
		t.Fatalf("unexpected ShadowTLS outbound: %#v", outbound)
	}
}

func TestGenerateShadowTLSPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	parsed, err := shadowtlsparser.New().Parse("shadowtls://secret@example.com:443?version=3&host=www.example.com&sni=www.example.com#ShadowTLS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box ShadowTLS check failed: %v\n%s", err, output)
	}
}

func TestGenerateNaiveAndTor(t *testing.T) {
	naiveNode, err := naiveparser.New().Parse("naive+https://user:pass@example.com:443?quic=1&quic-congestion-control=bbr&insecure-concurrency=4#Naive")
	if err != nil {
		t.Fatal(err)
	}
	torNode, err := torparser.New().Parse("tor+socks5://example.com:9050#Tor")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{naiveNode.(*ir.Node), torNode.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbounds := config["outbounds"].([]any)
	naive := findOutbound(t, outbounds, "Naive")
	if naive["type"] != "naive" || naive["username"] != "user" || naive["quic"] != true {
		t.Fatalf("unexpected Naive outbound: %#v", naive)
	}
	tor := findOutbound(t, outbounds, "Tor")
	if tor["type"] != "socks" || tor["version"] != "5" {
		t.Fatalf("unexpected Tor outbound: %#v", tor)
	}
}

func TestGenerateNaiveAndTorPassKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(binary), "libcronet.dll")); err != nil && runtime.GOOS == "windows" {
		t.Skip("sing-box Naive check requires libcronet.dll, which is not bundled with the locked kernel")
	}
	naiveNode, err := naiveparser.New().Parse("naive+https://user:pass@example.com:443?quic=1&quic-congestion-control=bbr&insecure-concurrency=4#Naive")
	if err != nil {
		t.Fatal(err)
	}
	torNode, err := torparser.New().Parse("tor+socks5://example.com:9050#Tor")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{naiveNode.(*ir.Node), torNode.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box Naive/Tor check failed: %v\n%s", err, output)
	}
}

func TestGenerateVLESS(t *testing.T) {
	for _, node := range []*ir.Node{ir.ExampleVLESSNode(), ir.ExampleVLESSNodeWS()} {
		generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
		if err != nil {
			t.Fatalf("Generate %s: %v", node.Name, err)
		}
		var config map[string]any
		if err := json.Unmarshal(generated, &config); err != nil {
			t.Fatal(err)
		}
		outbounds := config["outbounds"].([]any)
		outbound := findOutbound(t, outbounds, node.Name)
		if outbound["type"] != "vless" || outbound["uuid"] != node.Proto["uuid"] {
			t.Fatalf("unexpected VLESS outbound: %#v", outbound)
		}
	}
}

func TestGenerateVLESSPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleVLESSNodeWS()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box VLESS check failed: %v\n%s", err, output)
	}
}

func TestGenerateTrojan(t *testing.T) {
	for _, node := range []*ir.Node{ir.ExampleTrojanNode(), ir.ExampleTrojanNodeWS()} {
		generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
		if err != nil {
			t.Fatalf("Generate %s: %v", node.Name, err)
		}
		var config map[string]any
		if err := json.Unmarshal(generated, &config); err != nil {
			t.Fatal(err)
		}
		outbound := findOutbound(t, config["outbounds"].([]any), node.Name)
		if outbound["type"] != "trojan" || outbound["password"] != node.Proto["password"] {
			t.Fatalf("unexpected Trojan outbound: %#v", outbound)
		}
	}
}

func TestGenerateTrojanPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleTrojanNodeWS()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box Trojan check failed: %v\n%s", err, output)
	}
}

func TestGenerateHysteria2(t *testing.T) {
	node := parseHysteria2(t, "hysteria2://test-password@example.com:443?sni=example.com&obfs=salamander&obfs-password=obfs-secret#Hysteria2")
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbound := findOutbound(t, config["outbounds"].([]any), node.Name)
	if outbound["type"] != "hysteria2" || outbound["password"] != "test-password" {
		t.Fatalf("unexpected Hysteria2 outbound: %#v", outbound)
	}
}

func parseHysteria2(t *testing.T, input string) *ir.Node {
	t.Helper()
	parsed, err := hysteria2parser.New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.(*ir.Node)
}

func TestGenerateHysteria2PassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	node := parseHysteria2(t, "hysteria2://test-password@example.com:443?sni=example.com&obfs=salamander&obfs-password=obfs-secret#Hysteria2")
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box Hysteria2 check failed: %v\n%s", err, output)
	}
}

func TestGenerateTUIC(t *testing.T) {
	parsed, err := tuicparser.New().Parse("tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&alpn=h3&sni=example.com#TUIC")
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbound := findOutbound(t, config["outbounds"].([]any), node.Name)
	if outbound["type"] != "tuic" || outbound["uuid"] != node.Proto["uuid"] || outbound["password"] != node.Proto["password"] {
		t.Fatalf("unexpected TUIC outbound: %#v", outbound)
	}
}

func TestGenerateTUICPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	parsed, err := tuicparser.New().Parse("tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&alpn=h3&sni=example.com#TUIC")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box TUIC check failed: %v\n%s", err, output)
	}
}

func TestGenerateHysteria(t *testing.T) {
	parsed, err := hysteriaparser.New().Parse("hysteria://auth@example.com:443?upmbps=20&downmbps=100&sni=example.com#Hysteria")
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbound := findOutbound(t, config["outbounds"].([]any), node.Name)
	if outbound["type"] != "hysteria" || outbound["auth_str"] != "auth" {
		t.Fatalf("unexpected Hysteria outbound: %#v", outbound)
	}
}

func TestGenerateHysteriaPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	parsed, err := hysteriaparser.New().Parse("hysteria://auth@example.com:443?upmbps=20&downmbps=100&sni=example.com#Hysteria")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box Hysteria check failed: %v\n%s", err, output)
	}
}

func TestGenerateAnyTLS(t *testing.T) {
	parsed, err := anytlsparser.New().Parse("anytls://sample-password@example.com:443?sni=example.com&alpn=h2#AnyTLS")
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	outbound := findOutbound(t, config["outbounds"].([]any), node.Name)
	if outbound["type"] != "anytls" || outbound["password"] != "sample-password" {
		t.Fatalf("unexpected AnyTLS outbound: %#v", outbound)
	}
}

func TestGenerateAnyTLSPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	parsed, err := anytlsparser.New().Parse("anytls://sample-password@example.com:443?sni=example.com&alpn=h2#AnyTLS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box AnyTLS check failed: %v\n%s", err, output)
	}
}

func TestGeneratedConfigPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}

	nodes := []*ir.Node{
		ir.ExampleShadowsocksNode(),
		ir.ExampleShadowsocksNodeWithPlugin(),
		ir.ExampleVMessNode(),
		ir.ExampleVMessNodeGRPC(),
	}

	generated, err := NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(binary, "check", "-c", configPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sing-box check failed: %v\n%s", err, output)
	}
}

func TestGeneratedFakeIPAndTUNConfigPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}

	opts := generator.DefaultGenerateOptions()
	opts.DNSFakeIP = true
	opts.EnableTUN = true

	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNode()}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(binary, "check", "-c", configPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sing-box check failed: %v\n%s", err, output)
	}
}

func TestSystemProxyKeepsMixedInboundWithTUN(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.EnableTUN = true
	opts.SystemProxy = true
	opts.InboundPort = 1080

	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNode()}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(generated, &root); err != nil {
		t.Fatal(err)
	}
	inbounds, ok := root["inbounds"].([]any)
	if !ok {
		t.Fatalf("inbounds missing: %#v", root)
	}
	seen := map[string]bool{}
	for _, item := range inbounds {
		value, ok := item.(map[string]any)
		if ok {
			seen[value["tag"].(string)] = true
		}
	}
	if !seen["tun-in"] || !seen["mixed-in"] {
		t.Fatalf("system proxy TUN inbounds = %#v", seen)
	}
}

func findOutbound(t *testing.T, outbounds []any, tag string) map[string]any {
	t.Helper()

	for _, item := range outbounds {
		outbound, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if outbound["tag"] == tag {
			return outbound
		}
	}

	t.Fatalf("outbound %q not found", tag)
	return nil
}

func findEndpoint(t *testing.T, endpoints []any, tag string) map[string]any {
	t.Helper()

	for _, item := range endpoints {
		endpoint, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if endpoint["tag"] == tag {
			return endpoint
		}
	}

	t.Fatalf("endpoint %q not found", tag)
	return nil
}

func TestGenerateOpenConnect(t *testing.T) {
	node := &ir.Node{
		Name:     "work-vpn",
		Protocol: ir.ProtocolOpenConnect,
		Owner:    ir.KernelSetSingBox,
		Server:   ir.Server{Host: "vpn.example.com", Port: 8443},
		Security: ir.Security{
			Type: "tls",
			SNI:  "vpn.example.com",
			Opts: map[string]any{"insecure": false},
		},
		Proto: map[string]any{
			"flavor":   "gp",
			"username": "alice",
			"password": "secret123",
		},
	}

	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}

	// Verify OpenConnect is NOT in outbounds
	outbounds, ok := config["outbounds"].([]any)
	if !ok {
		t.Fatalf("outbounds missing from generated config")
	}
	for _, item := range outbounds {
		if outbound, ok := item.(map[string]any); ok {
			if outbound["tag"] == "work-vpn" {
				t.Errorf("OpenConnect endpoint found in outbounds array, should be in endpoints")
			}
		}
	}

	// Verify OpenConnect is in endpoints
	endpoints, ok := config["endpoints"].([]any)
	if !ok {
		t.Fatalf("endpoints missing from generated config")
	}

	endpoint := findEndpoint(t, endpoints, "work-vpn")

	// Verify basic fields
	if endpoint["type"] != "openconnect" {
		t.Errorf("type = %v, want openconnect", endpoint["type"])
	}
	if endpoint["server"] != "https://vpn.example.com:8443" {
		t.Errorf("server = %v, want https://vpn.example.com:8443", endpoint["server"])
	}
	if endpoint["flavor"] != "gp" {
		t.Errorf("flavor = %v, want gp", endpoint["flavor"])
	}
	if endpoint["username"] != "alice" {
		t.Errorf("username = %v, want alice", endpoint["username"])
	}
	if endpoint["password"] != "secret123" {
		t.Errorf("password = %v, want secret123", endpoint["password"])
	}

	// Verify TLS config
	tls, ok := endpoint["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls config missing")
	}
	if tls["server_name"] != "vpn.example.com" {
		t.Errorf("tls.server_name = %v, want vpn.example.com", tls["server_name"])
	}
	if tls["insecure"] != false {
		t.Errorf("tls.insecure = %v, want false", tls["insecure"])
	}
}

func TestGenerateOpenConnectMinimal(t *testing.T) {
	node := &ir.Node{
		Name:     "vpn-basic",
		Protocol: ir.ProtocolOpenConnect,
		Owner:    ir.KernelSetSingBox,
		Server:   ir.Server{Host: "vpn.test", Port: 443},
		Security: ir.Security{Type: "tls"},
		Proto:    map[string]any{},
	}

	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}

	endpoints := config["endpoints"].([]any)
	endpoint := findEndpoint(t, endpoints, "vpn-basic")

	if endpoint["type"] != "openconnect" {
		t.Errorf("type = %v, want openconnect", endpoint["type"])
	}
	if endpoint["server"] != "https://vpn.test:443" {
		t.Errorf("server = %v, want https://vpn.test:443", endpoint["server"])
	}

	// TLS should not be present if no SNI or insecure flag
	if _, hasTLS := endpoint["tls"]; hasTLS {
		t.Errorf("tls should not be present for minimal config")
	}
}

func TestGenerateOpenConnectAllFields(t *testing.T) {
	node := &ir.Node{
		Name:     "full-vpn",
		Protocol: ir.ProtocolOpenConnect,
		Owner:    ir.KernelSetSingBox,
		Server:   ir.Server{Host: "enterprise.vpn", Port: 8443},
		Security: ir.Security{
			Type: "tls",
			SNI:  "enterprise.vpn",
			Opts: map[string]any{"insecure": true},
		},
		Proto: map[string]any{
			"flavor":            "pulse",
			"username":          "admin",
			"password":          "pass",
			"auth_group":        "eng",
			"cookie":            "sessionid=xyz",
			"token":             "bearer123",
			"reported_os":       "Linux",
			"user_agent":        "Mozilla/5.0",
			"version":           "v1.0",
			"local_hostname":    "workstation",
			"system":            "Ubuntu",
			"name":              "client1",
			"mobile":            true,
			"csd":               false,
			"hip":               true,
			"tncc":              false,
			"mtu":               1400,
			"dpd_interval":      30,
			"reconnect_timeout": 60,
		},
	}

	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}

	endpoints := config["endpoints"].([]any)
	endpoint := findEndpoint(t, endpoints, "full-vpn")

	// Verify all protocol fields are present
	expectedFields := map[string]any{
		"type":              "openconnect",
		"server":            "https://enterprise.vpn:8443",
		"flavor":            "pulse",
		"username":          "admin",
		"password":          "pass",
		"auth_group":        "eng",
		"cookie":            "sessionid=xyz",
		"token":             "bearer123",
		"reported_os":       "Linux",
		"user_agent":        "Mozilla/5.0",
		"version":           "v1.0",
		"local_hostname":    "workstation",
		"system":            "Ubuntu",
		"name":              "client1",
		"mobile":            true,
		"csd":               false,
		"hip":               true,
		"tncc":              false,
		"mtu":               float64(1400),
		"dpd_interval":      float64(30),
		"reconnect_timeout": float64(60),
	}

	for key, expected := range expectedFields {
		if endpoint[key] != expected {
			t.Errorf("%s = %v, want %v", key, endpoint[key], expected)
		}
	}

	// Verify TLS
	tls := endpoint["tls"].(map[string]any)
	if tls["server_name"] != "enterprise.vpn" || tls["insecure"] != true {
		t.Errorf("tls config incorrect: %v", tls)
	}
}

func TestGenerateOpenConnectEndpointPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}

	node := &ir.Node{
		Name:     "test-vpn",
		Protocol: ir.ProtocolOpenConnect,
		Owner:    ir.KernelSetSingBox,
		Server:   ir.Server{Host: "vpn.test.local", Port: 443},
		Security: ir.Security{
			Type: "tls",
			SNI:  "vpn.test.local",
		},
		Proto: map[string]any{
			"flavor":   "anyconnect",
			"username": "testuser",
			"password": "testpass",
		},
	}

	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify structure before kernel check
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}

	// Verify endpoints array exists and contains the OpenConnect node
	endpoints, hasEndpoints := config["endpoints"].([]any)
	if !hasEndpoints {
		t.Fatalf("config missing endpoints array")
	}
	if len(endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(endpoints))
	}

	// Verify OpenConnect is NOT in outbounds
	outbounds := config["outbounds"].([]any)
	for _, item := range outbounds {
		if ob, ok := item.(map[string]any); ok {
			if ob["tag"] == "test-vpn" {
				t.Errorf("OpenConnect endpoint incorrectly placed in outbounds array")
			}
		}
	}

	// Run sing-box kernel validation
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(binary, "check", "-c", configPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sing-box check failed: %v\n%s", err, output)
	}
}

func TestGenerateSOCKSListenerRejectsTransparentPortCollision(t *testing.T) {
	node := &ir.Node{Name: "node", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
	for _, test := range []struct {
		name string
		mode string
		port int
	}{
		{name: "redirect", mode: "redirect", port: 18080},
		{name: "tproxy", mode: "tproxy", port: 18081},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := generator.DefaultGenerateOptions()
			opts.TransparentMode = test.mode
			opts.RedirectPort = test.port
			opts.TProxyPort = test.port
			opts.SOCKSListeners = []generator.SOCKSListener{{Port: test.port, Outbound: node.Name}}
			if _, err := NewGenerator().Generate([]*ir.Node{node}, opts); err == nil {
				t.Fatalf("listener colliding with %s port %d was accepted", test.mode, test.port)
			}
		})
	}
}
