package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/clashapi"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/generator/mihomo"
	"github.com/holi/duorelay/internal/generator/singbox"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/systemproxy"
	"github.com/holi/duorelay/internal/transparent"
	"gopkg.in/yaml.v3"
)

var ErrApply = errors.New("configuration apply failed")

const CurrentConfigVersion = 2

type Runtime interface {
	Validate(string, string) error
	StartSingBox(string) error
	StartMihomo(string) error
	StopAll() error
}
type Settings struct {
	ConfigVersion        int                          `json:"config_version"`
	Kernel               ir.KernelType                `json:"kernel"`
	InboundHost          string                       `json:"inbound_host"`
	InboundPort          int                          `json:"inbound_port"`
	MixedPort            int                          `json:"mixed_port"`
	SOCKSOnly            bool                         `json:"socks_only"`
	SOCKSPort            int                          `json:"socks_port"`
	LoadBalance          generator.LoadBalanceOptions `json:"load_balance"`
	AllowLAN             bool                         `json:"allow_lan"`
	LogLevel             string                       `json:"log_level"`
	DNSServers           []string                     `json:"dns_servers"`
	DNSFakeIP            bool                         `json:"dns_fake_ip"`
	EnableTUN            bool                         `json:"enable_tun"`
	TransparentMode      transparent.Mode             `json:"transparent_mode"`
	TransparentModes     []transparent.ModeInfo       `json:"transparent_modes,omitempty"`
	TransparentActive    transparent.Mode             `json:"transparent_active,omitempty"`
	TUNStack             string                       `json:"tun_stack"`
	SystemProxy          bool                         `json:"system_proxy"`
	SystemProxySupported bool                         `json:"system_proxy_supported"`
	RoutingMode          string                       `json:"routing_mode"`
	DefaultOutbound      string                       `json:"default_outbound"`
	RoutingRules         []generator.RoutingRule      `json:"routing_rules"`
}
type ApplyRequest struct {
	Kernel      ir.KernelType                 `json:"kernel,omitempty"`
	Content     string                        `json:"config,omitempty"`
	NodeIDs     []string                      `json:"node_ids,omitempty"`
	Mode        string                        `json:"mode,omitempty"`
	LoadBalance *generator.LoadBalanceOptions `json:"load_balance,omitempty"`
	outbound    string
}
type State struct {
	ConfigVersion     int                           `json:"config_version"`
	ID                string                        `json:"id"`
	Kernel            ir.KernelType                 `json:"kernel"`
	NodeIDs           []string                      `json:"node_ids"`
	Mode              string                        `json:"mode"`
	TransparentMode   transparent.Mode              `json:"transparent_mode,omitempty"`
	LoadBalance       *generator.LoadBalanceOptions `json:"load_balance,omitempty"`
	AppliedAt         time.Time                     `json:"applied_at"`
	Path              string                        `json:"path,omitempty"`
	Controller        string                        `json:"controller,omitempty"`
	Secret            string                        `json:"secret,omitempty"`
	Chain             *generator.ChainPlan          `json:"chain,omitempty"`
	Auxiliary         []RuntimeConfig               `json:"auxiliary,omitempty"`
	SystemProxy       *systemproxy.Snapshot         `json:"system_proxy,omitempty"`
	SystemProxyTarget string                        `json:"system_proxy_target,omitempty"`
}

type RuntimeConfig struct {
	Kernel     ir.KernelType `json:"kernel"`
	Path       string        `json:"path"`
	Controller string        `json:"controller"`
	Secret     string        `json:"secret"`
}

func (s *State) Public() *State {
	if s == nil {
		return nil
	}
	copy := *s
	copy.Path, copy.Controller, copy.Secret = "", "", ""
	copy.Auxiliary = nil
	copy.SystemProxy = nil
	copy.SystemProxyTarget = ""
	copy.NodeIDs = append([]string{}, s.NodeIDs...)
	return &copy
}

type Service struct {
	mu          sync.Mutex
	Store       *store.Store
	Runtime     Runtime
	Directory   string
	Generators  map[ir.KernelType]generator.Generator
	Transparent *transparent.Manager
}

func New(s *store.Store, runtime Runtime, directory string) *Service {
	return &Service{Store: s, Runtime: runtime, Directory: directory, Generators: map[ir.KernelType]generator.Generator{ir.KernelSingBox: singbox.NewGenerator(), ir.KernelMihomo: mihomo.NewGenerator()}}
}

func (s *Service) SetTransparent(manager *transparent.Manager) { s.Transparent = manager }
func defaults() Settings {
	return Settings{ConfigVersion: CurrentConfigVersion, Kernel: ir.KernelSingBox, InboundHost: "127.0.0.1", InboundPort: 1080, MixedPort: 7890, SOCKSPort: 10808, LoadBalance: generator.DefaultLoadBalanceOptions(), LogLevel: "info", DNSServers: []string{"223.5.5.5", "8.8.8.8"}, TUNStack: "system", TransparentMode: transparent.ModeOff, RoutingMode: "bypass_mainland", DefaultOutbound: "PROXY", RoutingRules: defaultRoutingRules()}
}

func defaultRoutingRules() []generator.RoutingRule {
	return []generator.RoutingRule{
		{Match: "domain_suffix", Values: []string{"cn", "中国", "local"}, Outbound: "DIRECT"},
		{Match: "ip_cidr", Values: []string{"10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16"}, Outbound: "DIRECT"},
	}
}

func legacyDefaultRoutingRules() []generator.RoutingRule {
	return []generator.RoutingRule{
		{Match: "geosite", Values: []string{"cn"}, Outbound: "DIRECT"},
		{Match: "geoip", Values: []string{"cn"}, Outbound: "DIRECT"},
		{Match: "domain_suffix", Values: []string{"cn", "中国", "local"}, Outbound: "DIRECT"},
		{Match: "ip_cidr", Values: []string{"10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16"}, Outbound: "DIRECT"},
	}
}

func sameRoutingRules(left, right []generator.RoutingRule) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Match != right[i].Match || left[i].Outbound != right[i].Outbound || len(left[i].Values) != len(right[i].Values) {
			return false
		}
		for j := range left[i].Values {
			if left[i].Values[j] != right[i].Values[j] {
				return false
			}
		}
	}
	return true
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	value := defaults()
	err := s.Store.State(ctx, "settings", &value)
	if errors.Is(err, store.ErrNotFound) {
		err = nil
	}
	if err == nil && sameRoutingRules(value.RoutingRules, legacyDefaultRoutingRules()) {
		value.RoutingRules = defaultRoutingRules()
		if saveErr := s.Store.SaveState(ctx, "settings", value); saveErr != nil {
			return value, saveErr
		}
	}
	if value.RoutingMode == "" {
		value.RoutingMode = "bypass_mainland"
	}
	if value.ConfigVersion == 0 {
		value.ConfigVersion = CurrentConfigVersion
		if saveErr := s.Store.SaveState(ctx, "settings", value); saveErr != nil {
			return value, saveErr
		}
	}
	if len(value.RoutingRules) == 0 {
		value.RoutingRules = defaultRoutingRules()
	}
	if sameRoutingRules(value.RoutingRules, defaultRoutingRules()) {
		for i := range value.RoutingRules {
			value.RoutingRules[i].Source = "bypass"
			value.RoutingRules[i].Enabled = true
			value.RoutingRules[i].ID = fmt.Sprintf("bypass-%d", i+1)
		}
	}
	for i := range value.RoutingRules {
		if value.RoutingRules[i].Source == "" {
			value.RoutingRules[i].Source = "user"
		}
		if value.RoutingRules[i].ID == "" {
			value.RoutingRules[i].ID = fmt.Sprintf("rule-%d", i+1)
		}
		if !value.RoutingRules[i].Enabled {
			// The old model had no enabled field, so old rules remain active.
			value.RoutingRules[i].Enabled = true
		}
	}
	if value.TransparentMode == "" {
		if value.EnableTUN {
			value.TransparentMode = transparent.ModeTUN
		} else {
			value.TransparentMode = transparent.ModeOff
		}
	}
	value.SystemProxySupported = systemproxy.Supported()
	if s.Transparent != nil {
		value.TransparentModes = s.Transparent.Capabilities()
	}
	if state, stateErr := s.Current(ctx); stateErr == nil && state != nil {
		value.TransparentActive = state.TransparentMode
	}
	return value, err
}
func (s *Service) SaveSettings(ctx context.Context, value Settings) error {
	if value.ConfigVersion == 0 {
		value.ConfigVersion = CurrentConfigVersion
	}
	if value.SOCKSOnly && value.EnableTUN {
		return fmt.Errorf("%w: SOCKS-only mode cannot be combined with TUN takeover", store.ErrInvalid)
	}
	if value.TransparentMode == "" {
		if value.EnableTUN {
			value.TransparentMode = transparent.ModeTUN
		} else {
			value.TransparentMode = transparent.ModeOff
		}
	}
	if s.Transparent != nil {
		found, supported, reason := false, false, ""
		for _, info := range s.Transparent.Capabilities() {
			if info.Mode == value.TransparentMode {
				found, supported, reason = true, info.Supported, info.Reason
				break
			}
		}
		if !found || !supported {
			return fmt.Errorf("%w: transparent mode %s unavailable: %s", store.ErrInvalid, value.TransparentMode, reason)
		}
	}
	value.EnableTUN = value.TransparentMode == transparent.ModeTUN
	// Preserve compatibility with clients that predate these optional settings.
	if value.SOCKSPort == 0 {
		value.SOCKSPort = defaults().SOCKSPort
	}
	if value.LoadBalance == (generator.LoadBalanceOptions{}) {
		value.LoadBalance = generator.DefaultLoadBalanceOptions()
	}
	if value.SOCKSOnly && value.EnableTUN {
		return fmt.Errorf("%w: SOCKS-only mode cannot be combined with TUN takeover", store.ErrInvalid)
	}
	if value.SOCKSPort < 1 || value.SOCKSPort > 65535 || (value.SOCKSOnly && (value.SOCKSPort == value.MixedPort || value.SOCKSPort == value.InboundPort)) {
		return fmt.Errorf("%w: SOCKS5 端口无效或与混合端口冲突", store.ErrInvalid)
	}
	if err := value.LoadBalance.Validate(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	if value.Kernel != ir.KernelSingBox && value.Kernel != ir.KernelMihomo {
		return fmt.Errorf("%w: invalid kernel", store.ErrInvalid)
	}
	if value.SystemProxy && !systemproxy.Supported() {
		return fmt.Errorf("%w: 当前系统不支持系统代理；Linux 请确认 gsettings 与 GNOME D-Bus 会话可用，服务器环境请使用 TUN", store.ErrInvalid)
	}
	if net.ParseIP(value.InboundHost) == nil || value.InboundPort < 1 || value.InboundPort > 65535 || value.MixedPort < 1 || value.MixedPort > 65535 {
		return fmt.Errorf("%w: invalid inbound address/port", store.ErrInvalid)
	}
	switch value.LogLevel {
	case "trace", "debug", "info", "warn", "error", "fatal", "panic":
	default:
		return fmt.Errorf("%w: invalid log level", store.ErrInvalid)
	}
	switch value.TUNStack {
	case "system", "gvisor", "mixed":
	default:
		return fmt.Errorf("%w: invalid TUN stack", store.ErrInvalid)
	}
	switch value.RoutingMode {
	case "bypass_mainland", "global", "blacklist":
	default:
		return fmt.Errorf("%w: invalid routing mode", store.ErrInvalid)
	}
	if value.DefaultOutbound == "" {
		return fmt.Errorf("%w: default_outbound is required", store.ErrInvalid)
	}
	if len(value.RoutingRules) == 0 {
		value.RoutingRules = defaultRoutingRules()
	}
	if err := generator.ValidateRoutingRules(value.RoutingRules); err != nil {
		return fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	if err := validateRoutingConflicts(value.RoutingRules); err != nil {
		return fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	value.SystemProxySupported = systemproxy.Supported()
	if s.Transparent != nil {
		value.TransparentModes = s.Transparent.Capabilities()
	}
	return s.Store.SaveState(ctx, "settings", value)
}

func validateRoutingConflicts(rules []generator.RoutingRule) error {
	seen := map[string]int{}
	for i, rule := range rules {
		if !rule.Enabled {
			continue
		}
		key := rule.Source + "|" + rule.Match + "|" + strings.Join(rule.Values, "\x00")
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("routing rule %d conflicts with rule %d: same matcher and values", i+1, previous+1)
		}
		seen[key] = i
	}
	return nil
}
func (s *Service) Current(ctx context.Context) (*State, error) {
	var state State
	if err := s.Store.State(ctx, "current_config", &state); err != nil {
		return nil, err
	}
	if state.ConfigVersion == 0 {
		state.ConfigVersion = CurrentConfigVersion
		if err := s.Store.SaveState(ctx, "current_config", state); err != nil {
			return nil, err
		}
	}
	return &state, nil
}

// Resume restores the last successfully applied configuration after the panel
// process itself restarts. An explicit Stop removes current_config, so this
// only resumes a configuration that was active before the process exited.
func (s *Service) Resume(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var state State
	if err := s.Store.State(ctx, "current_config", &state); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	if s.Runtime == nil {
		return fmt.Errorf("kernel runtime is not configured")
	}
	settings, settingsErr := s.Settings(ctx)
	if settingsErr != nil {
		return settingsErr
	}
	if err := s.prepareTransparent(settings); err != nil {
		return fmt.Errorf("%w: transparent setup: %v", ErrApply, err)
	}
	if err := s.startState(&state); err != nil {
		_ = s.Runtime.StopAll()
		_ = s.cleanupTransparent()
		if state.SystemProxy != nil {
			_ = systemproxy.Restore(*state.SystemProxy)
		}
		return fmt.Errorf("%w: resume: %v", ErrApply, err)
	}
	if state.SystemProxyTarget != "" {
		if err := setSystemProxyTarget(state.SystemProxyTarget); err != nil {
			_ = s.Runtime.StopAll()
			_ = s.cleanupTransparent()
			if state.SystemProxy != nil {
				_ = systemproxy.Restore(*state.SystemProxy)
			}
			return fmt.Errorf("%w: restore system proxy: %v", ErrApply, err)
		}
	}
	return nil
}

// Shutdown stops managed kernels and restores the previous system proxy while
// retaining current_config and generated files for the next panel start.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var state State
	err := s.Store.State(ctx, "current_config", &state)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if s.Runtime != nil {
		if stopErr := s.Runtime.StopAll(); stopErr != nil {
			return stopErr
		}
	}
	if cleanupErr := s.cleanupTransparent(); cleanupErr != nil {
		return cleanupErr
	}
	if err == nil && state.SystemProxy != nil {
		if restoreErr := systemproxy.Restore(*state.SystemProxy); restoreErr != nil {
			return restoreErr
		}
	}
	return nil
}

// Stop terminates the active kernel and removes the generated configuration
// from durable state so the UI returns to an explicitly unselected state.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state State
	err := s.Store.State(ctx, "current_config", &state)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if s.Runtime != nil {
		if err := s.Runtime.StopAll(); err != nil {
			return err
		}
	}
	if cleanupErr := s.cleanupTransparent(); cleanupErr != nil {
		return cleanupErr
	}
	if err == nil {
		if state.SystemProxy != nil {
			if restoreErr := systemproxy.Restore(*state.SystemProxy); restoreErr != nil {
				return restoreErr
			}
		}
		if deleteErr := s.Store.DeleteState(ctx, "current_config"); deleteErr != nil {
			return deleteErr
		}
		if state.Path != "" {
			if removeErr := os.Remove(state.Path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
		}
		for _, config := range state.Auxiliary {
			_ = os.Remove(config.Path)
		}
	}
	return nil
}
func (s *Service) start(kernel ir.KernelType, path string) error {
	if kernel == ir.KernelSingBox {
		return s.Runtime.StartSingBox(path)
	}
	return s.Runtime.StartMihomo(path)
}

func (s *Service) Apply(ctx context.Context, request ApplyRequest) (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	settings, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	if request.Mode == "chain" {
		return s.applyChain(ctx, request, settings)
	}
	if request.Kernel == "" {
		request.Kernel = settings.Kernel
		if request.Mode == "load_balance" {
			request.Kernel = ir.KernelMihomo
		}
	}
	if request.Kernel != ir.KernelSingBox && request.Kernel != ir.KernelMihomo {
		return nil, fmt.Errorf("%w: invalid kernel", store.ErrInvalid)
	}
	if len(request.NodeIDs) > 10000 {
		return nil, fmt.Errorf("%w: too many nodes", store.ErrInvalid)
	}
	if request.Content != "" && len(request.NodeIDs) > 0 {
		return nil, fmt.Errorf("%w: provide config or node_ids", store.ErrInvalid)
	}
	var composition *generator.Composition
	if request.Mode != "" {
		if request.Content != "" || len(request.NodeIDs) < 2 || len(request.NodeIDs) > 64 || request.Mode != "load_balance" {
			return nil, fmt.Errorf("%w: 多节点启动需指定方式并选择 2–64 个节点", store.ErrInvalid)
		}
		composition = &generator.Composition{Mode: request.Mode}
		lb := settings.LoadBalance
		if request.LoadBalance != nil {
			lb = *request.LoadBalance
		}
		composition.LoadBalance = &lb
		request.LoadBalance = &lb
	} else if request.LoadBalance != nil {
		return nil, fmt.Errorf("%w: 负载均衡设置需要配合负载均衡启动方式", store.ErrInvalid)
	}
	if settings.SOCKSOnly && request.Content != "" {
		return nil, fmt.Errorf("%w: 仅 SOCKS5 代理模式下请从节点生成配置，或先关闭该设置", store.ErrInvalid)
	}
	nodeIDs := []string{}
	content := []byte(request.Content)
	if len(content) == 0 {
		nodes := []*ir.Node{}
		if len(request.NodeIDs) == 0 {
			all, err := s.Store.Nodes(ctx, "")
			if err != nil {
				return nil, err
			}
			for _, node := range all {
				if node.CanRunOn(request.Kernel) {
					nodes = append(nodes, node)
					nodeIDs = append(nodeIDs, node.ID)
				}
			}
		} else {
			seen := map[string]bool{}
			for _, id := range request.NodeIDs {
				if seen[id] {
					if composition != nil {
						return nil, fmt.Errorf("%w: 请勿重复选择同一个节点", store.ErrInvalid)
					}
					continue
				}
				seen[id] = true
				node, err := s.Store.Node(ctx, id)
				if err != nil {
					return nil, err
				}
				if !node.CanRunOn(request.Kernel) {
					return nil, fmt.Errorf("%w: 节点 %q 不支持内核 %s", store.ErrInvalid, node.Name, request.Kernel)
				}
				nodes = append(nodes, node)
				nodeIDs = append(nodeIDs, id)
			}
		}
		defaultOutbound, routingRules := effectiveRouting(settings, request.outbound)
		if composition != nil && settings.RoutingMode != "blacklist" {
			defaultOutbound = "PROXY"
		}
		options := &generator.GenerateOptions{InboundHost: settings.InboundHost, InboundPort: settings.InboundPort, MixedPort: settings.MixedPort, SystemProxy: settings.SystemProxy, DNSServers: settings.DNSServers, DNSFakeIP: settings.DNSFakeIP, EnableTUN: settings.EnableTUN, TransparentMode: string(settings.TransparentMode), RedirectPort: settings.InboundPort, TProxyPort: settings.InboundPort + 1, TUNStack: settings.TUNStack, LogLevel: settings.LogLevel, DefaultOutbound: defaultOutbound, RoutingRules: routingRules}
		options.SOCKSOnly, options.SOCKSPort, options.Composition = settings.SOCKSOnly, settings.SOCKSPort, composition
		if settings.AllowLAN {
			options.InboundHost = "0.0.0.0"
		}
		gen := s.Generators[request.Kernel]
		if gen == nil {
			return nil, fmt.Errorf("generator is not configured")
		}
		content, err = gen.Generate(nodes, options)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", store.ErrInvalid, err)
		}
	}
	content, controller, secret, err := withController(content, request.Kernel, settings.AllowLAN)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory := s.Directory
	if directory == "" {
		directory = filepath.Join("data", "configs")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	path, err := filepath.Abs(filepath.Join(directory, id+".json"))
	if err != nil {
		return nil, err
	}
	if request.Kernel == ir.KernelMihomo {
		path = strings.TrimSuffix(path, ".json") + ".yaml"
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		return nil, err
	}
	retained := false
	defer func() {
		if !retained {
			_ = os.Remove(path)
		}
	}()
	if s.Runtime == nil {
		return nil, fmt.Errorf("kernel runtime is not configured")
	}
	if err := s.Runtime.Validate(string(request.Kernel), path); err != nil {
		return nil, fmt.Errorf("%w: validation: %v", ErrApply, err)
	}
	previous, err := s.Current(ctx)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	var proxySnapshot *systemproxy.Snapshot
	proxyTarget := ""
	if settings.SystemProxy {
		if !systemproxy.Supported() {
			return nil, fmt.Errorf("%w: 当前系统不支持系统代理；Linux 请确认 gsettings 与 GNOME D-Bus 会话可用，服务器环境请使用 TUN", store.ErrInvalid)
		}
		if previous != nil && previous.SystemProxy != nil {
			copy := *previous.SystemProxy
			proxySnapshot = &copy
		} else {
			snapshot, captureErr := systemproxy.Capture()
			if captureErr != nil {
				return nil, fmt.Errorf("%w: capture system proxy settings: %v", ErrApply, captureErr)
			}
			proxySnapshot = &snapshot
		}
		proxyTarget = systemProxyTarget(settings, request.Kernel)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.Runtime.StopAll(); err != nil {
		return nil, fmt.Errorf("%w: stop: %v", ErrApply, err)
	}
	rollback := func(cause error) error {
		stopErr := s.Runtime.StopAll()
		cleanupErr := s.cleanupTransparent()
		var restoreErr error
		if previous != nil {
			restoreErr = s.startState(previous)
		}
		if previous != nil && previous.SystemProxy != nil && previous.SystemProxyTarget != "" {
			restoreErr = errors.Join(restoreErr, setSystemProxyTarget(previous.SystemProxyTarget))
		} else if proxySnapshot != nil {
			restoreErr = errors.Join(restoreErr, systemproxy.Restore(*proxySnapshot))
		}
		return errors.Join(fmt.Errorf("%w: %v", ErrApply, cause), stopErr, cleanupErr, restoreErr)
	}
	if err := s.prepareTransparent(settings); err != nil {
		return nil, rollback(fmt.Errorf("transparent setup: %w", err))
	}
	if runtime, ok := s.Runtime.(interface{ SetMode(kernel.RunMode) error }); ok {
		if err := runtime.SetMode(kernel.ModeSingleSwitch); err != nil {
			return nil, rollback(err)
		}
	}
	if err := s.start(request.Kernel, path); err != nil {
		return nil, rollback(err)
	}
	if settings.SystemProxy {
		if err := setSystemProxyTarget(proxyTarget); err != nil {
			return nil, rollback(err)
		}
	} else if previous != nil && previous.SystemProxy != nil {
		if err := systemproxy.Restore(*previous.SystemProxy); err != nil {
			return nil, rollback(err)
		}
	}
	mode := request.Mode
	if mode == "" {
		mode = "single"
		if len(nodeIDs) > 1 {
			mode = "select"
		}
	}
	state := &State{ConfigVersion: CurrentConfigVersion, ID: id, Kernel: request.Kernel, NodeIDs: nodeIDs, Mode: mode, TransparentMode: settings.TransparentMode, LoadBalance: request.LoadBalance, AppliedAt: time.Now().UTC(), Path: path, Controller: controller, Secret: secret, SystemProxy: proxySnapshot, SystemProxyTarget: proxyTarget}
	// Finish the durable state update even if the HTTP client disconnected.
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Store.SaveState(persistCtx, "current_config", state); err != nil {
		return nil, rollback(err)
	}
	retained = true
	removeReplacedConfig(previous)
	return state, nil
}

func effectiveRouting(settings Settings, requestedOutbound string) (string, []generator.RoutingRule) {
	mode := settings.RoutingMode
	if mode == "" {
		mode = "bypass_mainland"
	}
	outbound := settings.DefaultOutbound
	if mode == "blacklist" {
		outbound = "DIRECT"
	} else if outbound == "" || outbound == "DIRECT" {
		outbound = "PROXY"
	}
	if requestedOutbound != "" && mode != "blacklist" {
		outbound = requestedOutbound
	}
	rules := make([]generator.RoutingRule, 0, len(settings.RoutingRules))
	for _, rule := range settings.RoutingRules {
		if rule.Source == "internal" || rule.Source == "bypass" || rule.Source == "" {
			rules = append(rules, rule)
		}
	}
	for _, rule := range settings.RoutingRules {
		if rule.Source == "user" {
			rules = append(rules, rule)
		}
	}
	return outbound, rules
}

func (s *Service) prepareTransparent(settings Settings) error {
	if s.Transparent == nil {
		return nil
	}
	return s.Transparent.Prepare(settings.TransparentMode, settings.InboundPort, settings.InboundPort+1)
}

func (s *Service) cleanupTransparent() error {
	if s.Transparent == nil {
		return nil
	}
	return s.Transparent.Cleanup()
}

func systemProxyTarget(settings Settings, kernel ir.KernelType) string {
	port := settings.InboundPort
	if kernel == ir.KernelMihomo {
		port = settings.MixedPort
	}
	if settings.SOCKSOnly {
		port = settings.SOCKSPort
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

func setSystemProxyTarget(target string) error {
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		return fmt.Errorf("invalid system proxy target: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("invalid system proxy target port: %w", err)
	}
	return systemproxy.Set(host, port)
}

func (s *Service) Activate(ctx context.Context, id string, kernel ir.KernelType) (*State, error) {
	node, err := s.Store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	if kernel == "" {
		settings, err := s.Settings(ctx)
		if err != nil {
			return nil, err
		}
		kernel = settings.Kernel
		if !node.CanRunOn(kernel) {
			kernel = node.PreferredKernel()
		}
	}
	if !node.CanRunOn(kernel) {
		return nil, fmt.Errorf("%w: incompatible kernel", store.ErrInvalid)
	}
	return s.Apply(ctx, ApplyRequest{Kernel: kernel, NodeIDs: []string{id}, outbound: node.Name})
}

func withController(content []byte, kernel ir.KernelType, allowLAN bool) ([]byte, string, string, error) {
	root := map[string]any{}
	var err error
	if kernel == ir.KernelSingBox {
		err = json.Unmarshal(content, &root)
	} else {
		err = yaml.Unmarshal(content, &root)
	}
	if err != nil || root == nil {
		return nil, "", "", fmt.Errorf("%w: malformed config", store.ErrInvalid)
	}
	container := root
	addressKey := "external-controller"
	defaultAddress := "127.0.0.1:19093"
	if kernel == ir.KernelSingBox {
		defaultAddress = "127.0.0.1:19094"
		addressKey = "external_controller"
		experimental, _ := root["experimental"].(map[string]any)
		if experimental == nil {
			experimental = map[string]any{}
			root["experimental"] = experimental
		}
		container, _ = experimental["clash_api"].(map[string]any)
		if container == nil {
			container = map[string]any{}
			experimental["clash_api"] = container
		}
	} else {
		root["allow-lan"] = allowLAN
	}
	address, _ := container[addressKey].(string)
	if address == "" {
		address = defaultAddress
		container[addressKey] = address
	}
	secret, _ := container["secret"].(string)
	if secret == "" {
		secret = uuid.NewString()
		container["secret"] = secret
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port == "" || port == "0" {
		return nil, "", "", fmt.Errorf("%w: invalid controller address", store.ErrInvalid)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	var output []byte
	if kernel == ir.KernelSingBox {
		output, err = json.MarshalIndent(root, "", "  ")
	} else {
		output, err = yaml.Marshal(root)
	}
	return output, "http://" + net.JoinHostPort(host, port), secret, err
}

func (s *Service) ProbeHTTP(ctx context.Context, node *ir.Node, target string, timeout time.Duration) (int64, error) {
	state, err := s.Current(ctx)
	if err != nil {
		return 0, err
	}
	found := false
	for _, id := range state.NodeIDs {
		if id == node.ID {
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("node is not in the active generated configuration")
	}
	client := clashapi.NewClient(state.Controller, state.Secret, clashapi.KernelType(state.Kernel))
	tag := node.Name
	if state.Chain != nil {
		for _, hop := range state.Chain.Hops {
			if hop.ID == node.ID {
				tag = hop.Name
				if hop.Kernel != state.Kernel {
					for _, aux := range state.Auxiliary {
						if aux.Kernel == hop.Kernel {
							client.Close()
							client = clashapi.NewClient(aux.Controller, aux.Secret, clashapi.KernelType(aux.Kernel))
						}
					}
				}
			}
		}
	}
	defer client.Close()
	response, err := client.TestDelay(ctx, tag, target, int(timeout/time.Millisecond))
	if err != nil {
		return 0, err
	}
	return int64(response.Delay), nil
}
