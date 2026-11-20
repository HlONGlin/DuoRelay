# 透明代理验收记录

简体中文 | [English](transparent-proxy-acceptance.en.md)

Date: 2026-11-03

2026-11-04 更新：本轮通过 orca-linux 普通 Linux 容器完成权限失败保护、重名节点、双内核实际启动、多 SOCKS5 端口和指定 DNS 请求验收，并交叉编译 Windows/macOS full/lite。本轮容器未提供 TUN 和 NET_ADMIN，未重新执行下文旧记录中的特权设置/清理测试；真实透明流量及 DNS 泄漏仍未验证。详见[本轮问题记录](issue-resolution-2026-11.md)。

面板提供 `off`、`redirect`、`tproxy` 和 `tun`。旧的 `enable_tun` 字段仍兼容，启用时会规范化为 `tun`。设置页显示当前环境的支持情况、前置条件、DNS 行为和当前生效状态。能力探测通过或模式已生效，不代表真实流量或 DNS 防泄漏验收通过。Windows TUN、Windows 系统代理恢复和 macOS TUN 尚未完成真实系统验收。

## 模式约束

| 系统 | redirect | tproxy | tun | DNS 行为 |
|---|---|---|---|---|
| Linux | 需要 root 或相应网络权限，以及 `iproute2`、nftables 或 iptables。只接管 TCP。 | 需要 root 或相应网络权限，以及 TPROXY、fwmark 和策略路由支持。 | 需要可读写的 `/dev/net/tun` 及 `CAP_NET_ADMIN`/`CAP_NET_RAW`。 | redirect 不接管 UDP DNS，系统 DNS 查询可能绕过代理。TPROXY 也需要显式配置代理 DNS。TUN 依赖内核的 DNS 接管配置，实际效果需验证。 |
| Windows | 不适用。 | 不适用。 | 需要管理员权限、Wintun 驱动和防火墙确认；真实系统未验证。 | 真实系统未验证。 |
| macOS | 不适用。 | 不适用。 | 需要 root 权限创建 utun 和管理路由；真实系统未验证。 | 真实系统未验证。 |

DuoRelay 只管理自有 nft table 或 `DUORELAY_*` iptables chain、TPROXY fwmark 规则和策略路由。停止及失败清理是幂等的：不存在的 chain 和 route 视为已清理。nft 规则排除 loopback，避免面板健康检查被自身 TPROXY 路径拦截。

## 验收矩阵

| 目标 | redirect | tproxy | tun | DNS 泄漏检查 | 清理 |
|---|---|---|---|---|---|
| Orca Debian 12 容器 | 设置/清理通过；真实透明请求因本地 bridge fixture 回环而未验证 | 设置/清理通过；真实透明请求受 fixture 限制未验证 | sing-box、mihomo 启停和 route/TUN 恢复通过 | 未验证：无独立观察器 | nft、策略路由、接口和 TUN 状态检查通过 |
| Linux Debian/Ubuntu 真实主机 | 未验证 | 未验证 | 未验证 | 未验证 | 未验证 |
| Linux Fedora/RHEL 真实主机 | 未验证 | 未验证 | 未验证 | 未验证 | 未验证 |
| Linux Arch 真实主机 | 未验证 | 未验证 | 未验证 | 未验证 | 未验证 |
| Windows 11 真实目标机 | 不适用 | 不适用 | 未验证：需驱动、管理员权限和防火墙确认 | 未验证 | 未验证 |
| macOS 真实目标机 | 不适用 | 不适用 | 未验证：需真实 TUN 测试 | 未验证 | 未验证 |

容器中运行 `scripts/acceptance-linux.sh` 时未提供测试 URL，脚本正确将三种模式报告为 `UNVERIFIED`；能力探测没有被误记为真实流量通过。
