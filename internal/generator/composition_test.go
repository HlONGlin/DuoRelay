//go:build !lite

package generator_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

func compositionNodes() []*ir.Node {
	return []*ir.Node{
		{ID: "one", Name: "entry", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 18001}},
		{ID: "two", Name: "exit", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 18002}},
	}
}

func TestCompositionGeneration(t *testing.T) {
	for _, kernel := range []ir.KernelType{ir.KernelSingBox, ir.KernelMihomo} {
		t.Run(string(kernel), func(t *testing.T) {
			opts := generator.DefaultGenerateOptions()
			opts.DefaultOutbound = "PROXY"
			opts.Composition = &generator.Composition{Mode: "chain"}
			opts.SOCKSOnly, opts.SOCKSPort = true, 10808
			var gen generator.Generator = singbox.NewGenerator()
			if kernel == ir.KernelMihomo {
				gen = mihomo.NewGenerator()
			}
			data, err := gen.Generate(compositionNodes(), opts)
			if err != nil {
				t.Fatal(err)
			}
			var root map[string]any
			if kernel == ir.KernelSingBox {
				if err := json.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				outbounds := root["outbounds"].([]any)
				if outbounds[3].(map[string]any)["detour"] != "entry" {
					t.Fatal("exit is not dialed through entry")
				}
				group := outbounds[4].(map[string]any)
				if group["default"] != "exit" {
					t.Fatal("chain target must be final exit")
				}
				rules := root["route"].(map[string]any)["rules"].([]any)
				guard := rules[0].(map[string]any)
				if guard["invert"] != true || guard["outbound"] != "DIRECT" || guard["inbound"].([]any)[0] != "socks-in" {
					t.Fatal("non-SOCKS ingress is not guarded")
				}
				if len(root["inbounds"].([]any)) != 2 {
					t.Fatal("dedicated SOCKS listener missing")
				}
			} else {
				if err := yaml.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				proxies := root["proxies"].([]any)
				if proxies[1].(map[string]any)["dialer-proxy"] != "entry" {
					t.Fatal("chain order reversed")
				}
				if root["rules"].([]any)[0] != "NOT,((IN-NAME,socks-in)),DIRECT" {
					t.Fatal("ingress scope must precede user rules")
				}
				if len(root["listeners"].([]any)) != 1 {
					t.Fatal("SOCKS listener missing")
				}
			}
		})
	}
}

func TestLoadBalanceGeneration(t *testing.T) {
	for _, strategy := range []string{"round-robin", "consistent-hashing", "sticky-sessions", "url-test", "fallback"} {
		t.Run(strategy, func(t *testing.T) {
			lb := generator.DefaultLoadBalanceOptions()
			lb.Strategy = strategy
			opts := generator.DefaultGenerateOptions()
			opts.DefaultOutbound = "PROXY"
			opts.Composition = &generator.Composition{Mode: "load_balance", LoadBalance: &lb}
			data, err := mihomo.NewGenerator().Generate(compositionNodes(), opts)
			if err != nil {
				t.Fatal(err)
			}
			var root map[string]any
			if err := yaml.Unmarshal(data, &root); err != nil {
				t.Fatal(err)
			}
			group := root["proxy-groups"].([]any)[0].(map[string]any)
			wantType := "load-balance"
			if strategy == "url-test" || strategy == "fallback" {
				wantType = strategy
			}
			if group["type"] != wantType || len(group["proxies"].([]any)) != 2 || group["url"] != lb.URL || group["interval"] != lb.Interval || group["timeout"] != lb.Timeout {
				t.Fatalf("wrong group: %#v", group)
			}
			if wantType == "load-balance" && group["strategy"] != strategy {
				t.Fatalf("wrong load-balance strategy: %#v", group)
			}
			if wantType != "load-balance" {
				if _, exists := group["strategy"]; exists {
					t.Fatalf("%s group must not have a load-balance strategy: %#v", wantType, group)
				}
			}
			if _, err := singbox.NewGenerator().Generate(compositionNodes(), opts); err == nil {
				t.Fatal("sing-box selector cannot be passed off as load balancing")
			}
		})
	}
}

func TestMihomoHealthCheckGroupsPassKernelValidation(t *testing.T) {
	binary := filepath.Join("..", "..", "kernels", "mihomo")
	if os.PathSeparator == '\\' {
		binary += ".exe"
	}
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("mihomo binary unavailable: %v", err)
	}
	for _, strategy := range []string{"url-test", "fallback"} {
		t.Run(strategy, func(t *testing.T) {
			lb := generator.DefaultLoadBalanceOptions()
			lb.Strategy = strategy
			opts := generator.DefaultGenerateOptions()
			opts.DefaultOutbound = "PROXY"
			opts.Composition = &generator.Composition{Mode: "load_balance", LoadBalance: &lb}
			config, err := mihomo.NewGenerator().Generate(compositionNodes(), opts)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, config, 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, binary, "-t", "-f", path).CombinedOutput(); err != nil {
				t.Fatalf("mihomo -t failed: %v\n%s", err, output)
			}
		})
	}
}

func TestCompositionValidation(t *testing.T) {
	lb := generator.DefaultLoadBalanceOptions()
	for _, change := range []func(*generator.LoadBalanceOptions){
		func(v *generator.LoadBalanceOptions) { v.Strategy = "bad" },
		func(v *generator.LoadBalanceOptions) { v.URL = "file:///secret" },
		func(v *generator.LoadBalanceOptions) { v.Interval = 0 },
		func(v *generator.LoadBalanceOptions) { v.Timeout = 0 },
	} {
		bad := lb
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid health check options accepted")
		}
	}
	for _, strategy := range []string{"url-test", "fallback"} {
		valid := lb
		valid.Strategy = strategy
		if err := valid.Validate(); err != nil {
			t.Fatalf("%s strategy rejected: %v", strategy, err)
		}
	}
	nodes := compositionNodes()
	nodes[1].Name = nodes[0].Name
	prepared, err := generator.PrepareComposition(nodes, &generator.Composition{Mode: "chain"}, ir.KernelMihomo)
	if err != nil || prepared[0].Name == prepared[1].Name || nodes[0].Name != nodes[1].Name {
		t.Fatal("duplicate names must be distinct at runtime without mutating stored nodes")
	}
	nodes[1].Protocol = ir.ProtocolWireGuard
	if _, err := generator.PrepareComposition(nodes, &generator.Composition{Mode: "chain"}, ir.KernelSingBox); err == nil || !strings.Contains(err.Error(), "链式") {
		t.Fatal("unsupported endpoint chain accepted")
	}
}

func TestSOCKSOnlyGuardWithTUN(t *testing.T) {
	for _, gen := range []generator.Generator{singbox.NewGenerator(), mihomo.NewGenerator()} {
		t.Run(string(gen.Kernel()), func(t *testing.T) {
			opts := generator.DefaultGenerateOptions()
			opts.EnableTUN, opts.SOCKSOnly, opts.SOCKSPort = true, true, 10808
			opts.DefaultOutbound = "PROXY"
			opts.RoutingRules = []generator.RoutingRule{{Match: "network", Values: []string{"tcp"}, Outbound: "PROXY"}}
			data, err := gen.Generate(compositionNodes(), opts)
			if err != nil {
				t.Fatal(err)
			}
			var root map[string]any
			if gen.Kernel() == ir.KernelSingBox {
				if err := json.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				if len(root["inbounds"].([]any)) != 3 {
					t.Fatal("TUN, mixed and SOCKS listeners must be present")
				}
				rules := root["route"].(map[string]any)["rules"].([]any)
				if rules[0].(map[string]any)["action"] != "hijack-dns" {
					t.Fatal("TUN DNS must be hijacked before user routes")
				}
				guard := rules[1].(map[string]any)
				if guard["invert"] != true || guard["outbound"] != "DIRECT" {
					t.Fatal("TUN must bypass custom proxy rules")
				}
			} else {
				if err := yaml.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				if root["tun"].(map[string]any)["enable"] != true || root["rules"].([]any)[0] != "NOT,((IN-NAME,socks-in)),DIRECT" {
					t.Fatal("TUN must bypass custom proxy rules")
				}
			}
		})
	}
}
