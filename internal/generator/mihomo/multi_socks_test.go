package mihomo

import (
	"testing"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

func TestGenerateMultipleSOCKSListenersStayOutsideDefaultProxyGroup(t *testing.T) {
	primary := &ir.Node{ID: "primary", Name: "primary", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
	first := &ir.Node{ID: "first", Name: "DuoRelay-SOCKS-first", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8081}}
	second := &ir.Node{ID: "second", Name: "DuoRelay-SOCKS-second", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8082}}
	opts := generator.DefaultGenerateOptions()
	opts.DefaultOutbound = "PROXY"
	opts.ExtraNodes = []*ir.Node{first, second}
	opts.SOCKSListeners = []generator.SOCKSListener{{Port: 18081, Outbound: first.Name}, {Port: 18082, Outbound: second.Name}}
	data, err := NewGenerator().Generate([]*ir.Node{primary}, opts)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	listeners := root["listeners"].([]any)
	if len(listeners) != 2 {
		t.Fatalf("listeners = %#v", listeners)
	}
	rules := root["rules"].([]any)
	if len(rules) < 2 || rules[0] != "IN-NAME,duorelay-socks-18081,"+first.Name || rules[1] != "IN-NAME,duorelay-socks-18082,"+second.Name {
		t.Fatalf("listener pinning rules are missing or out of order: %#v", rules)
	}
	group := root["proxy-groups"].([]any)[0].(map[string]any)
	if hasString(group["proxies"], first.Name) || hasString(group["proxies"], second.Name) {
		t.Fatalf("dedicated exits must stay out of the default group: %#v", group)
	}
}

func TestGenerateSOCKSListenerRejectsConflictingPort(t *testing.T) {
	opts := generator.DefaultGenerateOptions()
	opts.SOCKSListeners = []generator.SOCKSListener{{Port: opts.MixedPort, Outbound: "node"}}
	if _, err := NewGenerator().Generate([]*ir.Node{{Name: "node", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}}, opts); err == nil {
		t.Fatal("listener colliding with mixed port was accepted")
	}
}

func hasString(values any, want string) bool {
	switch items := values.(type) {
	case []string:
		for _, value := range items {
			if value == want {
				return true
			}
		}
	case []any:
		for _, value := range items {
			if value == want {
				return true
			}
		}
	}
	return false
}

func TestGenerateSOCKSListenerRejectsTransparentPortCollision(t *testing.T) {
	node := &ir.Node{Name: "node", Protocol: ir.ProtocolHTTP, Owner: ir.KernelSetBoth, Server: ir.Server{Host: "127.0.0.1", Port: 8080}}
	for _, test := range []struct {
		name string
		mode string
		port int
	}{
		{name: "redirect", mode: "redirect", port: 18080},
		{name: "tproxy", mode: "tproxy", port: 18081},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts := generator.DefaultGenerateOptions()
			opts.TransparentMode = test.mode
			opts.RedirectPort = test.port
			opts.TProxyPort = test.port
			opts.SOCKSListeners = []generator.SOCKSListener{{Port: test.port, Outbound: node.Name}}
			if _, err := NewGenerator().Generate([]*ir.Node{node}, opts); err == nil {
				t.Fatalf("listener colliding with %s port %d was accepted", test.mode, test.port)
			}
		})
	}
}
