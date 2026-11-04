//go:build darwin

package transparent

import (
	"fmt"
	"os"
)

type darwinBackend struct{}

func newBackend() backend { return darwinBackend{} }
func (darwinBackend) Capabilities() []ModeInfo {
	root := os.Geteuid() == 0
	reason := ""
	if !root {
		reason = "macOS TUN 需要 root 权限；请通过系统服务启动"
	}
	return []ModeInfo{
		{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限"},
		{Mode: ModeTUN, Supported: root, Reason: reason, Prerequisites: "需要 root 权限以创建 utun 和管理路由", DNSDescription: "内核负责 TUN 与 DNS 劫持"},
		{Mode: ModeRedirect, Reason: "仅 Linux 支持此模式"},
		{Mode: ModeTProxy, Reason: "仅 Linux 支持此模式"},
	}
}
func (darwinBackend) Preflight(mode Mode, _, _ int) error {
	if mode == "" || mode == ModeOff {
		return nil
	}
	if mode != ModeTUN {
		return fmt.Errorf("transparent mode %s is only supported on Linux", mode)
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("macOS TUN requires root privileges")
	}
	return nil
}
func (b darwinBackend) Prepare(mode Mode, a, c int) error { return b.Preflight(mode, a, c) }
func (darwinBackend) Cleanup() error                      { return nil }
