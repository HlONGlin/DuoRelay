# P0-03 协议矩阵核实报告

## 结论

核实基于项目锁定版本：sing-box v1.14.4 与 mihomo v1.19.33，并结合本地 Windows amd64 二进制版本输出、官方版本源码和配置文档。

企划书的 28 项可以继续作为 DuoRelay 的“产品覆盖项”，但不能再把 28 项都称作同层级的原生 outbound 协议。核实后的能力层级如下：

- 13 个双核共同 outbound：Shadowsocks、VMess、VLESS、Trojan、Hysteria、Hysteria2、TUIC、AnyTLS、Snell、SOCKS5、HTTP/HTTPS、SSH、WireGuard。
- 2 个跨层能力：OpenVPN、Tailscale。sing-box 侧是 endpoint，mihomo 侧是 outbound。
- 4 个 sing-box 独有能力：ShadowTLS、Naive、Tor、OpenConnect；OpenConnect 是 endpoint。此处 ShadowTLS 独有指独立 outbound，mihomo 仍支持其他 outbound 内嵌的 ShadowTLS 选项。
- 9 个 mihomo 独有能力：MASQUE、ShadowsocksR、Mieru、Sudoku、ShadowQUIC、TrustTunnel、Rematch、EasyTier、ZeroTier。

13 + 2 + 4 + 9 = 28。产品覆盖数量保持不变，但实现语义得到修正。矩阵的 outbound 列表示可独立配置的类型，不把其他协议的插件/混淆选项计作独立 outbound。

## 重点核实

### ShadowTLS

结论：sing-box v1.14.4 提供独立 `type: shadowtls` outbound；mihomo v1.19.33 仅在 Shadowsocks 和 Snell outbound 内嵌 ShadowTLS，不能把单独的 ShadowTLS 节点归属为双核共有。以下证据全部固定到 `kernels.lock` 的版本标签，不使用滚动文档推断锁定版本：

- sing-box 的 [outbound 配置文档](https://github.com/SagerNet/sing-box/blob/v1.14.4/docs/configuration/outbound/shadowtls.md#L1-L52) 直接给出 `type: shadowtls`、服务器、端口、版本、密码和 TLS 字段；`server`、`server_port`、`tls` 标为必需，`password` 用于 v2/v3，版本默认 v1。其 [注册和构造代码](https://github.com/SagerNet/sing-box/blob/v1.14.4/protocol/shadowtls/outbound.go#L21-L35) 将 `option.ShadowTLSOutboundOptions` 注册为 `C.TypeShadowTLS`，构造独立 outbound，并拒绝缺失/未启用 TLS 的配置。
- mihomo 的 [代理类型分派](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/parser.go#L29-L228) 没有 `shadowtls` 或 `shadow-tls` 独立类型分支；未知类型落入 `unsupport proxy type` 错误。存在 `transport/shadowtls` 实现不等于存在独立 outbound。
- mihomo 的 [Shadowsocks 选项](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/shadowsocks.go#L37-L53) 包含 `plugin`/`plugin-opts`；[插件分支](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/shadowsocks.go#L396-L421) 将 `plugin: shadow-tls` 解码为 ShadowTLS 选项，[连接代码](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/shadowsocks.go#L163-L208) 先包装 ShadowTLS，再建立 Shadowsocks 加密连接，仍然是 Shadowsocks outbound。
- mihomo 的 [Snell 连接路径](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/snell.go#L29-L69) 先按混淆模式包装连接，再调用 `snell.StreamConn`；[选项分支](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/adapter/outbound/snell.go#L169-L207) 从 `obfs-opts` 解码 `mode: shadow-tls`，仍然是 Snell outbound。
- 上述两个 mihomo 适配器使用同一 [模式常量](https://github.com/MetaCubeX/mihomo/blob/v1.19.33/transport/shadowtls/client.go#L20)：`shadowtls.Mode` 的值为 `shadow-tls`，不是新增的代理类型。

因此 `protocol-matrix.csv` 的 ShadowTLS 行应为 `sing_box_outbound=yes`、`mihomo_outbound=no`、`classification=sing-box outbound only`。现有 `internal/ir/protocol.go` 的 `ProtocolShadowTLS: KernelSetSingBox` 已正确，无需修改生产代码；所有权回归测试应同时确认 sing-box 可运行、mihomo 不可运行。此结论只核实独立配置能力，不代表已验证真实 ShadowTLS 服务端连通性。

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
- ShadowTLS：独立节点仅归属 sing-box；mihomo 的 `ss` 插件或 Snell 混淆选项属于各自父 outbound，不赋予独立 ShadowTLS 节点 mihomo 所有权。
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

## ShadowTLS 回归验证（2026-10-31）

- 本地版本输出确认 `sing-box version 1.14.4`（revision `c005f35300ce76427dc285481ec5695f5b179cd2`）和 `Mihomo Meta v1.19.33`，与 `kernels.lock` 一致。
- `go test ./internal/ir -count=1` 通过；新增 `TestShadowTLSStandaloneKernelEligibility` 从实际 `GetProtocolOwner` 构造节点，同时验证 sing-box 可运行、mihomo 不可运行，原有 `TestProtocolOwnership/shadowtls` 继续验证精确所有权。
- `git diff --check -- docs/protocol-matrix.csv docs/protocol-verification-report.md internal/ir/protocol_test.go` 通过；本次仅修改这两个文档及 IR 所有权测试，保留现有解析器/生成器工作。
