package vless

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("http://x") {
		t.Fatal("wrong scheme")
	}
	n, e := p.Parse("vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@example.com:443?security=reality&sni=example.com&pbk=k&sid=s&type=tcp#n")
	if e != nil || n == nil {
		t.Fatalf("Parse: %v", e)
	}
	if _, e := p.Parse("vless://@example.com:443"); e == nil {
		t.Fatal("missing uuid accepted")
	}
	if _, e := p.Parse("vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@example.com:443?type=invalid"); e == nil {
		t.Fatal("invalid transport accepted")
	}
	if _, e := p.Parse("vless://b8ca57c2-eb39-491a-8425-bba828fbad2d@example.com:443?type=ws&security=tls&path=%2Fx&host=example.com"); e != nil {
		t.Fatal(e)
	}
}
