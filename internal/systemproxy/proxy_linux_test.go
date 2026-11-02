//go:build linux

package systemproxy

import (
	"strings"
	"testing"
)

func TestLinuxSystemProxySetAndRestore(t *testing.T) {
	oldRunner := runGSettings
	oldLookPath := lookPath
	defer func() {
		runGSettings = oldRunner
		lookPath = oldLookPath
	}()
	lookPath = func(string) (string, error) { return "/usr/bin/gsettings", nil }

	values := map[string]string{}
	runGSettings = func(args ...string) ([]byte, error) {
		if len(args) == 1 && args[0] == "list-schemas" {
			return []byte(gnomeProxySchema + "\n"), nil
		}
		if len(args) == 2 && args[0] == "list-recursively" {
			return []byte(gnomeProxySchema + " mode manual\n"), nil
		}
		if len(args) >= 3 && args[0] == "get" {
			key := args[2]
			if len(args) >= 4 && strings.HasPrefix(args[1], gnomeProxySchema+".") {
				key = strings.TrimPrefix(args[1], gnomeProxySchema+".") + ":" + args[2]
			}
			return []byte(values[key]), nil
		}
		if len(args) >= 4 && args[0] == "set" {
			key := args[2]
			if strings.HasPrefix(args[1], gnomeProxySchema+".") {
				key = strings.TrimPrefix(args[1], gnomeProxySchema+".") + ":" + args[2]
			}
			values[key] = args[3]
			return nil, nil
		}
		return nil, nil
	}

	snapshot, err := Capture()
	if err != nil {
		t.Fatal(err)
	}
	if err := Set("127.0.0.1", 7890); err != nil {
		t.Fatal(err)
	}
	if values["http:host"] != "'127.0.0.1'" || values["http:port"] != "7890" || values["mode"] != "'manual'" {
		t.Fatalf("unexpected gsettings values: %#v", values)
	}
	if err := Restore(snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxSystemProxyRejectsInvalidTarget(t *testing.T) {
	if err := Set("proxy.example", 7890); err == nil {
		t.Fatal("hostname unexpectedly accepted")
	}
	if err := Set("127.0.0.1", 0); err == nil {
		t.Fatal("invalid port unexpectedly accepted")
	}
}
