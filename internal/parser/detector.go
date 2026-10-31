package parser

import (
	"encoding/json"
	"strings"
)

// DetectProtocol returns the protocol scheme for a share URI, or the protocol
// field from a JSON object. It intentionally does not guess from arbitrary YAML.
func DetectProtocol(input string) string {
	s := strings.TrimSpace(input)
	if i := strings.Index(s, "://"); i > 0 {
		return strings.ToLower(s[:i])
	}
	var object struct {
		Protocol string `json:"protocol"`
		Type     string `json:"type"`
	}
	if json.Unmarshal([]byte(s), &object) == nil {
		if object.Protocol != "" {
			return strings.ToLower(object.Protocol)
		}
		return strings.ToLower(object.Type)
	}
	return ""
}
