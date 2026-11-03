package api

import (
	"encoding/json"
	"testing"
)

func TestRoutingTextAPIReportsLineErrorsWithoutChangingSettings(t *testing.T) {
	s := apiForTest(t)
	before := call(t, s, "GET", "/api/config", "", 200)
	var exported struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(call(t, s, "GET", "/api/routing/export", "", 200), &exported); err != nil {
		t.Fatal(err)
	}
	if exported.Text == "" {
		t.Fatal("routing export is empty")
	}
	input := `{"text":"1\tuser\tdomain_suffix\texample.com\tDIRECT\ninvalid\tuser\tdomain\tbad\tDIRECT\n"}`
	var result struct {
		Rules  []map[string]any `json:"rules"`
		Errors []struct {
			Line int `json:"line"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(call(t, s, "POST", "/api/routing/import", input, 200), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Rules) != 1 || len(result.Errors) != 1 || result.Errors[0].Line != 2 {
		t.Fatalf("unexpected import result: %#v", result)
	}
	after := call(t, s, "GET", "/api/config", "", 200)
	if string(before) != string(after) {
		t.Fatalf("parse-only import changed settings: before=%s after=%s", before, after)
	}
}
