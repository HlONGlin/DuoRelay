package shadowquic

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

func TestSanitizedSamples(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/samples/shadowquic_samples.json")
	if err != nil {
		t.Fatal(err)
	}
	var samples []struct {
		Name       string
		Link       string
		ExpectedIR map[string]any `json:"expected_ir"`
	}
	if err := json.Unmarshal(data, &samples); err != nil {
		t.Fatal(err)
	}
	if len(samples) == 0 {
		t.Fatal("no samples")
	}
	p := New()
	if p.Protocol() != "shadowquic" || p.CanParse("https://example.com") {
		t.Fatal("protocol detection")
	}
	for _, sample := range samples {
		t.Run(sample.Name, func(t *testing.T) {
			if !p.CanParse("  " + sample.Link + "  ") {
				t.Fatal("scheme not detected")
			}
			result, err := p.Parse(sample.Link)
			if err != nil {
				t.Fatal(err)
			}
			node, ok := result.(*ir.Node)
			if !ok {
				t.Fatalf("unexpected result %T", result)
			}
			if node.Name == "" || node.Source.Type != "import" || node.Source.URL == "" || node.Server.Host == "" || node.Server.Port < 1 {
				t.Fatal("incomplete IR", node)
			}
			encoded, err := json.Marshal(node)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			assertFields(t, got, sample.ExpectedIR, "")
		})
	}
}

func assertFields(t *testing.T, got, want map[string]any, path string) {
	t.Helper()
	for key, expected := range want {
		field := path + "." + key
		if nested, ok := expected.(map[string]any); ok {
			actual, ok := got[key].(map[string]any)
			if !ok {
				t.Errorf("%s: missing object", field)
				continue
			}
			assertFields(t, actual, nested, field)
		} else if !reflect.DeepEqual(got[key], expected) {
			t.Errorf("%s: got %#v, want %#v", field, got[key], expected)
		}
	}
}

func TestInvalidInput(t *testing.T) {
	p := New()
	cases := []struct {
		input string
		want  error
	}{
		{"https://u:p@example.com:443", parser.ErrInvalidFormat},
		{"shadowquic://u:p@example.com:0", parser.ErrInvalidField},
		{"shadowquic://example.com:443", parser.ErrMissingField},
		{"shadowquic://p@example.com:443?quic-versions=v3", parser.ErrInvalidField},
		{"shadowquic://p@example.com:443?recv-window=-1", parser.ErrInvalidField},
		{"shadowquic://p@example.com:443?insecure=true", parser.ErrInvalidField},
		{"shadowquic://p@example.com:443?max-open-streams=0", parser.ErrInvalidField},
	}
	for i, tc := range cases {
		result, err := p.Parse(tc.input)
		if result != nil || !errors.Is(err, tc.want) {
			t.Errorf("case %d: result %T, error %v, want %v", i, result, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "sample-only") {
			t.Fatal("credential leaked in error")
		}
	}
}
