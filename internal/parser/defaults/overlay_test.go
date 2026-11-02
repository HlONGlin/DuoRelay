package defaults_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/defaults"
)

func TestOverlaySamplesThroughDefaultRegistry(t *testing.T) {
	for _, protocol := range []string{"tailscale", "easytier", "zerotier", "rematch"} {
		data, err := os.ReadFile("../../../testdata/samples/" + protocol + "_samples.json")
		if err != nil {
			t.Fatal(err)
		}
		var samples []struct {
			Link string
		}
		if err := json.Unmarshal(data, &samples); err != nil {
			t.Fatal(err)
		}
		if len(samples) == 0 {
			t.Fatal("no samples for", protocol)
		}
		for _, sample := range samples {
			value, err := defaults.New().ParseAny(sample.Link)
			if err != nil {
				t.Fatalf("%s: %v", protocol, err)
			}
			node, ok := value.(*ir.Node)
			if !ok || node.Protocol != protocol {
				t.Fatalf("%s: unexpected parsed value %#v", protocol, value)
			}
		}
	}
}
