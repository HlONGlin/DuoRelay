package vmess

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolVMess }
func (Parser) CanParse(input string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(input)), "vmess://")
}

func (p Parser) Parse(input string) (any, error) {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || !strings.EqualFold(u.Scheme, "vmess") {
		return nil, fmt.Errorf("%w: expected vmess URI", parser.ErrInvalidFormat)
	}
	payload := u.Opaque
	if payload == "" {
		payload = strings.TrimPrefix(u.Path, "//")
	}
	if payload == "" {
		payload = u.Host
	}
	raw, err := parser.DecodeBase64(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: vmess payload", parser.ErrInvalidFormat)
	}
	var v struct {
		Add  string `json:"add"`
		Port any    `json:"port"`
		ID   string `json:"id"`
		Aid  any    `json:"aid"`
		Scy  string `json:"scy"`
		Net  string `json:"net"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
		ALPN string `json:"alpn"`
		PS   string `json:"ps"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return nil, fmt.Errorf("%w: vmess JSON", parser.ErrInvalidFormat)
	}
	if v.Add == "" {
		v.Add = u.Hostname()
	}
	if v.ID == "" {
		return nil, fmt.Errorf("%w: uuid", parser.ErrMissingField)
	}
	base, _ := url.Parse("vmess://" + v.Add + ":" + fmt.Sprint(number(v.Port)))
	n := parser.NewNode(ir.ProtocolVMess, v.PS, base)
	n.Server.Host = v.Add
	n.Server.Port = number(v.Port)
	if n.Server.Port == 0 {
		n.Server.Port = 443
	}
	if v.PS == "" {
		parser.SetName(n, u.Fragment)
	} else {
		n.Name = v.PS
	}
	n.Proto["uuid"], n.Proto["alter_id"], n.Proto["security"] = v.ID, number(v.Aid), first(v.Scy, "auto")
	q := url.Values{}
	if v.Net != "" {
		q.Set("type", v.Net)
	}
	if v.Host != "" {
		q.Set("host", v.Host)
	}
	if v.Path != "" {
		q.Set("path", v.Path)
	}
	if v.TLS != "" {
		q.Set("security", "tls")
	}
	if v.SNI != "" {
		q.Set("sni", v.SNI)
	}
	if v.ALPN != "" {
		q.Set("alpn", v.ALPN)
	}
	if err := parser.SetTransport(n, q); err != nil {
		return nil, err
	}
	parser.SetTLS(n, q)
	return n, nil
}
func number(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		var n int
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}
func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
