package shadowtls

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if _, e := p.Parse("shadowtls://pass@example.com:443?version=3&host=www.example.com&sni=www.example.com"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("shadowtls://@example.com:443"); e == nil {
		t.Fatal()
	}
}
