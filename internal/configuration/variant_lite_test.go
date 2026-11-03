//go:build lite

package configuration

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

func TestLiteGeneratorsAndSavedFullPreference(t *testing.T) {
	s, _ := testService(t)
	if len(s.Generators) != 1 || s.Generators[ir.KernelMihomo] == nil {
		t.Fatal("lite registry must only contain mihomo")
	}
	ctx := context.Background()
	saved := defaults()
	saved.Kernel = ir.KernelSingBox
	if err := s.Store.SaveState(ctx, "settings", saved); err != nil {
		t.Fatal(err)
	}
	value, err := s.Settings(ctx)
	if err != nil || value.Kernel != ir.KernelMihomo {
		t.Fatalf("saved preference projection: %v %v", value, err)
	}
	var after Settings
	if err := s.Store.State(ctx, "settings", &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, after) {
		t.Fatal("reading lite rewrote full settings")
	}
	for _, protocol := range []string{ir.ProtocolShadowTLS, ir.ProtocolNaive, ir.ProtocolTor, ir.ProtocolOpenConnect} {
		n := &ir.Node{Name: string(protocol), Protocol: protocol, Owner: ir.KernelSetSingBox, Server: ir.Server{Host: "example.com", Port: 443}}
		if err := s.Store.SaveNode(ctx, n); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Activate(ctx, n.ID, ""); !errors.Is(err, store.ErrInvalid) || !strings.Contains(err.Error(), "精简版不支持") {
			t.Fatalf("unsupported activation: %v", err)
		}
		got, err := s.Store.Node(ctx, n.ID)
		if err != nil || got.Protocol != protocol || got.Owner != ir.KernelSetSingBox {
			t.Fatal("unsupported node lost")
		}
	}
}

func TestLiteCompositionValidationRollbackAndResume(t *testing.T) {
	s, _ := testService(t)
	runtime := &chainRuntime{}
	s.Runtime = runtime
	ctx := context.Background()
	ids := []string{}
	for _, name := range []string{"entry", "exit"} {
		n := &ir.Node{Name: name, Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8088}}
		if err := s.Store.SaveNode(ctx, n); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n.ID)
	}
	for _, mode := range []string{"single", "load_balance", "chain"} {
		request := ApplyRequest{NodeIDs: ids, Mode: mode}
		if mode == "single" {
			request.Mode = ""
			request.NodeIDs = ids[:1]
		}
		state, err := s.Apply(ctx, request)
		if err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if state.Kernel != ir.KernelMihomo || len(state.Auxiliary) != 0 {
			t.Fatal("not a native mihomo config")
		}
		if mode == "chain" {
			if state.Chain == nil || state.Chain.CrossKernel {
				t.Fatal("chain plan missing or bridged")
			}
			data, err := os.ReadFile(state.Path)
			if err != nil || !strings.Contains(string(data), "dialer-proxy: entry") || strings.Contains(string(data), "DR-BRIDGE-") {
				t.Fatalf("chain config: %s %v", data, err)
			}
		}
		stops := runtime.stops
		runtime.validationFailure = "mihomo"
		if _, err := s.Apply(ctx, request); !errors.Is(err, ErrApply) || runtime.stops != stops {
			t.Fatalf("validation disrupted runtime: %v", err)
		}
		runtime.validationFailure = ""
		runtime.failKernel = "mihomo"
		if _, err := s.Apply(ctx, request); !errors.Is(err, ErrApply) {
			t.Fatalf("startup failure hidden: %v", err)
		}
		current, err := s.Current(ctx)
		if err != nil || current.ID != state.ID || runtime.current != state.Path {
			t.Fatal("failed startup did not roll back")
		}
		if err := s.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		if err := s.Resume(ctx); err != nil {
			t.Fatal(err)
		}
		if runtime.current != state.Path {
			t.Fatal("resume did not restore mihomo")
		}
	}
}
