package subscription

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"gopkg.in/yaml.v3"
)

func decodeBase64(content []byte) ([]byte, error) {
	content = bytes.Join(bytes.Fields(content), nil)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if data, err := encoding.DecodeString(string(content)); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 subscription")
}

// ParseFormat recognizes native configuration lists, share links and base64.
func ParseFormat(content []byte) (string, error) {
	content = bytes.TrimSpace(bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf}))
	if len(content) == 0 {
		return "", fmt.Errorf("empty subscription")
	}
	var root map[string]any
	if json.Unmarshal(content, &root) == nil {
		if _, ok := root["outbounds"]; ok {
			return "sing-box", nil
		}
	}
	if yaml.Unmarshal(content, &root) == nil {
		if _, ok := root["proxies"]; ok {
			return "clash", nil
		}
	}
	if bytes.Contains(content, []byte("://")) && !bytes.ContainsAny(content, "{}") {
		return "links", nil
	}
	if data, err := decodeBase64(content); err == nil && bytes.Contains(data, []byte("://")) {
		return "base64", nil
	}
	return "", fmt.Errorf("unrecognized subscription format")
}

func (m *Manager) Parse(content []byte) ([]*ir.Node, error) {
	format, err := ParseFormat(content)
	if err != nil {
		return nil, err
	}
	nodes := []*ir.Node{}
	if format == "links" || format == "base64" {
		if format == "base64" {
			content, err = decodeBase64(content)
			if err != nil {
				return nil, err
			}
		}
		if m.Registry == nil {
			return nil, fmt.Errorf("share-link parser registry is not configured")
		}
		for index, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			value, err := m.Registry.ParseAny(line)
			if err != nil {
				return nil, fmt.Errorf("line %d: invalid or unsupported share link", index+1)
			}
			var node *ir.Node
			switch value := value.(type) {
			case *ir.Node:
				node = value
			case ir.Node:
				node = &value
			default:
				return nil, fmt.Errorf("line %d: parser did not return an IR node", index+1)
			}
			if err := validateParsedNode(node); err != nil {
				return nil, fmt.Errorf("line %d: %w", index+1, err)
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	}
	var root struct {
		Proxies   []map[string]any `yaml:"proxies"`
		Outbounds []map[string]any `yaml:"outbounds"`
	}
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("invalid subscription document")
	}
	entries := root.Proxies
	if format == "sing-box" {
		entries = root.Outbounds
	}
	for index, entry := range entries {
		typ := text(entry["type"])
		if format == "sing-box" {
			switch typ {
			case "direct", "block", "dns", "selector", "urltest":
				continue
			}
		}
		node, err := nativeNode(entry, format)
		if err != nil {
			return nil, fmt.Errorf("node %d: %w", index+1, err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func text(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func number(v any) int { value, _ := strconv.Atoi(text(v)); return value }
func object(v any) map[string]any {
	value, _ := v.(map[string]any)
	if value == nil {
		return map[string]any{}
	}
	return value
}
func texts(v any) []string {
	var values []string
	switch list := v.(type) {
	case []any:
		for _, value := range list {
			values = append(values, text(value))
		}
	case []string:
		values = list
	}
	return values
}
func validateParsedNode(n *ir.Node) error {
	if n == nil || !ir.IsValidProtocol(n.Protocol) || strings.TrimSpace(n.Server.Host) == "" || n.Server.Port < 1 || n.Server.Port > 65535 {
		return fmt.Errorf("invalid protocol or server address")
	}
	if n.Name == "" {
		n.Name = n.Server.Host + ":" + strconv.Itoa(n.Server.Port)
	}
	if n.Owner == 0 {
		n.Owner = ir.GetProtocolOwner(n.Protocol)
	}
	return nil
}
func nativeNode(entry map[string]any, format string) (*ir.Node, error) {
	typ := text(entry["type"])
	switch typ {
	case "ss":
		typ = ir.ProtocolShadowsocks
	case "ssr":
		typ = ir.ProtocolShadowsocksR
	case "socks":
		typ = ir.ProtocolSOCKS5
	case "hy2":
		typ = ir.ProtocolHysteria2
	}
	n := &ir.Node{Protocol: typ, Owner: ir.GetProtocolOwner(typ), Name: text(entry["name"]), Server: ir.Server{Host: text(entry["server"]), Port: number(entry["port"])}, Proto: map[string]any{}, Extra: map[string]any{}, Transport: ir.Transport{Type: "tcp", Opts: map[string]any{}}, Security: ir.Security{Type: "none", Opts: map[string]any{}}}
	if format == "sing-box" {
		n.Name = text(entry["tag"])
		n.Server.Port = number(entry["server_port"])
	}
	// Keep the native record for fields that have no canonical IR equivalent.
	n.Extra[format] = entry
	for key, value := range entry {
		n.Proto[key] = value
	}
	for _, key := range []string{"type", "tag", "name", "server", "port", "server_port", "tls", "transport", "network", "ws-opts", "grpc-opts"} {
		delete(n.Proto, key)
	}
	if format == "clash" {
		if typ == ir.ProtocolShadowsocks {
			n.Proto["method"] = entry["cipher"]
		}
		if typ == ir.ProtocolVMess {
			n.Proto["security"] = entry["cipher"]
			n.Proto["alter_id"] = number(entry["alterId"])
		}
		if network := text(entry["network"]); network != "" {
			n.Transport.Type = network
		}
		ws := object(entry["ws-opts"])
		n.Transport.Path = text(ws["path"])
		n.Transport.Host = text(object(ws["headers"])["Host"])
		if n.Transport.Type == "grpc" {
			n.Transport.Path = text(object(entry["grpc-opts"])["grpc-service-name"])
		}
		if entry["tls"] == true || typ == ir.ProtocolTrojan || typ == ir.ProtocolHysteria || typ == ir.ProtocolHysteria2 || typ == ir.ProtocolTUIC || typ == ir.ProtocolAnyTLS {
			n.Security.Type = "tls"
		}
		n.Security.SNI = text(entry["servername"])
		if n.Security.SNI == "" {
			n.Security.SNI = text(entry["sni"])
		}
		n.Security.ALPN = texts(entry["alpn"])
		n.Security.Fingerprint = text(entry["client-fingerprint"])
		if skip, ok := entry["skip-cert-verify"].(bool); ok {
			n.Security.Opts["insecure"] = skip
		}
		if reality, ok := entry["reality-opts"].(map[string]any); ok {
			n.Security.Type = "reality"
			n.Security.Opts["public_key"] = reality["public-key"]
			n.Security.Opts["short_id"] = reality["short-id"]
		}
	} else {
		transport := object(entry["transport"])
		if typ := text(transport["type"]); typ != "" {
			n.Transport.Type = typ
		}
		n.Transport.Path = text(transport["path"])
		if n.Transport.Type == "grpc" {
			n.Transport.Path = text(transport["service_name"])
		}
		n.Transport.Host = text(object(transport["headers"])["Host"])
		for key, value := range transport {
			if key != "type" && key != "path" && key != "headers" && key != "service_name" {
				n.Transport.Opts[key] = value
			}
		}
		tls := object(entry["tls"])
		if tls["enabled"] == true {
			n.Security.Type = "tls"
		}
		n.Security.SNI = text(tls["server_name"])
		n.Security.ALPN = texts(tls["alpn"])
		n.Security.Fingerprint = text(object(tls["utls"])["fingerprint"])
		for key, value := range tls {
			if key != "enabled" && key != "server_name" && key != "alpn" && key != "utls" {
				n.Security.Opts[key] = value
			}
		}
		if reality := object(tls["reality"]); reality["enabled"] == true {
			n.Security.Type = "reality"
			n.Security.Opts["public_key"] = reality["public_key"]
			n.Security.Opts["short_id"] = reality["short_id"]
		}
	}
	if err := validateParsedNode(n); err != nil {
		return nil, err
	}
	return n, nil
}
