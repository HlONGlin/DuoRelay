package generator

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/holi/duorelay/internal/ir"
)

// Composition preserves node order: a chain is traversed from first to last.
type Composition struct {
	Mode        string              `json:"mode"`
	LoadBalance *LoadBalanceOptions `json:"load_balance,omitempty"`
}

type LoadBalanceOptions struct {
	Strategy string `json:"strategy"`
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	Timeout  int    `json:"timeout"`
	Lazy     bool   `json:"lazy"`
}

func DefaultLoadBalanceOptions() LoadBalanceOptions {
	return LoadBalanceOptions{Strategy: "round-robin", URL: "https://www.gstatic.com/generate_204", Interval: 300, Timeout: 5000, Lazy: true}
}

func (o LoadBalanceOptions) Validate() error {
	switch o.Strategy {
	case "round-robin", "consistent-hashing", "sticky-sessions", "url-test", "fallback":
	default:
		return fmt.Errorf("不支持的负载均衡策略")
	}
	u, err := url.Parse(o.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("健康检查地址必须是有效的 HTTP 或 HTTPS URL")
	}
	if o.Interval < 10 || o.Interval > 86400 || o.Timeout < 100 || o.Timeout > 60000 {
		return fmt.Errorf("健康检查间隔需为 10–86400 秒，超时需为 100–60000 毫秒")
	}
	return nil
}

// PrepareComposition validates instead of silently omitting incompatible nodes.
// Duplicate display names get distinct runtime tags without changing the store.
func PrepareComposition(nodes []*ir.Node, c *Composition, kernel ir.KernelType) ([]*ir.Node, error) {
	if c == nil {
		return nodes, nil
	}
	if len(nodes) < 2 || len(nodes) > 64 {
		return nil, fmt.Errorf("请选择 2–64 个不同节点")
	}
	switch c.Mode {
	case "load_balance":
		if kernel != ir.KernelMihomo {
			return nil, fmt.Errorf("负载均衡需要使用 mihomo 内核")
		}
		if c.LoadBalance == nil {
			return nil, fmt.Errorf("缺少负载均衡设置")
		}
		if err := c.LoadBalance.Validate(); err != nil {
			return nil, err
		}
	case "chain":
		if len(nodes) > 8 {
			return nil, fmt.Errorf("链式代理最多支持 8 跳")
		}
	default:
		return nil, fmt.Errorf("不支持的多节点启动方式")
	}
	used := map[string]bool{"DIRECT": true, "REJECT": true, "PROXY": true}
	ids := map[string]bool{}
	prepared := make([]*ir.Node, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			return nil, fmt.Errorf("节点不存在")
		}
		if !node.CanRunOn(kernel) {
			return nil, fmt.Errorf("节点 %q 不支持所选内核", node.Name)
		}
		if node.ID != "" && ids[node.ID] {
			return nil, fmt.Errorf("请勿重复选择同一个节点")
		}
		ids[node.ID] = true
		copy := *node
		base := strings.TrimSpace(node.Name)
		if base == "" {
			base = "节点"
		}
		copy.Name = base
		for suffix := 2; used[copy.Name]; suffix++ {
			copy.Name = fmt.Sprintf("%s (%d)", base, suffix)
		}
		used[copy.Name] = true
		prepared = append(prepared, &copy)
	}
	if c.Mode == "chain" {
		if _, err := PlanChain(prepared, kernel, true); err != nil {
			return nil, err
		}
	}
	return prepared, nil
}
