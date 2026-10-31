package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
)

type fakeRuntime struct {
	current string
	stops   int
	starts  int
}

func (f *fakeRuntime) Validate(name, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "fail_validation") {
		return errors.New("invalid config")
	}
	return nil
}
func (f *fakeRuntime) StartSingBox(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	f.starts++
	if strings.Contains(string(data), "fail_start") {
		return errors.New("startup failed")
	}
	f.current = path
	return nil
}
func (f *fakeRuntime) StartMihomo(path string) error { return f.StartSingBox(path) }
func (f *fakeRuntime) StopAll() error                { f.current = ""; f.stops++; return nil }
func testService(t *testing.T) (*Service, *fakeRuntime) {
	t.Helper()
	s, err := store.New(&store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	runtime := &fakeRuntime{}
	return New(s, runtime, t.TempDir()), runtime
}

func TestApplyValidationAndRollback(t *testing.T) {
	s, runtime := testService(t)
	ctx := context.Background()
	state, err := s.Apply(ctx, ApplyRequest{Kernel: ir.KernelSingBox, Content: `{"outbounds":[{"type":"direct"}]}`})
	if err != nil {
		t.Fatal(err)
	}
	if state.Controller == "" || state.Secret == "" || state.Path == "" || runtime.current != state.Path {
		t.Fatalf("bad state: %#v", state)
	}
	if public := state.Public(); public.Secret != "" || public.Path != "" || public.Controller != "" {
		t.Fatal("public state exposed secrets")
	}
	stops := runtime.stops
	if _, err := s.Apply(ctx, ApplyRequest{Content: `{"fail_validation":true}`}); !errors.Is(err, ErrApply) {
		t.Fatalf("validation accepted: %v", err)
	}
	if runtime.stops != stops || runtime.current != state.Path {
		t.Fatal("validation stopped old runtime")
	}
	if _, err := s.Apply(ctx, ApplyRequest{Content: `{"fail_start":true}`}); !errors.Is(err, ErrApply) {
		t.Fatalf("start failure accepted: %v", err)
	}
	current, err := s.Current(ctx)
	if err != nil || current.ID != state.ID || runtime.current != state.Path {
		t.Fatal("failed apply did not roll back")
	}
	files, err := os.ReadDir(s.Directory)
	if err != nil || len(files) != 1 {
		t.Fatalf("failed files retained: %v %v", files, err)
	}
}

func TestActivationAndSettings(t *testing.T) {
	s, _ := testService(t)
	ctx := context.Background()
	settings, err := s.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.LogLevel = "debug"
	if err := s.SaveSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	settings.Kernel = "invalid"
	if err := s.SaveSettings(ctx, settings); !errors.Is(err, store.ErrInvalid) {
		t.Fatal("invalid settings accepted")
	}
	node := &ir.Node{Name: "selected", Protocol: ir.ProtocolShadowsocks, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8388}, Proto: map[string]any{"method": "aes-256-gcm", "password": "test"}}
	if err := s.Store.SaveNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	state, err := s.Activate(ctx, node.ID, ir.KernelSingBox)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.NodeIDs) != 1 || state.NodeIDs[0] != node.ID {
		t.Fatal("activation lost node membership")
	}
	data, err := os.ReadFile(state.Path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if root["route"].(map[string]any)["final"] != "selected" {
		t.Fatal("activation did not route to node")
	}
	if _, err := s.Apply(ctx, ApplyRequest{Kernel: "invalid"}); !errors.Is(err, store.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Apply(ctx, ApplyRequest{Content: "not JSON"}); !errors.Is(err, store.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestHTTPProbeMembership(t *testing.T) {
	s, _ := testService(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" || !strings.HasSuffix(r.URL.Path, "/delay") {
			t.Error("bad Clash request")
		}
		fmt.Fprint(w, `{"delay":17}`)
	}))
	defer server.Close()
	state := State{Kernel: ir.KernelSingBox, NodeIDs: []string{"node"}, Controller: server.URL, Secret: "secret"}
	if err := s.Store.SaveState(context.Background(), "current_config", state); err != nil {
		t.Fatal(err)
	}
	delay, err := s.ProbeHTTP(context.Background(), &ir.Node{ID: "node", Name: "node"}, "https://example.com", time.Second)
	if err != nil || delay != 17 {
		t.Fatalf("probe: %d %v", delay, err)
	}
	if _, err := s.ProbeHTTP(context.Background(), &ir.Node{ID: "missing"}, "https://example.com", time.Second); err == nil {
		t.Fatal("node outside active configuration accepted")
	}
}

func TestRealConfigurationApply(t *testing.T) {
	if os.Getenv("DUORELAY_BACKEND_KERNEL_INTEGRATION") != "1" {
		t.Skip("set DUORELAY_BACKEND_KERNEL_INTEGRATION=1")
	}
	db, err := store.New(&store.Config{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dir, err := filepath.Abs(filepath.Join("..", "..", "kernels"))
	if err != nil {
		t.Fatal(err)
	}
	runtime := kernel.NewManager(dir)
	defer runtime.StopAll()
	s := New(db, runtime, t.TempDir())
	address := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		value := listener.Addr().String()
		listener.Close()
		return value
	}
	for _, name := range []ir.KernelType{ir.KernelSingBox, ir.KernelMihomo} {
		var config string
		if name == ir.KernelSingBox {
			config = fmt.Sprintf(`{"outbounds":[{"type":"direct","tag":"DIRECT"}],"experimental":{"clash_api":{"external_controller":%q}}}`, address())
		} else {
			config = fmt.Sprintf(`{"mode":"direct","mixed-port":0,"dns":{"enable":false},"external-controller":%q}`, address())
		}
		state, err := s.Apply(context.Background(), ApplyRequest{Kernel: name, Content: config})
		if err != nil {
			t.Fatal(err)
		}
		if err := runtime.HealthCheck(context.Background(), string(name)); err != nil {
			t.Fatal(err)
		}
		if current, err := s.Current(context.Background()); err != nil || current.ID != state.ID {
			t.Fatal("configuration not persisted")
		}
	}
	// A generated activation also passes the actual kernel's validator.
	settings, _ := s.Settings(context.Background())
	settings.DNSServers = nil
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listener.Close()
	settings.InboundPort, _ = strconv.Atoi(port)
	if err := s.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	node := &ir.Node{Name: "selected", Protocol: ir.ProtocolShadowsocks, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8388}, Proto: map[string]any{"method": "aes-256-gcm", "password": "test"}}
	if err := db.SaveNode(context.Background(), node); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Activate(context.Background(), node.ID, ir.KernelSingBox); err != nil {
		t.Fatal(err)
	}
}
