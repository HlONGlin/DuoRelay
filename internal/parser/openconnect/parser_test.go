package openconnect

import (
	"testing"

	"github.com/holi/duorelay/internal/ir"
)

func TestParseSingBoxEndpoint(t *testing.T) {
	input := "{\"type\":\"openconnect\",\"tag\":\"work-vpn\",\"server\":\"https://vpn.example:8443\",\"flavor\":\"gp\",\"username\":\"alice\",\"password\":\"secret\",\"tls\":{\"server_name\":\"vpn.example\",\"insecure\":false}}"
	parsed, err := New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Name != "work-vpn" || node.Owner != ir.KernelSetSingBox || node.Server.Host != "vpn.example" || node.Server.Port != 8443 {
		t.Fatalf("unexpected node: %+v", node)
	}
	if node.Proto["flavor"] != "gp" || node.Proto["password"] != "secret" || node.Security.SNI != "vpn.example" || node.Security.Opts["insecure"] != false {
		t.Fatalf("endpoint options were not retained: %+v", node)
	}
}

func TestParseDefaultsAndRejectsInvalidInput(t *testing.T) {
	p := New()
	valid := "{\"type\":\"openconnect\",\"server\":\"vpn.example\"}"
	if !p.CanParse(valid) || p.CanParse("{\"type\":\"openvpn-client\"}") {
		t.Fatal("incorrect endpoint detection")
	}
	parsed, err := p.Parse(valid)
	if err != nil || parsed.(*ir.Node).Server.Port != 443 {
		t.Fatalf("default port: %v %v", parsed, err)
	}
	for _, input := range []string{"{\"type\":\"openconnect\"}", "{\"type\":\"openconnect\",\"server\":\"http://vpn.example\"}", "{\"type\":\"openconnect\",\"server\":\"https://user:pass@vpn.example\"}", "not-json"} {
		if _, err := p.Parse(input); err == nil {
			t.Errorf("expected failure for %s", input)
		}
	}
}
