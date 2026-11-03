package routing

import (
	"reflect"
	"testing"

	"github.com/holi/duorelay/internal/generator"
)

func TestTextRoundTripPreservesOrderAndFields(t *testing.T) {
	want := []generator.RoutingRule{
		{Source: "bypass", Enabled: true, Match: "domain_suffix", Values: []string{"example.com"}, Outbound: "DIRECT"},
		{Source: "user", Enabled: false, Match: "port", Values: []string{"443", "8443"}, Outbound: "REJECT"},
	}
	got := Parse(Format(want))
	for i := range got.Rules {
		got.Rules[i].ID = ""
	}
	if len(got.Errors) != 0 || !reflect.DeepEqual(got.Rules, want) {
		t.Fatalf("round trip got rules=%#v errors=%#v want=%#v", got.Rules, got.Errors, want)
	}
}

func TestTextImportReportsInvalidLinesWithoutValidRuleLoss(t *testing.T) {
	result := Parse("1\tuser\tdomain_suffix\texample.com\tDIRECT\ninvalid\tuser\tdomain\tbad\tDIRECT\n1\tuser\tunknown\tx\tDIRECT\n")
	if len(result.Rules) != 1 || result.Rules[0].Values[0] != "example.com" {
		t.Fatalf("valid rules were lost: %#v", result.Rules)
	}
	if len(result.Errors) != 2 || result.Errors[0].Line != 2 || result.Errors[1].Line != 3 {
		t.Fatalf("line errors = %#v", result.Errors)
	}
}
