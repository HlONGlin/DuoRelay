package hysteria2

import (
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/parser"
	"net/url"
	"strconv"
	"strings"
)

type Parser struct{}

func New() Parser               { return Parser{} }
func (Parser) Protocol() string { return ir.ProtocolHysteria2 }
func (Parser) CanParse(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(lower, "hysteria2://") || strings.HasPrefix(lower, "hy2://")
}
func (p Parser) Parse(input string) (any, error) {
	raw := strings.TrimSpace(input)
	if strings.HasPrefix(strings.ToLower(raw), "hy2://") {
		raw = "hysteria2://" + raw[len("hy2://"):]
	}
	if strings.HasPrefix(strings.ToLower(raw), "hysteria2://") {
		raw = "hysteria2://" + raw[len("hysteria2://"):]
	}
	portHopping := ""
	if strings.HasPrefix(strings.ToLower(raw), "hysteria2://") {
		rest := strings.TrimPrefix(raw, "hysteria2://")
		hostEnd := len(rest)
		if i := strings.IndexAny(rest, "?#"); i >= 0 {
			hostEnd = i
		}
		if colon := strings.LastIndex(rest[:hostEnd], ":"); colon >= 0 && strings.Contains(rest[colon+1:hostEnd], "-") {
			portHopping = rest[colon+1 : hostEnd]
			raw = "hysteria2://" + rest[:colon] + ":443" + rest[hostEnd:]
		}
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "hysteria2") || u.Host == "" {
		return nil, fmt.Errorf("%w: expected hysteria2 URI", parser.ErrInvalidFormat)
	}
	portText := u.Port()
	if u.User == nil || u.User.Username() == "" {
		return nil, fmt.Errorf("%w: password", parser.ErrMissingField)
	}
	port := 443
	if pstr := portText; pstr != "" && !strings.Contains(pstr, "-") {
		var err error
		port, err = strconv.Atoi(pstr)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("%w: port", parser.ErrInvalidField)
		}
	}
	n := parser.NewNode(ir.ProtocolHysteria2, "", u)
	n.Server.Port = port
	parser.SetName(n, u.Fragment)
	q := u.Query()
	n.Proto["password"] = u.User.Username()
	if password, ok := u.User.Password(); ok {
		n.Proto["password"] = u.User.Username() + ":" + password
	}
	n.Proto["udp"] = true
	if ob := q.Get("obfs"); ob != "" {
		n.Proto["obfs"] = ob
		n.Proto["obfs_password"] = q.Get("obfs-password")
	}
	if portHopping != "" {
		n.Proto["port_hopping"] = portHopping
	}
	parser.SetTLS(n, q)
	n.Security.Type = "tls"
	return n, nil
}
