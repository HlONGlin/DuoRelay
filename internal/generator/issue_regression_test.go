//go:build !lite

package generator_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/subscription"
	"gopkg.in/yaml.v3"
)

func checkPinnedKernel(t *testing.T, gen generator.Generator, data []byte) {
	t.Helper()
	binary, err := filepath.Abs(filepath.Join("..", "..", "kernels", string(gen.Kernel())))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binary); err != nil {
		if os.Getenv("DUORELAY_REQUIRE_KERNELS") == "1" {
			t.Fatal(err)
		}
		t.Skip("pinned Linux kernel binary unavailable")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	args := []string{"check", "-c", file}
	if gen.Kernel() == ir.KernelMihomo {
		file = filepath.Join(dir, "config.yaml")
		args = []string{"-t", "-d", dir, "-f", file}
	}
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, args...).CombinedOutput(); err != nil {
		t.Fatalf("pinned %s rejected generated config: %v\n%s", gen.Kernel(), err, output)
	}
}

func TestIssueDuplicateNamesRealityAndDNSKernelCheck(t *testing.T) {
	manager := subscription.NewManager()
	nodes, _, err := manager.ParseDetailed([]byte("vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@127.0.0.1:443?security=reality&sni=example.org&pbk=pGLYq6nS9_P3hNx0bfrDlfEBO-wFQ9RRTT9vPgCL72E&sid=abcd#same\n" +
		"ss://YWVzLTI1Ni1nY206cGFzcw@127.0.0.1:8388#same\nss://YWVzLTI1Ni1nY206cGFzcw@127.0.0.1:8389#DIRECT"))
	if err != nil {
		t.Fatal(err)
	}
	for _, gen := range []generator.Generator{singbox.NewGenerator(), mihomo.NewGenerator()} {
		t.Run(string(gen.Kernel()), func(t *testing.T) {
			opts := generator.DefaultGenerateOptions()
			opts.DefaultOutbound = "PROXY"
			opts.DNSDetour = "PROXY"
			opts.DNSServers = []string{"https://1.1.1.1/dns-query"}
			opts.EnableTUN = true
			opts.ExtraNodes = []*ir.Node{nodes[1]}
			opts.SOCKSListeners = []generator.SOCKSListener{{Port: 20801, Outbound: nodes[1].Name}}
			data, err := gen.Generate(nodes, opts)
			if err != nil {
				t.Fatal(err)
			}
			if nodes[0].Name != "same" || nodes[1].Name != "same" || nodes[2].Name != "DIRECT" {
				t.Fatal("generation mutated display names")
			}
			if gen.Kernel() == ir.KernelSingBox {
				var root map[string]any
				if err := json.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				rules := root["route"].(map[string]any)["rules"].([]any)
				if rules[0].(map[string]any)["action"] != "sniff" || rules[1].(map[string]any)["action"] != "hijack-dns" {
					t.Fatal("DNS hijack must follow sniff")
				}
				if root["route"].(map[string]any)["default_domain_resolver"] != "dns-bootstrap" {
					t.Fatal("proxy hostname lookup must use bootstrap")
				}
				outbounds := root["outbounds"].([]any)
				if outbounds[0].(map[string]any)["domain_resolver"] != "dns-0" {
					t.Fatal("DIRECT destinations must use business DNS")
				}
				servers := root["dns"].(map[string]any)["servers"].([]any)
				if _, exists := servers[0].(map[string]any)["detour"]; exists {
					t.Fatal("bootstrap must use its own direct dialer")
				}
				if servers[1].(map[string]any)["detour"] != "PROXY" {
					t.Fatal("application DNS must use proxy")
				}
			} else {
				var root map[string]any
				if err := yaml.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
			}
			checkPinnedKernel(t, gen, data)
		})
	}
}

func TestIssueNaiveQUICAndXHTTPKernelCheck(t *testing.T) {
	m := subscription.NewManager()
	for _, tc := range []struct {
		name, link string
		gen        generator.Generator
	}{
		{"naive-quic", "naive+quic://user:pass@127.0.0.1:443#naive", singbox.NewGenerator()},
		{"xhttp", "vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@127.0.0.1:443?net=xhttp&path=%2Ftest&mode=auto&scMaxEachPostBytes=1000000#xhttp", mihomo.NewGenerator()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := m.Parse([]byte(tc.link))
			if err != nil {
				t.Fatal(err)
			}
			data, err := tc.gen.Generate(nodes, nil)
			if err != nil {
				t.Fatal(err)
			}
			checkPinnedKernel(t, tc.gen, data)
		})
	}
	_, skipped, err := m.ParseDetailed([]byte("vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@127.0.0.1:443?net=xhttp&pqv=unsupported\nsocks5://127.0.0.1:1080"))
	if err != nil || len(skipped) != 1 {
		t.Fatalf("unsupported pqv must be skipped explicitly: %v %v", skipped, err)
	}
}
