package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/holi/duorelay/internal/configuration"
	"github.com/holi/duorelay/internal/ir"
)

func TestProxyGroupStatusReturnsCurrentAndHealth(t *testing.T) {
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, "{\"proxies\":{\"PROXY\":{\"now\":\"fast\",\"all\":[\"fast\",\"backup\"]},\"fast\":{\"alive\":true,\"history\":[{\"time\":\"2026-11-02T12:00:00Z\",\"delay\":42}]},\"backup\":{\"alive\":false}}}")
	}))
	defer control.Close()

	s := apiForTest(t)
	_, err := s.Config.Apply(context.Background(), configuration.ApplyRequest{
		Kernel:  ir.KernelMihomo,
		Content: "external-controller: 127.0.0.1:19093\nsecret: test\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.Config.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state.Mode = "load_balance"
	state.Controller, state.Secret = control.URL, "test"
	if err := s.Config.Store.SaveState(context.Background(), "current_config", state); err != nil {
		t.Fatal(err)
	}

	data := call(t, s, http.MethodGet, "/api/config/proxy-group", "", http.StatusOK)
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	members := got["members"].([]any)
	first, second := members[0].(map[string]any), members[1].(map[string]any)
	if got["active"] != true || got["now"] != "fast" || len(members) != 2 || first["name"] != "fast" || first["alive"] != true || first["delay_ms"] != float64(42) || second["alive"] != false {
		t.Fatalf("unexpected proxy group status: %#v", got)
	}
}
