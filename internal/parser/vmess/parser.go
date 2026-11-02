package vmess

import (
	"encoding/json"
	"fmt"
	"net"
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
		if u.User != nil {
			return parseEndpoint(input)
		}
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
		TLS  any    `json:"tls"`
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
	if v.Port == nil || fmt.Sprint(v.Port) == "" {
		n.Server.Port = 443
	}
	if n.Server.Host == "" || n.Server.Port < 1 || n.Server.Port > 65535 {
		return nil, fmt.Errorf("%w: server or port", parser.ErrInvalidField)
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
	if tls := strings.ToLower(fmt.Sprint(v.TLS)); tls == "tls" || tls == "true" || tls == "1" {
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

func parseEndpoint(input string) (any, error) {
	u, err := parser.ParseEndpointURI(input, "vmess", 443)
	if err != nil {
		return nil, err
	}
	credential := u.User.Username()
	cipher, uuid := "auto", credential
	if password, plain := u.User.Password(); plain {
		cipher, uuid = credential, password
	} else if raw, err := parser.DecodeBase64(credential); err == nil {
		if method, id, ok := strings.Cut(string(raw), ":"); ok {
			cipher, uuid = method, id
		}
	}
	if uuid == "" {
		return nil, parser.ErrMissingField
	}
	node := parser.NewNode(ir.ProtocolVMess, "", u)
	parser.SetName(node, u.Fragment)
	node.Proto["uuid"], node.Proto["security"], node.Proto["alter_id"] = uuid, cipher, 0
	query := u.Query()
	if query.Get("type") == "" {
		query.Set("type", query.Get("network"))
	}
	if query.Get("security") == "" && query.Get("tls") == "1" {
		query.Set("security", "tls")
	}
	if err := parser.SetTransport(node, query); err != nil {
		return nil, err
	}
	parser.SetTLS(node, query)
	if net.ParseIP(node.Server.Host) == nil && strings.ContainsAny(node.Server.Host, " \t\r\n") {
		return nil, parser.ErrInvalidField
	}
	return node, nil
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
