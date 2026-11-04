//go:build !lite

package configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
	"gopkg.in/yaml.v3"
	"io"
	"net"
)

type chainBridge struct {
	from, to       ir.KernelType
	tag, target    string
	port           int
	user, password string
	udp            bool
}

func (s *Service) applyChain(ctx context.Context, request ApplyRequest, settings Settings) (*State, error) {
	if request.Content != "" || request.LoadBalance != nil {
		return nil, fmt.Errorf("%w: 链式代理不能同时传入原始配置或负载均衡设置", store.ErrInvalid)
	}
	nodes, err := s.chainNodes(ctx, request.NodeIDs)
	if err != nil {
		return nil, err
	}
	preferred := request.Kernel
	if preferred != "" && preferred != ir.KernelSingBox && preferred != ir.KernelMihomo {
		return nil, fmt.Errorf("%w: invalid kernel", store.ErrInvalid)
	}
	if preferred == "" {
		preferred = settings.Kernel
	}
	plan, err := s.planChain(nodes, preferred, request.Kernel != "")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	if s.Runtime == nil {
		return nil, fmt.Errorf("kernel runtime is not configured")
	}
	if _, ok := s.Runtime.(modeRuntime); plan.CrossKernel && !ok {
		return nil, fmt.Errorf("%w: 当前运行环境不支持双内核链式代理", store.ErrInvalid)
	}
	// A rule naming an earlier hop would skip the rest of the chain.
	_, rules := effectiveRouting(settings, request.outbound)
	for _, r := range rules {
		if r.Outbound != "PROXY" && r.Outbound != "DIRECT" && r.Outbound != "REJECT" && r.Outbound != nodes[len(nodes)-1].Name {
			return nil, fmt.Errorf("%w: 链式代理的路由只能选择 PROXY、DIRECT、REJECT 或最终出口，请调整指定节点的规则", store.ErrInvalid)
		}
	}
	var reservations []io.Closer
	release := func() {
		for _, c := range reservations {
			_ = c.Close()
		}
		reservations = nil
	}
	defer release()
	reserve := func(udp bool) (int, error) {
		for attempt := 0; attempt < 10; attempt++ {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				return 0, err
			}
			port := listener.Addr().(*net.TCPAddr).Port
			reserved := port == settings.InboundPort || port == settings.MixedPort || port == settings.SOCKSPort
			for _, listener := range settings.SOCKSListeners {
				reserved = reserved || port == listener.Port
			}
			if reserved {
				listener.Close()
				continue
			}
			if udp {
				packet, err := net.ListenPacket("udp4", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					listener.Close()
					continue
				}
				reservations = append(reservations, packet)
			}
			reservations = append(reservations, listener)
			return port, nil
		}
		return 0, fmt.Errorf("无法分配本地链式桥接端口")
	}
	bridges := map[int]chainBridge{}
	for i := 1; i < len(nodes); i++ {
		if plan.Hops[i].Kernel == plan.Hops[i-1].Kernel {
			continue
		}
		port, err := reserve(plan.Hops[i-1].UDP)
		if err != nil {
			return nil, err
		}
		bridges[i] = chainBridge{from: plan.Hops[i-1].Kernel, to: plan.Hops[i].Kernel, tag: "DR-BRIDGE-" + uuid.NewString(), target: nodes[i-1].Name, port: port, user: uuid.NewString(), password: uuid.NewString(), udp: plan.Hops[i-1].UDP}
	}
	var configs []generatedChainConfig
	chainExtraNodes, chainSOCKSListeners, err := s.resolveSOCKSListeners(ctx, plan.Kernel, settings.SOCKSListeners)
	if err != nil {
		return nil, err
	}
	for _, k := range []ir.KernelType{ir.KernelSingBox, ir.KernelMihomo} {
		var members []*ir.Node
		for i, n := range nodes {
			if plan.Hops[i].Kernel == k {
				members = append(members, n)
			}
		}
		if len(members) == 0 {
			continue
		}
		primary := k == plan.Kernel
		outbound, routing := effectiveRouting(settings, request.outbound)
		if settings.RoutingMode != "blacklist" {
			outbound = "PROXY"
		}
		opts := &generator.GenerateOptions{InboundHost: settings.InboundHost, InboundPort: settings.InboundPort, MixedPort: settings.MixedPort, SystemProxy: settings.SystemProxy, SOCKSOnly: settings.SOCKSOnly, SOCKSPort: settings.SOCKSPort, EnableTUN: settings.EnableTUN, TransparentMode: string(settings.TransparentMode), RedirectPort: settings.InboundPort, TProxyPort: settings.InboundPort + 1, TUNStack: settings.TUNStack, DNSServers: settings.DNSServers, DNSFakeIP: settings.DNSFakeIP, LogLevel: settings.LogLevel, DefaultOutbound: outbound, RoutingRules: routing}
		if settings.AllowLAN {
			opts.InboundHost = "0.0.0.0"
		}
		opts.DNSBootstrap, opts.DNSDetour = settings.DNSBootstrap, settings.DNSDetour
		if !primary {
			opts.DefaultOutbound = "DIRECT"
			opts.DNSDetour = "DIRECT"
			opts.RoutingRules = nil
			opts.SOCKSOnly = false
			opts.EnableTUN = false
			opts.TransparentMode = "off"
			opts.SystemProxy = false
		} else {
			// Generate the selector even when blacklist routing defaults to DIRECT.
			opts.DefaultOutbound = "PROXY"
			opts.ExtraNodes = chainExtraNodes
			opts.SOCKSListeners = chainSOCKSListeners
		}
		gen := s.Generators[k]
		if gen == nil {
			return nil, fmt.Errorf("generator is not configured")
		}
		content, err := gen.Generate(members, opts)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", store.ErrInvalid, err)
		}
		root := map[string]any{}
		if k == ir.KernelSingBox {
			err = json.Unmarshal(content, &root)
		} else {
			err = yaml.Unmarshal(content, &root)
		}
		if err != nil {
			return nil, err
		}
		if primary {
			if k == ir.KernelSingBox {
				root["route"].(map[string]any)["final"] = outbound
			} else {
				rules := chainArray(root, "rules")
				rules[len(rules)-1] = "MATCH," + outbound
			}
		}
		key, nameKey, detour := "outbounds", "tag", "detour"
		if k == ir.KernelMihomo {
			key, nameKey, detour = "proxies", "name", "dialer-proxy"
		}
		byName := map[string]map[string]any{}
		for _, entry := range append(chainEntries(root, key), chainEntries(root, "endpoints")...) {
			byName[fmt.Sprint(entry[nameKey])] = entry
		}
		for i, n := range nodes {
			if plan.Hops[i].Kernel != k {
				continue
			}
			entry := byName[n.Name]
			if entry == nil {
				return nil, fmt.Errorf("%w: 第 %d 跳未能生成配置", store.ErrInvalid, i+1)
			}
			// Never retain imported detours: the explicit hop order owns the path.
			delete(entry, detour)
			if k == ir.KernelMihomo && n.Protocol != ir.ProtocolHTTP && n.Protocol != ir.ProtocolSSH {
				entry["udp"] = plan.Hops[i].UDP
			}
			if i > 0 {
				previous := nodes[i-1].Name
				if b, ok := bridges[i]; ok {
					previous = b.tag
					proxy := map[string]any{"type": "socks", "tag": b.tag, "server": "127.0.0.1", "server_port": b.port, "version": "5", "username": b.user, "password": b.password}
					if k == ir.KernelMihomo {
						proxy = map[string]any{"type": "socks5", "name": b.tag, "server": "127.0.0.1", "port": b.port, "username": b.user, "password": b.password, "udp": b.udp}
					}
					chainAppend(root, key, proxy)
				}
				entry[detour] = previous
			}
		}
		if !primary {
			if k == ir.KernelSingBox {
				root["inbounds"] = []any{}
			} else {
				delete(root, "mixed-port")
				root["listeners"] = []any{}
			}
		}
		if primary {
			last := nodes[len(nodes)-1].Name
			if k == ir.KernelSingBox {
				group := byName["PROXY"]
				if group == nil {
					return nil, fmt.Errorf("missing PROXY selector")
				}
				group["outbounds"] = []string{last}
				group["default"] = last
			} else {
				root["proxy-groups"] = []any{map[string]any{"name": "PROXY", "type": "select", "proxies": []string{last}}}
			}
			if !plan.UDP {
				generator.RestrictChainUDP(root, k)
			}
		}
		// Bridge routing precedes ingress-scope and user routing rules. This is
		// essential when the primary kernel is also used by an earlier hop.
		for i := 1; i < len(nodes); i++ {
			if b, ok := bridges[i]; ok && b.from == k {
				chainAddBridge(root, k, b)
			}
		}
		controllerPort, err := reserve(false)
		if err != nil {
			return nil, err
		}
		if k == ir.KernelSingBox {
			root["experimental"] = map[string]any{"clash_api": map[string]any{"external_controller": fmt.Sprintf("127.0.0.1:%d", controllerPort)}}
			content, err = json.MarshalIndent(root, "", "  ")
		} else {
			root["external-controller"] = fmt.Sprintf("127.0.0.1:%d", controllerPort)
			content, err = yaml.Marshal(root)
		}
		if err != nil {
			return nil, err
		}
		configs = append(configs, generatedChainConfig{k, content})
	}
	return s.installChain(ctx, request, plan, configs, settings, release, settings.SOCKSListeners)
}

func chainAddBridge(root map[string]any, k ir.KernelType, b chainBridge) {
	if k == ir.KernelSingBox {
		chainAppend(root, "inbounds", map[string]any{"type": "socks", "tag": b.tag, "listen": "127.0.0.1", "listen_port": b.port, "users": []map[string]any{{"username": b.user, "password": b.password}}})
		route := root["route"].(map[string]any)
		route["rules"] = append([]any{map[string]any{"inbound": []string{b.tag}, "outbound": b.target}}, chainArray(route, "rules")...)
	} else {
		chainAppend(root, "listeners", map[string]any{"name": b.tag, "type": "socks", "listen": "127.0.0.1", "port": b.port, "udp": b.udp, "users": []map[string]any{{"username": b.user, "password": b.password}}})
		root["rules"] = append([]any{"IN-NAME," + b.tag + "," + b.target}, chainArray(root, "rules")...)
	}
}
