package http

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("ftp://x") {
		t.Fatal()
	}
	if _, e := p.Parse("https://u:p@example.com:443#x"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("http://example.com:80"); e != nil {
		t.Fatal(e)
	}
}
