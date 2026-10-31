package shadowsocks

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("http://x") {
		t.Fatal("wrong scheme")
	}
	if !p.CanParse("ss://x@example.com:1") {
		t.Fatal("CanParse")
	}
	n, e := p.Parse("ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@example.com:8388#n")
	if e != nil || n == nil {
		t.Fatalf("Parse: %v", e)
	}
	if _, e := p.Parse("ss://bad@example.com:8388"); e == nil {
		t.Fatal("bad payload accepted")
	}
	if _, e := p.Parse("ss://YWVzLTI1Ni1nY206cGFzcw==@example.com:8388?plugin=obfs-local%3Bobfs%3Dhttp#n"); e != nil {
		t.Fatal(e)
	}
}
