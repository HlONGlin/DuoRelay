package subscription

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// Structured document positions use the one-based entry index as line.
func (m *Manager) parseJSONDetailed(content []byte, format string) ([]*ir.Node, []ParseSkip, error) {
	var raw any
	if json.Unmarshal(content, &raw) != nil {
		return nil, nil, fmt.Errorf("invalid subscription JSON")
	}
	var entries []any
	switch root := raw.(type) {
	case []any:
		entries = root
	case map[string]any:
		key := map[string]string{"xray": "outbounds", "v2rayn": "configs", "sip008": "servers"}[format]
		if list, ok := root[key].([]any); ok {
			entries = list
		} else {
			entries = []any{root}
		}
	}
	var nodes []*ir.Node
	skipped := []ParseSkip{}
	for i, entry := range entries {
		data, _ := json.Marshal(entry)
		var items []*ir.Node
		var err error
		switch format {
		case "xray", "xray-single":
			items, err = parseXray(data, "xray-single")
		case "sip008":
			data, _ = json.Marshal(map[string]any{"servers": []any{entry}})
			items, err = parseSIP008(data)
		case "v2rayn":
			items, err = m.parseV2RayNJSON(data)
		}
		if err != nil {
			reason := "invalid or unsupported " + format + " entry"
			if strings.Contains(err.Error(), "pqv") {
				reason = "Reality pqv is not supported by the pinned kernels"
			} else if strings.Contains(err.Error(), "XHTTP") {
				reason = "unsupported XHTTP transport option"
			}
			skipped = append(skipped, ParseSkip{Line: i + 1, Reason: reason})
			continue
		}
		nodes = append(nodes, items...)
	}
	if len(nodes) == 0 {
		return nil, skipped, fmt.Errorf("subscription contains no usable nodes")
	}
	return nodes, skipped, nil
}

func (m *Manager) parseINIDetailed(content []byte) ([]*ir.Node, []ParseSkip, error) {
	var nodes []*ir.Node
	skipped := []ParseSkip{}
	section := ""
	for i, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			continue
		}
		if section != "" && section != "[proxy]" && section != "[server_local]" {
			continue
		}
		items, err := m.parseProxyINI([]byte(line))
		if err != nil {
			skipped = append(skipped, ParseSkip{Line: i + 1, Reason: "invalid or unsupported proxy definition"})
			continue
		}
		nodes = append(nodes, items...)
	}
	if len(nodes) == 0 {
		return nil, skipped, fmt.Errorf("subscription contains no usable nodes")
	}
	return nodes, skipped, nil
}
