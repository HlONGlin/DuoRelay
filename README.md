# DuoRelay

**双内核全协议代理面板**

DuoRelay 是一个基于 sing-box 和 mihomo 双内核的 Web 管理面板。当前发布版本 `v0.3.0-beta.1`：双内核配置编排、节点/订阅管理、链式代理、负载均衡、基础路由和 Linux/Windows 部署能力已接入；28 项协议或网络能力具备统一 IR、解析器或配置生成覆盖，具体运行能力取决于内核和协议限制。

## 特性

- 🚀 **能力覆盖**: 支持 28 项代理协议、端点、组网能力和路由适配器（sing-box 20 项 + mihomo 24 项的并集）
- 🔄 **双核协同**: 智能切换内核，无感混用独有协议
- 🌐 **Web 管理**: Go 后端与 Vue 前端，浏览器完成节点、订阅、内核和配置管理
- 📊 **功能完善**: 测速、连接管理、流量统计、规则系统、故障诊断
- 🎯 **订阅兼容**: 支持 Base64、Clash/Mihomo、sing-box、v2rayN、Xray、SIP008、Surge、Loon、Mieru 等常见格式
- 🔒 **安全可靠**: 配置校验、自动回滚、崩溃自动拉起

## 支持的协议

### 两核共同支持 (16种)
Shadowsocks, VMess, VLESS, Trojan, Hysteria, Hysteria2, TUIC, AnyTLS, Snell, SOCKS5, HTTP, SSH, WireGuard, OpenVPN, MASQUE, Tailscale

### sing-box 独有 (4种)
ShadowTLS, Naive, Tor, OpenConnect

### mihomo 独有 (8种)
ShadowsocksR, Mieru, Sudoku, ShadowQUIC, TrustTunnel, Rematch, EasyTier, ZeroTier

## 快速开始

### 构建

```bash
# 克隆仓库
git clone https://github.com/HlONGlin/DuoRelay.git
cd duorelay

# 安装依赖
make web

# 构建
make build

# 运行（Linux/macOS）
./build/duorelay -kernels ./kernels

# Windows PowerShell
.\build\duorelay.exe -kernels .\kernels
```

### 开发

```bash
# 安装开发工具
make install-tools

# 前端开发模式
make web-dev

# 后端开发模式（另一个终端）
make dev
```

## 当前状态与边界

- 当前主流程已可运行；路由规则、流量历史、日志历史和面板重启恢复已接入，仍需要按目标 Linux 发行版做 TUN、DNS 和权限验收。
- 启动配置会由 `kernel.Manager` 调用实际的 `sing-box check` 或 `mihomo -t` 校验；生成器自身的 `Validate` 负责 JSON/YAML 结构校验。
- TUN、Windows 系统代理、DNS 接管和权限恢复仍需要按目标系统进一步联调，不能等同于完整系统代理接管。
- Mieru、Rematch、ShadowTLS、VPN/组网协议存在内核或远端配置限制；Parser/Generator 通过不代表所有公网节点都能连通。
- `/traffic` 是 Clash API 速率采样，并将滚动历史保存到 SQLite；它不是按用户或节点计费的累计流量账单。内核日志同时保留内存环和 JSONL 历史。

详细格式说明见 [订阅格式](docs/subscription-formats.md)，多节点链式代理见 [多节点代理说明](docs/multi-node-proxy.md)。

## 架构

```
┌─────────────────────────────────────┐
│  Web 前端（Vue 3 + TS）              │
└───────────┬──────────────┬──────────┘
            │ REST/HTTP 轮询
┌───────────▼──────────────▼──────────┐
│  面板后端（Go）                      │
│  ├─ 节点/订阅管理（SQLite）          │
│  ├─ 分享链接解析器 × 28              │
│  ├─ 统一节点模型（IR）               │
│  ├─ 配置生成器（双核）               │
│  └─ 内核进程管理                     │
└──────┬───────────────────┬──────────┘
       ▼                   ▼
 [sing-box 进程]      [mihomo 进程]
```

## 许可证

待定（P4-06 决策）

内核：
- sing-box: GPLv3
- mihomo: GPLv3

## 声明

本项目仅供学习研究使用，使用者需遵守所在地法律法规。

---

🤖 项目由 [Claude Code](https://claude.com/claude-code) 辅助开发

## Linux 部署补充

Linux 是 DuoRelay 的主要运行目标。服务器、容器和无桌面系统优先使用 TUN；GNOME 桌面可在当前用户 D-Bus 会话中启用系统代理。发布包、systemd 单元、安装卸载和验收命令见 [Linux 部署](docs/linux-deployment.md)。
