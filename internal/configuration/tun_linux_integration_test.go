//go:build linux

package configuration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/speedtest"
	"github.com/holi/duorelay/internal/transparent"
)

func TestLinuxTUNCaptureAndCleanup(t *testing.T) {
	if os.Getenv("DUORELAY_TUN_INTEGRATION") != "1" {
		t.Skip("set DUORELAY_TUN_INTEGRATION=1 inside an isolated NET_ADMIN Linux namespace")
	}
	kernelsDir := os.Getenv("DUORELAY_KERNELS_DIR")
	if kernelsDir == "" {
		kernelsDir = filepath.Join("..", "..", "kernels")
	}
	if _, err := exec.LookPath("ip"); err != nil {
		t.Fatal("iproute2 is required for TUN route cleanup checks")
	}

	for _, item := range []struct {
		name      string
		kernel    ir.KernelType
		generator generator.Generator
	}{
		{name: "sing-box", kernel: ir.KernelSingBox, generator: registeredGenerators()[ir.KernelSingBox]},
		{name: "mihomo", kernel: ir.KernelMihomo, generator: registeredGenerators()[ir.KernelMihomo]},
	} {
		if item.generator == nil {
			continue
		}
		t.Run(item.name, func(t *testing.T) {
			options := generator.DefaultGenerateOptions()
			options.EnableTUN = true
			config, err := item.generator.Generate(nil, options)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), item.name+map[bool]string{true: ".yaml", false: ".json"}[item.kernel == ir.KernelMihomo])
			if err := os.WriteFile(path, config, 0600); err != nil {
				t.Fatal(err)
			}
			runtimeManager := kernel.NewManager(kernelsDir)
			t.Cleanup(func() {
				if err := runtimeManager.StopAll(); err != nil {
					t.Errorf("cleanup kernel: %v", err)
				}
			})
			if err := runtimeManager.Validate(string(item.kernel), path); err != nil {
				t.Fatalf("kernel config validation: %v", err)
			}
			before, err := linuxNetworkState()
			if err != nil {
				t.Fatal(err)
			}
			start := runtimeManager.StartSingBox
			if item.kernel == ir.KernelMihomo {
				start = runtimeManager.StartMihomo
			}
			if err := start(path); err != nil {
				t.Fatalf("start TUN: %v", err)
			}
			if !waitForTUNAddress(item.kernel, 5*time.Second) {
				logs := runtimeManager.GetLogs(string(item.kernel), 100)
				t.Fatalf("TUN interface did not acquire the generated IPv4/IPv6 address; logs: %#v", logs)
			}
			if err := runtimeManager.StopAll(); err != nil {
				t.Fatalf("stop kernel: %v", err)
			}
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				after, snapshotErr := linuxNetworkState()
				if snapshotErr == nil && after == before && !hasTUNAddress(item.kernel) {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			after, _ := linuxNetworkState()
			t.Fatalf("TUN cleanup did not restore routes/rules: before=%q after=%q address=%v", before, after, hasTUNAddress(item.kernel))
		})
	}
}

func linuxNetworkState() (string, error) {
	var output strings.Builder
	for _, args := range [][]string{{"-details", "rule", "show"}, {"-details", "route", "show", "table", "all"}} {
		data, err := exec.Command("ip", args...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, data)
		}
		output.Write(data)
	}
	return output.String(), nil
}

func waitForTUNAddress(kernelType ir.KernelType, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hasTUNAddress(kernelType) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func hasTUNAddress(kernelType ir.KernelType) bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	wantIPv6 := "fdfe:dcba:9876::1/"
	wantIPv4 := ""
	if kernelType == ir.KernelSingBox {
		wantIPv4 = "172.19.0.1/"
	}
	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		ipv4, ipv6 := false, false
		for _, address := range addresses {
			value := address.String()
			if ip, _, err := net.ParseCIDR(value); err == nil {
				ipv4 = ipv4 || (ip.To4() != nil && (wantIPv4 == "" || strings.HasPrefix(value, wantIPv4)))
				ipv6 = ipv6 || (ip.To4() == nil && strings.HasPrefix(strings.ToLower(value), wantIPv6))
			}
		}
		if ipv4 && ipv6 {
			return true
		}
	}
	return false
}

// The fixture is a listening TCP socket on the pre-TUN network, with the next
// port closed. The fixture address must be routed through the default interface.
// If DUORELAY_TUN_PROBE_DNS is set, it must resolve probe.duorelay.invalid to
// the fixture address and the same probe is run by hostname.
func TestLinuxTUNProbeUsesDirectRoute(t *testing.T) {
	target := os.Getenv("DUORELAY_TUN_PROBE_TARGET")
	if target == "" {
		t.Skip("set DUORELAY_TUN_PROBE_TARGET in the isolated TUN integration container")
	}
	host, portString, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	kernelsDir := os.Getenv("DUORELAY_KERNELS_DIR")
	if kernelsDir == "" {
		kernelsDir = filepath.Join("..", "..", "kernels")
	}
	for kernelType := range registeredGenerators() {
		t.Run(string(kernelType), func(t *testing.T) {
			ctx := context.Background()
			s, _ := testService(t)
			s.Runtime = kernel.NewManager(kernelsDir)
			t.Cleanup(func() {
				if err := s.Stop(ctx); err != nil {
					t.Error(err)
				}
			})
			settings := defaults()
			settings.Kernel = kernelType
			settings.TransparentMode = transparent.ModeTUN
			settings.RoutingMode = "global"
			probeHost := host
			if dns := os.Getenv("DUORELAY_TUN_PROBE_DNS"); dns != "" {
				settings.DNSBootstrap = dns
				probeHost = "probe.duorelay.invalid."
			}
			settings.RoutingRules = []generator.RoutingRule{{Match: "ip_cidr", Values: []string{host + "/32"}, Outbound: "REJECT"}}
			if err := s.SaveSettings(ctx, settings); err != nil {
				t.Fatal(err)
			}
			// Use a listener on loopback as the current proxy, while the nodes
			// being measured are inactive and live on the other interface.
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("probe reached the active proxy")
				w.WriteHeader(http.StatusForbidden)
			}))
			defer proxy.Close()
			_, proxyPortString, _ := net.SplitHostPort(proxy.Listener.Addr().String())
			proxyPort, _ := strconv.Atoi(proxyPortString)
			active := &ir.Node{Name: "active", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: proxyPort}}
			if err := s.Store.SaveNode(ctx, active); err != nil {
				t.Fatal(err)
			}
			state, err := s.Activate(ctx, active.ID, kernelType)
			if err != nil {
				t.Fatal(err)
			}
			if !waitForTUNAddress(kernelType, 5*time.Second) {
				t.Fatal("missing TUN interface")
			}
			// Confirm an ordinary socket is captured by the TUN's reject rule.
			conn, err := net.DialTimeout("tcp", target, time.Second)
			if err == nil {
				_ = conn.SetReadDeadline(time.Now().Add(time.Second))
				data := make([]byte, 1)
				if _, err := conn.Read(data); err == nil {
					t.Error("ordinary TCP connection reached the fixture despite the TUN reject rule")
				}
				conn.Close()
			}
			engine := speedtest.NewEngine(s.Store, nil)
			engine.ProbeTCP = s.ProbeTCP
			for offset := range 2 {
				node := &ir.Node{Name: fmt.Sprintf("node-%d", offset), Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: probeHost, Port: port + offset}}
				if err := s.Store.SaveNode(ctx, node); err != nil {
					t.Fatal(err)
				}
				result, err := engine.Test(ctx, node.ID, speedtest.Options{TimeoutMS: 1500})
				if err != nil || result.Success != (offset == 0) {
					t.Fatalf("TCP probe for %s:%d: %+v, %v", host, port+offset, result, err)
				}
			}
			current, err := s.Current(ctx)
			if err != nil || current.ID != state.ID {
				t.Fatalf("active configuration changed during test: %+v, %v", current, err)
			}
		})
	}
}
