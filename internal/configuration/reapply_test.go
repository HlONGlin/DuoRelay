package configuration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/variant"
)

func TestActivateSelectModeSwitchesProxyWithoutRestart(t *testing.T) {
	kernelType := ir.KernelMihomo
	if !variant.SupportsKernel(string(kernelType)) {
		kernelType = ir.KernelSingBox
	}
	s, runtime := testService(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPut || r.URL.Path != "/proxies/PROXY" {
			t.Errorf("unexpected selection request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		} else if body["name"] != "second-node" {
			t.Errorf("selected proxy = %q, want second-node", body["name"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "active-config")
	if err := os.WriteFile(path, []byte("active"), 0600); err != nil {
		t.Fatal(err)
	}
	state := State{
		ID: "active", Kernel: kernelType, Mode: "select",
		NodeIDs:        []string{"first", "second"},
		RuntimeNames:   map[string]string{"first": "first-node", "second": "second-node"},
		SelectedNodeID: "first", Path: path, Controller: server.URL,
	}
	if err := s.Store.SaveState(context.Background(), "current_config", state); err != nil {
		t.Fatal(err)
	}

	got, err := s.ActivateManual(context.Background(), "second", kernelType)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || got == nil || got.SelectedNodeID != "second" {
		t.Fatalf("switch result = %#v, requests = %d", got, requests)
	}
	if runtime.starts != 0 || runtime.stops != 0 {
		t.Fatalf("hot switch restarted runtime: starts=%d stops=%d", runtime.starts, runtime.stops)
	}
	var persisted State
	if err := s.Store.State(context.Background(), "current_config", &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.SelectedNodeID != "second" {
		t.Fatalf("persisted selection = %q, want second", persisted.SelectedNodeID)
	}
}
