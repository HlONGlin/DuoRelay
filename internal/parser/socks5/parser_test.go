package socks5

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if !p.CanParse("socks5://x@example.com:1") {
		t.Fatal()
	}
	if _, e := p.Parse("socks5://u:p@example.com:1080#x"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("socks5://@example.com:1080"); e != nil {
		t.Fatal(e)
	}
}
