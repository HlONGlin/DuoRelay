package clashapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/holi/duorelay/internal/clashapi"
)

func TestAdapterWithDeterministicMockAPI(t *testing.T) {
	for _, kernel := range []clashapi.KernelType{clashapi.KernelSingBox, clashapi.KernelMihomo} {
		t.Run(string(kernel), func(t *testing.T) {
			server := newMockClashServer(t, kernel)
			defer server.Close()

			adapter := clashapi.NewAdapter(server.URL, "test-secret", kernel)
			defer adapter.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			version, err := adapter.GetVersion(ctx)
			if err != nil {
				t.Fatalf("GetVersion: %v", err)
			}
			if version.Version != "mock-1.0.0" {
				t.Fatalf("version = %q, want mock-1.0.0", version.Version)
			}

			proxies, err := adapter.GetProxies(ctx)
			if err != nil {
				t.Fatalf("GetProxies: %v", err)
			}
			if got := proxies.Proxies["DIRECT"].Type; got != "direct" {
				t.Fatalf("DIRECT type = %q, want direct", got)
			}
			if kernel == clashapi.KernelMihomo && !proxies.Proxies["RELAY"].Alive {
				t.Fatal("mihomo RELAY should preserve alive metadata")
			}

			proxy, err := adapter.GetProxy(ctx, "DIRECT")
			if err != nil {
				t.Fatalf("GetProxy: %v", err)
			}
			if proxy.Name != "DIRECT" {
				t.Fatalf("proxy name = %q, want DIRECT", proxy.Name)
			}

			connections, err := adapter.GetConnections(ctx)
			if err != nil {
				t.Fatalf("GetConnections: %v", err)
			}
			if len(connections.Connections) != 1 || connections.DownloadTotal != 2048 {
				t.Fatalf("unexpected connections response: %#v", connections)
			}
			if kernel == clashapi.KernelMihomo && connections.Memory != 4096 {
				t.Fatalf("mihomo memory = %d, want 4096", connections.Memory)
			}

			delay, err := adapter.TestProxyDelay(ctx, "RELAY", "http://example.test/ping", 1500)
			if err != nil {
				t.Fatalf("TestProxyDelay: %v", err)
			}
			if delay.Delay != 88 {
				t.Fatalf("delay = %d, want 88", delay.Delay)
			}

			traffic, err := adapter.ReadTraffic(ctx)
			if err != nil {
				t.Fatalf("ReadTraffic: %v", err)
			}
			if traffic.Up != 128 || traffic.Down != 2048 {
				t.Fatalf("unexpected traffic sample: %#v", traffic)
			}
		})
	}
}

func TestAdapterMockAPIRejectsInvalidSecret(t *testing.T) {
	server := newMockClashServer(t, clashapi.KernelMihomo)
	defer server.Close()

	adapter := clashapi.NewAdapter(server.URL, "wrong-secret", clashapi.KernelMihomo)
	defer adapter.Close()
	if _, err := adapter.GetVersion(context.Background()); err == nil {
		t.Fatal("GetVersion should reject an invalid secret")
	}
	if _, err := adapter.ReadTraffic(context.Background()); err == nil {
		t.Fatal("ReadTraffic should reject an invalid secret")
	}
}

func TestAdapterMockAPINotFoundProxy(t *testing.T) {
	server := newMockClashServer(t, clashapi.KernelSingBox)
	defer server.Close()

	adapter := clashapi.NewAdapter(server.URL, "test-secret", clashapi.KernelSingBox)
	defer adapter.Close()
	if _, err := adapter.GetProxy(context.Background(), "MISSING"); err == nil {
		t.Fatal("GetProxy should fail for an unknown proxy")
	}
}

func newMockClashServer(t *testing.T, kernel clashapi.KernelType) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			writeJSON(t, w, http.StatusUnauthorized, map[string]string{"message": "unauthorized"})
			return
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/traffic":
			upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			_ = conn.WriteJSON(clashapi.TrafficData{Up: 128, Down: 2048})
		case r.Method == http.MethodGet && r.URL.Path == "/version":
			writeJSON(t, w, http.StatusOK, clashapi.Version{Version: "mock-1.0.0", Meta: kernel == clashapi.KernelMihomo})
		case r.Method == http.MethodGet && r.URL.Path == "/proxies":
			relay := clashapi.Proxy{Name: "RELAY", Type: "http", UDP: true}
			if kernel == clashapi.KernelMihomo {
				relay.Alive = true
				relay.ID = "mock-relay"
			}
			writeJSON(t, w, http.StatusOK, clashapi.ProxiesResponse{Proxies: map[string]clashapi.Proxy{
				"DIRECT": {Name: "DIRECT", Type: "direct"},
				"RELAY":  relay,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/connections":
			memory := int64(0)
			if kernel == clashapi.KernelMihomo {
				memory = 4096
			}
			writeJSON(t, w, http.StatusOK, clashapi.ConnectionsResponse{
				DownloadTotal: 2048,
				UploadTotal:   512,
				Memory:        memory,
				Connections:   []clashapi.Connection{{ID: "conn-1", Start: time.Unix(1, 0).UTC()}},
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/proxies/RELAY/delay"):
			if r.URL.Query().Get("timeout") != "1500" || r.URL.Query().Get("url") != "http://example.test/ping" {
				writeJSON(t, w, http.StatusBadRequest, map[string]string{"message": "invalid delay query"})
				return
			}
			writeJSON(t, w, http.StatusOK, clashapi.DelayTestResponse{Delay: 88})
		default:
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		}
	}))
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode mock response: %v", err)
	}
}
