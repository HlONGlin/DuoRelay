package api

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

func validateEditedNode(n *ir.Node) error {
	if strings.TrimSpace(n.Name) == "" || len(n.Name) > 256 || strings.ContainsAny(n.Name, "\r\n\x00") {
		return fmt.Errorf("%w: invalid node name", store.ErrInvalid)
	}
	if strings.ContainsAny(n.Server.Host, " /\r\n\t\x00") {
		return fmt.Errorf("%w: invalid server hostname", store.ErrInvalid)
	}
	if n.Transport.Type == "xhttp" && n.Protocol != ir.ProtocolVLESS {
		return fmt.Errorf("%w: XHTTP requires VLESS", store.ErrInvalid)
	}
	if n.Security.Type != "" && n.Security.Type != "none" && n.Security.Type != "tls" && n.Security.Type != "reality" {
		return fmt.Errorf("%w: invalid security type", store.ErrInvalid)
	}
	if n.Security.Type == "reality" && n.Protocol != ir.ProtocolVLESS {
		return fmt.Errorf("%w: Reality is supported for VLESS", store.ErrInvalid)
	}
	var required []string
	switch n.Protocol {
	case ir.ProtocolVLESS, ir.ProtocolVMess:
		required = []string{"uuid"}
	case ir.ProtocolShadowsocks:
		required = []string{"method", "password"}
	case ir.ProtocolTrojan, ir.ProtocolAnyTLS:
		required = []string{"password"}
	}
	for _, key := range required {
		value, ok := n.Proto[key].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s is required", store.ErrInvalid, key)
		}
	}
	if err := ir.ValidateConnectionFeatures(n); err != nil {
		return fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	return nil
}
