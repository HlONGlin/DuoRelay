//go:build !linux

package transparent

import "runtime"

type unsupportedBackend struct{}

func newBackend() backend { return unsupportedBackend{} }

func (unsupportedBackend) Capabilities() []ModeInfo {
	if runtime.GOOS == "windows" {
		return []ModeInfo{
			{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限", DNSDescription: "不接管 DNS。"},
			{Mode: ModeTUN, Supported: true, Prerequisites: "首次使用需管理员权限、TUN 驱动和防火墙确认", DNSDescription: "由内核 TUN DNS hijack 接管；需确认防火墙与驱动。"},
			{Mode: ModeRedirect, Supported: false, Reason: "Windows 没有 Linux redirect 透明代理链", Prerequisites: "不适用", DNSDescription: "不适用。"},
			{Mode: ModeTProxy, Supported: false, Reason: "Windows 没有 Linux TPROXY 策略路由", Prerequisites: "不适用", DNSDescription: "不适用。"},
		}
	}
	return []ModeInfo{
		{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限", DNSDescription: "不接管 DNS。"},
		{Mode: ModeTUN, Supported: false, Reason: "macOS TUN 尚未完成真实系统验收", Prerequisites: "需在 macOS 真实系统验证", DNSDescription: "尚未验证，不能声明已接管。"},
		{Mode: ModeRedirect, Supported: false, Reason: "仅 Linux 支持此模式", Prerequisites: "不适用", DNSDescription: "不适用。"},
		{Mode: ModeTProxy, Supported: false, Reason: "仅 Linux 支持此模式", Prerequisites: "不适用", DNSDescription: "不适用。"},
	}
}

func (unsupportedBackend) Prepare(mode Mode, _, _ int) error { return nil }
func (unsupportedBackend) Cleanup() error                    { return nil }
