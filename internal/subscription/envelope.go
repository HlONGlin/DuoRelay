package subscription

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/holi/duorelay/internal/ir"
)

func normalizeContent(content []byte) ([]byte, error) {
	if len(content) >= 2 && (bytes.Equal(content[:2], []byte{0xff, 0xfe}) || bytes.Equal(content[:2], []byte{0xfe, 0xff})) {
		var order binary.ByteOrder = binary.LittleEndian
		if content[0] == 0xfe {
			order = binary.BigEndian
		}
		if len(content)%2 != 0 {
			return nil, fmt.Errorf("invalid UTF-16 subscription")
		}
		words := make([]uint16, (len(content)-2)/2)
		for i := range words {
			words[i] = order.Uint16(content[2+i*2:])
		}
		content = []byte(string(utf16.Decode(words)))
	}
	content = bytes.TrimSpace(content)
	content = bytes.TrimSpace(bytes.TrimPrefix(content, []byte{0xef, 0xbb, 0xbf}))
	if !utf8.Valid(content) {
		return nil, fmt.Errorf("subscription must use UTF-8 or BOM-marked UTF-16")
	}
	content = bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(content, []byte("\r"), []byte("\n")), nil
}

func parseSIP008(content []byte) ([]*ir.Node, error) {
	var root struct {
		Servers []map[string]any `json:"servers"`
	}
	var entries []map[string]any
	if json.Unmarshal(content, &entries) != nil {
		if json.Unmarshal(content, &root) != nil {
			return nil, fmt.Errorf("invalid SIP008 document")
		}
		entries = root.Servers
	}
	return sipNodes(entries)
}

func sipNodes(entries []map[string]any) ([]*ir.Node, error) {
	nodes := make([]*ir.Node, 0, len(entries))
	for i, entry := range entries {
		if firstText(entry, "method") == "" || firstText(entry, "password") == "" {
			return nil, fmt.Errorf("node %d: SIP008 method and password are required", i+1)
		}
		entry["type"] = ir.ProtocolShadowsocks
		entry["tag"] = firstText(entry, "remarks", "name", "server")
		node, err := nativeNode(entry, "sing-box")
		if err != nil {
			return nil, fmt.Errorf("node %d: invalid SIP008 server", i+1)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

func parseSSD(link string) ([]*ir.Node, error) {
	raw, err := decodeBase64([]byte(strings.SplitN(link[len("ssd://"):], "#", 2)[0]))
	if err != nil {
		return nil, err
	}
	var root struct {
		Port       int              `json:"port"`
		Encryption string           `json:"encryption"`
		Password   string           `json:"password"`
		Servers    []map[string]any `json:"servers"`
	}
	if json.Unmarshal(raw, &root) != nil {
		return nil, fmt.Errorf("invalid SSD document")
	}
	for _, entry := range root.Servers {
		entry["server_port"] = entry["port"]
		if entry["server_port"] == nil {
			entry["server_port"] = root.Port
		}
		entry["method"] = entry["encryption"]
		if entry["method"] == nil {
			entry["method"] = root.Encryption
		}
		if entry["password"] == nil {
			entry["password"] = root.Password
		}
		entry["plugin_opts"] = entry["plugin_options"]
	}
	return sipNodes(root.Servers)
}
