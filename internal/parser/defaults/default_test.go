package defaults

import "testing"

func TestNewRegistersImplementedParsers(t *testing.T) {
	r := New()
	for _, name := range []string{"shadowsocks", "vmess", "vless", "trojan", "hysteria2", "tuic", "socks5", "http", "shadowsocksr", "wireguard", "snell", "hysteria", "ssh", "shadowtls", "naive", "tor", "masque", "anytls", "mieru", "sudoku", "shadowquic", "trusttunnel", "openvpn", "openconnect", "tailscale", "easytier", "zerotier", "rematch"} {
		if _, err := r.Get(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
