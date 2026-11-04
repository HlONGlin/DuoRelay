package configuration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/systemproxy"
	"github.com/holi/duorelay/internal/transparent"
	"github.com/holi/duorelay/internal/variant"
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
	return generator.PrepareRuntimeNames(nodes), nil
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
	if variant.Name == "lite" {
		if force && preferred != ir.KernelMihomo {
			return nil, fmt.Errorf("精简版不支持 sing-box")
		}
		preferred, force = ir.KernelMihomo, true
		for _, n := range nodes {
			if !variant.SupportsProtocol(string(n.Protocol)) {
				return nil, fmt.Errorf("精简版不支持 %s", n.Protocol)
			}
		}
	}
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

func (s *Service) startState(state *State) error {
	if s.Transparent != nil {
		port, err := stateTransparentPort(state)
		if err != nil {
			return err
		}
		if err := s.Transparent.Preflight(state.TransparentMode, port, port+1); err != nil {
			return err
		}
		if err := s.Transparent.Prepare(state.TransparentMode, port, port+1); err != nil {
			return err
		}
	}
	if variant.Name == "lite" && (!variant.SupportsKernel(string(state.Kernel)) || len(state.Auxiliary) > 0) {
		return fmt.Errorf("精简版不支持已保存的双核配置；数据保留，请重新选择 mihomo 节点")
	}
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

// Older states did not persist the applied redirect/TPROXY port. Read the
// retained configuration instead of using settings the user has since edited.
func stateTransparentPort(state *State) (int, error) {
	if state.TransparentPort != 0 {
		return state.TransparentPort, nil
	}
	if state.TransparentMode != transparent.ModeRedirect && state.TransparentMode != transparent.ModeTProxy {
		return defaults().InboundPort, nil
	}
	data, err := os.ReadFile(state.Path)
	if err != nil {
		return 0, fmt.Errorf("read applied transparent port: %w", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return 0, err
	}
	var value any
	if state.Kernel == ir.KernelMihomo {
		key := "redir-port"
		if state.TransparentMode == transparent.ModeTProxy {
			key = "tproxy-port"
		}
		value = root[key]
	} else {
		for _, inbound := range chainEntries(root, "inbounds") {
			if inbound["type"] == string(state.TransparentMode) {
				value = inbound["listen_port"]
				break
			}
		}
	}
	port, err := strconv.Atoi(fmt.Sprint(value))
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("cannot determine the previous transparent port; reapply the configuration")
	}
	if state.TransparentMode == transparent.ModeTProxy {
		port--
	}
	return port, nil
}

func (s *Service) installChain(ctx context.Context, request ApplyRequest, plan *generator.ChainPlan, configs []generatedChainConfig, settings Settings, release func(), socksListeners []SOCKSListener) (*State, error) {
	directory := s.Directory
	if directory == "" {
		directory = filepath.Join("data", "configs")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	state := &State{ConfigVersion: CurrentConfigVersion, ID: uuid.NewString(), Kernel: plan.Kernel, Mode: "chain", SelectedNodeID: request.compositionNodeID, Chain: plan, NodeIDs: append([]string{}, request.NodeIDs...), SOCKSListeners: append([]SOCKSListener{}, socksListeners...), AppliedAt: time.Now().UTC()}
	state.TransparentMode, state.TransparentPort = settings.TransparentMode, settings.InboundPort
	state.RuntimeNames = map[string]string{}
	for _, hop := range plan.Hops {
		state.RuntimeNames[hop.ID] = hop.Name
	}
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
		content, controller, secret, err := withController(config.content, config.kernel, settings.AllowLAN && config.kernel == plan.Kernel)
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
	if settings.SystemProxy {
		if !systemproxy.Supported() {
			return nil, fmt.Errorf("%w: system proxy is unavailable", store.ErrInvalid)
		}
		if previous != nil && previous.SystemProxy != nil {
			copy := *previous.SystemProxy
			state.SystemProxy = &copy
		} else {
			snapshot, err := systemproxy.Capture()
			if err != nil {
				return nil, fmt.Errorf("%w: capture system proxy: %v", ErrApply, err)
			}
			state.SystemProxy = &snapshot
		}
		state.SystemProxyTarget = systemProxyTarget(settings, state.Kernel)
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
		if previous != nil && previous.SystemProxyTarget != "" {
			restoreErr = errors.Join(restoreErr, setSystemProxyTarget(previous.SystemProxyTarget))
		} else if state.SystemProxy != nil {
			restoreErr = errors.Join(restoreErr, systemproxy.Restore(*state.SystemProxy))
		}
		return errors.Join(fmt.Errorf("%w: %v", ErrApply, cause), stopErr, cleanupErr, restoreErr)
	}
	release()
	if err := s.startState(state); err != nil {
		return nil, rollback(err)
	}
	if state.SystemProxyTarget != "" {
		if err := setSystemProxyTarget(state.SystemProxyTarget); err != nil {
			return nil, rollback(err)
		}
	} else if previous != nil && previous.SystemProxy != nil {
		if err := systemproxy.Restore(*previous.SystemProxy); err != nil {
			return nil, rollback(err)
		}
	}
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.Store.SaveState(persistCtx, "current_config", state); err != nil {
		return nil, rollback(err)
	}
	retained = true
	pathsToKeep := []string{state.Path}
	for _, auxiliary := range state.Auxiliary {
		pathsToKeep = append(pathsToKeep, auxiliary.Path)
	}
	removeReplacedConfig(previous, pathsToKeep...)
	return state, nil
}

const configHistoryLimit = 10

func removeReplacedConfig(previous *State, current ...string) {
	directories := map[string]struct{}{}
	addDirectory := func(path string) {
		if path != "" {
			directories[filepath.Dir(path)] = struct{}{}
		}
	}
	if previous != nil {
		addDirectory(previous.Path)
		for _, config := range previous.Auxiliary {
			addDirectory(config.Path)
		}
	}
	for _, path := range current {
		addDirectory(path)
	}
	for directory := range directories {
		pruneConfigHistory(directory, current...)
	}
}

func pruneConfigHistory(directory string, current ...string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	keep := map[string]struct{}{}
	for _, path := range current {
		if path != "" {
			keep[filepath.Base(path)] = struct{}{}
		}
	}
	type configFile struct {
		name string
		date time.Time
	}
	files := make([]configFile, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || (filepath.Ext(entry.Name()) != ".json" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		stem := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		stem = strings.TrimSuffix(strings.TrimSuffix(stem, "-sing-box"), "-mihomo")
		if _, err := uuid.Parse(stem); err != nil {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			files = append(files, configFile{name: entry.Name(), date: info.ModTime()})
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].date.After(files[j].date) })
	for index, file := range files {
		if index < configHistoryLimit {
			continue
		}
		if _, explicitlyKept := keep[file.name]; explicitlyKept {
			continue
		}
		_ = os.Remove(filepath.Join(directory, file.name))
	}
}

// PruneConfigHistory is also called on startup; current files always survive.
func (s *Service) PruneConfigHistory(state *State) {
	var keep []string
	if state != nil {
		keep = append(keep, state.Path)
		for _, aux := range state.Auxiliary {
			keep = append(keep, aux.Path)
		}
	}
	pruneConfigHistory(s.Directory, keep...)
}
