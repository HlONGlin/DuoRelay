//go:build lite

package configuration

import (
	"context"
	"fmt"
	"github.com/holi/duorelay/internal/ir"
	"github.com/holi/duorelay/internal/store"
)

// Use native mihomo composition and the shared apply transaction, including
// transparent setup, system proxy restoration and failure rollback.
func (s *Service) applyChain(ctx context.Context, request ApplyRequest, settings Settings) (*State, error) {
	if request.Content != "" || request.LoadBalance != nil {
		return nil, fmt.Errorf("%w: 链式代理不能同时传入原始配置或负载均衡设置", store.ErrInvalid)
	}
	nodes, err := s.chainNodes(ctx, request.NodeIDs)
	if err != nil {
		return nil, err
	}
	plan, err := s.planChain(nodes, request.Kernel, request.Kernel != "")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	_, routing := effectiveRouting(settings, request.outbound)
	for _, r := range routing {
		if r.Outbound != "PROXY" && r.Outbound != "DIRECT" && r.Outbound != "REJECT" && r.Outbound != nodes[len(nodes)-1].Name {
			return nil, fmt.Errorf("%w: 链式代理的路由只能选择 PROXY、DIRECT、REJECT 或最终出口，请调整指定节点的规则", store.ErrInvalid)
		}
	}
	request.Kernel, request.chain = ir.KernelMihomo, plan
	return s.applyPrepared(ctx, request, settings)
}
