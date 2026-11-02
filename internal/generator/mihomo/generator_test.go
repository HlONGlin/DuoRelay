package mihomo

import (
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
	masqueparser "github.com/holi/duorelay/internal/parser/masque"
	mieruparser "github.com/holi/duorelay/internal/parser/mieru"
	shadowquicparser "github.com/holi/duorelay/internal/parser/shadowquic"
	snellparser "github.com/holi/duorelay/internal/parser/snell"
	socks5parser "github.com/holi/duorelay/internal/parser/socks5"
	sshparser "github.com/holi/duorelay/internal/parser/ssh"
	sudokuparser "github.com/holi/duorelay/internal/parser/sudoku"
	trusttunnelparser "github.com/holi/duorelay/internal/parser/trusttunnel"
	tuicparser "github.com/holi/duorelay/internal/parser/tuic"
	"gopkg.in/yaml.v3"
)

var updateGolden = os.Getenv("UPDATE_GOLDEN") == "1"

func goldenPath(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "golden", name)
}

func kernelBinary() string {
	if runtime.GOOS == "windows" {
		return "mihomo.exe"
	}
	return "mihomo"
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

	assertGolden(t, generated, "mihomo-ss-basic.yaml")
}

func TestTUNEnablesStrictDualStackRoutingAndDNSHijack(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.EnableTUN = true
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNode()}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	tun, _ := config["tun"].(map[string]any)
	if tun["strict-route"] != true || tun["dns-mode"] != "hijack" || tun["auto-detect-interface"] != true {
		t.Fatalf("TUN hardening missing: %#v", tun)
	}
	if config["ipv6"] != true {
		t.Fatalf("IPv6 TUN is disabled: %#v", config["ipv6"])
	}
	if _, ok := tun["inet6-address"]; !ok {
		t.Fatalf("IPv6 TUN address missing: %#v", tun)
	}
	if _, ok := tun["inet4-address"]; ok {
		t.Fatalf("mihomo assigns its IPv4 TUN address automatically: %#v", tun)
	}
	dnsHijack, _ := tun["dns-hijack"].([]any)
	if len(dnsHijack) != 2 {
		t.Fatalf("UDP and TCP DNS hijack missing: %#v", dnsHijack)
	}
	if runtime.GOOS == "linux" && tun["auto-redirect"] != true {
		t.Fatalf("Linux TUN auto redirect is disabled: %#v", tun)
	}
}

func TestGenerateVMessWebSocket(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleVMessNode()}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	assertGolden(t, generated, "mihomo-vmess-ws.yaml")
}

func TestGenerateShadowsocksWithPlugin(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleShadowsocksNodeWithPlugin()}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}

	proxies, ok := config["proxies"].([]any)
	if !ok || len(proxies) != 1 {
		t.Fatalf("expected one proxy, got %v", config["proxies"])
	}

	proxy, ok := proxies[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected proxy type %T", proxies[0])
	}

	if proxy["plugin"] != "obfs" {
		t.Errorf("plugin = %v, want obfs", proxy["plugin"])
	}

	pluginOpts, ok := proxy["plugin-opts"].(map[string]any)
	if !ok {
		t.Fatalf("plugin-opts missing from proxy")
	}
	if pluginOpts["mode"] != "http" {
		t.Errorf("plugin-opts.mode = %v, want http", pluginOpts["mode"])
	}
	if pluginOpts["host"] != "www.bing.com" {
		t.Errorf("plugin-opts.host = %v, want www.bing.com", pluginOpts["host"])
	}
}

func TestGenerateFiltersNodesForOtherKernel(t *testing.T) {
	node := &ir.Node{
		Name:     "singbox-only",
		Protocol: ir.ProtocolShadowTLS,
		Owner:    ir.KernelSetSingBox,
		Server:   ir.Server{Host: "example.com", Port: 443},
	}

	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if strings.Contains(string(generated), "singbox-only") {
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
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)
	if proxy["type"] != "snell" || proxy["psk"] != "sample-psk" || proxy["version"] != 4 {
		t.Fatalf("unexpected Snell proxy: %#v", proxy)
	}
	obfsOpts := proxy["obfs-opts"].(map[string]any)
	if obfsOpts["mode"] != "http" {
		t.Fatalf("obfs-opts.mode = %v, want http", obfsOpts["mode"])
	}
}

func TestGenerateSnellPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	parsed, err := snellparser.New().Parse("snell://sample-psk@example.com:443?version=4&obfs=http#Snell")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo Snell check failed: %v\n%s", err, output)
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
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxies := config["proxies"].([]any)
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(proxies))
	}
	socks := proxies[0].(map[string]any)
	if socks["type"] != "socks5" || socks["username"] != "user" || socks["password"] != "pass" || socks["udp"] != true {
		t.Fatalf("unexpected SOCKS5 proxy: %#v", socks)
	}
	http := proxies[1].(map[string]any)
	if http["type"] != "http" || http["username"] != "user" || http["password"] != "pass" || http["tls"] != true || http["sni"] != "example.com" {
		t.Fatalf("unexpected HTTPS proxy: %#v", http)
	}
}

func TestGenerateSOCKS5AndHTTPSPassKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
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
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo SOCKS5/HTTPS check failed: %v\n%s", err, output)
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
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "ssh" || proxy["username"] != "alice" || proxy["password"] != "secret" {
		t.Fatalf("unexpected SSH proxy: %#v", proxy)
	}
}

func TestGenerateSSHPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	parsed, err := sshparser.New().Parse("ssh://alice:secret@example.com:22#SSH")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo SSH check failed: %v\n%s", err, output)
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

func TestGenerateWireGuard(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{wireGuardTestNode()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxies := config["proxies"].([]any)
	proxy := proxies[0].(map[string]any)
	if proxy["type"] != "wireguard" || proxy["private-key"] == "" || proxy["public-key"] == "" {
		t.Fatalf("unexpected WireGuard proxy: %#v", proxy)
	}
}

func TestGenerateWireGuardPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{wireGuardTestNode()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo WireGuard check failed: %v\n%s", err, output)
	}
}

func shadowsocksRTestNode() *ir.Node {
	return &ir.Node{
		Name:     "SSR",
		Protocol: ir.ProtocolShadowsocksR,
		Owner:    ir.KernelSetMihomo,
		Server:   ir.Server{Host: "example.com", Port: 8388},
		Proto: map[string]any{
			"password": "secret",
			"method":   "aes-128-cfb",
			"protocol": "origin",
			"obfs":     "plain",
			"udp":      true,
		},
	}
}

func TestGenerateShadowsocksR(t *testing.T) {
	generated, err := NewGenerator().Generate([]*ir.Node{shadowsocksRTestNode()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "ssr" || proxy["cipher"] != "aes-128-cfb" || proxy["protocol"] != "origin" || proxy["obfs"] != "plain" {
		t.Fatalf("unexpected SSR proxy: %#v", proxy)
	}
}

func TestGenerateShadowsocksRPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{shadowsocksRTestNode()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo SSR check failed: %v\n%s", err, output)
	}
}

func TestGenerateMihomoExclusiveProtocols(t *testing.T) {
	mieruNode, err := mieruparser.New().Parse("mieru://user:pass@example.com:443?transport=tcp&multiplexing=high&handshake-mode=standard#Mieru")
	if err != nil {
		t.Fatal(err)
	}
	sudokuNode, err := sudokuparser.New().Parse("sudoku://key@example.com:443?aead-method=chacha20-poly1305&table-type=prefer_entropy&udp=1#Sudoku")
	if err != nil {
		t.Fatal(err)
	}
	shadowNode, err := shadowquicparser.New().Parse("shadowquic://user:pass@example.com:443?sni=example.com&congestion-controller=bbr#ShadowQUIC")
	if err != nil {
		t.Fatal(err)
	}
	trustNode, err := trusttunnelparser.New().Parse("trusttunnel://user:pass@example.com:443?sni=example.com&health-check=1&max-connections=2#TrustTunnel")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{mieruNode.(*ir.Node), sudokuNode.(*ir.Node), shadowNode.(*ir.Node), trustNode.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxies := config["proxies"].([]any)
	if len(proxies) != 4 {
		t.Fatalf("expected 4 proxies, got %d", len(proxies))
	}
	for i, expected := range []string{"mieru", "sudoku", "shadowquic", "trusttunnel"} {
		proxy := proxies[i].(map[string]any)
		if proxy["type"] != expected {
			t.Fatalf("proxy %d type = %v, want %s", i, proxy["type"], expected)
		}
	}
}

func TestGenerateMihomoExclusiveProtocolsPassKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	mieruNode, err := mieruparser.New().Parse("mieru://user:pass@example.com:443?transport=tcp&multiplexing=high&handshake-mode=standard#Mieru")
	if err != nil {
		t.Fatal(err)
	}
	sudokuNode, err := sudokuparser.New().Parse("sudoku://key@example.com:443?aead-method=chacha20-poly1305&table-type=prefer_entropy&udp=1#Sudoku")
	if err != nil {
		t.Fatal(err)
	}
	shadowNode, err := shadowquicparser.New().Parse("shadowquic://user:pass@example.com:443?sni=example.com&congestion-controller=bbr#ShadowQUIC")
	if err != nil {
		t.Fatal(err)
	}
	trustNode, err := trusttunnelparser.New().Parse("trusttunnel://user:pass@example.com:443?sni=example.com&health-check=1&max-connections=2#TrustTunnel")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{mieruNode.(*ir.Node), sudokuNode.(*ir.Node), shadowNode.(*ir.Node), trustNode.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo exclusive protocol check failed: %v\n%s", err, output)
	}
}

func TestGenerateMASQUE(t *testing.T) {
	link := "masque://example.com:443?private-key=MHcCAQEEIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABoAoGCCqGSM49AwEHoUQDQgAEaxfR8uEsQkf4vOblY6RA8ncDfYEt6zOg9KE5RdiYwpZP40Li/hp/m47n60p8D54WK84zV2sxXs7LtkBoN79R9Q%3D%3D&public-key=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEaxfR8uEsQkf4vOblY6RA8ncDfYEt6zOg9KE5RdiYwpZP40Li/hp/m47n60p8D54WK84zV2sxXs7LtkBoN79R9Q%3D%3D&network=h3-l4proxy#MASQUE"
	parsed, err := masqueparser.New().Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "masque" || proxy["network"] != "h3-l4proxy" || proxy["private-key"] == "" {
		t.Fatalf("unexpected MASQUE proxy: %#v", proxy)
	}
}

func TestGenerateMASQUEPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	link := "masque://example.com:443?private-key=MHcCAQEEIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABoAoGCCqGSM49AwEHoUQDQgAEaxfR8uEsQkf4vOblY6RA8ncDfYEt6zOg9KE5RdiYwpZP40Li/hp/m47n60p8D54WK84zV2sxXs7LtkBoN79R9Q%3D%3D&public-key=MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEaxfR8uEsQkf4vOblY6RA8ncDfYEt6zOg9KE5RdiYwpZP40Li/hp/m47n60p8D54WK84zV2sxXs7LtkBoN79R9Q%3D%3D&network=h3-l4proxy#MASQUE"
	parsed, err := masqueparser.New().Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo MASQUE check failed: %v\n%s", err, output)
	}
}

func TestGenerateVLESS(t *testing.T) {
	for _, node := range []*ir.Node{ir.ExampleVLESSNode(), ir.ExampleVLESSNodeWS()} {
		generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
		if err != nil {
			t.Fatalf("Generate %s: %v", node.Name, err)
		}
		var config map[string]any
		if err := yaml.Unmarshal(generated, &config); err != nil {
			t.Fatal(err)
		}
		proxies := config["proxies"].([]any)
		proxy := proxies[0].(map[string]any)
		if proxy["type"] != "vless" || proxy["uuid"] != node.Proto["uuid"] {
			t.Fatalf("unexpected VLESS proxy: %#v", proxy)
		}
	}
}

func TestGenerateVLESSPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleVLESSNodeWS()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo VLESS check failed: %v\n%s", err, output)
	}
}

func TestGenerateTrojan(t *testing.T) {
	for _, node := range []*ir.Node{ir.ExampleTrojanNode(), ir.ExampleTrojanNodeWS()} {
		generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
		if err != nil {
			t.Fatalf("Generate %s: %v", node.Name, err)
		}
		var config map[string]any
		if err := yaml.Unmarshal(generated, &config); err != nil {
			t.Fatal(err)
		}
		proxies := config["proxies"].([]any)
		proxy := proxies[0].(map[string]any)
		if proxy["type"] != "trojan" || proxy["password"] != node.Proto["password"] {
			t.Fatalf("unexpected Trojan proxy: %#v", proxy)
		}
	}
}

func TestGenerateTrojanPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	generated, err := NewGenerator().Generate([]*ir.Node{ir.ExampleTrojanNodeWS()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo Trojan check failed: %v\n%s", err, output)
	}
}

func TestGenerateHysteria2(t *testing.T) {
	parsed, err := hysteria2parser.New().Parse("hysteria2://test-password@example.com:443?sni=example.com&obfs=salamander&obfs-password=obfs-secret#Hysteria2")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "hysteria2" || proxy["password"] != "test-password" {
		t.Fatalf("unexpected Hysteria2 proxy: %#v", proxy)
	}
}

func TestGenerateHysteria2PassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	parsed, err := hysteria2parser.New().Parse("hysteria2://test-password@example.com:443?sni=example.com&obfs=salamander&obfs-password=obfs-secret#Hysteria2")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo Hysteria2 check failed: %v\n%s", err, output)
	}
}

func TestGenerateTUIC(t *testing.T) {
	parsed, err := tuicparser.New().Parse("tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&alpn=h3&sni=example.com#TUIC")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "tuic" || proxy["uuid"] != "b8ca57c2-eb39-491a-8425-bba828fbad2d" {
		t.Fatalf("unexpected TUIC proxy: %#v", proxy)
	}
}

func TestGenerateTUICPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	parsed, err := tuicparser.New().Parse("tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:test-password@example.com:443?congestion_control=bbr&udp_relay_mode=native&alpn=h3&sni=example.com#TUIC")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo TUIC check failed: %v\n%s", err, output)
	}
}

func TestGenerateHysteria(t *testing.T) {
	parsed, err := hysteriaparser.New().Parse("hysteria://auth@example.com:443?upmbps=20&downmbps=100&sni=example.com#Hysteria")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "hysteria" || proxy["auth-str"] != "auth" {
		t.Fatalf("unexpected Hysteria proxy: %#v", proxy)
	}
}

func TestGenerateHysteriaPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	parsed, err := hysteriaparser.New().Parse("hysteria://auth@example.com:443?upmbps=20&downmbps=100&sni=example.com#Hysteria")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo Hysteria check failed: %v\n%s", err, output)
	}
}

func TestGenerateAnyTLS(t *testing.T) {
	parsed, err := anytlsparser.New().Parse("anytls://sample-password@example.com:443?sni=example.com&alpn=h2&idle-session-timeout=60#AnyTLS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxy := config["proxies"].([]any)[0].(map[string]any)
	if proxy["type"] != "anytls" || proxy["password"] != "sample-password" {
		t.Fatalf("unexpected AnyTLS proxy: %#v", proxy)
	}
}

func TestGenerateAnyTLSPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	parsed, err := anytlsparser.New().Parse("anytls://sample-password@example.com:443?sni=example.com&alpn=h2&idle-session-timeout=60#AnyTLS")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := NewGenerator().Generate([]*ir.Node{parsed.(*ir.Node)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo AnyTLS check failed: %v\n%s", err, output)
	}
}

func TestGeneratedConfigPassesKernelCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
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

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(binary, "-t", "-f", configPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mihomo config test failed: %v\n%s", err, output)
	}
}
