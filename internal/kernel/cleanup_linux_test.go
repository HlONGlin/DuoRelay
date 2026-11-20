//go:build linux

package kernel

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMihomoTUNCleanupRequiresExclusiveMetaRoutes(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
		want bool
	}{
		{name: "owned", data: `[{"dev":"Meta"}]`, want: true},
		{name: "mixed", data: `[{"dev":"Meta"},{"dev":"eth0"}]`},
		{name: "unrelated", data: `[{"dev":"wg0"}]`},
		{name: "empty", data: `[]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := exclusivelyUsesMeta([]byte(test.data))
			if err != nil || got != test.want {
				t.Fatalf("ownership = %t, %v; want %t", got, err, test.want)
			}
		})
	}
}

func TestMihomoTUNCleanupSelectsOnlyOwnedRules(t *testing.T) {
	data := []byte(`[
		{"priority":9002,"src":"all","fwmark":"0x1620/0x1620","table":2022},
		{"priority":9002,"src":"10.99.0.0/16","table":"main"},
		{"priority":9002,"src":"10.99.0.0/16","table":2022},
		{"priority":10000,"src":"all","table":2022}
	]`)
	got, err := metaOwnedRuleDeleteArgs(data)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"rule", "del", "priority", "9002", "fwmark", "0x1620/0x1620", "table", "2022"},
		{"rule", "del", "priority", "9002", "from", "10.99.0.0/16", "table", "2022"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delete selectors = %#v, want %#v", got, want)
	}
}

func TestTableNumberAcceptsJSONNumberAndName(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
	}{{`2022`, "2022"}, {`"2022"`, "2022"}, {`"main"`, "main"}} {
		if got := tableNumber(json.RawMessage(test.value)); got != test.want {
			t.Errorf("tableNumber(%s) = %q, want %q", test.value, got, test.want)
		}
	}
}
