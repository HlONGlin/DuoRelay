package mihomo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
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
