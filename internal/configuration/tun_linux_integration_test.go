//go:build linux

package configuration

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
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
