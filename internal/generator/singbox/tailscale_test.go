package singbox

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/holi/duorelay/internal/ir"
)

func TestGenerateTailscaleEndpoint(t *testing.T) {
	node := &ir.Node{
		Name: "tailnet", Protocol: ir.ProtocolTailscale, Owner: ir.KernelSetBoth,
		Proto: map[string]any{
			"state_directory": "C:/tailscale",
			"hostname":        "duorelay",
			"accept_routes":   true,
		},
	}
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(generated, &config); err != nil {
		t.Fatal(err)
	}
	endpoints := config["endpoints"].([]any)
	if len(endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %d", len(endpoints))
	}
	endpoint := endpoints[0].(map[string]any)
	if endpoint["type"] != "tailscale" || endpoint["tag"] != "tailnet" || endpoint["state_directory"] != "C:/tailscale" {
		t.Fatalf("unexpected endpoint: %#v", endpoint)
	}
}

func TestTailscaleEndpointPassesSingBoxCheck(t *testing.T) {
	binary := filepath.Join("..", "..", "..", "kernels", kernelBinary())
	if _, err := os.Stat(binary); err != nil {
		t.Skip("sing-box binary not available")
	}
	node := &ir.Node{
		Name: "tailnet", Protocol: ir.ProtocolTailscale, Owner: ir.KernelSetBoth,
		Proto: map[string]any{"state_directory": "C:/tailscale", "hostname": "duorelay"},
	}
	generated, err := NewGenerator().Generate([]*ir.Node{node}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(binary, "check", "-c", configPath).CombinedOutput(); err != nil {
		t.Fatalf("sing-box tailscale check failed: %v\n%s\n%s", err, output, generated)
	}
}
