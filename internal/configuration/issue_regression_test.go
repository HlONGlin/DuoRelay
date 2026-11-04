package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/transparent"
	"github.com/holi/duorelay/internal/variant"
)

func TestIssueSettingsDistinguishesPersistedStateFromRunningKernel(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	runtime := kernel.NewManager("")
	defer runtime.CloseLogs()
	s.Runtime = runtime
	settings := defaults()
	settings.TransparentMode = transparent.ModeTUN
	if err := s.Store.SaveState(ctx, "settings", settings); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.SaveState(ctx, "current_config", State{Kernel: ir.KernelType(variant.DefaultKernel), TransparentMode: transparent.ModeTUN}); err != nil {
		t.Fatal(err)
	}
	value, err := s.Settings(ctx)
	if err != nil || value.TransparentActive != transparent.ModeOff || value.TransparentReason == "" {
		t.Fatal("stopped persisted config was reported as active", value, err)
	}
}

func TestIssueDisabledRoutingRuleStaysDisabled(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	settings := defaults()
	settings.RoutingRules[0].ID = "custom-disabled"
	settings.RoutingRules[0].Enabled = false
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	value, err := s.Settings(ctx)
	if err != nil || value.RoutingRules[0].Enabled {
		t.Fatal("disabled rule was re-enabled", err)
	}
}

func TestIssueLegacyTransparentPortUsesAppliedConfig(t *testing.T) {
	for _, tc := range []struct {
		kernel  ir.KernelType
		mode    transparent.Mode
		content string
		want    int
	}{
		{ir.KernelSingBox, transparent.ModeRedirect, `{"inbounds":[{"type":"redirect","listen_port":24680}]}`, 24680},
		{ir.KernelMihomo, transparent.ModeTProxy, "tproxy-port: 24681\n", 24680},
	} {
		path := filepath.Join(t.TempDir(), "applied-config")
		if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
			t.Fatal(err)
		}
		port, err := stateTransparentPort(&State{Kernel: tc.kernel, TransparentMode: tc.mode, Path: path})
		if err != nil || port != tc.want {
			t.Fatal("legacy port was not recovered", port, err)
		}
	}
}

func TestIssuePermissionPreflightPreservesWorkingKernel(t *testing.T) {
	s, runtime := testService(t)
	ctx := context.Background()
	node := &ir.Node{Name: "working", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 30001}}
	if err := s.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	state, err := s.Apply(ctx, ApplyRequest{NodeIDs: []string{node.ID}})
	if err != nil {
		t.Fatal(err)
	}
	s.SetTransparent(transparent.New())
	settings, _ := s.Settings(ctx)
	settings.TransparentMode = "invalid"
	if err := s.Store.SaveState(ctx, "settings", settings); err != nil {
		t.Fatal(err)
	}
	stops := runtime.stops
	if _, err := s.Apply(ctx, ApplyRequest{NodeIDs: []string{node.ID}}); err == nil {
		t.Fatal("invalid transparent mode accepted")
	}
	if runtime.stops != stops || runtime.current != state.Path {
		t.Fatal("preflight stopped the working kernel")
	}
}

func TestIssueConfigHistoryOnlyDeletesGeneratedFiles(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, uuid.NewString()+"-"+variant.DefaultKernel+".json")
	if err := os.WriteFile(current, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	os.Chtimes(current, old, old)
	for i := 0; i < 20; i++ {
		path := filepath.Join(dir, uuid.NewString()+".json")
		if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(dir, "custom-config.json")
	if err := os.WriteFile(unrelated, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(unrelated, old, old)
	pruneConfigHistory(dir, current)
	for _, path := range []string{current, unrelated} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("deleted current/unrelated file", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) > configHistoryLimit+2 {
		t.Fatal("history not bounded", len(entries), err)
	}
}

func TestIssueControllerPortDoesNotConflict(t *testing.T) {
	occupied, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	k := ir.KernelType(variant.DefaultKernel)
	content := []byte("proxies: []\nrules: [MATCH,DIRECT]\n")
	if k == ir.KernelSingBox {
		content = []byte(`{"outbounds":[{"type":"direct","tag":"DIRECT"}]}`)
	}
	_, controller, _, err := withController(content, k, false)
	if err != nil || controller == "http://"+occupied.Addr().String() {
		t.Fatal("controller collided", controller, err)
	}
	if k == ir.KernelSingBox {
		address := occupied.Addr().String()
		raw := fmt.Sprintf(`{"experimental":{"clash_api":{"external_controller":%q}},"outbounds":[]}`, address)
		data, _, _, err := withController([]byte(raw), k, false)
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]any
		json.Unmarshal(data, &root)
		if root["experimental"].(map[string]any)["clash_api"].(map[string]any)["external_controller"] != address {
			t.Fatal("explicit controller address ignored")
		}
	}
}
