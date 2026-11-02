package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
)

type chainRuntime struct {
	fakeRuntime
	mode              kernel.RunMode
	paths             []string
	failKernel        string
	validationFailure string
}

func (r *chainRuntime) SetMode(mode kernel.RunMode) error { r.mode = mode; return nil }
func (r *chainRuntime) StopAll() error                    { r.paths = nil; return r.fakeRuntime.StopAll() }
func (r *chainRuntime) Validate(k, path string) error {
	if k == r.validationFailure {
		return errors.New("intentional validation failure")
	}
	return r.fakeRuntime.Validate(k, path)
}
func (r *chainRuntime) StartSingBox(path string) error { return r.start("sing-box", path) }
func (r *chainRuntime) StartMihomo(path string) error  { return r.start("mihomo", path) }
func (r *chainRuntime) start(k, path string) error {
	if k == r.failKernel {
		r.failKernel = ""
		return errors.New("intentional startup failure")
	}
	r.paths = append(r.paths, path)
	return r.fakeRuntime.StartSingBox(path)
}

func TestCrossKernelLifecycle(t *testing.T) {
	s, _ := testService(t)
	runtime := &chainRuntime{}
	s.Runtime = runtime
	ctx := context.Background()
	var ids []string
	for _, owner := range []ir.KernelSet{ir.KernelSetSingBox, ir.KernelSetMihomo, ir.KernelSetSingBox} {
		n := &ir.Node{Name: "PROXY", Protocol: "http", Owner: owner, Server: ir.Server{Host: "127.0.0.1", Port: 8088}}
		if err := s.Store.SaveNode(ctx, n); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n.ID)
	}
	state, err := s.Apply(ctx, ApplyRequest{NodeIDs: ids, Mode: "chain"})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Chain.CrossKernel || len(state.Auxiliary) != 1 || len(runtime.paths) != 2 || runtime.mode != kernel.ModeCoexist {
		t.Fatal("both runtimes not started")
	}
	if state.Chain.Hops[0].Name == "PROXY" || state.Chain.Hops[0].Name == state.Chain.Hops[2].Name {
		t.Fatal("duplicate/reserved names not normalized")
	}
	public, _ := json.Marshal(state.Public())
	for _, secret := range []string{state.Secret, state.Controller, state.Path, state.Auxiliary[0].Secret, state.Auxiliary[0].Path} {
		if strings.Contains(string(public), secret) {
			t.Fatal("public state contains a secret or runtime path")
		}
	}
	stops := runtime.stops
	runtime.validationFailure = "mihomo"
	if _, err := s.Apply(ctx, ApplyRequest{NodeIDs: ids, Mode: "chain"}); !errors.Is(err, ErrApply) || runtime.stops != stops {
		t.Fatal("validation disrupted old chain")
	}
	runtime.validationFailure = ""
	runtime.failKernel = "sing-box"
	if _, err := s.Apply(ctx, ApplyRequest{NodeIDs: ids, Mode: "chain"}); !errors.Is(err, ErrApply) {
		t.Fatal("start failure not reported")
	}
	current, err := s.Current(ctx)
	if err != nil || current.ID != state.ID || len(runtime.paths) != 2 {
		t.Fatal("previous bundle not restored")
	}
	files, _ := os.ReadDir(s.Directory)
	if len(files) != 2 {
		t.Fatalf("failed configurations retained: %d", len(files))
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	files, _ = os.ReadDir(s.Directory)
	if len(files) != 0 || len(runtime.paths) != 0 {
		t.Fatal("stop did not remove both configurations")
	}
}

func TestInvalidChainPreservesCurrent(t *testing.T) {
	s, runtime := testService(t)
	ctx := context.Background()
	var ids []string
	for _, p := range []string{"http", "tuic"} {
		n := &ir.Node{Name: p, Protocol: p, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8088}}
		if err := s.Store.SaveNode(ctx, n); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, n.ID)
	}
	plan, err := s.PreviewChain(ctx, ApplyRequest{NodeIDs: ids})
	if err != nil || plan.Supported || !strings.Contains(plan.Reason, "UDP") {
		t.Fatal("preview did not explain transport mismatch")
	}
	if _, err := s.Apply(ctx, ApplyRequest{Mode: "chain", NodeIDs: ids}); !errors.Is(err, store.ErrInvalid) || runtime.stops != 0 {
		t.Fatal("invalid chain changed runtime")
	}
}
