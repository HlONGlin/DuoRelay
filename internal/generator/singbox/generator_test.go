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
