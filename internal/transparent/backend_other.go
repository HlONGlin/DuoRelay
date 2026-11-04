//go:build !linux && !windows && !darwin

package transparent

import (
	"fmt"
)

type unsupportedBackend struct{}

func newBackend() backend { return unsupportedBackend{} }

func (unsupportedBackend) Capabilities() []ModeInfo {
	return []ModeInfo{
		{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限", DNSDescription: "不接管 DNS。"},
		{Mode: ModeTUN, Supported: false, Reason: "当前操作系统不支持 TUN 模式", Prerequisites: "请使用显式 HTTP/SOCKS5 代理", DNSDescription: "不接管 DNS。"},
		{Mode: ModeRedirect, Supported: false, Reason: "仅 Linux 支持此模式", Prerequisites: "不适用", DNSDescription: "不适用。"},
		{Mode: ModeTProxy, Supported: false, Reason: "仅 Linux 支持此模式", Prerequisites: "不适用", DNSDescription: "不适用。"},
	}
}

func (unsupportedBackend) Prepare(mode Mode, _, _ int) error {
	return (unsupportedBackend{}).Preflight(mode, 0, 0)
}
func (unsupportedBackend) Preflight(mode Mode, _, _ int) error {
	if mode != ModeOff && mode != "" {
		return unsupportedBackend{}.modeError(mode)
	}
	return nil
}
func (unsupportedBackend) modeError(mode Mode) error {
	return fmt.Errorf("transparent mode %s is not supported on this operating system", mode)
}
func (unsupportedBackend) Cleanup() error { return nil }
