//go:build !lite

package subscription

import (
	"encoding/json"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestImportToGeneratorPreservesProtocolOptions(t *testing.T) {
	m := NewManager()
	nodes, err := m.Parse([]byte(xrayExample))
	if err != nil {
		t.Fatal(err)
	}
	config, err := singbox.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sb struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if json.Unmarshal(config, &sb) != nil {
		t.Fatal("bad sing-box output")
	}
	found := false
	for _, out := range sb.Outbounds {
		if out["type"] == "vless" {
			found = true
			if object(out["transport"])["service_name"] != "grpc-service" || object(object(out["tls"])["reality"])["public_key"] != "test-key" {
				t.Fatal("generated gRPC/Reality differs from imported node")
			}
		}
	}
	if !found {
		t.Fatal("VLESS endpoint missing from generated config")
	}
	nodes, err = m.Parse([]byte("mierus://alice:pass@mieru.example?profile=range&port=3000-3010&protocol=TCP"))
	if err != nil {
		t.Fatal(err)
	}
	config, err = mihomo.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	var mh struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if yaml.Unmarshal(config, &mh) != nil || len(mh.Proxies) != 1 || mh.Proxies[0]["port-range"] != "3000-3010" || mh.Proxies[0]["port"] != nil {
		t.Fatal("generated Mieru range differs from imported binding")
	}
	nodes, err = m.Parse([]byte("[Proxy]\nWS = vmess, vm.example, 443, username=" + sampleUUID + ", ws=true, ws-headers=Host:cdn.example|Origin:https://cdn.example, tls=true"))
	if err != nil {
		t.Fatal(err)
	}
	config, err = mihomo.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if yaml.Unmarshal(config, &mh) != nil || object(object(mh.Proxies[0]["ws-opts"])["headers"])["Origin"] != "https://cdn.example" {
		t.Fatal("mihomo dropped Origin header")
	}
	config, err = singbox.NewGenerator().Generate(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(config, &sb) != nil {
		t.Fatal("bad sing-box output")
	}
	found = false
	for _, out := range sb.Outbounds {
		if out["type"] == "vmess" {
			found = true
			if object(object(out["transport"])["headers"])["Origin"] != "https://cdn.example" {
				t.Fatal("sing-box dropped Origin header")
			}
		}
	}
	if !found {
		t.Fatal("VMess node missing")
	}
}
