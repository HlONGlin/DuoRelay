package hysteria

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if !p.CanParse("hysteria://x@example.com:443") {
		t.Fatal()
	}
	if _, e := p.Parse("hysteria://auth@example.com:443?upmbps=20&downmbps=100"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("hysteria://@example.com:443"); e == nil {
		t.Fatal()
	}
}
