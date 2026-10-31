package hysteria2

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("http://x") {
		t.Fatal("wrong scheme")
	}
	n, e := p.Parse("hysteria2://pass@example.com:10000-20000?sni=example.com&obfs=salamander&obfs-password=x#n")
	if e != nil || n == nil {
		t.Fatalf("Parse: %v", e)
	}
	if _, e := p.Parse("hysteria2://@example.com:443"); e == nil {
		t.Fatal("missing password accepted")
	}
}
