package zerotier

import (
	"errors"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestParseConfig(t *testing.T) {
	input := `{"type":"zerotier","name":"zt","network":"8056c2e21c000001","state-dir":"C:/zerotier","identity-secret":"secret","dns":["10.0.0.53"],"orbit":[{"world":"1","seed":"abc"}],"mtu":2800}`
	parsed, err := New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Name != "zt" || node.Owner != ir.KernelSetMihomo || node.Proto["network"] != "8056c2e21c000001" {
		t.Fatalf("unexpected node: %+v", node)
	}
	if node.Proto["state_dir"] != "C:/zerotier" || node.Proto["dns"].([]any)[0] != "10.0.0.53" {
		t.Fatalf("options were not normalized: %+v", node.Proto)
	}
}

func TestRequiresNetworkAndValidOrbit(t *testing.T) {
	if _, err := New().Parse(`{"type":"zerotier"}`); !errors.Is(err, parser.ErrMissingField) {
		t.Fatalf("missing network error = %v", err)
	}
	if _, err := New().Parse(`{"type":"zerotier","network":"id","orbit":["bad"]}`); !errors.Is(err, parser.ErrInvalidField) {
		t.Fatalf("invalid orbit error = %v", err)
	}
}
