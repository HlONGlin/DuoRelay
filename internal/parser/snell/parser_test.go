package snell

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if _, e := p.Parse("snell://psk@example.com:443?version=4&obfs=http"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("snell://example.com:443"); e == nil {
		t.Fatal()
	}
}
