package subscription

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

func isNativeMieru(link string) bool {
	lower := strings.ToLower(link)
	return strings.HasPrefix(lower, "mierus://") || strings.HasPrefix(lower, "mieru://") && !strings.Contains(strings.SplitN(link, "?", 2)[0], "@")
}

func parseNativeMieru(link string) ([]*ir.Node, error) {
	bad := fmt.Errorf("invalid native Mieru link or unsupported profile options")
	if strings.HasPrefix(strings.ToLower(link), "mierus://") {
		u, err := url.Parse(link)
		if err != nil || u.User == nil || u.Hostname() == "" {
			return nil, bad
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil || len(q["profile"]) != 1 || q.Get("profile") == "" || u.Port() != "" || u.Path != "" {
			return nil, bad
		}
		ports, protocols := q["port"], q["protocol"]
		if len(ports) == 0 || len(ports) != len(protocols) {
			return nil, bad
		}
		for key, values := range q {
			if key != "port" && key != "protocol" && len(values) != 1 {
				return nil, bad
			}
			switch key {
			case "profile", "port", "protocol", "multiplexing", "handshake-mode", "mtu", "traffic-pattern":
			default:
				return nil, bad
			}
		}
		pass, _ := u.User.Password()
		if mtu := q.Get("mtu"); mtu != "" && mtu != "1400" {
			return nil, bad
		}
		nodes := []*ir.Node{}
		for i, port := range ports {
			node, err := mieruEndpoint(q.Get("profile"), u.Hostname(), port, protocols[i], u.User.Username(), pass)
			if err != nil {
				return nil, bad
			}
			if multiplex := q.Get("multiplexing"); multiplex != "" {
				if err := setMieruMultiplex(node, multiplex); err != nil {
					return nil, bad
				}
			}
			if handshake := q.Get("handshake-mode"); handshake != "" {
				switch handshake {
				case "HANDSHAKE_DEFAULT", "HANDSHAKE_STANDARD", "HANDSHAKE_NO_WAIT":
					node.Proto["handshake_mode"] = handshake
				default:
					return nil, bad
				}
			}
			nodes = append(nodes, node)
			if pattern := q.Get("traffic-pattern"); pattern != "" {
				raw, err := decodeBase64([]byte(pattern))
				if err != nil {
					return nil, bad
				}
				if _, err := readWire(raw); err != nil {
					return nil, bad
				}
				node.Proto["traffic_pattern"] = pattern
			}
		}
		if fragment := nativeMieruFragment(link); fragment != "" {
			for _, node := range nodes {
				node.Name = fragment
			}
		} else {
			disambiguateMieruNames(nodes)
		}
		return nodes, nil
	}
	payload := strings.SplitN(link[len("mieru://"):], "#", 2)[0]
	raw, err := decodeBase64([]byte(payload))
	if err != nil {
		return nil, bad
	}
	config, err := readWire(raw)
	if err != nil {
		return nil, bad
	}
	nodes := []*ir.Node{}
	for _, profileField := range config[1] {
		profile, err := readWire(profileField.data)
		if err != nil {
			return nil, bad
		}
		user, err := readWire(wireData(profile, 2))
		if err != nil || len(wireData(user, 3)) != 0 {
			return nil, bad
		}
		// Local listener options are intentionally not imported. Non-default
		// profile features without an IR mapping must not silently disappear.
		if mtu := wireNumber(profile, 4); mtu != 0 && mtu != 1400 {
			return nil, bad
		}
		if len(profile[8]) != 0 {
			return nil, bad
		}
		for _, serverField := range profile[3] {
			server, err := readWire(serverField.data)
			if err != nil {
				return nil, bad
			}
			host := firstNonBlank(string(wireData(server, 2)), string(wireData(server, 1)))
			for _, bindingField := range server[3] {
				binding, err := readWire(bindingField.data)
				if err != nil {
					return nil, bad
				}
				protocol := ""
				switch wireNumber(binding, 2) {
				case 1:
					protocol = "UDP"
				case 2:
					protocol = "TCP"
				}
				port := string(wireData(binding, 3))
				if port == "" {
					port = strconv.FormatUint(wireNumber(binding, 1), 10)
				}
				node, err := mieruEndpoint(string(wireData(profile, 1)), host, port, protocol, string(wireData(user, 1)), string(wireData(user, 2)))
				if err != nil {
					return nil, bad
				}
				if len(profile[5]) != 0 {
					mux, err := readWire(wireData(profile, 5))
					if err != nil {
						return nil, bad
					}
					if err := setMieruMultiplex(node, strconv.FormatUint(wireNumber(mux, 1), 10)); err != nil {
						return nil, bad
					}
				}
				switch wireNumber(profile, 6) {
				case 0:
				case 1:
					node.Proto["handshake_mode"] = "HANDSHAKE_STANDARD"
				case 2:
					node.Proto["handshake_mode"] = "HANDSHAKE_NO_WAIT"
				default:
					return nil, bad
				}
				if pattern := wireData(profile, 7); len(pattern) != 0 {
					if _, err := readWire(pattern); err != nil {
						return nil, bad
					}
					node.Proto["traffic_pattern"] = base64.StdEncoding.EncodeToString(pattern)
				}
				nodes = append(nodes, node)
			}
		}
	}
	if len(nodes) == 0 {
		return nil, bad
	}
	// Native Mieru links commonly carry the display name in the URI fragment.
	// The protobuf payload has its own profile name, but it is not always the
	// name chosen by the subscription provider.
	if fragment := nativeMieruFragment(link); fragment != "" {
		for _, node := range nodes {
			node.Name = fragment
		}
	}
	disambiguateMieruNames(nodes)
	return nodes, nil
}

func nativeMieruFragment(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Fragment)
}

func mieruEndpoint(name, host, port, protocol, user, password string) (*ir.Node, error) {
	if user == "" || password == "" || host == "" {
		return nil, fmt.Errorf("missing Mieru credentials or server")
	}
	protocol = strings.ToUpper(protocol)
	if protocol != "TCP" && protocol != "UDP" {
		return nil, fmt.Errorf("invalid Mieru transport")
	}
	low, high, err := parsePortRange(port)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = "Mieru"
	}
	node := &ir.Node{Name: name, Protocol: ir.ProtocolMieru, Owner: ir.KernelSetMihomo, Server: ir.Server{Host: host, Port: low}, Transport: ir.Transport{Type: strings.ToLower(protocol)}, Security: ir.Security{Type: "none"}, Proto: map[string]any{"username": user, "password": password, "transport": protocol}, Extra: map[string]any{}, Source: ir.Source{Type: "import"}}
	if low != high {
		node.Proto["port_range"] = port
	}
	return node, validateParsedNode(node)
}

func disambiguateMieruNames(nodes []*ir.Node) {
	if len(nodes) < 2 {
		return
	}
	counts := map[string]int{}
	for _, node := range nodes {
		counts[node.Name]++
	}
	for _, node := range nodes {
		if counts[node.Name] < 2 {
			continue
		}
		node.Name += " · " + net.JoinHostPort(node.Server.Host, strconv.Itoa(node.Server.Port)) + " " + strings.ToUpper(node.Transport.Type)
	}
}

func parsePortRange(value string) (int, int, error) {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid port range")
	}
	low, err := strconv.Atoi(parts[0])
	if err != nil || low < 1 || low > 65535 {
		return 0, 0, fmt.Errorf("invalid port")
	}
	high := low
	if len(parts) == 2 {
		high, err = strconv.Atoi(parts[1])
		if err != nil || high < low || high > 65535 {
			return 0, 0, fmt.Errorf("invalid port range")
		}
	}
	return low, high, nil
}

func setMieruMultiplex(node *ir.Node, value string) error {
	levels := map[string]string{"0": "DEFAULT", "1": "OFF", "2": "LOW", "3": "MIDDLE", "4": "HIGH"}
	value = strings.TrimPrefix(strings.ToUpper(value), "MULTIPLEXING_")
	if level, ok := levels[value]; ok {
		value = level
	}
	switch value {
	case "DEFAULT", "OFF", "LOW", "MIDDLE", "HIGH":
		node.Proto["multiplexing"] = "MULTIPLEXING_" + value
		return nil
	}
	return fmt.Errorf("invalid Mieru multiplexing")
}

type wireValue struct {
	data   []byte
	number uint64
}
type wireMessage map[uint64][]wireValue

// Decode only the protobuf wire envelope; field meanings follow upstream
// clientcfg.proto/base.proto. Unknown fields can be skipped without recursion.
func readWire(data []byte) (wireMessage, error) {
	message := wireMessage{}
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 || tag>>3 == 0 {
			return nil, fmt.Errorf("invalid protobuf tag")
		}
		data = data[n:]
		var value wireValue
		switch tag & 7 {
		case 0:
			v, n := binary.Uvarint(data)
			if n <= 0 {
				return nil, fmt.Errorf("invalid protobuf varint")
			}
			value.number = v
			data = data[n:]
		case 1:
			if len(data) < 8 {
				return nil, fmt.Errorf("truncated protobuf")
			}
			data = data[8:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 || size > uint64(len(data)-n) {
				return nil, fmt.Errorf("invalid protobuf length")
			}
			value.data = data[n : n+int(size)]
			data = data[n+int(size):]
		case 5:
			if len(data) < 4 {
				return nil, fmt.Errorf("truncated protobuf")
			}
			data = data[4:]
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type")
		}
		message[tag>>3] = append(message[tag>>3], value)
	}
	return message, nil
}
func wireData(message wireMessage, field uint64) []byte {
	values := message[field]
	if len(values) == 0 {
		return nil
	}
	return values[len(values)-1].data
}
func wireNumber(message wireMessage, field uint64) uint64 {
	values := message[field]
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1].number
}
