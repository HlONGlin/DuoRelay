package clashapi_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/clashapi"
)

// TestSingBoxClashAPI tests sing-box Clash API endpoints
// Run with: go test -v ./internal/clashapi -run TestSingBoxClashAPI
// Prerequisites: sing-box running with testdata/configs/singbox-minimal.json
func TestSingBoxClashAPI(t *testing.T) {
	requireLiveIntegration(t)

	adapter := clashapi.NewAdapter("http://127.0.0.1:19094", "test123", clashapi.KernelSingBox)
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("Version", func(t *testing.T) {
		version, err := adapter.GetVersion(ctx)
		if err != nil {
			t.Fatalf("GetVersion failed: %v", err)
		}
		if version.Version == "" {
			t.Error("Version is empty")
		}
		t.Logf("sing-box version: %s", version.Version)
	})

	t.Run("Proxies", func(t *testing.T) {
		proxies, err := adapter.GetProxies(ctx)
		if err != nil {
			t.Fatalf("GetProxies failed: %v", err)
		}
		if len(proxies.Proxies) == 0 {
			t.Error("No proxies returned")
		}
		t.Logf("Found %d proxies", len(proxies.Proxies))
		for name, proxy := range proxies.Proxies {
			t.Logf("  - %s (%s)", name, proxy.Type)
		}
	})

	t.Run("Connections", func(t *testing.T) {
		connections, err := adapter.GetConnections(ctx)
		if err != nil {
			t.Fatalf("GetConnections failed: %v", err)
		}
		t.Logf("Active connections: %d", len(connections.Connections))
		t.Logf("Download: %d, Upload: %d", connections.DownloadTotal, connections.UploadTotal)
	})
}

// TestMihomoClashAPI tests mihomo Clash API endpoints
// Run with: go test -v ./internal/clashapi -run TestMihomoClashAPI
// Prerequisites: mihomo running with testdata/configs/mihomo-minimal.yaml
func TestMihomoClashAPI(t *testing.T) {
	requireLiveIntegration(t)

	adapter := clashapi.NewAdapter("http://127.0.0.1:19093", "test123", clashapi.KernelMihomo)
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("Version", func(t *testing.T) {
		version, err := adapter.GetVersion(ctx)
		if err != nil {
			t.Fatalf("GetVersion failed: %v", err)
		}
		if version.Version == "" {
			t.Error("Version is empty")
		}
		t.Logf("mihomo version: %s", version.Version)
	})

	t.Run("Proxies", func(t *testing.T) {
		proxies, err := adapter.GetProxies(ctx)
		if err != nil {
			t.Fatalf("GetProxies failed: %v", err)
		}
		if len(proxies.Proxies) == 0 {
			t.Error("No proxies returned")
		}
		t.Logf("Found %d proxies", len(proxies.Proxies))
		for name, proxy := range proxies.Proxies {
			t.Logf("  - %s (%s) alive=%v", name, proxy.Type, proxy.Alive)
		}
	})

	t.Run("Connections", func(t *testing.T) {
		connections, err := adapter.GetConnections(ctx)
		if err != nil {
			t.Fatalf("GetConnections failed: %v", err)
		}
		t.Logf("Active connections: %d", len(connections.Connections))
		t.Logf("Download: %d, Upload: %d, Memory: %d",
			connections.DownloadTotal, connections.UploadTotal, connections.Memory)
	})

	t.Run("GetProxy", func(t *testing.T) {
		proxy, err := adapter.GetProxy(ctx, "DIRECT")
		if err != nil {
			t.Fatalf("GetProxy failed: %v", err)
		}
		t.Logf("DIRECT proxy: type=%s, udp=%v, alive=%v",
			proxy.Type, proxy.UDP, proxy.Alive)
	})
}

func requireLiveIntegration(t *testing.T) {
	t.Helper()
	if testing.Short() || os.Getenv("DUORELAY_CLASHAPI_INTEGRATION") != "1" {
		t.Skip("live Clash API integration disabled; set DUORELAY_CLASHAPI_INTEGRATION=1 to enable")
	}
}
