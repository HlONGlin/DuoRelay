package generator

import (
	"fmt"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// ChainPlan is safe to return to the UI: no addresses, credentials or bridge ports.
type ChainPlan struct {
	Supported   bool          `json:"supported"`
	Reason      string        `json:"reason,omitempty"`
	Kernel      ir.KernelType `json:"kernel,omitempty"`
	CrossKernel bool          `json:"cross_kernel"`
	UDP         bool          `json:"udp"`
	Hops        []ChainHop    `json:"hops"`
	Warnings    []string      `json:"warnings"`
}

type ChainHop struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Protocol string        `json:"protocol"`
	Kernel   ir.KernelType `json:"kernel"`
	Network  string        `json:"network"`
	UDP      bool          `json:"udp"`
}

type chainCapability struct {
	network   string
	udp       bool
	nativeUDP bool // forwarding a datagram also needs UDP on the preceding hop
	wrapper   bool
	warning   string
}

func chainFlag(node *ir.Node, key string, fallback bool) bool {
	if v, ok := node.Proto[key].(bool); ok {
		return v
	}
	return fallback
}

func chainText(node *ir.Node, key string) string {
	if v, ok := node.Proto[key].(string); ok {
		return strings.ToLower(v)
	}
	return ""
}

// Capabilities describe the wire transport to the server, separately from the
// protocols it can forward. TCP proxies with packet-over-stream can restore UDP
// capability after a TCP-only hop; native SOCKS/SS datagrams cannot.
func chainCaps(n *ir.Node, kernel ir.KernelType) (chainCapability, error) {
	c := chainCapability{network: "tcp"}
	if n.Protocol == ir.ProtocolRematch {
		return c, fmt.Errorf("Rematch 是重匹配路由适配器，没有独立代理连接，不能作为代理跳")
	}
	if !n.CanRunOn(kernel) {
		return c, fmt.Errorf("不支持 %s 内核", kernel)
	}
	if !ir.IsValidProtocol(n.Protocol) {
		return c, fmt.Errorf("未知协议 %s", n.Protocol)
	}
	if n.Protocol == ir.ProtocolMASQUE && kernel != ir.KernelMihomo {
		return c, fmt.Errorf("MASQUE 需要 mihomo 内核")
	}
	if n.Protocol == ir.ProtocolSnell && n.Proto["version"] != nil {
		version := fmt.Sprint(n.Proto["version"])
		if kernel == ir.KernelSingBox && version != "3" && version != "4" && version != "6" {
			return c, fmt.Errorf("此 Snell 版本需要 mihomo 内核")
		}
		if kernel == ir.KernelMihomo && version != "1" && version != "2" && version != "3" && version != "4" && version != "5" {
			return c, fmt.Errorf("此 Snell 版本需要 sing-box 内核")
		}
	}
	switch n.Protocol {
	case ir.ProtocolShadowTLS:
		c.wrapper = true
	case ir.ProtocolHTTP, ir.ProtocolSSH, ir.ProtocolTor, ir.ProtocolNaive:
	case ir.ProtocolSOCKS5:
		c.udp, c.nativeUDP = chainFlag(n, "udp", true), true
	case ir.ProtocolShadowsocks, ir.ProtocolShadowsocksR:
		c.udp, c.nativeUDP = chainFlag(n, "udp", true), true
	case ir.ProtocolVMess, ir.ProtocolVLESS, ir.ProtocolTrojan, ir.ProtocolAnyTLS, ir.ProtocolSnell, ir.ProtocolMieru, ir.ProtocolTrustTunnel, ir.ProtocolMASQUE:
		c.udp = chainFlag(n, "udp", true)
	case ir.ProtocolSudoku, ir.ProtocolHysteria, ir.ProtocolHysteria2, ir.ProtocolTUIC, ir.ProtocolShadowQUIC:
		c.udp = true
	case ir.ProtocolWireGuard, ir.ProtocolOpenVPN, ir.ProtocolOpenConnect, ir.ProtocolTailscale, ir.ProtocolEasyTier, ir.ProtocolZeroTier:
		c.udp = chainFlag(n, "udp", true)
	}
	switch n.Protocol {
	case ir.ProtocolHysteria, ir.ProtocolHysteria2, ir.ProtocolTUIC, ir.ProtocolShadowQUIC, ir.ProtocolWireGuard:
		c.network = "udp"
	case ir.ProtocolNaive, ir.ProtocolTrustTunnel:
		if strings.EqualFold(n.Transport.Type, "quic") || chainFlag(n, "quic", false) {
			c.network = "udp"
		}
	case ir.ProtocolMieru:
		if strings.EqualFold(n.Transport.Type, "udp") || chainText(n, "transport") == "udp" {
			c.network = "udp"
		}
	case ir.ProtocolMASQUE:
		if chainText(n, "network") != "h2" {
			c.network = "udp"
		}
	case ir.ProtocolOpenVPN:
		c.network = "udp"
		if strings.EqualFold(n.Transport.Type, "tcp") || chainText(n, "network") == "tcp" {
			c.network = "tcp"
		}
		// Profiles may contain TCP and UDP failover remotes.
		if remotes, ok := n.Proto["remotes"].([]any); ok {
			for _, r := range remotes {
				if remote, ok := r.(map[string]any); ok && remote["network"] != c.network {
					c.network = "tcp+udp"
				}
			}
		}
	case ir.ProtocolOpenConnect, ir.ProtocolTailscale, ir.ProtocolEasyTier, ir.ProtocolZeroTier:
		c.network = "tcp+udp"
	}
	if strings.EqualFold(n.Transport.Type, "quic") && (n.Protocol == ir.ProtocolVMess || n.Protocol == ir.ProtocolVLESS || n.Protocol == ir.ProtocolTrojan) {
		c.network = "udp"
	}
	if n.Protocol == ir.ProtocolTailscale || n.Protocol == ir.ProtocolEasyTier || n.Protocol == ir.ProtocolZeroTier || n.Protocol == ir.ProtocolWireGuard {
		c.warning = "组网/VPN 节点只能访问服务端允许的网段；访问公网需配置出口节点或默认路由"
	}
	return c, nil
}

// PlanChain searches every possible assignment (at most 2^8) so a viable
// single-kernel chain is preferred, while exclusive protocols may use bridges.
// forceKernel means the caller explicitly selected one kernel.
func PlanChain(nodes []*ir.Node, preferred ir.KernelType, forceKernel bool) (*ChainPlan, error) {
	if len(nodes) < 2 || len(nodes) > 8 {
		return nil, fmt.Errorf("链式代理需要 2–8 个不同节点")
	}
	ids := map[string]bool{}
	for _, n := range nodes {
		if n == nil {
			return nil, fmt.Errorf("节点不存在")
		}
		if n.ID != "" && ids[n.ID] {
			return nil, fmt.Errorf("请勿重复选择同一个节点")
		}
		ids[n.ID] = true
	}
	if preferred == "" {
		preferred = ir.KernelSingBox
	}
	var best *ChainPlan
	bestScore := 10000
	firstError := "所选节点没有可用的内核组合"
	errorDepth := -1
	for mask := 0; mask < 1<<len(nodes); mask++ {
		plan := &ChainPlan{Supported: true, Hops: []ChainHop{}, Warnings: []string{}}
		udp, score, valid := true, 0, true
		for i, node := range nodes {
			kernel := ir.KernelSingBox
			if mask&(1<<i) != 0 {
				kernel = ir.KernelMihomo
			}
			if forceKernel && kernel != preferred {
				valid = false
				break
			}
			cap, err := chainCaps(node, kernel)
			if err != nil {
				if i > errorDepth {
					firstError = fmt.Sprintf("第 %d 跳「%s」：%v", i+1, node.Name, err)
					errorDepth = i
				}
				valid = false
				break
			}
			if cap.wrapper && (i == len(nodes)-1 || nodes[i+1].Protocol != ir.ProtocolShadowsocks) {
				firstError = fmt.Sprintf("第 %d 跳「%s」：ShadowTLS 是传输包装，后面必须紧接 Shadowsocks，不能作为最终出口", i+1, node.Name)
				errorDepth = len(nodes)
				valid = false
				break
			}
			if i > 0 {
				if nodes[i-1].Protocol == ir.ProtocolShadowTLS && kernel != plan.Hops[i-1].Kernel {
					firstError = "ShadowTLS 与其后的 Shadowsocks 必须使用同一内核"
					errorDepth = len(nodes)
					valid = false
					break
				}
				if (cap.network == "udp" || cap.network == "tcp+udp") && !udp {
					firstError = fmt.Sprintf("第 %d 跳「%s」需要 UDP 传输，但前一跳「%s」无法转发 UDP；请更换入口或调整顺序", i+1, node.Name, nodes[i-1].Name)
					errorDepth = len(nodes)
					valid = false
					break
				}
				if kernel != plan.Hops[i-1].Kernel {
					score += 100
					plan.CrossKernel = true
				}
			}
			if kernel != preferred {
				score++
			}
			udp = cap.udp && (!cap.nativeUDP || udp)
			plan.Hops = append(plan.Hops, ChainHop{ID: node.ID, Name: node.Name, Protocol: node.Protocol, Kernel: kernel, Network: cap.network, UDP: udp})
			if cap.warning != "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("第 %d 跳：%s", i+1, cap.warning))
			}
		}
		if valid && score < bestScore {
			plan.Kernel, plan.UDP = plan.Hops[len(nodes)-1].Kernel, udp
			if !udp {
				plan.Warnings = append(plan.Warnings, "当前链路仅支持 TCP，走代理链的 UDP 请求会被拒绝；直连规则保持生效")
			}
			best, bestScore = plan, score
		}
	}
	if best == nil {
		return nil, fmt.Errorf("链式代理不可用：%s", firstError)
	}
	return best, nil
}
