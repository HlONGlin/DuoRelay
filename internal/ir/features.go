package ir

import "fmt"

// ValidateConnectionFeatures rejects options the pinned kernels cannot render.
// Keep diagnostics independent of option values, which may contain credentials.
func ValidateConnectionFeatures(n *Node) error {
	if n.Protocol == ProtocolVLESS && (containsPQV(n.Proto) || containsPQV(n.Security.Opts) || containsPQV(n.Extra)) {
		return fmt.Errorf("Reality pqv is not supported by the pinned kernels")
	}
	if n.Transport.Type != "xhttp" {
		return nil
	}
	if n.Protocol != ProtocolVLESS {
		return fmt.Errorf("XHTTP requires VLESS")
	}
	for key, value := range n.Transport.Opts {
		switch key {
		case "mode":
			mode, ok := value.(string)
			if !ok || (mode != "" && mode != "auto" && mode != "packet-up" && mode != "stream-up" && mode != "stream-one") {
				return fmt.Errorf("unsupported XHTTP mode")
			}
		case "scMaxEachPostBytes", "scMinPostsIntervalMs", "xPaddingBytes", "xPaddingKey":
		default:
			return fmt.Errorf("unsupported XHTTP option")
		}
	}
	return nil
}

func containsPQV(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "pqv" && item != nil && item != "" {
				return true
			}
			if containsPQV(item) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if containsPQV(item) {
				return true
			}
		}
	}
	return false
}
