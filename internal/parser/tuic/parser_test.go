package tuic

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("http://x") {
		t.Fatal("wrong scheme")
	}
	n, e := p.Parse("tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:pass@example.com:443?congestion_control=bbr&alpn=h3#n")
	if e != nil || n == nil {
		t.Fatalf("Parse: %v", e)
	}
	if _, e := p.Parse("tuic://uuid@example.com:443"); e == nil {
		t.Fatal("missing password accepted")
	}
	if _, e := p.Parse("tuic://b8ca57c2-eb39-491a-8425-bba828fbad2d:pass@example.com:443?udp_relay_mode=native"); e != nil {
		t.Fatal(e)
	}
}
