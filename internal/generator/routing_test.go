package generator_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

func testRoutingRules() []generator.RoutingRule {
	return []generator.RoutingRule{
		{Match: "domain_suffix", Values: []string{"example.com", "example.org"}, Outbound: "PROXY"},
		{Match: "ip_cidr", Values: []string{"10.0.0.0/8"}, Outbound: "DIRECT"},
		{Match: "port", Values: []string{"443", "8443"}, Outbound: "PROXY"},
	}
}

func TestValidateRoutingRules(t *testing.T) {
	valid := testRoutingRules()
	if err := generator.ValidateRoutingRules(valid); err != nil {
		t.Fatalf("valid rules rejected: %v", err)
	}

	for name, rules := range map[string][]generator.RoutingRule{
		"unknown matcher":  {{Match: "process_name", Values: []string{"curl"}, Outbound: "DIRECT"}},
		"missing values":   {{Match: "domain", Outbound: "DIRECT"}},
		"missing outbound": {{Match: "domain", Values: []string{"example.com"}}},
		"empty value":      {{Match: "domain", Values: []string{" "}, Outbound: "DIRECT"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := generator.ValidateRoutingRules(rules); err == nil {
				t.Fatal("invalid rules accepted")
			}
		})
	}
}

func TestValidateOutboundReferences(t *testing.T) {
	available := map[string]struct{}{"DIRECT": {}, "PROXY": {}, "node-a": {}}
	if err := generator.ValidateOutboundReferences("PROXY", []generator.RoutingRule{{Match: "domain", Values: []string{"example.com"}, Outbound: "node-a"}}, available); err != nil {
		t.Fatalf("valid outbound references rejected: %v", err)
	}
	if err := generator.ValidateOutboundReferences("missing", nil, available); err == nil {
		t.Fatal("unknown default outbound accepted")
	}
	if err := generator.ValidateOutboundReferences("DIRECT", []generator.RoutingRule{{Match: "domain", Values: []string{"example.com"}, Outbound: "missing"}}, available); err == nil {
		t.Fatal("unknown rule outbound accepted")
	}
}

func TestSingBoxRoutingRules(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.DefaultOutbound = "DIRECT"
	opts.RoutingRules = testRoutingRules()
	node := &ir.Node{
		Name:     "sample-node",
		Protocol: ir.ProtocolHTTP,
		Owner:    ir.KernelSetBoth,
		Server:   ir.Server{Host: "example.com", Port: 8080},
		Proto:    map[string]any{"username": "user", "password": "pass"},
		Security: ir.Security{Type: "none"},
	}
	data, err := singbox.NewGenerator().Generate([]*ir.Node{node}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	route := config["route"].(map[string]any)
	rules := route["rules"].([]any)
	if len(rules) != 4 {
		t.Fatalf("route rule count = %d, want 4", len(rules))
	}
	first := rules[0].(map[string]any)
	if first["domain_suffix"].([]any)[0] != "example.com" || first["outbound"] != "PROXY" {
		t.Fatalf("unexpected first sing-box rule: %#v", first)
	}
	if route["final"] != "DIRECT" {
		t.Fatalf("route final = %v, want DIRECT", route["final"])
	}
}

func TestSingBoxProxyOutboundSelector(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.DefaultOutbound = "PROXY"
	node := &ir.Node{
		Name:     "sample-node",
		Protocol: ir.ProtocolHTTP,
		Owner:    ir.KernelSetBoth,
		Server:   ir.Server{Host: "example.com", Port: 8080},
		Proto:    map[string]any{"username": "user", "password": "pass"},
		Security: ir.Security{Type: "none"},
	}
	data, err := singbox.NewGenerator().Generate([]*ir.Node{node}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	outbounds := config["outbounds"].([]any)
	for _, value := range outbounds {
		outbound := value.(map[string]any)
		if outbound["tag"] == "PROXY" {
			if outbound["type"] != "selector" {
				t.Fatalf("PROXY type = %v, want selector", outbound["type"])
			}
			return
		}
	}
	t.Fatal("sing-box config is missing PROXY selector")
}

func TestMihomoRoutingRules(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.DefaultOutbound = "DIRECT"
	opts.RoutingRules = testRoutingRules()
	node := &ir.Node{
		Name:     "sample-node",
		Protocol: ir.ProtocolHTTP,
		Owner:    ir.KernelSetBoth,
		Server:   ir.Server{Host: "example.com", Port: 8080},
		Proto:    map[string]any{"username": "user", "password": "pass"},
	}
	data, err := mihomo.NewGenerator().Generate([]*ir.Node{node}, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	rules := config["rules"].([]any)
	got := make([]string, len(rules))
	for i, value := range rules {
		got[i] = value.(string)
	}
	want := []string{
		"DOMAIN-SUFFIX,example.com,PROXY",
		"DOMAIN-SUFFIX,example.org,PROXY",
		"IP-CIDR,10.0.0.0/8,DIRECT",
		"DST-PORT,443,PROXY",
		"DST-PORT,8443,PROXY",
		"MATCH,DIRECT",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("rules = %#v, want %#v", got, want)
	}
}

func TestGeoRoutingRules(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.RoutingRules = []generator.RoutingRule{
		{Match: "geosite", Values: []string{"cn"}, Outbound: "DIRECT"},
		{Match: "geoip", Values: []string{"cn"}, Outbound: "DIRECT"},
	}
	singBoxData, err := singbox.NewGenerator().Generate(nil, opts)
	if err != nil {
		t.Fatalf("sing-box geo generate: %v", err)
	}
	var singBox map[string]any
	if err := json.Unmarshal(singBoxData, &singBox); err != nil {
		t.Fatal(err)
	}
	route := singBox["route"].(map[string]any)
	ruleSets := route["rule_set"].([]any)
	if len(ruleSets) != 2 {
		t.Fatalf("sing-box rule-set count = %d", len(ruleSets))
	}
	mihomoData, err := mihomo.NewGenerator().Generate(nil, opts)
	if err != nil {
		t.Fatalf("mihomo geo generate: %v", err)
	}
	var mihomoConfig map[string]any
	if err := yaml.Unmarshal(mihomoData, &mihomoConfig); err != nil {
		t.Fatal(err)
	}
	rules := mihomoConfig["rules"].([]any)
	if rules[0] != "GEOSITE,cn,DIRECT" || rules[1] != "GEOIP,cn,DIRECT" {
		t.Fatalf("unexpected geo rules: %#v", rules[:2])
	}
}

func TestRoutingRulesPassPinnedKernelChecks(t *testing.T) {
	// Keep executable checks offline. Geo rules are covered structurally above;
	// mihomo otherwise downloads GeoIP databases during validation.
	rules := []generator.RoutingRule{
		{Match: "domain_suffix", Values: []string{"example.com"}, Outbound: "DIRECT"},
		{Match: "ip_cidr", Values: []string{"10.0.0.0/8"}, Outbound: "DIRECT"},
	}

	singBoxOptions := generator.DefaultGenerateOptions()
	singBoxOptions.RoutingRules = rules
	singBoxConfig, err := singbox.NewGenerator().Generate(nil, singBoxOptions)
	if err != nil {
		t.Fatalf("sing-box Generate: %v", err)
	}
	singBoxBinary := filepath.Join("..", "..", "kernels", "sing-box")
	if runtime.GOOS == "windows" {
		singBoxBinary += ".exe"
	}
	if _, err := os.Stat(singBoxBinary); err == nil {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, singBoxConfig, 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(singBoxBinary, "check", "-c", path).CombinedOutput(); err != nil {
			t.Fatalf("sing-box route check: %v\n%s", err, output)
		}
	}

	mihomoOptions := generator.DefaultGenerateOptions()
	mihomoOptions.RoutingRules = rules
	mihomoConfig, err := mihomo.NewGenerator().Generate(nil, mihomoOptions)
	if err != nil {
		t.Fatalf("mihomo Generate: %v", err)
	}
	mihomoBinary := filepath.Join("..", "..", "kernels", "mihomo")
	if runtime.GOOS == "windows" {
		mihomoBinary += ".exe"
	}
	if _, err := os.Stat(mihomoBinary); err == nil {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, mihomoConfig, 0o600); err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(mihomoBinary, "-t", "-f", path).CombinedOutput(); err != nil {
			t.Fatalf("mihomo route check: %v\n%s", err, output)
		}
	}
}
