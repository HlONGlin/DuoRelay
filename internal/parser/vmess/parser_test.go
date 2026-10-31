package vmess

import "testing"

func TestParser(t *testing.T) {
	p := New()
	if p.CanParse("http://x") {
		t.Fatal("wrong scheme")
	}
	if !p.CanParse("vmess://x") {
		t.Fatal("CanParse")
	}
	n, e := p.Parse("vmess://eyJ2IjoiMiIsImFkZCI6ImV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiYjhjYTU3YzItZWIzOS00OTFhLTg0MjUtYmJhODI4ZmJhZDJkIiwicHMiOiJ0IiwibmV0IjoidGNwIiwidGxzIjoidGxzIn0=")
	if e != nil || n == nil {
		t.Fatalf("Parse: %v", e)
	}
	if _, e := p.Parse("vmess://bad"); e == nil {
		t.Fatal("bad payload accepted")
	}
	if _, e := p.Parse("vmess://eyJ2IjoiMiIsImFkZCI6ImV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyJ9"); e == nil {
		t.Fatal("missing uuid accepted")
	}
	if _, e := p.Parse("vmess://eyJhZGQiOiJ3cy5leGFtcGxlLmNvbSIsInBvcnQiOiI0NDMiLCJpZCI6ImI4Y2E1N2MyLWViMzktNDkxYS04NDI1LWJiYTgyOGZiYWQyZCIsIm5ldCI6IndzIiwiaG9zdCI6IndzLmV4YW1wbGUuY29tIiwicGF0aCI6Ii92bWVzcyIsInRscyI6InRscyJ9"); e != nil {
		t.Fatal(e)
	}
}
