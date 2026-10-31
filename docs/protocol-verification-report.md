# P0-03 协议矩阵核实报告

## 结论

核实基于项目锁定版本：sing-box v1.14.4 与 mihomo v1.19.33，并结合本地 Windows amd64 二进制版本输出、官方版本源码和配置文档。

企划书的 28 项可以继续作为 DuoRelay 的“产品覆盖项”，但不能再把 28 项都称作同层级的原生 outbound 协议。核实后的能力层级如下：

- 14 个双核共同 outbound：Shadowsocks、VMess、VLESS、Trojan、Hysteria、Hysteria2、TUIC、AnyTLS、Snell、SOCKS5、HTTP/HTTPS、SSH、WireGuard、ShadowTLS。
- 2 个跨层能力：OpenVPN、Tailscale。sing-box 侧是 endpoint，mihomo 侧是 outbound。
- 3 个 sing-box 独有能力：Naive、Tor、OpenConnect；OpenConnect 是 endpoint。
- 9 个 mihomo 独有能力：MASQUE、ShadowsocksR、Mieru、Sudoku、ShadowQUIC、TrustTunnel、Rematch、EasyTier、ZeroTier。

14 + 2 + 3 + 9 = 28。产品覆盖数量保持不变，但实现语义得到修正。

## 重点核实

### Rematch

Rematch 不是独立远端协议。mihomo v1.19.33 的 `adapter/outbound/rematch.go` 要求 `target-rematch-name` 或 `target-sub-rule`，其 Dial/ListenPacket 路径只是把重匹配信息写入连接元数据。因此 DuoRelay 应把 Rematch 实现为 mihomo 路由控制适配器，不应为它设计服务器协议字段、分享链接或独立连通性验收。

### AnyTLS

两个内核均存在 AnyTLS outbound 配置入口，属于共同 outbound。它至少需要服务器、端口、密码和 TLS/拨号字段。配置类型存在不等于真实节点可用，连通性仍需后续靶场或真实节点验收。

### OpenVPN

sing-box v1.14.4 通过 endpoint 提供 OpenVPN client；mihomo v1.19.33 通过 outbound adapter 提供 OpenVPN。两者配置模型和生命周期不同，不能复用普通代理 outbound 的生成路径。

### MASQUE

mihomo v1.19.33 有 `adapter/outbound/masque.go`；sing-box v1.14.4 的 outbound 与 endpoint 清单没有 MASQUE，因此不能标为 sing-box 支持。

### Tailscale

sing-box v1.14.4 提供 Tailscale endpoint，mihomo v1.19.33 提供 Tailscale outbound。两者都依赖状态目录、控制面注册和路由/exit node 配置，属于外部账号或状态依赖能力。

## 实现影响

- outbound：生成对应内核的 outbound/proxy 配置。
- endpoint：生成 sing-box endpoint；mihomo 侧按 outbound 适配器处理。
- Rematch：生成 mihomo 路由/代理控制配置，不生成远端服务器连接配置。
- EasyTier/ZeroTier：作为内嵌组网能力处理，需要状态目录、身份和网络依赖检查。

## 证据

- `kernels.lock`
- `kernels/sing-box.exe version`
- `kernels/mihomo.exe -v`
- sing-box v1.14.4 `docs/configuration/outbound/index.md`、`docs/configuration/endpoint/openvpn-client.md`、`docs/configuration/endpoint/openconnect.md`、`docs/configuration/endpoint/tailscale.md`
- mihomo v1.19.33 `constant/adapters.go` 与 `adapter/outbound/*.go`
- 官方源码标签：SagerNet/sing-box v1.14.4、MetaCubeX/mihomo v1.19.33
