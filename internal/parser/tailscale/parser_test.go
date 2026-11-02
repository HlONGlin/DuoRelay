package tailscale

import (
	"errors"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestParseConfig(t *testing.T) {
	input := `{"type":"tailscale","tag":"tailnet","state-directory":"C:/tailscale","auth-key":"tskey-auth","exit-node":"100.64.0.2","accept-routes":true,"advertise-routes":["10.0.0.0/8"],"listen-port":41641}`
	parsed, err := New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Name != "tailnet" || node.Owner != ir.KernelSetBoth {
		t.Fatalf("unexpected node identity: %+v", node)
	}
	if node.Proto["state_directory"] != "C:/tailscale" || node.Proto["auth_key"] != "tskey-auth" || node.Proto["accept_routes"] != true {
		t.Fatalf("options were not normalized: %+v", node.Proto)
	}
	if node.Proto["advertise_routes"].([]any)[0] != "10.0.0.0/8" || node.Proto["listen_port"] != float64(41641) {
		t.Fatalf("list or integer option was lost: %+v", node.Proto)
	}
}

func TestRejectsWrongTypes(t *testing.T) {
	for _, input := range []string{
		`{"type":"tailscale","accept-routes":"sometimes"}`,
		`{"type":"tailscale","listen-port":12.5}`,
		`{"type":"other"}`,
	} {
		if _, err := New().Parse(input); err == nil {
			t.Errorf("expected parse error for %s", input)
		}
	}
	if _, err := New().Parse(`{"type":"tailscale","name":""}`); !errors.Is(err, parser.ErrMissingField) {
		t.Fatalf("blank name error = %v", err)
	}
}
