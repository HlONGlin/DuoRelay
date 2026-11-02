package ssr

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("ss://x") {
		t.Fatal()
	}
	if _, e := p.Parse("ssr://ZXhhbXBsZS5jb206ODM4ODphdXRvOmFlcy0yNTYtY2ZiOm9iZnM6cGFzc3dvcmQ="); e != nil {
		t.Fatal(e)
	}
	if _, e := p.Parse("ssr://bad"); e == nil {
		t.Fatal()
	}
}
