package generator

import (
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
)

func chainNode(protocol string) *ir.Node {
	return &ir.Node{Name: protocol, Protocol: protocol, Owner: ir.GetProtocolOwner(protocol), Transport: ir.Transport{Type: "tcp"}, Proto: map[string]any{"udp": true}}
}

func TestChainProtocolCoverage(t *testing.T) {
	for _, protocol := range ir.GetAllProtocols() {
		t.Run(protocol, func(t *testing.T) {
			entry := chainNode(ir.ProtocolAnyTLS)
			node := chainNode(protocol)
			node.ID = "second"
			nodes := []*ir.Node{entry, node}
			if protocol == ir.ProtocolShadowTLS {
				nodes = []*ir.Node{node, chainNode(ir.ProtocolShadowsocks)}
			}
			plan, err := PlanChain(nodes, ir.KernelSingBox, false)
			if protocol == ir.ProtocolRematch {
				if err == nil || !strings.Contains(err.Error(), "路由适配器") {
					t.Fatalf("missing Rematch explanation: %v", err)
				}
				return
			}
			if err != nil || !plan.Supported {
				t.Fatalf("protocol omitted from capabilities: %v", err)
			}
		})
	}
}

func TestChainTransportAndPosition(t *testing.T) {
	for _, tc := range []struct {
		name    string
		nodes   []*ir.Node
		ok, udp bool
		reason  string
	}{
		{"HTTP to QUIC", []*ir.Node{chainNode("http"), chainNode("hysteria2")}, false, false, "需要 UDP"},
		{"QUIC to HTTP", []*ir.Node{chainNode("hysteria2"), chainNode("http")}, true, false, ""},
		{"TCP packet tunnel restores UDP", []*ir.Node{chainNode("http"), chainNode("anytls"), chainNode("tuic")}, true, true, ""},
		{"native UDP cannot bypass TCP predecessor", []*ir.Node{chainNode("http"), chainNode("socks5"), chainNode("tuic")}, false, false, "需要 UDP"},
		{"ShadowTLS cannot exit", []*ir.Node{chainNode("http"), chainNode("shadowtls")}, false, false, "传输包装"},
		{"ShadowTLS requires SS", []*ir.Node{chainNode("shadowtls"), chainNode("http")}, false, false, "Shadowsocks"},
		{"ShadowTLS wraps SS", []*ir.Node{chainNode("shadowtls"), chainNode("shadowsocks")}, true, false, ""},
		{"Mieru TCP", []*ir.Node{chainNode("http"), chainNode("mieru")}, true, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PlanChain(tc.nodes, ir.KernelSingBox, false)
			if tc.ok {
				if err != nil || plan.UDP != tc.udp {
					t.Fatalf("plan=%+v err=%v", plan, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("expected %s, got %v", tc.reason, err)
			}
		})
	}
	for _, proto := range []string{"naive", "trusttunnel", "mieru", "masque"} {
		n := chainNode(proto)
		if proto == "mieru" {
			n.Proto["transport"] = "UDP"
		} else if proto == "masque" {
			n.Proto["network"] = "h3"
		} else {
			n.Proto["quic"] = true
		}
		if _, err := PlanChain([]*ir.Node{chainNode("http"), n}, ir.KernelSingBox, false); err == nil {
			t.Fatalf("UDP mode ignored for %s", proto)
		}
	}
	masque := chainNode("masque")
	masque.Proto["network"] = "h2"
	if _, err := PlanChain([]*ir.Node{chainNode("http"), masque}, ir.KernelSingBox, false); err != nil {
		t.Fatal(err)
	}
}

func TestChainKernelPlanning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		nodes  []*ir.Node
		cross  bool
		kernel ir.KernelType
	}{
		{"prefer saved kernel", []*ir.Node{chainNode("http"), chainNode("anytls")}, false, ir.KernelMihomo},
		{"shared protocols stay with exclusive", []*ir.Node{chainNode("naive"), chainNode("anytls")}, false, ir.KernelSingBox},
		{"exclusive protocols bridge", []*ir.Node{chainNode("naive"), chainNode("mieru")}, true, ir.KernelMihomo},
		{"reverse bridge", []*ir.Node{chainNode("sudoku"), chainNode("naive")}, true, ir.KernelSingBox},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := PlanChain(tc.nodes, ir.KernelMihomo, false)
			if err != nil || plan.CrossKernel != tc.cross || plan.Kernel != tc.kernel {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if tc.cross {
				if _, err := PlanChain(tc.nodes, ir.KernelMihomo, true); err == nil {
					t.Fatal("explicit kernel restriction ignored")
				}
			}
		})
	}
}
