package defaults_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser/defaults"
	"github.com/holi/duorelay/internal/subscription"
)

// Exercise the registry used by real subscriptions, including aggregate Base64
// envelopes. This catches parsers that work directly but were never registered.
func TestP2CProtocolsThroughSubscription(t *testing.T) {
	protocols := []string{"naive", "tor", "masque", "anytls", "mieru", "sudoku", "shadowquic", "trusttunnel"}
	registry := defaults.New()
	links := []string{}
	for _, protocol := range protocols {
		if _, err := registry.Get(protocol); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile("../../../testdata/samples/" + protocol + "_samples.json")
		if err != nil {
			t.Fatal(err)
		}
		var samples []struct{ Link string }
		if err := json.Unmarshal(data, &samples); err != nil {
			t.Fatal(err)
		}
		if len(samples) == 0 {
			t.Fatal("no samples for", protocol)
		}
		for _, sample := range samples {
			node, err := registry.ParseAny(sample.Link)
			if err != nil {
				t.Fatal(protocol, err)
			}
			if node.(*ir.Node).Protocol != protocol {
				t.Fatal("wrong parser", protocol)
			}
		}
		links = append(links, samples[0].Link)
	}
	manager := subscription.NewManager()
	plain := strings.Join(links, "\n")
	for _, body := range []string{plain, base64.StdEncoding.EncodeToString([]byte(plain))} {
		nodes, err := manager.Parse([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if len(nodes) != len(protocols) {
			t.Fatalf("got %d nodes, want %d", len(nodes), len(protocols))
		}
		for i, node := range nodes {
			if node.Protocol != protocols[i] {
				t.Fatal("protocol/order mismatch", i, node.Protocol)
			}
			if node.Protocol == "masque" && node.Owner != ir.KernelSetMihomo {
				t.Fatal("MASQUE ownership mismatch")
			}
		}
	}
}
