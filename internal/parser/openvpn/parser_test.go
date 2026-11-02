package openvpn

import (
	"testing"

	"github.com/holi/duorelay/internal/ir"
)

func TestParseProfile(t *testing.T) {
	input := `client
dev tun
proto tcp-client
remote vpn.example 443 tcp
remote backup.example 1194
remote-random
data-ciphers AES-256-GCM:AES-128-GCM
<ca>
certificate data
</ca>
<tls-crypt>
key data
</tls-crypt>`
	parsed, err := New().Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Protocol != ir.ProtocolOpenVPN || node.Owner != ir.KernelSetBoth {
		t.Fatalf("unexpected protocol ownership: %+v", node)
	}
	if node.Server.Host != "vpn.example" || node.Server.Port != 443 || node.Transport.Type != "tcp" {
		t.Fatalf("unexpected primary remote: %+v %+v", node.Server, node.Transport)
	}
	remotes := node.Proto["remotes"].([]map[string]any)
	if len(remotes) != 2 || remotes[1]["host"] != "backup.example" || remotes[1]["port"] != 1194 {
		t.Fatalf("unexpected remotes: %#v", remotes)
	}
	if node.Proto["remote_random"] != true || len(node.Proto["data_ciphers"].([]string)) != 2 {
		t.Fatalf("missing profile options: %#v", node.Proto)
	}
	if node.Proto["ca"] != "certificate data" || node.Proto["tls_crypt"] != "key data" {
		t.Fatalf("inline credentials not preserved: %#v", node.Proto)
	}
}

func TestParseShareURIAndErrors(t *testing.T) {
	parsed, err := New().Parse("openvpn://vpn.example:443?proto=tcp&username=user&password=secret#Office")
	if err != nil {
		t.Fatal(err)
	}
	node := parsed.(*ir.Node)
	if node.Name != "Office" || node.Server.Port != 443 || node.Proto["password"] != "secret" {
		t.Fatalf("unexpected URI parse: %+v", node)
	}
	for _, input := range []string{"remote vpn.example 0 udp\nclient", "remote vpn.example 443 icmp\nclient", "<ca>\nunterminated"} {
		if _, err := New().Parse(input); err == nil {
			t.Errorf("expected error for %q", input)
		}
	}
}

func TestCanParseConfigNarrowly(t *testing.T) {
	p := New()
	if !p.CanParse("client\nremote vpn.example 1194") {
		t.Fatal("valid profile was not detected")
	}
	if p.CanParse("remote vpn.example 1194") {
		t.Fatal("generic remote line was detected as an OpenVPN profile")
	}
	if p.CanParse("{\"remote\":\"vpn.example\"}") {
		t.Fatal("JSON was detected as an OpenVPN profile")
	}
}
