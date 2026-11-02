package configuration

import (
	"context"
	"errors"
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

func TestApplyComposedNodes(t *testing.T) {
	s, runtime := testService(t)
	ctx := context.Background()
	settings := defaults()
	settings.SOCKSOnly = true
	settings.LoadBalance.Strategy = "sticky-sessions"
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, name := range []string{"entry", "exit"} {
		node := &ir.Node{Name: name, Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8088}}
		if err := s.Store.SaveNode(ctx, node); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, node.ID)
	}
	state, err := s.Apply(ctx, ApplyRequest{NodeIDs: ids, Mode: "load_balance"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Kernel != ir.KernelMihomo || state.Mode != "load_balance" || state.LoadBalance.Strategy != "sticky-sessions" || len(state.NodeIDs) != 2 {
		t.Fatalf("wrong active state: %#v", state)
	}
	stops := runtime.stops
	bad := generator.DefaultLoadBalanceOptions()
	bad.Strategy = "bad"
	for _, request := range []ApplyRequest{
		{NodeIDs: ids, Mode: "load_balance", LoadBalance: &bad},
		{NodeIDs: []string{ids[0]}, Mode: "chain"},
		{NodeIDs: ids, Mode: "unsupported"},
		{NodeIDs: []string{ids[0], ids[1], ids[0]}, Mode: "chain"},
		{NodeIDs: ids, Mode: "load_balance", Kernel: ir.KernelSingBox},
		{Content: `{"outbounds":[]}`},
	} {
		if _, err := s.Apply(ctx, request); !errors.Is(err, store.ErrInvalid) {
			t.Fatalf("invalid composition accepted: %v", err)
		}
		if runtime.stops != stops {
			t.Fatal("invalid composition stopped running kernel")
		}
	}
	reversed := []string{ids[1], ids[0]}
	state, err = s.Apply(ctx, ApplyRequest{NodeIDs: reversed, Mode: "chain"})
	if err != nil {
		t.Fatal(err)
	}
	if state.Kernel != ir.KernelSingBox || state.NodeIDs[0] != ids[1] || state.NodeIDs[1] != ids[0] {
		t.Fatal("chain order was not preserved")
	}
}

func TestSOCKSOnlyPortValidation(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	settings := defaults()
	settings.SOCKSOnly = true
	settings.SOCKSPort = settings.InboundPort
	if err := s.SaveSettings(ctx, settings); !errors.Is(err, store.ErrInvalid) {
		t.Fatal("conflicting SOCKS port accepted")
	}
	settings.SOCKSPort = 10808
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Settings(ctx)
	if err != nil || !loaded.SOCKSOnly || loaded.SOCKSPort != 10808 {
		t.Fatal("SOCKS settings did not persist")
	}
}
