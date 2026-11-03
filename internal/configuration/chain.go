package configuration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/holi/duorelay/internal/generator"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/kernel"
	"github.com/holi/duorelay/internal/store"
	"github.com/holi/duorelay/internal/variant"
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

func (s *Service) installChain(ctx context.Context, request ApplyRequest, plan *generator.ChainPlan, configs []generatedChainConfig, allowLAN bool, release func(), socksListeners []SOCKSListener) (*State, error) {
	directory := s.Directory
	if directory == "" {
		directory = filepath.Join("data", "configs")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	state := &State{ConfigVersion: CurrentConfigVersion, ID: uuid.NewString(), Kernel: plan.Kernel, Mode: "chain", Chain: plan, NodeIDs: append([]string{}, request.NodeIDs...), SOCKSListeners: append([]SOCKSListener{}, socksListeners...), AppliedAt: time.Now().UTC()}
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
