package defaults

import (
	"github.com/holi/duorelay/internal/parser"
	"github.com/holi/duorelay/internal/parser/hysteria2"
	"github.com/holi/duorelay/internal/parser/shadowsocks"
	"github.com/holi/duorelay/internal/parser/trojan"
	"github.com/holi/duorelay/internal/parser/tuic"
	"github.com/holi/duorelay/internal/parser/vless"
	"github.com/holi/duorelay/internal/parser/vmess"
)

func New() *parser.Registry {
	r := parser.NewRegistry()
	r.Register(shadowsocks.New())
	r.Register(vmess.New())
	r.Register(vless.New())
	r.Register(trojan.New())
	r.Register(hysteria2.New())
	r.Register(tuic.New())
	return r
}
