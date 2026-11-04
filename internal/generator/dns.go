package generator

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type DNSAddress struct {
	Type, Host, Path string
	Port             int
}

func ParseDNSAddress(value string) (DNSAddress, error) {
	value = strings.TrimSpace(value)
	if net.ParseIP(value) != nil {
		return DNSAddress{Type: "udp", Host: value, Port: 53}, nil
	}
	if !strings.Contains(value, "://") {
		value = "udp://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return DNSAddress{}, fmt.Errorf("invalid DNS address")
	}
	port := 53
	switch u.Scheme {
	case "udp", "tcp":
	case "tls":
		port = 853
	case "https":
		port = 443
	default:
		return DNSAddress{}, fmt.Errorf("DNS supports UDP, TCP, TLS or HTTPS")
	}
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return DNSAddress{}, fmt.Errorf("invalid DNS port")
		}
	}
	if u.Scheme != "https" && u.Path != "" {
		return DNSAddress{}, fmt.Errorf("only HTTPS DNS accepts a path")
	}
	path := u.EscapedPath()
	if u.Scheme == "https" && path == "" {
		path = "/dns-query"
	}
	return DNSAddress{Type: u.Scheme, Host: u.Hostname(), Port: port, Path: path}, nil
}

func DNSDetour(opts *GenerateOptions) string {
	if opts.DNSDetour != "" {
		return opts.DNSDetour
	}
	if opts.DefaultOutbound != "" && opts.DefaultOutbound != "REJECT" {
		return opts.DefaultOutbound
	}
	return "DIRECT"
}

func DNSBootstrap(opts *GenerateOptions) string {
	if opts.DNSBootstrap != "" {
		return opts.DNSBootstrap
	}
	return "223.5.5.5"
}
