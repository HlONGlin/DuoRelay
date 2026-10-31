# DuoRelay

**双内核全协议代理面板**

DuoRelay 是一个基于 sing-box 和 mihomo 双内核的 Web 管理面板，实现 28 种代理协议的全覆盖。

## 特性

- 🚀 **协议全覆盖**: 支持 28 种代理协议（sing-box 20种 + mihomo 24种的并集）
- 🔄 **双核协同**: 智能切换内核，无感混用独有协议
- 🌐 **Web 管理**: 单二进制部署，浏览器完成全部管理
- 📊 **功能完善**: 测速、连接管理、流量统计、规则系统、故障诊断
- 🎯 **订阅兼容**: 支持 Base64、Clash YAML、sing-box JSON 等主流订阅格式
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
git clone https://github.com/holi/duorelay.git
cd duorelay

# 安装依赖
make web

# 构建
make build

# 运行
./build/duorelay
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

## 开发状态

当前处于 **P0 技术验证阶段**，正在搭建工程骨架。

预计完整开发周期：20 周（100 人日）

- [x] P0-01: 工程骨架（进行中）
- [ ] P0-02: 内核获取与版本锁定
- [ ] P0-03: 协议与对标核实
- [ ] ...更多任务详见 [企划书](企划书_v2.0.md)

## 架构

```
┌─────────────────────────────────────┐
│  Web 前端（Vue 3 + TS）              │
└───────────┬──────────────┬──────────┘
            │ REST         │ WebSocket
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
