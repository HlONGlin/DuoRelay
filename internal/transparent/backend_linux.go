//go:build linux

package transparent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	redirectTable = "duorelay"
	tproxyTable   = 100
	tproxyMark    = "0xD0"
)

type linuxBackend struct {
	active Mode
	tool   string
}

func newBackend() backend { return &linuxBackend{} }

func (b *linuxBackend) Capabilities() []ModeInfo {
	hasTun := fileAvailable("/dev/net/tun")
	nft := commandAvailable("nft")
	iptables := commandAvailable("iptables")
	admin := commandAvailable("ip") && hasNetAdmin() && hasNetRaw()
	firewall := nft || iptables
	base := "需要 root 或 CAP_NET_ADMIN/CAP_NET_RAW；" + "iproute2；"
	if !firewall {
		base += "nftables 或 iptables"
	}
	return []ModeInfo{
		{Mode: ModeOff, Supported: true, Prerequisites: "无需系统权限", DNSDescription: "不接管 DNS；按应用自身代理设置运行。"},
		{Mode: ModeRedirect, Supported: firewall && admin, Reason: unavailable(firewall && admin, "缺少 nftables/iptables 或 iproute2"), Prerequisites: base + "redirect 只接管 TCP；DNS 不自动劫持，需使用内核 DNS 配置或系统明确指向代理 DNS。", DNSDescription: "redirect 不接管 UDP DNS；使用系统 DNS 会产生 DNS 泄漏，面板会明确标记。"},
		{Mode: ModeTProxy, Supported: firewall && admin, Reason: unavailable(firewall && admin, "缺少 nftables/iptables 或 iproute2"), Prerequisites: base + "内核需支持 TPROXY、fwmark 和策略路由。", DNSDescription: "TPROXY 可接管 TCP/UDP，但 DuoRelay 当前只建立代理流量链，DNS 仍需显式配置到代理 DNS，避免把系统 DNS 当成已接管。"},
		{Mode: ModeTUN, Supported: hasTun && hasNetAdmin() && hasNetRaw(), Reason: linuxCapabilityReason(hasTun), Prerequisites: "需要 root 或 CAP_NET_ADMIN/CAP_NET_RAW，并可读写 /dev/net/tun；内核负责 TUN 路由和 DNS hijack。", DNSDescription: "TUN 通过内核 DNS hijack 接管 TCP/UDP 53；内部 DNS 分流规则先于用户规则，避免 DNS 泄漏。"},
	}
}

func linuxCapabilityReason(hasTun bool) string {
	if !hasTun {
		return "/dev/net/tun 不可用或不可读写；服务需 DeviceAllow 或容器需显式映射"
	}
	if !hasNetAdmin() || !hasNetRaw() {
		return "当前进程缺少 CAP_NET_ADMIN/CAP_NET_RAW；请使用 root、systemd Capability 配置或容器 --cap-add"
	}
	return ""
}

func (b *linuxBackend) Preflight(mode Mode, redirectPort, tproxyPort int) error {
	if mode == ModeOff || mode == "" {
		return nil
	}
	if redirectPort < 1 || redirectPort > 65535 || tproxyPort < 1 || tproxyPort > 65535 {
		return fmt.Errorf("transparent proxy ports are invalid")
	}
	if !hasNetAdmin() || !hasNetRaw() {
		return fmt.Errorf("transparent proxy requires effective CAP_NET_ADMIN and CAP_NET_RAW; root inside a restricted container is insufficient")
	}
	if mode == ModeTUN {
		if !fileAvailable("/dev/net/tun") {
			return fmt.Errorf("/dev/net/tun is unavailable or not readable/writable")
		}
		return nil
	}
	if !commandAvailable("ip") {
		return fmt.Errorf("iproute2 command is required")
	}
	if mode != ModeRedirect && mode != ModeTProxy {
		return fmt.Errorf("unsupported transparent mode %q", mode)
	}
	_, err := firewallTool(mode)
	return err
}

func firewallTool(mode Mode) (string, error) {
	var failures []error
	if commandAvailable("nft") {
		if err := runCommand("nft", "list", "tables"); err == nil {
			return "nft", nil
		} else {
			failures = append(failures, err)
		}
	}
	if commandAvailable("iptables") {
		lockPath := os.Getenv("XTABLES_LOCKFILE")
		if lockPath == "" {
			lockPath = "/run/xtables.lock"
		}
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
		if err == nil {
			_ = lock.Close()
			table := "nat"
			if mode == ModeTProxy {
				table = "mangle"
			}
			err = runCommand("iptables", "-w", "2", "-t", table, "-L", "-n")
			if err == nil {
				return "iptables", nil
			}
		}
		failures = append(failures, fmt.Errorf("iptables/xtables.lock access: %w", err))
	}
	return "", fmt.Errorf("no usable firewall backend (use the root system service for redirect/tproxy): %v", failures)
}

func unavailable(ok bool, reason string) string {
	if ok {
		return ""
	}
	return reason
}

func (b *linuxBackend) Prepare(mode Mode, redirectPort, tproxyPort int) error {
	if err := b.Preflight(mode, redirectPort, tproxyPort); err != nil {
		return err
	}
	if err := b.Cleanup(); err != nil {
		return err
	}
	if mode != ModeRedirect && mode != ModeTProxy {
		return nil
	}
	tool, err := firewallTool(mode)
	if err != nil {
		return err
	}
	b.tool, b.active = tool, mode
	// Remove only DuoRelay's stale rules, using the backend that passed preflight.
	if err := b.Cleanup(); err != nil {
		return err
	}
	b.tool, b.active = tool, mode
	if tool == "nft" {
		if err := b.prepareNFT(mode, redirectPort, tproxyPort); err != nil {
			return errors.Join(err, b.Cleanup())
		}
	} else {
		if err := b.prepareIPTables(mode, redirectPort, tproxyPort); err != nil {
			return errors.Join(err, b.Cleanup())
		}
	}
	b.active = mode
	return nil
}

func (b *linuxBackend) prepareNFT(mode Mode, redirectPort, tproxyPort int) error {
	var script string
	if mode == ModeRedirect {
		script = fmt.Sprintf("table ip %s {\n chain prerouting {\n  type nat hook prerouting priority dstnat; policy accept;\n  ip daddr != 127.0.0.0/8 meta l4proto tcp redirect to :%d\n }\n}\n", redirectTable, redirectPort)
	} else {
		script = fmt.Sprintf("table inet %s {\n chain prerouting {\n  type filter hook prerouting priority mangle; policy accept;\n  ip daddr != 127.0.0.0/8 meta l4proto { tcp, udp } tproxy ip to :%d meta mark set %s\n }\n}\n", redirectTable, tproxyPort, tproxyMark)
	}
	if err := runInput("nft", script); err != nil {
		return err
	}
	if mode == ModeTProxy {
		return runCommands([][]string{
			{"ip", "rule", "add", "fwmark", tproxyMark, "table", strconv.Itoa(tproxyTable)},
			{"ip", "route", "add", "local", "0.0.0.0/0", "dev", "lo", "table", strconv.Itoa(tproxyTable)},
		})
	}
	return nil
}

func (b *linuxBackend) prepareIPTables(mode Mode, redirectPort, tproxyPort int) error {
	if mode == ModeRedirect {
		commands := [][]string{
			{"iptables", "-t", "nat", "-N", "DUORELAY_REDIRECT"},
			{"iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "-j", "DUORELAY_REDIRECT"},
			{"iptables", "-t", "nat", "-A", "DUORELAY_REDIRECT", "-d", "127.0.0.0/8", "-j", "RETURN"},
			{"iptables", "-t", "nat", "-A", "DUORELAY_REDIRECT", "-p", "tcp", "-j", "REDIRECT", "--to-ports", strconv.Itoa(redirectPort)},
		}
		return runCommands(commands)
	}
	commands := [][]string{
		{"iptables", "-t", "mangle", "-N", "DUORELAY_TPROXY"},
		{"iptables", "-t", "mangle", "-A", "PREROUTING", "-p", "tcp", "-j", "DUORELAY_TPROXY"},
		{"iptables", "-t", "mangle", "-A", "PREROUTING", "-p", "udp", "-j", "DUORELAY_TPROXY"},
		{"iptables", "-t", "mangle", "-A", "DUORELAY_TPROXY", "-j", "TPROXY", "--on-port", strconv.Itoa(tproxyPort), "--tproxy-mark", tproxyMark + "/0xff"},
		{"ip", "rule", "add", "fwmark", tproxyMark, "table", strconv.Itoa(tproxyTable)},
		{"ip", "route", "add", "local", "0.0.0.0/0", "dev", "lo", "table", strconv.Itoa(tproxyTable)},
	}
	return runCommands(commands)
}

func (b *linuxBackend) Cleanup() error {
	if b.tool == "" {
		return nil
	}
	var errs []string
	if b.tool == "nft" {
		if err := runCommand("nft", "delete", "table", "ip", redirectTable); err != nil && !isMissing(err) {
			errs = append(errs, err.Error())
		}
		if err := runCommand("nft", "delete", "table", "inet", redirectTable); err != nil && !isMissing(err) {
			errs = append(errs, err.Error())
		}
	}
	if b.tool == "iptables" {
		for _, args := range [][]string{
			{"-t", "nat", "-D", "PREROUTING", "-p", "tcp", "-j", "DUORELAY_REDIRECT"},
			{"-t", "nat", "-F", "DUORELAY_REDIRECT"}, {"-t", "nat", "-X", "DUORELAY_REDIRECT"},
			{"-t", "mangle", "-D", "PREROUTING", "-p", "tcp", "-j", "DUORELAY_TPROXY"},
			{"-t", "mangle", "-D", "PREROUTING", "-p", "udp", "-j", "DUORELAY_TPROXY"},
			{"-t", "mangle", "-F", "DUORELAY_TPROXY"}, {"-t", "mangle", "-X", "DUORELAY_TPROXY"},
		} {
			if err := runCommand("iptables", args...); err != nil && !isMissing(err) {
				errs = append(errs, err.Error())
			}
		}
	}
	if b.active == ModeTProxy && commandAvailable("ip") {
		for _, args := range [][]string{{"rule", "del", "fwmark", tproxyMark, "table", strconv.Itoa(tproxyTable)}, {"route", "del", "local", "0.0.0.0/0", "dev", "lo", "table", strconv.Itoa(tproxyTable)}} {
			if err := runCommand("ip", args...); err != nil && !isMissing(err) {
				errs = append(errs, err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("transparent cleanup: %s", strings.Join(errs, "; "))
	}
	b.active, b.tool = ModeOff, ""
	return nil
}

func runInput(name, input string) error {
	cmd := exec.Command(name, "-f", "-")
	cmd.Stdin = strings.NewReader(input)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(output.String()))
	}
	return nil
}

func runCommands(commands [][]string) error {
	for _, command := range commands {
		if err := runCommand(command[0], command[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func runCommand(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func isMissing(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such file") ||
		strings.Contains(message, "not found") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "no such process") ||
		strings.Contains(message, "no chain/target/match") ||
		strings.Contains(message, "chain .* does not exist")
}
func commandAvailable(name string) bool { _, err := exec.LookPath(name); return err == nil }
func fileAvailable(name string) bool {
	file, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = file.Close()
	return true
}

func hasNetAdmin() bool { return effectiveCapability(12) }
func hasNetRaw() bool   { return effectiveCapability(13) }
func effectiveCapability(bit uint) bool {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "CapEff:") {
			continue
		}
		value, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(line, "CapEff:")), 16, 64)
		return err == nil && value&(uint64(1)<<bit) != 0
	}
	return false
}
