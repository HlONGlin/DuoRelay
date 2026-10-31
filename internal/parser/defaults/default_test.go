package defaults

import "testing"

func TestNewRegistersSixParsers(t *testing.T) {
	r := New()
	for _, name := range []string{"shadowsocks", "vmess", "vless", "trojan", "hysteria2", "tuic"} {
		if _, err := r.Get(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
