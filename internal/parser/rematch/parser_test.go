package rematch

import (
	"errors"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestParseConfig(t *testing.T) {
	parsed, err := New().Parse(`{"type":"rematch","name":"recheck","target-rematch-name":"proxy-rule"}`)
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Name != "recheck" || node.Owner != ir.KernelSetMihomo || node.Server.Host != "" || node.Server.Port != 0 {
		t.Fatalf("unexpected rematch node: %+v", node)
	}
	if node.Proto["target_rematch_name"] != "proxy-rule" {
		t.Fatalf("target was not normalized: %+v", node.Proto)
	}
}

func TestRequiresTarget(t *testing.T) {
	if _, err := New().Parse(`{"type":"rematch","name":"missing"}`); !errors.Is(err, parser.ErrMissingField) {
		t.Fatalf("missing target error = %v", err)
	}
	if _, err := New().Parse(`{"type":"rematch","target-sub-rule":12}`); !errors.Is(err, parser.ErrInvalidField) {
		t.Fatalf("wrong target type error = %v", err)
	}
}
