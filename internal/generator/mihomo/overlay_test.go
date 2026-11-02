package mihomo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

func TestGenerateOverlayProtocols(t *testing.T) {
	nodes := []*ir.Node{
		{
			Name: "tailnet", Protocol: ir.ProtocolTailscale, Owner: ir.KernelSetBoth,
			Proto: map[string]any{"state_directory": "C:/tailscale", "hostname": "duorelay"},
		},
		{
			Name: "mesh", Protocol: ir.ProtocolEasyTier, Owner: ir.KernelSetMihomo,
			Proto: map[string]any{"network_name": "office", "network_secret": "secret", "peers": []any{"tcp://peer.example:11010"}, "mtu": 1400},
		},
		{
			Name: "zt", Protocol: ir.ProtocolZeroTier, Owner: ir.KernelSetMihomo,
			Proto: map[string]any{"network": "8056c2e21c000001"},
		},
		{
			Name: "recheck", Protocol: ir.ProtocolRematch, Owner: ir.KernelSetMihomo,
			Proto: map[string]any{"target_rematch_name": "proxy-rule"},
		},
	}
	generated, err := NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	proxies, ok := config["proxies"].([]any)
	if !ok || len(proxies) != len(nodes) {
		t.Fatalf("unexpected proxies: %#v", config["proxies"])
	}
	types := map[string]bool{}
	for _, item := range proxies {
		proxy := item.(map[string]any)
		types[proxy["type"].(string)] = true
	}
	for _, typ := range []string{"tailscale", "easytier", "zerotier", "rematch"} {
		if !types[typ] {
			t.Errorf("missing generated proxy type %q", typ)
		}
	}
}

func TestOverlayConfigPassesMihomoCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("mihomo binary not available")
	}
	nodes := []*ir.Node{
		{
			Name: "tailnet", Protocol: ir.ProtocolTailscale, Owner: ir.KernelSetBoth,
			Proto: map[string]any{"state_directory": "C:/tailscale", "hostname": "duorelay"},
		},
		{
			Name: "mesh", Protocol: ir.ProtocolEasyTier, Owner: ir.KernelSetMihomo,
			Proto: map[string]any{"network_name": "office", "network_secret": "secret", "peers": []any{"tcp://peer.example:11010"}, "mtu": 1400},
		},
		{
			Name: "zt", Protocol: ir.ProtocolZeroTier, Owner: ir.KernelSetMihomo,
			Proto: map[string]any{"network": "8056c2e21c000001"},
		},
		{
			Name: "recheck", Protocol: ir.ProtocolRematch, Owner: ir.KernelSetMihomo,
			Proto: map[string]any{"target_rematch_name": "proxy-rule"},
		},
	}
	generated, err := NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "-t", "-f", configPath).CombinedOutput(); err != nil {
		t.Fatalf("mihomo overlay check failed: %v\n%s\n%s", err, output, generated)
	}
}
