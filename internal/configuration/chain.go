package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
	"gopkg.in/yaml.v3"
)

type modeRuntime interface{ SetMode(kernel.RunMode) error }

func (s *Service) chainNodes(ctx context.Context, ids []string) ([]*ir.Node, error) {
	if len(ids) < 2 || len(ids) > 8 {
		return nil, fmt.Errorf("%w: 链式代理需要 2–8 个不同节点", store.ErrInvalid)
	}
	nodes := make([]*ir.Node, 0, len(ids))
	used := map[string]bool{"DIRECT": true, "REJECT": true, "PROXY": true}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, fmt.Errorf("%w: 请勿重复选择同一个节点", store.ErrInvalid)
		}
		seen[id] = true
		n, err := s.Store.Node(ctx, id)
		if err != nil {
			return nil, err
		}
		copy := *n
		if copy.Name == "" {
			copy.Name = "节点"
		}
		for suffix := 2; used[copy.Name]; suffix++ {
			copy.Name = fmt.Sprintf("%s (%d)", n.Name, suffix)
		}
		used[copy.Name] = true
		nodes = append(nodes, &copy)
	}
	return nodes, nil
}

func (s *Service) PreviewChain(ctx context.Context, request ApplyRequest) (*generator.ChainPlan, error) {
	nodes, err := s.chainNodes(ctx, request.NodeIDs)
	if err != nil {
		return nil, err
	}
	settings, err := s.Settings(ctx)
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
		return &generator.ChainPlan{Supported: false, Reason: err.Error(), Hops: []generator.ChainHop{}, Warnings: []string{}}, nil
	}
	return plan, nil
}

// Narrow the protocol ownership by the actual node parameters. For example,
// an OpenVPN static-key profile can use sing-box even when mihomo is preferred.
func (s *Service) planChain(nodes []*ir.Node, preferred ir.KernelType, force bool) (*generator.ChainPlan, error) {
	compatible := make([]*ir.Node, 0, len(nodes))
	for i, n := range nodes {
		copy := *n
		var reasons []string
		for _, k := range []ir.KernelType{ir.KernelSingBox, ir.KernelMihomo} {
			if !n.CanRunOn(k) {
				continue
			}
			bit := ir.KernelSetSingBox
			if k == ir.KernelMihomo {
				bit = ir.KernelSetMihomo
			}
			gen := s.Generators[k]
			var err error
			if gen == nil {
				err = fmt.Errorf("内核生成器不可用")
			} else {
				opts := generator.DefaultGenerateOptions()
				opts.DefaultOutbound = "DIRECT"
				_, err = gen.Generate([]*ir.Node{n}, opts)
			}
			if err != nil {
				copy.Owner &^= bit
				reasons = append(reasons, fmt.Sprintf("%s: %v", k, err))
			}
		}
		if copy.Owner == 0 {
			return nil, fmt.Errorf("第 %d 跳「%s」参数无法生成内核配置：%s", i+1, n.Name, strings.Join(reasons, "；"))
		}
		compatible = append(compatible, &copy)
	}
	return generator.PlanChain(compatible, preferred, force)
}

type generatedChainConfig struct {
	kernel  ir.KernelType
	content []byte
}
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
			if port == settings.InboundPort || port == settings.MixedPort || port == settings.SOCKSPort {
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
		opts := &generator.GenerateOptions{InboundHost: settings.InboundHost, InboundPort: settings.InboundPort, MixedPort: settings.MixedPort, SystemProxy: settings.SystemProxy, SOCKSOnly: settings.SOCKSOnly, SOCKSPort: settings.SOCKSPort, EnableTUN: settings.EnableTUN, TUNStack: settings.TUNStack, DNSServers: settings.DNSServers, DNSFakeIP: settings.DNSFakeIP, LogLevel: settings.LogLevel, DefaultOutbound: outbound, RoutingRules: routing}
		if settings.AllowLAN {
			opts.InboundHost = "0.0.0.0"
		}
		if !primary {
			opts.DefaultOutbound = "DIRECT"
			opts.RoutingRules = nil
			opts.SOCKSOnly = false
			opts.EnableTUN = false
			opts.SystemProxy = false
		} else {
			// Generate the selector even when blacklist routing defaults to DIRECT.
			opts.DefaultOutbound = "PROXY"
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
	return s.installChain(ctx, request, plan, configs, settings.AllowLAN, release)
}

func chainEntries(root map[string]any, key string) []map[string]any {
	var result []map[string]any
	for _, v := range chainArray(root, key) {
		if entry, ok := v.(map[string]any); ok {
			result = append(result, entry)
		}
	}
	return result
}
func chainArray(root map[string]any, key string) []any {
	switch a := root[key].(type) {
	case []any:
		return a
	case []map[string]any:
		result := make([]any, len(a))
		for i, v := range a {
			result[i] = v
		}
		return result
	case []string:
		result := make([]any, len(a))
		for i, v := range a {
			result[i] = v
		}
		return result
	}
	return nil
}
func chainAppend(root map[string]any, key string, entry any) {
	root[key] = append(chainArray(root, key), entry)
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

func (s *Service) startState(state *State) error {
	if runtime, ok := s.Runtime.(modeRuntime); ok {
		mode := kernel.ModeSingleSwitch
		if len(state.Auxiliary) > 0 {
			mode = kernel.ModeCoexist
		}
		if err := runtime.SetMode(mode); err != nil {
			return err
		}
	} else if len(state.Auxiliary) > 0 {
		return fmt.Errorf("运行环境不支持双内核")
	}
	for _, aux := range state.Auxiliary {
		if err := s.start(aux.Kernel, aux.Path); err != nil {
			return err
		}
	}
	return s.start(state.Kernel, state.Path)
}

func (s *Service) installChain(ctx context.Context, request ApplyRequest, plan *generator.ChainPlan, configs []generatedChainConfig, allowLAN bool, release func()) (*State, error) {
	directory := s.Directory
	if directory == "" {
		directory = filepath.Join("data", "configs")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	state := &State{ID: uuid.NewString(), Kernel: plan.Kernel, Mode: "chain", Chain: plan, NodeIDs: append([]string{}, request.NodeIDs...), AppliedAt: time.Now().UTC()}
	var paths []string
	retained := false
	defer func() {
		if !retained {
			for _, p := range paths {
				_ = os.Remove(p)
			}
		}
	}()
	for _, config := range configs {
		content, controller, secret, err := withController(config.content, config.kernel, allowLAN && config.kernel == plan.Kernel)
		if err != nil {
			return nil, err
		}
		ext := ".json"
		if config.kernel == ir.KernelMihomo {
			ext = ".yaml"
		}
		path, err := filepath.Abs(filepath.Join(directory, state.ID+"-"+string(config.kernel)+ext))
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, content, 0600); err != nil {
			return nil, err
		}
		paths = append(paths, path)
		if err := s.Runtime.Validate(string(config.kernel), path); err != nil {
			return nil, fmt.Errorf("%w: %s validation: %v", ErrApply, config.kernel, err)
		}
		if config.kernel == plan.Kernel {
			state.Path, state.Controller, state.Secret = path, controller, secret
		} else {
			state.Auxiliary = append(state.Auxiliary, RuntimeConfig{config.kernel, path, controller, secret})
		}
	}
	previous, err := s.Current(ctx)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.Runtime.StopAll(); err != nil {
		return nil, fmt.Errorf("%w: stop: %v", ErrApply, err)
	}
	rollback := func(cause error) error {
		stopErr := s.Runtime.StopAll()
		var restoreErr error
		if previous != nil {
			restoreErr = s.startState(previous)
		}
		return errors.Join(fmt.Errorf("%w: %v", ErrApply, cause), stopErr, restoreErr)
	}
	release()
	if err := s.startState(state); err != nil {
		return nil, rollback(err)
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Store.SaveState(persistCtx, "current_config", state); err != nil {
		return nil, rollback(err)
	}
	retained = true
	removeReplacedConfig(previous)
	return state, nil
}

func removeReplacedConfig(previous *State) {
	if previous == nil {
		return
	}
	if previous.Path != "" {
		_ = os.Remove(previous.Path)
	}
	for _, config := range previous.Auxiliary {
		_ = os.Remove(config.Path)
	}
}
