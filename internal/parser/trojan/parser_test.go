package trojan

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("http://x") {
		t.Fatal("wrong scheme")
	}
	n, e := p.Parse("trojan://pass@example.com:443?type=grpc&serviceName=x&sni=example.com#n")
	if e != nil || n == nil {
		t.Fatalf("Parse: %v", e)
	}
	if _, e := p.Parse("trojan://@example.com:443"); e == nil {
		t.Fatal("missing password accepted")
	}
	if _, e := p.Parse("trojan://pass@example.com:443?type=ws&path=%2Fx&host=example.com"); e != nil {
		t.Fatal(e)
	}
}
