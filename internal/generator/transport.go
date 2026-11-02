package generator

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// WebSocketHeaders merges imported headers with the canonical Host field.
func WebSocketHeaders(node *ir.Node) map[string]string {
	headers := map[string]string{}
	switch values := node.Transport.Opts["headers"].(type) {
	case map[string]string:
		for key, value := range values {
			headers[key] = value
		}
	case map[string]any:
		for key, value := range values {
			switch v := value.(type) {
			case []any:
				parts := make([]string, len(v))
				for i, part := range v {
					parts[i] = fmt.Sprint(part)
				}
				headers[key] = strings.Join(parts, ",")
			case []string:
				headers[key] = strings.Join(v, ",")
			case string:
				headers[key] = v
			}
		}
	}
	if node.Transport.Host != "" {
		headers["Host"] = node.Transport.Host
	}
	return headers
}
