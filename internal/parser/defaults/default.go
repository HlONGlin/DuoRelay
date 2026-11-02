package defaults

import (
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/anytls"
	"github.com/holi/duorelay/internal/parser/easytier"
	"github.com/holi/duorelay/internal/parser/http"
	"github.com/holi/duorelay/internal/parser/hysteria"
	"github.com/holi/duorelay/internal/parser/hysteria2"
	"github.com/holi/duorelay/internal/parser/masque"
	"github.com/holi/duorelay/internal/parser/mieru"
	"github.com/holi/duorelay/internal/parser/naive"
	"github.com/holi/duorelay/internal/parser/openconnect"
	"github.com/holi/duorelay/internal/parser/openvpn"
	"github.com/holi/duorelay/internal/parser/rematch"
	"github.com/holi/duorelay/internal/parser/shadowquic"
	"github.com/holi/duorelay/internal/parser/shadowsocks"
	"github.com/holi/duorelay/internal/parser/shadowtls"
	"github.com/holi/duorelay/internal/parser/snell"
	"github.com/holi/duorelay/internal/parser/socks5"
	"github.com/holi/duorelay/internal/parser/ssh"
	"github.com/holi/duorelay/internal/parser/ssr"
	"github.com/holi/duorelay/internal/parser/sudoku"
	"github.com/holi/duorelay/internal/parser/tailscale"
	"github.com/holi/duorelay/internal/parser/tor"
	"github.com/holi/duorelay/internal/parser/trojan"
	"github.com/holi/duorelay/internal/parser/trusttunnel"
	"github.com/holi/duorelay/internal/parser/tuic"
	"github.com/holi/duorelay/internal/parser/vless"
	"github.com/holi/duorelay/internal/parser/vmess"
	"github.com/holi/duorelay/internal/parser/wireguard"
	"github.com/holi/duorelay/internal/parser/zerotier"
)

func New() *parser.Registry {
	r := parser.NewRegistry()
	r.Register(shadowsocks.New())
	r.Register(vmess.New())
	r.Register(vless.New())
	r.Register(trojan.New())
	r.Register(hysteria2.New())
	r.Register(tuic.New())
	r.Register(socks5.New())
	r.Register(http.New())
	r.Register(ssr.New())
	r.Register(wireguard.New())
	r.Register(snell.New())
	r.Register(hysteria.New())
	r.Register(ssh.New())
	r.Register(shadowtls.New())
	r.Register(openvpn.New())
	r.Register(openconnect.New())
	r.Register(naive.New())
	r.Register(tor.New())
	r.Register(masque.New())
	r.Register(anytls.New())
	r.Register(mieru.New())
	r.Register(sudoku.New())
	r.Register(shadowquic.New())
	r.Register(trusttunnel.New())
	r.Register(tailscale.New())
	r.Register(easytier.New())
	r.Register(zerotier.New())
	r.Register(rematch.New())
	return r
}
