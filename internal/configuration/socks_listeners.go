package configuration

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/variant"
)

const maxSOCKSListeners = generator.MaxSOCKSListeners

// SOCKSListener binds one local SOCKS5 port directly to a stored proxy node.
type SOCKSListener struct {
	Port   int    `json:"port"`
	NodeID string `json:"node_id"`
}

func validateSOCKSListeners(settings Settings, listeners []SOCKSListener) error {
	if len(listeners) > maxSOCKSListeners {
		return fmt.Errorf("%w: 最多可配置 %d 个额外 SOCKS5 端口", store.ErrInvalid, maxSOCKSListeners)
	}
	reserved := map[int]string{
		settings.InboundPort: "sing-box 混合端口",
		settings.MixedPort:   "mihomo 混合端口",
	}
	if settings.SOCKSOnly {
		reserved[settings.SOCKSPort] = "独立 SOCKS5 端口"
	}
	mode := settings.TransparentMode
	if mode == "" && settings.EnableTUN {
		mode = "tun"
	}
	if mode == "redirect" {
		reserved[settings.InboundPort] = "透明代理端口"
	}
	if mode == "tproxy" && settings.InboundPort < 65535 {
		reserved[settings.InboundPort+1] = "TPROXY 端口"
	}
	seenPorts := make(map[int]int, len(listeners))
	for i, listener := range listeners {
		if listener.Port < 1 || listener.Port > 65535 {
			return fmt.Errorf("%w: 第 %d 个 SOCKS5 端口必须在 1..65535 之间", store.ErrInvalid, i+1)
		}
		if previous, ok := seenPorts[listener.Port]; ok {
			return fmt.Errorf("%w: SOCKS5 端口 %d 重复（第 %d、%d 项）", store.ErrInvalid, listener.Port, previous+1, i+1)
		}
		seenPorts[listener.Port] = i
		if name, ok := reserved[listener.Port]; ok {
			return fmt.Errorf("%w: SOCKS5 端口 %d 与%s冲突", store.ErrInvalid, listener.Port, name)
		}
		if strings.TrimSpace(listener.NodeID) == "" {
			return fmt.Errorf("%w: 第 %d 个 SOCKS5 端口尚未分配节点", store.ErrInvalid, i+1)
		}
	}
	return nil
}

// resolveSOCKSListeners creates uniquely named, independent outbounds so a
// listener stays pinned to its selected node even when the main proxy is a group.
func (s *Service) resolveSOCKSListeners(ctx context.Context, kernel ir.KernelType, listeners []SOCKSListener) ([]*ir.Node, []generator.SOCKSListener, error) {
	extraNodes := make([]*ir.Node, 0, len(listeners))
	generated := make([]generator.SOCKSListener, 0, len(listeners))
	byID := make(map[string]string, len(listeners))
	for _, listener := range listeners {
		node, err := s.Store.Node(ctx, listener.NodeID)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: SOCKS5 端口 %d 指定的节点不存在", store.ErrInvalid, listener.Port)
		}
		if !variant.SupportsProtocol(node.Protocol) {
			return nil, nil, fmt.Errorf("%w: 当前版本不支持端口 %d 指定的 %s 节点", store.ErrInvalid, listener.Port, node.Protocol)
		}
		if !node.CanRunOn(kernel) {
			return nil, nil, fmt.Errorf("%w: 端口 %d 指定的节点 %q 不支持内核 %s", store.ErrInvalid, listener.Port, node.Name, kernel)
		}
		name, ok := byID[node.ID]
		if !ok {
			name = socksOutboundName(node.ID)
			copy := *node
			copy.Name = name
			extraNodes = append(extraNodes, &copy)
			byID[node.ID] = name
		}
		generated = append(generated, generator.SOCKSListener{Port: listener.Port, Outbound: name})
	}
	return extraNodes, generated, nil
}

func socksOutboundName(nodeID string) string {
	digest := sha256.Sum256([]byte(nodeID))
	return fmt.Sprintf("DuoRelay-SOCKS-%x", digest[:12])
}
