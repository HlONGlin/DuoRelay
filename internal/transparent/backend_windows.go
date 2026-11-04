//go:build windows

package transparent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

type windowsBackend struct{}

func newBackend() backend { return windowsBackend{} }

func (windowsBackend) Capabilities() []ModeInfo {
	admin := windows.GetCurrentProcessToken().IsElevated()
	tunReason := ""
	if !admin {
		tunReason = "当前进程未以管理员权限运行；请使用提升的 PowerShell 或服务账户"
	}
	return []ModeInfo{
		{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限", DNSDescription: "不接管 DNS。"},
		{Mode: ModeTUN, Supported: admin, Reason: tunReason, Prerequisites: "需要管理员权限、Wintun 驱动和防火墙确认；内核负责 TUN 路由和 DNS hijack。", DNSDescription: "由内核 TUN DNS hijack 接管；需确认防火墙与驱动。"},
		{Mode: ModeRedirect, Supported: false, Reason: "Windows 没有 Linux redirect 透明代理链", Prerequisites: "不适用", DNSDescription: "不适用。"},
		{Mode: ModeTProxy, Supported: false, Reason: "Windows 没有 Linux TPROXY 策略路由", Prerequisites: "不适用", DNSDescription: "不适用。"},
	}
}

func (windowsBackend) Preflight(mode Mode, redirectPort, tproxyPort int) error {
	if mode == ModeOff || mode == "" {
		return nil
	}
	if mode != ModeTUN {
		return fmt.Errorf("transparent mode %s is not supported on Windows", mode)
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("Windows TUN transparent proxy requires administrator privileges")
	}
	return nil
}

func (b windowsBackend) Prepare(mode Mode, redirectPort, tproxyPort int) error {
	return b.Preflight(mode, redirectPort, tproxyPort)
}

func (windowsBackend) Cleanup() error { return nil }
