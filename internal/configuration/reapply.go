package configuration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/holi/duorelay/internal/clashapi"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

// reapply is called with the service lock held. The existing files and state
// remain available until the replacement starts and its state is committed.
func (s *Service) reapply(ctx context.Context, state *State, settings Settings, migrating bool) (*State, error) {
	request := ApplyRequest{Kernel: settings.Kernel, NodeIDs: state.NodeIDs, Mode: state.Mode, LoadBalance: state.LoadBalance, compositionNodeID: state.SelectedNodeID}
	if state.Raw || (len(state.NodeIDs) == 0 && len(state.RuntimeNames) == 0 && state.Mode == "single") {
		if migrating || state.Kernel != settings.Kernel {
			return nil, fmt.Errorf("%w: a raw configuration must be replaced with one for %s", store.ErrInvalid, settings.Kernel)
		}
		return nil, fmt.Errorf("%w: reapply the raw configuration to change its listeners", store.ErrInvalid)
	}
	switch state.Mode {
	case "single", "select":
		request.Mode = ""
	case "chain":
		return s.applyChain(ctx, request, settings)
	}
	if state.Mode == "single" && len(state.NodeIDs) == 1 {
		node, err := s.Store.Node(ctx, state.NodeIDs[0])
		if err != nil {
			return nil, err
		}
		request.outbound = node.Name
	}
	return s.applyPrepared(ctx, request, settings)
}

func (s *Service) ActivateManual(ctx context.Context, id string, requested ir.KernelType) (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.Current(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if state.Mode != "manual" || (requested != "" && state.Kernel != requested) || !slices.Contains(state.NodeIDs, id) {
		return nil, nil
	}
	if _, err := os.Stat(state.Path); err != nil {
		return nil, err
	}
	previous := state.SelectedNodeID
	if err := s.selectManual(ctx, state, id); err != nil {
		return nil, err
	}
	if err := s.Store.SaveState(context.WithoutCancel(ctx), "current_config", state); err != nil {
		return nil, errors.Join(err, s.selectManual(context.WithoutCancel(ctx), state, previous))
	}
	return state, nil
}

func (s *Service) selectManual(ctx context.Context, state *State, id string) error {
	name := state.RuntimeNames[id]
	if state.Controller == "" || name == "" {
		return fmt.Errorf("%w: manual selector is unavailable", ErrApply)
	}
	client := clashapi.NewClient(state.Controller, state.Secret, clashapi.KernelType(state.Kernel))
	defer client.Close()
	if err := client.SelectProxy(ctx, "PROXY", name); err != nil {
		return fmt.Errorf("%w: switch proxy: %v", ErrApply, err)
	}
	state.SelectedNodeID = id
	return nil
}
