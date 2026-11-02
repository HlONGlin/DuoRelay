package easytier

import (
	"errors"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestParseConfig(t *testing.T) {
	input := `{"type":"easytier","name":"mesh","network-name":"office","network-secret":"secret","peers":["tcp://peer.example:11010"],"listeners":["tcp://0.0.0.0:11010"],"mtu":1400,"udp":true,"enable-encryption":true}`
	parsed, err := New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Name != "mesh" || node.Owner != ir.KernelSetMihomo {
		t.Fatalf("unexpected node identity: %+v", node)
	}
	if node.Proto["network_name"] != "office" || node.Proto["network_secret"] != "secret" || node.Proto["mtu"] != float64(1400) {
		t.Fatalf("options were not normalized: %+v", node.Proto)
	}
	if node.Proto["peers"].([]any)[0] != "tcp://peer.example:11010" || node.Proto["enable_encryption"] != true {
		t.Fatalf("list or boolean option was lost: %+v", node.Proto)
	}
}

func TestRejectsWrongTypes(t *testing.T) {
	if _, err := New().Parse(`{"type":"easytier","peers":["tcp://peer"],"mtu":"large"}`); !errors.Is(err, parser.ErrInvalidField) {
		t.Fatalf("wrong mtu error = %v", err)
	}
}
