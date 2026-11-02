package ssh

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if _, e := p.Parse("ssh://user:pass@example.com:22#x"); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("ssh://@example.com:22"); e == nil {
		t.Fatal()
	}
}
