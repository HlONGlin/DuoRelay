# lite 实现与验收报告

简体中文 | [English](lite-acceptance.en.md)

日期：2026-11-03。分支：`feat/lite-variant`。基线 master：`0570cd74237030249f0b2bf15ecbbe3a89e2a789`。本次是同版本 `0.3.0` 的本地验收构建，不另行发版，不修改既有 beta.1 版本表述。包在提交前构建，build-info 中的 commit 是基线 HEAD；完整实现以此分支的单个 lite 提交为准。

## 实现

- 新增 `lite` / `!lite` 构建分界：内核能力注册、进程差异、生成器注册与链式实现分开。full 保持双核生成器与跨核桥接原实现；lite 不编译 sing-box 生成器或跨核 SOCKS5 桥接。
- 两种构建嵌入同一份生产面板。面板先读取后端变体，再禁用 lite 不支持协议的启动、组合勾选与选择器；列表保留这些节点，说明“精简版不支持”。新增关于分类显示 full/lite、版本与提交。
- lite 单节点、负载均衡、原生链式复用共享应用事务，保留真核校验、透明代理准备、系统代理恢复及失败回滚。full 的链式渲染仅移入 `chain_full.go`。
- lite 首次启动从 MetaCubeX 官方发布渠道缓存 Geo，失败明确退出；full 不强制预下载。没有变更 IR、解析器、节点/订阅存储、数据库密钥、订阅加密或透明代理规则逻辑。
- Linux 打包添加 `--lite`、`--kernels-dir`，Windows 添加 `-Lite`，新增 Make 目标；既有脚本保留。Linux 与 Windows 的 lite 暂存目录分开。

## 静态与构建

| 检查 | Windows 工作区 | Linux 隔离容器 |
|---|---|---|
| `go test ./...` | PASS | PASS |
| `go vet ./...` | PASS | PASS |
| `go test -tags lite ./...` | PASS | PASS |
| `go vet -tags lite ./...` | PASS | PASS |
| `npm ci`、`npm run type-check` | PASS | PASS |
| `npm test` | PASS，12 文件 / 53 测试 | PASS，12 文件 / 53 测试 |
| `npm run build` | PASS | PASS（`make build-lite` 生产前端步骤） |
| `make build-lite VERSION=0.3.0` | 未运行 Make；Windows 打包脚本已运行 | PASS |
| Linux amd64 full/lite 包 | PASS，交叉构建与包检查 | amd64 面板、内核实际运行 PASS |
| Linux arm64 full/lite 包 | PASS，交叉构建；ELF 架构与内核组成检查 PASS | arm64 实际运行 UNVERIFIED |
| Windows amd64 full/lite 包 | PASS，PE 架构与内核组成检查 PASS | 不适用 |
| lite 依赖图 | `go list -tags lite -deps ./cmd/duorelay` 不含 sing-box 生成器 | 相同源码目标 |
| lite 内核/生成器注册表 | PASS，仅 mihomo | PASS，仅 mihomo |
| 不支持协议不可选且有说明 | PASS，真实 Vue 组件/组合选择器测试，24/28 能力断言 | 相同 53 项前端测试 PASS |
| 原始配置自由应用边界 | full 原有测试 PASS，lite 新增测试 PASS | HTTP API 返回 400，PASS |

工具：Windows Go 1.27.0；容器 Go 1.27.3、Node 22、Debian 12；内核按原有锁定版本 sing-box 1.14.4、mihomo 1.19.33，arm64 从相同版本官方发布下载。未执行 Windows race 工具链安装；`go test -race` 不计为通过。full 专属断言在 lite 中明确跳过，full 中继续执行；sing-box 包及直接依赖它的生成器测试按 build tag 排除。共享订阅兼容测试继续在两种目标执行，只有跨两种生成器的测试移入 full 文件。

## 容器运行（独立于真实系统）

先读取本机 `C:/Users/honglin/.local/bin/orca-linux.ps1` 和其 Dockerfile。该本地 wrapper 使用 `orca-linux:local`、工作区挂载与共享 `/root` 卷，并没有传入设备或 capability 的接口。本次采用其同一基础镜像，添加 Go、iproute2、nftables/iptables，使用独立的一次性容器；不复用个人 home 卷，不注入宿主机代理，不挂载宿主机真实数据库。

实际启动参数包括 `--device /dev/net/tun:/dev/net/tun --cap-add NET_ADMIN --network bridge`。`docker inspect` 确认设备映射、`CAP_NET_ADMIN`、bridge；`test -c /dev/net/tun` 成功。源快照经只读 artifact 挂载复制到容器 `/tmp/duorelay-src`，面板在容器内构建。没有 host networking，没有发布面板到宿主机端口，没有修改宿主机代理或防火墙。fixture 的 dummy 接口、地址、规则与数据仅存在于此容器网络/文件空间。

| 用例 | 结果与证据 |
|---|---|
| lite 面板与首次设置 | PASS，内嵌 HTML HTTP 200；独立新库创建临时管理员、登录，默认内核 mihomo |
| 首次 Geo 下载与缓存 | PASS，空缓存自动下载两份官方文件；再次启动 mtime 不变 |
| Geo 下载失败 | PASS，单测覆盖 HTTP 403/404、空响应，明确报错且不留下成功缓存或临时文件 |
| mihomo 启动 | PASS，面板 API 返回 running / mihomo；运行目录仅放 mihomo |
| 至少一条真实代理请求 | PASS，curl → 面板管理的 mihomo → 临时 HTTP 上游 → 临时源站，响应与上游计数均核对 |
| 路由分流 | PASS，同一运行配置下分别验证 PROXY、DIRECT、REJECT；DIRECT 不增加上游计数，REJECT 请求失败 |
| 负载均衡 | PASS，面板应用两节点配置并实际请求 |
| 原生链式 | PASS，面板应用两跳，实际请求观察到两次临时上游代理转发，无跨核辅助实例 |
| 非法内核替换 | PASS，400，current_config ID 不变且原代理仍可请求 |
| 真核校验失败 | PASS，非法 Shadowsocks cipher 被 mihomo 实际校验拒绝，422 / KERNEL_CHECK_FAILED；旧配置 ID 与代理请求保持有效 |
| 启动失败回滚 | PASS，lite 单节点/负载均衡/链式单测注入启动失败，恢复原路径与持久化状态；不冒充远端故障实测 |
| full → lite → full | PASS，6 个节点（含 4 个不支持协议）、1 个加密订阅、3 条路由规则 API 内容一致；数据库节点/订阅原始记录与密钥哈希不变；full 原 sing-box 偏好保留 |
| lite 新建 → full | PASS，lite 新建 2 节点、1 订阅及 3 规则，full 再次打开逐项比较一致 |
| redirect / single、chain | PASS，面板启动，nft 自有规则存在；停止后规则/路由快照完全恢复 |
| tproxy / single、chain | PASS，面板启动，nft TPROXY 与策略路由生命周期、停止后快照恢复 |
| tun / single、chain | PASS，面板应用与内核启动，停止后规则/路由快照恢复 |
| full TUN 创建/清理 | PASS，既有 Linux TUN 集成测试对 sing-box、mihomo 均通过，含 IPv4/IPv6 地址及路由清理 |
| lite TUN 创建/清理 | PASS，同一集成测试仅 mihomo，地址与路由清理通过 |
| 实际内核崩溃恢复 | PASS，full 两核、lite 单核的既有真实内核 kill/restart/健康探测及非法配置拒绝测试 |
| lite 协议链式配置矩阵 | PASS，既有 fixture 中 mihomo 支持协议分别处于入口/出口，经真核配置校验；Rematch 作为非网络适配器按原测试排除 |
| full 实际配置应用 | PASS，启用 `DUORELAY_BACKEND_KERNEL_INTEGRATION=1` 的原用例 |

单节点配置沿用现有 PROXY 手动选择组，其初始项为 DIRECT。fixture 通过临时实例的 Clash 控制 API 显式选择 `entry`，再观察真实代理分支；没有编辑原始配置，没有改变既有默认行为。临时地址使用 dummy 接口的 `10.203.0.2/3/4`，避免把内核对 loopback 的固有直连当作代理通过。

Geo 缓存实测如下，发布包中为 0 字节：

| 文件 | 下载后字节 | SHA256 |
|---|---:|---|
| GeoSite.dat | 4,356,022 | `3fee9b84efce1ef142974a18b48b9b12d2cf8ccb8017dd4ec16e3906bcb7da69` |
| geoip.metadb | 8,424,898 | `f74ad728d3d3e1a7ca51886a147a18d82406733f3bc0db8cf9117f66de437239` |
| 合计 | 12,780,920 | 官方 latest 数据会更新，本表仅为本次下载 |

## 额外验收发现的既有失败

额外运行 `DUORELAY_CHAIN_INTEGRATION=1 go test -v ./internal/configuration -run '^TestReal' -count=1`：full 的 `TestRealChainTraffic/sing-box` 与 `mihomo-to-sing-box` 返回混合端口 connection refused，记 **FAIL（基线同样失败）**。同容器解包原 master `0570cd7`、使用相同二进制与库运行同一测试，也复现这两个失败；其余 mihomo、sing-box-to-mihomo、8 跳交替和 scoped 双向 HTTP 用例通过。full 原生/双向跨核四种 UDP 组合用例全部 PASS。该 opt-in 组不能写成全通过，未在 lite 任务内改动 full 的既有链式行为。

容器首次 full 默认测试曾因缺少 Cronet 库失败；补入仓库原有 `libcronet.so` 并设置容器内 `LD_LIBRARY_PATH` 后，full/lite 默认测试及 vet 完整重跑通过。这是验收环境修复，未把 Cronet 加入 lite 包。前两次代理 fixture 没有实际经过上游（loopback 固有绕过、选择组默认 DIRECT），均未计通过；最终改用独立地址与显式选组后核对上游计数通过。

## 发布包实测

| 平台 | 变体 | 压缩包 | 面板（含前端） | mihomo | sing-box | Cronet | Geo | 其他文件 |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| linux/amd64 | full | 62,342,500 | 21,745,491 | 62,017,662 | 81,416,421 | 0 | 0 | 14,042 |
| linux/amd64 | lite | 34,663,941 | 21,516,546 | 62,017,662 | 0 | 0 | 0 | 14,042 |
| linux/arm64 | full | 56,908,686 | 20,672,027 | 57,475,198 | 75,956,453 | 0 | 0 | 14,042 |
| linux/arm64 | lite | 31,832,481 | 20,489,042 | 57,475,198 | 0 | 0 | 0 | 14,042 |
| windows/amd64 | full | 66,558,506 | 21,812,224 | 61,247,488 | 82,074,624 | 9,529,344 | 0 | 9,126 |
| windows/amd64 | lite | 34,336,092 | 21,584,384 | 61,247,488 | 0 | 0 | 0 | 9,126 |


所有数值为字节；面板已嵌入同一份生产前端，未额外计算外置 web 目录。包大小由归档文件读取，组成由 ZIP/TAR 元数据读取；同时检查面板和内核的 ELF/PE 架构、build-info 的平台/版本/变体。lite 无 sing-box 二进制、无 libcronet.dll、无 Geo 文件；full 保留原有内核组成（Linux 两核；Windows 两核加 Cronet）。Linux full 原脚本不打包 libcronet.so，本次不改变该行为，因此 full 包中 Naive 的额外动态库部署不作为已验证项。

| v2rayA 镜像（本次 registry 读取） | 压缩层总字节 | 十进制 MB |
|---|---:|---:|
| `mzz2017/v2raya@sha256:255c166e59b3b889d00b35db37c7e4b12a7a9570e188cbbb8bfa1b9ad6b203d8`（linux/amd64） | 42,061,749 | 42.061749 |
| `mzz2017/v2raya@sha256:ae77951d6f3ef496f1b637872ae6ea581e19fb693bd17f032a7ec9f8b0fe17ca`（linux/arm64） | 40,297,837 | 40.297837 |


上表 v2rayA 数值为本次从 Docker Registry manifest 读取的 gzip 层 size 之和，不是解压后镜像大小，也不计协议/manifest 传输开销。对照仅限体积量级：DuoRelay 是压缩发布包，v2rayA 是压缩镜像层，基础系统及 Geo 组成不同。本次没有复制 v2rayA 代码，未为了达到某数值裁剪面板功能。

## UNVERIFIED 与范围限制

| 项目 | 状态 | 原因 |
|---|---|---|
| Windows 真机系统代理、TUN、服务覆盖安装 | UNVERIFIED | 本机仅执行 Go/前端测试、打包与文件检查，未触碰当前用户真实代理或防火墙 |
| Linux 真实主机 systemd/root/桌面环境 | UNVERIFIED | 仅在隔离容器运行；实际 install-linux.sh 服务覆盖安装未执行 |
| macOS 构建与真实 TUN/系统行为 | UNVERIFIED | 本次无 macOS 环境 |
| Linux arm64 真实运行 | UNVERIFIED | 仅交叉构建与归档 ELF 架构检查 |
| redirect / TPROXY / TUN 的真实透明转发端到端流量 | UNVERIFIED | 本次验证规则/接口/路由生命周期；实际请求使用显式代理，不能替代透明流量 |
| DNS 接管/泄漏检测 | UNVERIFIED | 没有独立 DNS 观察器，不声明防泄漏通过 |
| 24 项协议的所有远端真实连通 | UNVERIFIED | 真核配置矩阵不等于各协议外部服务验收；真实流量仅临时 HTTP 上游 |
| 实际公网代理出口 | UNVERIFIED | 使用本地临时上游与源站，未使用用户的真实节点 |
| Geo 网络断流、超时的实际公网注入 | UNVERIFIED | HTTP/空文件失败在单测验证；没有对宿主机网络注入故障 |

## 文件清单与证据

本次修改/新增 59 个文件：

- `Makefile`
- `README.md`
- `cmd/duorelay/main.go`
- `docs/README.md`
- `docs/lite-acceptance.md`
- `docs/lite-package-sizes.json`
- `docs/lite.md`
- `internal/api/server.go`
- `internal/api/server_test.go`
- `internal/api/variant_lite_test.go`
- `internal/configuration/chain.go`
- `internal/configuration/chain_full.go`
- `internal/configuration/chain_integration_test.go`
- `internal/configuration/chain_lite.go`
- `internal/configuration/chain_matrix_test.go`
- `internal/configuration/chain_test.go`
- `internal/configuration/chain_udp_test.go`
- `internal/configuration/composition_test.go`
- `internal/configuration/generators_full.go`
- `internal/configuration/generators_lite.go`
- `internal/configuration/routing_test.go`
- `internal/configuration/service.go`
- `internal/configuration/service_test.go`
- `internal/configuration/tun_linux_integration_test.go`
- `internal/configuration/variant_lite_test.go`
- `internal/generator/composition_test.go`
- `internal/generator/routing_test.go`
- `internal/generator/singbox/generator.go`
- `internal/generator/singbox/generator_test.go`
- `internal/generator/singbox/openvpn.go`
- `internal/generator/singbox/openvpn_test.go`
- `internal/generator/singbox/tailscale_test.go`
- `internal/generator/validation_test.go`
- `internal/kernel/geodata_lite.go`
- `internal/kernel/manager.go`
- `internal/kernel/manager_test.go`
- `internal/kernel/persistent_log.go`
- `internal/kernel/variant_full.go`
- `internal/kernel/variant_lite.go`
- `internal/kernel/variant_lite_test.go`
- `internal/subscription/compat_full_test.go`
- `internal/subscription/compat_test.go`
- `internal/variant/full.go`
- `internal/variant/lite.go`
- `internal/variant/variant.go`
- `scripts/acceptance-lite.py`
- `scripts/measure-variants.py`
- `scripts/package-linux.sh`
- `scripts/package-windows.ps1`
- `web/src/components/NodeCard.spec.ts`
- `web/src/components/NodeCard.vue`
- `web/src/components/ProxyGroupDialog.spec.ts`
- `web/src/components/ProxyGroupDialog.vue`
- `web/src/main.ts`
- `web/src/stores/variant.spec.ts`
- `web/src/stores/variant.ts`
- `web/src/views/NodeList.vue`
- `web/src/views/Settings.spec.ts`
- `web/src/views/Settings.vue`

本地原始日志与归档位于 `build/lite-work/`、`dist/`（按仓库原有规则忽略，不提交二进制/数据库/密钥）。关键证据：`full-go-test.log`、`lite-go-test.log`、`web-test.log`、`container-final.log`、`container-go-test-{full,lite}.log`、`container-lite-protocol-matrix.log`、`container-tun-{full,lite}.log`、`container-kernel-{full,lite}.log`、`container-chain-full.log`、`container-chain-baseline.log`、`package-measurements.json`。完整逐文件包组成见本报告相邻的 `lite-package-sizes.json`；可用 `scripts/measure-variants.py` 重新采集。验收容器收尾后删除；镜像保留在本地供复查，不作发布镜像。
