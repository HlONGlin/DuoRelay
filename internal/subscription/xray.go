package subscription

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// Import outbound endpoints only; inbound, DNS and routing policies remain local.
func parseXray(content []byte, format string) ([]*ir.Node, error) {
	var root map[string]any
	if json.Unmarshal(content, &root) != nil {
		return nil, fmt.Errorf("invalid Xray/V2Ray JSON")
	}
	var outbounds []any
	if format == "xray-single" {
		outbounds = []any{root}
	} else {
		outbounds, _ = root["outbounds"].([]any)
	}
	nodes := []*ir.Node{}
	for index, value := range outbounds {
		out := object(value)
		typ := strings.ToLower(text(out["protocol"]))
		switch typ {
		case "freedom", "blackhole", "dns", "loopback":
			continue
		}
		settings := object(out["settings"])
		servers, _ := settings["servers"].([]any)
		if typ == "vmess" || typ == "vless" {
			servers, _ = settings["vnext"].([]any)
		}
		if len(servers) == 0 {
			return nil, fmt.Errorf("outbound %d: unsupported protocol or missing servers", index+1)
		}
		for _, serverValue := range servers {
			server := object(serverValue)
			users, _ := server["users"].([]any)
			if typ == "socks" || typ == "http" {
				users, _ = server["users"].([]any)
			}
			if len(users) == 0 {
				users = []any{map[string]any{}}
			}
			for userIndex, userValue := range users {
				user := object(userValue)
				entry := map[string]any{"protocol": typ, "address": firstText(server, "address", "server"), "port": server["port"], "ps": out["tag"]}
				for key, value := range server {
					if key != "users" {
						entry[key] = value
					}
				}
				for key, value := range user {
					entry[key] = value
				}
				entry["username"] = firstText(user, "user", "username")
				entry["password"] = firstNonBlank(firstText(user, "pass", "password"), firstText(server, "password"))
				if len(users) > 1 {
					entry["ps"] = fmt.Sprintf("%s (%d)", text(out["tag"]), userIndex+1)
				}
				node, err := v2rayNNode(entry)
				if err != nil {
					return nil, fmt.Errorf("outbound %d: invalid or unsupported endpoint", index+1)
				}
				if err := applyXrayStream(node, object(out["streamSettings"])); err != nil {
					return nil, fmt.Errorf("outbound %d: %w", index+1, err)
				}
				node.Extra = map[string]any{"xray": out}
				nodes = append(nodes, node)
			}
		}
	}
	return nodes, nil
}

func applyXrayStream(node *ir.Node, stream map[string]any) error {
	network := strings.ToLower(text(stream["network"]))
	switch network {
	case "", "tcp", "raw":
		node.Transport.Type = "tcp"
	case "ws":
		node.Transport.Type = network
		opts := object(stream[network+"Settings"])
		node.Transport.Path = text(opts["path"])
		node.Transport.Host = text(object(opts["headers"])["Host"])
		if node.Transport.Host == "" {
			node.Transport.Host = text(opts["host"])
		}
		node.Transport.Opts["headers"] = object(opts["headers"])
	case "grpc":
		node.Transport.Type = "grpc"
		node.Transport.Path = text(object(stream["grpcSettings"])["serviceName"])
		node.Transport.Opts["service_name"] = node.Transport.Path
	case "xhttp":
		node.Transport.Type = "xhttp"
		opts := object(stream["xhttpSettings"])
		node.Transport.Path, node.Transport.Host = text(opts["path"]), text(opts["host"])
		for key, value := range opts {
			if key != "path" && key != "host" {
				node.Transport.Opts[key] = value
			}
		}
		node.Owner = ir.KernelSetMihomo
	default:
		return fmt.Errorf("unsupported Xray transport")
	}
	security := strings.ToLower(text(stream["security"]))
	switch security {
	case "", "none":
		if node.Protocol == ir.ProtocolTrojan {
			node.Security.Type = "tls"
		}
	case "tls", "reality":
		node.Security.Type = security
		opts := object(stream[security+"Settings"])
		node.Security.SNI = text(opts["serverName"])
		node.Security.ALPN = texts(opts["alpn"])
		node.Security.Fingerprint = text(opts["fingerprint"])
		if skip, ok := opts["allowInsecure"].(bool); ok {
			node.Security.Opts["insecure"] = skip
		}
		if security == "reality" {
			node.Security.Opts["public_key"] = firstText(opts, "publicKey", "password")
			node.Security.Opts["short_id"] = text(opts["shortId"])
			if value, ok := opts["pqv"]; ok {
				node.Security.Opts["pqv"] = value
			}
		}
	default:
		return fmt.Errorf("unsupported Xray security")
	}
	return ir.ValidateConnectionFeatures(node)
}
