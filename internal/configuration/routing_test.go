package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

func TestRoutingRulesPersistAndGenerate(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()
	settings, err := service.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.RoutingRules = []generator.RoutingRule{{
		Match:    "domain_suffix",
		Values:   []string{"example.com"},
		Outbound: "selected",
	}}
	if err := service.SaveSettings(ctx, settings); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	settings, err = service.Settings(ctx)
	if err != nil || len(settings.RoutingRules) != 1 || settings.RoutingRules[0].Outbound != "selected" {
		t.Fatalf("routing rules were not persisted: %#v, %v", settings.RoutingRules, err)
	}

	node := &ir.Node{
		Name:     "selected",
		Protocol: ir.ProtocolShadowsocks,
		Owner:    ir.KernelSetBoth,
		Server:   ir.Server{Host: "127.0.0.1", Port: 8388},
		Proto:    map[string]any{"method": "aes-256-gcm", "password": "test"},
	}
	if err := service.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	state, err := service.Activate(ctx, node.ID, ir.KernelSingBox)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	data, err := os.ReadFile(state.Path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	rules := root["route"].(map[string]any)["rules"].([]any)
	first := rules[0].(map[string]any)
	if first["domain_suffix"].([]any)[0] != "example.com" || first["outbound"] != "selected" {
		t.Fatalf("generated routing rule = %#v", first)
	}
}

func TestSaveSettingsRejectsInvalidRoutingRule(t *testing.T) {
	service, _ := testService(t)
	settings, err := service.Settings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	settings.RoutingRules = []generator.RoutingRule{{Match: "unknown", Values: []string{"x"}, Outbound: "DIRECT"}}
	if err := service.SaveSettings(context.Background(), settings); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("invalid routing rule error = %v", err)
	}
}
