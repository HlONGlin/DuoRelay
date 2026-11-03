package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/variant"
	"gopkg.in/yaml.v3"
)

func TestApplyConfiguredSOCKSListenersPinsPortsToNodes(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	makeNode := func(name string) *ir.Node {
		node := &ir.Node{Name: name, Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
		if err := s.Store.SaveNode(ctx, node); err != nil {
			t.Fatal(err)
		}
		return node
	}
	primary, first, second := makeNode("primary"), makeNode("first exit"), makeNode("second exit")
	settings := defaults()
	settings.SOCKSListeners = []SOCKSListener{{Port: 18081, NodeID: first.ID}, {Port: 18082, NodeID: second.ID}}
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	state, err := s.Apply(ctx, ApplyRequest{NodeIDs: []string{primary.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.SOCKSListeners) != 2 || len(state.NodeIDs) != 1 || state.NodeIDs[0] != primary.ID {
		t.Fatalf("active state omitted assigned SOCKS nodes: %#v", state)
	}
	data, err := os.ReadFile(state.Path)
	if err != nil {
		t.Fatal(err)
	}
	firstTag := socksOutboundName(first.ID)
	secondTag := socksOutboundName(second.ID)
	if state.Kernel == ir.KernelSingBox {
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			t.Fatal(err)
		}
		inbounds := root["inbounds"].([]any)
		if !hasConfigEntry(inbounds, "tag", "duorelay-socks-18081") || !hasConfigEntry(inbounds, "tag", "duorelay-socks-18082") {
			t.Fatalf("SOCKS listeners missing: %#v", inbounds)
		}
		rules := root["route"].(map[string]any)["rules"].([]any)
		if !hasSOCKSRoute(rules, "duorelay-socks-18081", firstTag) || !hasSOCKSRoute(rules, "duorelay-socks-18082", secondTag) {
			t.Fatalf("SOCKS routes do not match their assigned nodes: %#v", rules)
		}
		outbounds := root["outbounds"].([]any)
		if !hasConfigEntry(outbounds, "tag", firstTag) || !hasConfigEntry(outbounds, "tag", secondTag) {
			t.Fatalf("pinned outbounds missing: %#v", outbounds)
		}
	} else {
		var root map[string]any
		if err := yaml.Unmarshal(data, &root); err != nil {
			t.Fatal(err)
		}
		listeners := root["listeners"].([]any)
		if !hasConfigEntry(listeners, "name", "duorelay-socks-18081") || !hasConfigEntry(listeners, "name", "duorelay-socks-18082") {
			t.Fatalf("SOCKS listeners missing: %#v", listeners)
		}
		rules := root["rules"].([]any)
		if len(rules) < 2 || rules[0] != "IN-NAME,duorelay-socks-18081,"+firstTag || rules[1] != "IN-NAME,duorelay-socks-18082,"+secondTag {
			t.Fatalf("SOCKS routing rules do not pin each listener: %#v", rules)
		}
		proxies := root["proxies"].([]any)
		if !hasConfigEntry(proxies, "name", firstTag) || !hasConfigEntry(proxies, "name", secondTag) {
			t.Fatalf("pinned proxies missing: %#v", proxies)
		}
		group := root["proxy-groups"].([]any)[0].(map[string]any)
		if hasString(group["proxies"], firstTag) || hasString(group["proxies"], secondTag) {
			t.Fatalf("pinned proxies leaked into the default group: %#v", group)
		}
	}
	if len(state.Public().SOCKSListeners) != 2 {
		t.Fatal("public active state omitted SOCKS listener mappings")
	}
}

func TestSaveSettingsPreservesSOCKSListenersForOlderClients(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	node := &ir.Node{Name: "assigned", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
	if err := s.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	settings := defaults()
	settings.SOCKSListeners = []SOCKSListener{{Port: 18081, NodeID: node.ID}}
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	legacyUpdate, err := s.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	legacyUpdate.SOCKSListeners = nil // Old clients omit this field entirely.
	legacyUpdate.LogLevel = "debug"
	if err := s.SaveSettings(ctx, legacyUpdate); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Settings(ctx)
	if err != nil || len(loaded.SOCKSListeners) != 1 || loaded.SOCKSListeners[0].NodeID != node.ID {
		t.Fatalf("legacy settings update lost port mappings: %#v, %v", loaded.SOCKSListeners, err)
	}
	loaded.SOCKSListeners = []SOCKSListener{}
	if err := s.SaveSettings(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, err = s.Settings(ctx)
	if err != nil || len(loaded.SOCKSListeners) != 0 {
		t.Fatalf("explicit empty mapping list did not clear settings: %#v, %v", loaded.SOCKSListeners, err)
	}
}

func TestSaveSettingsValidatesSOCKSListenerMappings(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	node := &ir.Node{Name: "assigned", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
	if err := s.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	for _, listeners := range [][]SOCKSListener{
		{{Port: 0, NodeID: node.ID}},
		{{Port: 18081, NodeID: node.ID}, {Port: 18081, NodeID: node.ID}},
		{{Port: 1080, NodeID: node.ID}},
		{{Port: 18081}},
		{{Port: 18081, NodeID: "missing"}},
	} {
		settings := defaults()
		settings.SOCKSListeners = listeners
		if err := s.SaveSettings(ctx, settings); !errors.Is(err, store.ErrInvalid) {
			t.Fatalf("invalid mapping accepted: %#v, %v", listeners, err)
		}
	}
	settings := defaults()
	for i := 0; i < maxSOCKSListeners+1; i++ {
		settings.SOCKSListeners = append(settings.SOCKSListeners, SOCKSListener{Port: 20000 + i, NodeID: node.ID})
	}
	if err := s.SaveSettings(ctx, settings); !errors.Is(err, store.ErrInvalid) {
		t.Fatal("more than the supported listener limit was accepted")
	}
	if variant.Name == "lite" && settings.Kernel != ir.KernelMihomo {
		t.Fatal("lite defaults did not select mihomo")
	}
}

func hasConfigEntry(entries []any, key, want string) bool {
	for _, value := range entries {
		if entry, ok := value.(map[string]any); ok && entry[key] == want {
			return true
		}
	}
	return false
}

func hasSOCKSRoute(entries []any, inbound, outbound string) bool {
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok || entry["outbound"] != outbound {
			continue
		}
		inbounds, ok := entry["inbound"].([]any)
		if ok && len(inbounds) == 1 && inbounds[0] == inbound {
			return true
		}
	}
	return false
}

func hasString(values any, want string) bool {
	switch items := values.(type) {
	case []string:
		for _, item := range items {
			if item == want {
				return true
			}
		}
	case []any:
		for _, item := range items {
			if item == want {
				return true
			}
		}
	}
	return false
}
